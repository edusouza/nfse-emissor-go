// Package middleware provides HTTP middleware for the NFS-e API.
package middleware

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	goredis "github.com/redis/go-redis/v9"

	"github.com/eduardo/nfse-nacional/internal/api/handlers"
)

// Rate limit header names.
const (
	HeaderRateLimitLimit     = "X-RateLimit-Limit"
	HeaderRateLimitRemaining = "X-RateLimit-Remaining"
	HeaderRateLimitReset     = "X-RateLimit-Reset"
	HeaderRetryAfter         = "Retry-After"
)

// RateLimitMiddleware provides rate limiting using the GCRA (Generic Cell Rate Algorithm).
type RateLimitMiddleware struct {
	limiter        *redis_rate.Limiter
	defaultRPM     int
	defaultBurst   int
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	// RequestsPerMinute is the rate limit for requests per minute.
	RequestsPerMinute int
	// Burst is the maximum burst size.
	Burst int
}

// NewRateLimitMiddleware creates a new rate limiting middleware.
func NewRateLimitMiddleware(redisClient *goredis.Client, defaultRPM, defaultBurst int) *RateLimitMiddleware {
	limiter := redis_rate.NewLimiter(redisClient)

	return &RateLimitMiddleware{
		limiter:      limiter,
		defaultRPM:   defaultRPM,
		defaultBurst: defaultBurst,
	}
}

// RateLimit returns a Gin middleware handler that enforces rate limits.
// It uses the GCRA algorithm implemented by go-redis/redis_rate.
// Rate limit configuration is obtained from the authenticated API key.
func (m *RateLimitMiddleware) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get rate limit config from API key in context
		// Note: burst is handled internally by redis_rate GCRA algorithm
		rpm, _ := m.getRateLimitConfig(c)

		// Build the rate limit key
		key := m.buildKey(c)

		// Apply rate limiting using GCRA
		result, err := m.limiter.Allow(c.Request.Context(), key, redis_rate.PerMinute(rpm))
		if err != nil {
			// On rate limiter error, log and allow the request (fail open for availability)
			// In production, you might want to fail closed for security-sensitive endpoints
			c.Next()
			return
		}

		// Set rate limit headers
		m.setRateLimitHeaders(c, result, rpm)

		// Check if request is allowed
		if result.Allowed == 0 {
			retryAfter := int(result.RetryAfter.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}

			handlers.TooManyRequests(c,
				fmt.Sprintf("Rate limit exceeded. Please retry after %d seconds.", retryAfter),
				retryAfter,
			)
			c.Abort()
			return
		}

		c.Next()
	}
}

// getRateLimitConfig retrieves rate limit configuration from the API key context.
// Falls back to default values if no API key is present or config is invalid.
func (m *RateLimitMiddleware) getRateLimitConfig(c *gin.Context) (rpm int, burst int) {
	apiKey := GetAPIKeyFromContext(c)
	if apiKey == nil {
		return m.defaultRPM, m.defaultBurst
	}

	rpm = apiKey.RateLimit.RequestsPerMinute
	burst = apiKey.RateLimit.Burst

	// Apply defaults if values are invalid
	if rpm <= 0 {
		rpm = m.defaultRPM
	}
	if burst <= 0 {
		burst = m.defaultBurst
	}

	return rpm, burst
}

// buildKey constructs the rate limit key for the current request.
// Keys are scoped by API key prefix to ensure per-client rate limiting.
func (m *RateLimitMiddleware) buildKey(c *gin.Context) string {
	// Try to use API key prefix for the key
	apiKey := GetAPIKeyFromContext(c)
	if apiKey != nil && apiKey.KeyPrefix != "" {
		return fmt.Sprintf("ratelimit:%s", apiKey.KeyPrefix)
	}

	// Fall back to client IP if no API key
	clientIP := c.ClientIP()
	return fmt.Sprintf("ratelimit:ip:%s", clientIP)
}

// setRateLimitHeaders adds rate limit information to response headers.
func (m *RateLimitMiddleware) setRateLimitHeaders(c *gin.Context, result *redis_rate.Result, limit int) {
	c.Header(HeaderRateLimitLimit, strconv.Itoa(limit))
	c.Header(HeaderRateLimitRemaining, strconv.Itoa(result.Remaining))
	c.Header(HeaderRateLimitReset, strconv.FormatInt(result.ResetAfter.Milliseconds(), 10))
}

// IPRateLimit returns a middleware that rate limits by IP address only.
// This is useful for public endpoints that don't require authentication.
func (m *RateLimitMiddleware) IPRateLimit(rpm int) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("ratelimit:ip:%s", clientIP)

		result, err := m.limiter.Allow(c.Request.Context(), key, redis_rate.PerMinute(rpm))
		if err != nil {
			// Fail open on error
			c.Next()
			return
		}

		// Set rate limit headers
		m.setRateLimitHeaders(c, result, rpm)

		if result.Allowed == 0 {
			retryAfter := int(result.RetryAfter.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}

			handlers.TooManyRequests(c,
				fmt.Sprintf("Rate limit exceeded. Please retry after %d seconds.", retryAfter),
				retryAfter,
			)
			c.Abort()
			return
		}

		c.Next()
	}
}

// CheckRateLimit allows programmatic rate limit checking without middleware.
// Returns true if the request should be allowed, false if rate limited.
func (m *RateLimitMiddleware) CheckRateLimit(ctx context.Context, key string, rpm int) (bool, error) {
	result, err := m.limiter.Allow(ctx, key, redis_rate.PerMinute(rpm))
	if err != nil {
		return false, fmt.Errorf("rate limit check failed: %w", err)
	}
	return result.Allowed > 0, nil
}

// Default rate limit for query endpoints (requests per minute).
const DefaultQueryRPM = 200

// RateLimitWithConfig returns a Gin middleware handler that enforces rate limits
// with a custom requests-per-minute configuration. It uses API key-based limiting
// when available, falling back to IP-based limiting otherwise.
// This is useful for route groups that need different rate limits than the default.
func (m *RateLimitMiddleware) RateLimitWithConfig(rpm int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Build the rate limit key (uses API key prefix if available, else IP)
		key := m.buildKey(c)

		// Apply rate limiting using GCRA with the custom RPM
		result, err := m.limiter.Allow(c.Request.Context(), key, redis_rate.PerMinute(rpm))
		if err != nil {
			// On rate limiter error, log and allow the request (fail open for availability)
			c.Next()
			return
		}

		// Set rate limit headers
		m.setRateLimitHeaders(c, result, rpm)

		// Check if request is allowed
		if result.Allowed == 0 {
			retryAfter := int(result.RetryAfter.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}

			handlers.TooManyRequests(c,
				fmt.Sprintf("Rate limit exceeded. Please retry after %d seconds.", retryAfter),
				retryAfter,
			)
			c.Abort()
			return
		}

		c.Next()
	}
}

// QueryRateLimit returns a middleware that applies query-specific rate limits.
// Query endpoints are read-only operations and can handle higher request volumes,
// so they default to 200 requests per minute (vs 100 for emission endpoints).
// Uses API key-based limiting when available, falling back to IP-based limiting.
func (m *RateLimitMiddleware) QueryRateLimit() gin.HandlerFunc {
	return m.RateLimitWithConfig(DefaultQueryRPM)
}
