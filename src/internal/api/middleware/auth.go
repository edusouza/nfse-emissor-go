// Package middleware provides HTTP middleware for the NFS-e API.
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/eduardo/nfse-nacional/internal/api/handlers"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
)

// Context keys for storing request-scoped values.
const (
	// APIKeyContextKey is the context key for the authenticated API key.
	APIKeyContextKey = "api_key"

	// APIKeyHeaderName is the HTTP header name for the API key.
	APIKeyHeaderName = "X-API-Key"
)

// APIKeyRepository defines the interface for API key lookups.
type APIKeyRepository interface {
	FindByKeyHash(ctx context.Context, keyHash string) (*mongodb.APIKey, error)
}

// AuthMiddleware provides API key authentication.
type AuthMiddleware struct {
	repo APIKeyRepository
}

// NewAuthMiddleware creates a new authentication middleware.
func NewAuthMiddleware(repo APIKeyRepository) *AuthMiddleware {
	return &AuthMiddleware{
		repo: repo,
	}
}

// Authenticate returns a Gin middleware handler that authenticates requests.
// It extracts the API key from the X-API-Key header, hashes it with SHA-256,
// looks up the key in the database, and verifies it is active.
func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract API key from header
		apiKey := c.GetHeader(APIKeyHeaderName)
		if apiKey == "" {
			handlers.Unauthorized(c, "Missing API key. Include the X-API-Key header in your request.")
			c.Abort()
			return
		}

		// Clean the API key (trim whitespace)
		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			handlers.Unauthorized(c, "Invalid API key format. The X-API-Key header cannot be empty.")
			c.Abort()
			return
		}

		// Hash the API key
		keyHash := hashAPIKey(apiKey)

		// Look up the key in the database
		storedKey, err := m.repo.FindByKeyHash(c.Request.Context(), keyHash)
		if err != nil {
			if errors.Is(err, mongodb.ErrAPIKeyNotFound) {
				handlers.Unauthorized(c, "Invalid API key. The provided key was not found.")
				c.Abort()
				return
			}
			// Log the error but don't expose internal details
			handlers.InternalError(c, "An error occurred while validating the API key.")
			c.Abort()
			return
		}

		// Check if the key is active
		if !storedKey.Active {
			handlers.Unauthorized(c, "API key is inactive. Please contact support to reactivate.")
			c.Abort()
			return
		}

		// Store the API key in the context for later use
		c.Set(APIKeyContextKey, storedKey)

		c.Next()
	}
}

// GetAPIKeyFromContext retrieves the authenticated API key from the Gin context.
// Returns nil if no API key is present (request not authenticated).
func GetAPIKeyFromContext(c *gin.Context) *mongodb.APIKey {
	value, exists := c.Get(APIKeyContextKey)
	if !exists {
		return nil
	}

	apiKey, ok := value.(*mongodb.APIKey)
	if !ok {
		return nil
	}

	return apiKey
}

// MustGetAPIKeyFromContext retrieves the authenticated API key from the context.
// Panics if the API key is not present. Use this only after authentication middleware.
func MustGetAPIKeyFromContext(c *gin.Context) *mongodb.APIKey {
	apiKey := GetAPIKeyFromContext(c)
	if apiKey == nil {
		panic("api key not found in context - authentication middleware may not have run")
	}
	return apiKey
}

// hashAPIKey computes the SHA-256 hash of an API key.
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// HashAPIKey is the exported version for use when creating API keys.
func HashAPIKey(key string) string {
	return hashAPIKey(key)
}

// GetAPIKeyPrefix returns the first 8 characters of an API key for identification.
func GetAPIKeyPrefix(key string) string {
	if len(key) < 8 {
		return key
	}
	return key[:8]
}
