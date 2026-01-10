// Package middleware provides HTTP middleware for the NFS-e API.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Request ID header names.
const (
	// HeaderXRequestID is the standard request ID header.
	HeaderXRequestID = "X-Request-ID"

	// RequestIDContextKey is the context key for the request ID.
	RequestIDContextKey = "request_id"
)

// RequestID returns a Gin middleware that generates or propagates request IDs.
// It first checks for an existing X-Request-ID header to support distributed tracing.
// If no header is present, it generates a new UUID v4.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for existing request ID header (for distributed tracing)
		requestID := c.GetHeader(HeaderXRequestID)

		// Generate new request ID if not provided
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Store in context
		c.Set(RequestIDContextKey, requestID)

		// Add to response headers
		c.Header(HeaderXRequestID, requestID)

		c.Next()
	}
}

// generateRequestID creates a new UUID v4 request ID.
func generateRequestID() string {
	return uuid.New().String()
}

// GetRequestIDFromContext retrieves the request ID from the Gin context.
// Returns an empty string if no request ID is present.
func GetRequestIDFromContext(c *gin.Context) string {
	value, exists := c.Get(RequestIDContextKey)
	if !exists {
		return ""
	}

	requestID, ok := value.(string)
	if !ok {
		return ""
	}

	return requestID
}

// MustGetRequestIDFromContext retrieves the request ID from the context.
// Panics if the request ID is not present. Use only after RequestID middleware.
func MustGetRequestIDFromContext(c *gin.Context) string {
	requestID := GetRequestIDFromContext(c)
	if requestID == "" {
		panic("request ID not found in context - RequestID middleware may not have run")
	}
	return requestID
}

// WithRequestID adds a request ID to an outgoing request context.
// This is useful when making outbound HTTP calls to propagate the request ID.
func WithRequestID(c *gin.Context, headers map[string]string) map[string]string {
	if headers == nil {
		headers = make(map[string]string)
	}

	requestID := GetRequestIDFromContext(c)
	if requestID != "" {
		headers[HeaderXRequestID] = requestID
	}

	return headers
}
