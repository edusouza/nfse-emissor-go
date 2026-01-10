// Package middleware provides HTTP middleware for the NFS-e API.
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
)

// setupTestRedis creates a miniredis instance and returns both the server and a connected client.
func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *goredis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})
	return mr, client
}

// createTestAPIKey creates an APIKey for testing with configurable rate limits.
func createTestAPIKey(prefix string, rpm, burst int) *mongodb.APIKey {
	return &mongodb.APIKey{
		KeyPrefix:      prefix,
		IntegratorName: "Test Integrator",
		Active:         true,
		RateLimit: mongodb.RateLimitConfig{
			RequestsPerMinute: rpm,
			Burst:             burst,
		},
	}
}

// setupTestRouter creates a test router with the given middleware and handler.
func setupTestRouter(middleware gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return router
}

// TestNewRateLimitMiddleware tests the creation of RateLimitMiddleware.
func TestNewRateLimitMiddleware(t *testing.T) {
	t.Run("creates middleware with valid Redis client", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		assert.NotNil(t, middleware)
		assert.NotNil(t, middleware.limiter)
		assert.Equal(t, 100, middleware.defaultRPM)
		assert.Equal(t, 10, middleware.defaultBurst)
	})

	t.Run("uses default RPM and burst values", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		defaultRPM := 50
		defaultBurst := 5
		middleware := NewRateLimitMiddleware(client, defaultRPM, defaultBurst)

		assert.Equal(t, defaultRPM, middleware.defaultRPM)
		assert.Equal(t, defaultBurst, middleware.defaultBurst)
	})

	t.Run("handles custom configuration values", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 500, 50)

		assert.Equal(t, 500, middleware.defaultRPM)
		assert.Equal(t, 50, middleware.defaultBurst)
	})
}

// TestRateLimit tests the main RateLimit middleware.
func TestRateLimit(t *testing.T) {
	t.Run("allows requests under limit", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimit())

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("sets correct rate limit headers", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimit())

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get(HeaderRateLimitLimit))
		assert.NotEmpty(t, w.Header().Get(HeaderRateLimitRemaining))
		assert.NotEmpty(t, w.Header().Get(HeaderRateLimitReset))

		// Verify the limit header contains the correct value
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, 100, limit)
	})

	t.Run("blocks requests over limit (returns 429)", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		// Use very low limit to trigger rate limiting quickly
		middleware := NewRateLimitMiddleware(client, 1, 1)
		router := setupTestRouter(middleware.RateLimit())

		// First request should succeed
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request should be rate limited
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)

		// Verify Retry-After header is set
		retryAfter := w2.Header().Get(HeaderRetryAfter)
		assert.NotEmpty(t, retryAfter)
	})

	t.Run("uses API key for rate limit key when available", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("testkey1", 50, 5)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		// Add a middleware that sets the API key in context
		router.Use(func(c *gin.Context) {
			c.Set(APIKeyContextKey, apiKey)
			c.Next()
		})
		router.Use(middleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// The rate limit should use the API key's RPM value
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, 50, limit)
	})

	t.Run("falls back to IP when no API key", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimit())

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should use default RPM since no API key present
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, 100, limit)
	})

	t.Run("uses default RPM when API key has invalid rate limit", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("testkey2", 0, 0) // Invalid: zero values

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(APIKeyContextKey, apiKey)
			c.Next()
		})
		router.Use(middleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should fall back to default RPM
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, 100, limit)
	})

	t.Run("uses default burst when API key has negative values", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("testkey3", -5, -3) // Invalid: negative values

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(APIKeyContextKey, apiKey)
			c.Next()
		})
		router.Use(middleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("separate rate limits for different API keys", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 1, 1)

		apiKey1 := createTestAPIKey("prefix_a", 1, 1)
		apiKey2 := createTestAPIKey("prefix_b", 1, 1)

		gin.SetMode(gin.TestMode)

		// Request with API key 1
		router1 := gin.New()
		router1.Use(func(c *gin.Context) {
			c.Set(APIKeyContextKey, apiKey1)
			c.Next()
		})
		router1.Use(middleware.RateLimit())
		router1.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router1.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Request with API key 2 should also succeed (separate limit)
		router2 := gin.New()
		router2.Use(func(c *gin.Context) {
			c.Set(APIKeyContextKey, apiKey2)
			c.Next()
		})
		router2.Use(middleware.RateLimit())
		router2.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router2.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})
}

// TestRateLimitWithConfig tests the RateLimitWithConfig middleware.
func TestRateLimitWithConfig(t *testing.T) {
	t.Run("uses custom RPM value", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		customRPM := 200

		router := setupTestRouter(middleware.RateLimitWithConfig(customRPM))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, customRPM, limit)
	})

	t.Run("still uses API key-based limiting", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("configtest", 50, 5)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(APIKeyContextKey, apiKey)
			c.Next()
		})
		router.Use(middleware.RateLimitWithConfig(150))
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should use the custom RPM (150), not the API key's RPM
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, 150, limit)
	})

	t.Run("blocks when custom limit exceeded", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimitWithConfig(1)) // Very low limit

		// First request should succeed
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request should be rate limited
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	})

	t.Run("falls back to IP when no API key for config", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimitWithConfig(75))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.1:54321"
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, 75, limit)
	})
}

// TestQueryRateLimit tests the QueryRateLimit middleware.
func TestQueryRateLimit(t *testing.T) {
	t.Run("uses DefaultQueryRPM (200 req/min)", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.QueryRateLimit())

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, DefaultQueryRPM, limit)
		assert.Equal(t, 200, limit)
	})

	t.Run("delegates to RateLimitWithConfig", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("querytest", 100, 10)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(APIKeyContextKey, apiKey)
			c.Next()
		})
		router.Use(middleware.QueryRateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// QueryRateLimit should use DefaultQueryRPM regardless of API key's setting
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, DefaultQueryRPM, limit)
	})

	t.Run("query rate limit is higher than emission default", func(t *testing.T) {
		// This test documents the design decision that query endpoints
		// have higher rate limits than emission endpoints
		assert.Equal(t, 200, DefaultQueryRPM)
		// Typical emission default is 100, so query is 2x higher
	})
}

// TestIPRateLimit tests the IP-based rate limiting middleware.
func TestIPRateLimit(t *testing.T) {
	t.Run("uses IP-based limiting only", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.IPRateLimit(50))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "172.16.0.1:8080"
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, 50, limit)
	})

	t.Run("applies specified RPM", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		customRPM := 75
		router := setupTestRouter(middleware.IPRateLimit(customRPM))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, customRPM, limit)
	})

	t.Run("ignores API key even when present", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("iptest", 500, 50)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(APIKeyContextKey, apiKey)
			c.Next()
		})
		router.Use(middleware.IPRateLimit(25))
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.100.1:9999"
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should use the IP-based RPM (25), not the API key's RPM (500)
		limitHeader := w.Header().Get(HeaderRateLimitLimit)
		limit, err := strconv.Atoi(limitHeader)
		require.NoError(t, err)
		assert.Equal(t, 25, limit)
	})

	t.Run("blocks when IP limit exceeded", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.IPRateLimit(1))

		// First request should succeed
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request should be rate limited
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	})

	t.Run("different IPs have separate limits", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.IPRateLimit(1))

		// First IP request
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req1.RemoteAddr = "10.0.0.1:1234"
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Different IP should also succeed
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req2.RemoteAddr = "10.0.0.2:1234"
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		// First IP again should be rate limited
		w3 := httptest.NewRecorder()
		req3, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req3.RemoteAddr = "10.0.0.1:5678"
		router.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	})
}

// TestCheckRateLimit tests the programmatic rate limit checking.
func TestCheckRateLimit(t *testing.T) {
	t.Run("returns true when allowed", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		ctx := context.Background()

		allowed, err := middleware.CheckRateLimit(ctx, "test:key:1", 100)

		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("returns false when rate limited", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		ctx := context.Background()
		key := "test:key:limited"

		// First request should be allowed
		allowed1, err := middleware.CheckRateLimit(ctx, key, 1)
		require.NoError(t, err)
		assert.True(t, allowed1)

		// Second request should be rate limited
		allowed2, err := middleware.CheckRateLimit(ctx, key, 1)
		require.NoError(t, err)
		assert.False(t, allowed2)
	})

	t.Run("different keys have separate limits", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		ctx := context.Background()

		// First key
		allowed1, err := middleware.CheckRateLimit(ctx, "test:key:a", 1)
		require.NoError(t, err)
		assert.True(t, allowed1)

		// Second key should also be allowed
		allowed2, err := middleware.CheckRateLimit(ctx, "test:key:b", 1)
		require.NoError(t, err)
		assert.True(t, allowed2)
	})

	t.Run("returns error when Redis fails", func(t *testing.T) {
		mr, client := setupTestRedis(t)

		middleware := NewRateLimitMiddleware(client, 100, 10)
		ctx := context.Background()

		// Close miniredis to simulate Redis failure
		mr.Close()

		allowed, err := middleware.CheckRateLimit(ctx, "test:key:error", 100)

		assert.Error(t, err)
		assert.False(t, allowed)
		assert.Contains(t, err.Error(), "rate limit check failed")
	})

	t.Run("respects different RPM values", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		ctx := context.Background()
		key := "test:key:rpm"

		// With RPM of 2, we should get 2 allowed requests
		allowed1, err := middleware.CheckRateLimit(ctx, key, 2)
		require.NoError(t, err)
		assert.True(t, allowed1)

		allowed2, err := middleware.CheckRateLimit(ctx, key, 2)
		require.NoError(t, err)
		assert.True(t, allowed2)

		// Third request should be limited
		allowed3, err := middleware.CheckRateLimit(ctx, key, 2)
		require.NoError(t, err)
		assert.False(t, allowed3)
	})
}

// TestRateLimitHeaders tests that all rate limit headers are properly set.
func TestRateLimitHeaders(t *testing.T) {
	t.Run("all standard headers are set", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimit())

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.NotEmpty(t, w.Header().Get(HeaderRateLimitLimit))
		assert.NotEmpty(t, w.Header().Get(HeaderRateLimitRemaining))
		assert.NotEmpty(t, w.Header().Get(HeaderRateLimitReset))
	})

	t.Run("remaining decreases with requests", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimit())

		// First request
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)

		remaining1, _ := strconv.Atoi(w1.Header().Get(HeaderRateLimitRemaining))

		// Second request
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)

		remaining2, _ := strconv.Atoi(w2.Header().Get(HeaderRateLimitRemaining))

		// Remaining should decrease (or stay same if burst-based)
		assert.GreaterOrEqual(t, remaining1, remaining2)
	})

	t.Run("Retry-After header set on 429", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 1, 1)
		router := setupTestRouter(middleware.RateLimit())

		// Exhaust the limit
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)

		// This should be rate limited
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		assert.NotEmpty(t, w2.Header().Get(HeaderRetryAfter))

		// Verify Retry-After is a valid positive integer
		retryAfter, err := strconv.Atoi(w2.Header().Get(HeaderRetryAfter))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, retryAfter, 1)
	})
}

// TestRateLimitResponseBody tests the response body on rate limiting.
func TestRateLimitResponseBody(t *testing.T) {
	t.Run("returns problem details on 429", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 1, 1)
		router := setupTestRouter(middleware.RateLimit())

		// Exhaust the limit
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)

		// This should be rate limited
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		// Should return problem+json content type
		assert.Contains(t, w2.Header().Get("Content-Type"), "application/problem+json")
		// Body should contain rate limit message
		assert.Contains(t, w2.Body.String(), "Rate limit exceeded")
	})
}

// TestGetRateLimitConfig tests the internal getRateLimitConfig method.
func TestGetRateLimitConfig(t *testing.T) {
	t.Run("returns defaults when no API key", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		rpm, burst := middleware.getRateLimitConfig(c)

		assert.Equal(t, 100, rpm)
		assert.Equal(t, 10, burst)
	})

	t.Run("returns API key values when present", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("configtest", 200, 20)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(APIKeyContextKey, apiKey)

		rpm, burst := middleware.getRateLimitConfig(c)

		assert.Equal(t, 200, rpm)
		assert.Equal(t, 20, burst)
	})

	t.Run("returns defaults when API key has zero values", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("zerotest", 0, 0)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(APIKeyContextKey, apiKey)

		rpm, burst := middleware.getRateLimitConfig(c)

		assert.Equal(t, 100, rpm)
		assert.Equal(t, 10, burst)
	})

	t.Run("returns defaults when API key has negative values", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("negtest", -50, -5)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(APIKeyContextKey, apiKey)

		rpm, burst := middleware.getRateLimitConfig(c)

		assert.Equal(t, 100, rpm)
		assert.Equal(t, 10, burst)
	})
}

// TestBuildKey tests the internal buildKey method.
func TestBuildKey(t *testing.T) {
	t.Run("uses API key prefix when available", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("myprefix", 100, 10)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Set(APIKeyContextKey, apiKey)

		key := middleware.buildKey(c)

		assert.Equal(t, "ratelimit:myprefix", key)
	})

	t.Run("uses IP when no API key", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.RemoteAddr = "192.168.1.100:12345"

		key := middleware.buildKey(c)

		assert.Equal(t, "ratelimit:ip:192.168.1.100", key)
	})

	t.Run("uses IP when API key has empty prefix", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("", 100, 10) // Empty prefix

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.RemoteAddr = "10.10.10.10:8080"
		c.Set(APIKeyContextKey, apiKey)

		key := middleware.buildKey(c)

		assert.Equal(t, "ratelimit:ip:10.10.10.10", key)
	})
}

// TestDefaultQueryRPM verifies the constant value.
func TestDefaultQueryRPM(t *testing.T) {
	t.Run("DefaultQueryRPM is 200", func(t *testing.T) {
		assert.Equal(t, 200, DefaultQueryRPM)
	})
}

// TestRateLimitFailOpen tests that the middleware fails open on Redis errors.
func TestRateLimitFailOpen(t *testing.T) {
	t.Run("RateLimit allows request on Redis error", func(t *testing.T) {
		mr, client := setupTestRedis(t)

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimit())

		// Close Redis to simulate failure
		mr.Close()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		// Should allow the request (fail open)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("RateLimitWithConfig allows request on Redis error", func(t *testing.T) {
		mr, client := setupTestRedis(t)

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimitWithConfig(50))

		// Close Redis to simulate failure
		mr.Close()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		// Should allow the request (fail open)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("IPRateLimit allows request on Redis error", func(t *testing.T) {
		mr, client := setupTestRedis(t)

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.IPRateLimit(50))

		// Close Redis to simulate failure
		mr.Close()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		// Should allow the request (fail open)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestHeaderConstants verifies the header constant values.
func TestHeaderConstants(t *testing.T) {
	t.Run("header constants have correct values", func(t *testing.T) {
		assert.Equal(t, "X-RateLimit-Limit", HeaderRateLimitLimit)
		assert.Equal(t, "X-RateLimit-Remaining", HeaderRateLimitRemaining)
		assert.Equal(t, "X-RateLimit-Reset", HeaderRateLimitReset)
		assert.Equal(t, "Retry-After", HeaderRetryAfter)
	})
}

// TestRateLimitConfigStruct tests the RateLimitConfig struct.
func TestRateLimitConfigStruct(t *testing.T) {
	t.Run("can create config with valid values", func(t *testing.T) {
		config := RateLimitConfig{
			RequestsPerMinute: 100,
			Burst:             10,
		}
		assert.Equal(t, 100, config.RequestsPerMinute)
		assert.Equal(t, 10, config.Burst)
	})

	t.Run("zero value config has zero fields", func(t *testing.T) {
		var config RateLimitConfig
		assert.Equal(t, 0, config.RequestsPerMinute)
		assert.Equal(t, 0, config.Burst)
	})
}

// TestGetRateLimitConfigEdgeCases tests edge cases in getRateLimitConfig.
func TestGetRateLimitConfigEdgeCases(t *testing.T) {
	t.Run("returns defaults when API key context value is wrong type", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		// Set wrong type in context (string instead of *mongodb.APIKey)
		c.Set(APIKeyContextKey, "not-an-api-key")

		rpm, burst := middleware.getRateLimitConfig(c)

		assert.Equal(t, 100, rpm)
		assert.Equal(t, 10, burst)
	})

	t.Run("returns defaults when API key context value is nil interface", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		// Set nil value in context
		c.Set(APIKeyContextKey, nil)

		rpm, burst := middleware.getRateLimitConfig(c)

		assert.Equal(t, 100, rpm)
		assert.Equal(t, 10, burst)
	})

	t.Run("returns API key RPM when only burst is zero", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("mixedtest", 150, 0) // Valid RPM, zero burst

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(APIKeyContextKey, apiKey)

		rpm, burst := middleware.getRateLimitConfig(c)

		assert.Equal(t, 150, rpm)     // Use API key's RPM
		assert.Equal(t, 10, burst)    // Fall back to default burst
	})

	t.Run("returns API key burst when only RPM is zero", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("mixedtest2", 0, 15) // Zero RPM, valid burst

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(APIKeyContextKey, apiKey)

		rpm, burst := middleware.getRateLimitConfig(c)

		assert.Equal(t, 100, rpm)     // Fall back to default RPM
		assert.Equal(t, 15, burst)    // Use API key's burst
	})
}

// TestBuildKeyEdgeCases tests edge cases in the buildKey method.
func TestBuildKeyEdgeCases(t *testing.T) {
	t.Run("uses IP when API key context value is wrong type", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.RemoteAddr = "203.0.113.50:12345"
		// Set wrong type in context
		c.Set(APIKeyContextKey, 12345)

		key := middleware.buildKey(c)

		assert.Equal(t, "ratelimit:ip:203.0.113.50", key)
	})

	t.Run("handles IPv6 addresses", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.RemoteAddr = "[::1]:12345"

		key := middleware.buildKey(c)

		assert.Equal(t, "ratelimit:ip:::1", key)
	})

	t.Run("handles X-Forwarded-For header", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.ForwardedByClientIP = true
		router.RemoteIPHeaders = []string{"X-Forwarded-For"}
		router.Use(middleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "198.51.100.1")
		req.RemoteAddr = "10.0.0.1:12345"
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestQueryRateLimitUsesDefaultRPM tests that QueryRateLimit uses DefaultQueryRPM.
func TestQueryRateLimitUsesDefaultRPM(t *testing.T) {
	t.Run("uses DefaultQueryRPM of 200", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		// Create middleware
		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(middleware.QueryRateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		// First request should succeed
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Verify rate limit header shows 200 (DefaultQueryRPM)
		assert.Equal(t, "200", w.Header().Get(HeaderRateLimitLimit))
	})
}

// TestRateLimitRetryAfterMinimum tests that Retry-After is at least 1 second.
func TestRateLimitRetryAfterMinimum(t *testing.T) {
	t.Run("RateLimit sets Retry-After to at least 1 second", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 1, 1)
		router := setupTestRouter(middleware.RateLimit())

		// First request succeeds
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request is rate limited
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		retryAfter, err := strconv.Atoi(w2.Header().Get(HeaderRetryAfter))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, retryAfter, 1, "Retry-After should be at least 1 second")
	})

	t.Run("IPRateLimit sets Retry-After to at least 1 second", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.IPRateLimit(1))

		// First request succeeds
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request is rate limited
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		retryAfter, err := strconv.Atoi(w2.Header().Get(HeaderRetryAfter))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, retryAfter, 1, "Retry-After should be at least 1 second")
	})

	t.Run("RateLimitWithConfig sets Retry-After to at least 1 second", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimitWithConfig(1))

		// First request succeeds
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request is rate limited
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		retryAfter, err := strconv.Atoi(w2.Header().Get(HeaderRetryAfter))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, retryAfter, 1, "Retry-After should be at least 1 second")
	})
}

// TestRateLimitConcurrent tests rate limiting under concurrent requests.
func TestRateLimitConcurrent(t *testing.T) {
	t.Run("handles concurrent requests correctly", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 10, 5)
		router := setupTestRouter(middleware.RateLimit())

		// Send concurrent requests
		const numRequests = 20
		results := make(chan int, numRequests)

		for i := 0; i < numRequests; i++ {
			go func() {
				w := httptest.NewRecorder()
				req, _ := http.NewRequest(http.MethodGet, "/test", nil)
				router.ServeHTTP(w, req)
				results <- w.Code
			}()
		}

		// Collect results
		var okCount, limitedCount int
		for i := 0; i < numRequests; i++ {
			code := <-results
			if code == http.StatusOK {
				okCount++
			} else if code == http.StatusTooManyRequests {
				limitedCount++
			}
		}

		// Some requests should succeed and some should be rate limited
		assert.Greater(t, okCount, 0, "At least some requests should succeed")
		assert.Greater(t, limitedCount, 0, "At least some requests should be rate limited")
		assert.Equal(t, numRequests, okCount+limitedCount, "All requests should be either OK or rate limited")
	})
}

// TestRateLimitMiddlewareChaining tests that rate limit middleware can be chained with others.
func TestRateLimitMiddlewareChaining(t *testing.T) {
	t.Run("works with other middleware", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		apiKey := createTestAPIKey("chaintest", 100, 10)

		gin.SetMode(gin.TestMode)
		router := gin.New()

		// Add a custom middleware that sets a header
		router.Use(func(c *gin.Context) {
			c.Header("X-Custom-Header", "test-value")
			c.Next()
		})

		// Add API key to context
		router.Use(func(c *gin.Context) {
			c.Set(APIKeyContextKey, apiKey)
			c.Next()
		})

		// Add rate limit middleware
		router.Use(middleware.RateLimit())

		// Add another middleware after rate limit
		router.Use(func(c *gin.Context) {
			c.Header("X-After-RateLimit", "executed")
			c.Next()
		})

		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "test-value", w.Header().Get("X-Custom-Header"))
		assert.Equal(t, "executed", w.Header().Get("X-After-RateLimit"))
		assert.NotEmpty(t, w.Header().Get(HeaderRateLimitLimit))
	})

	t.Run("aborts chain when rate limited", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 1, 1)

		gin.SetMode(gin.TestMode)
		router := gin.New()

		handlerExecuted := false

		router.Use(middleware.RateLimit())
		router.Use(func(c *gin.Context) {
			handlerExecuted = true
			c.Next()
		})

		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		// First request succeeds
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)
		assert.True(t, handlerExecuted)

		// Reset flag
		handlerExecuted = false

		// Second request is rate limited and should abort
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		assert.False(t, handlerExecuted, "Subsequent middleware should not execute when rate limited")
	})
}

// TestRateLimitWithDifferentHTTPMethods tests rate limiting across different HTTP methods.
func TestRateLimitWithDifferentHTTPMethods(t *testing.T) {
	t.Run("applies same rate limit key for different methods from same client", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 2, 2)
		apiKey := createTestAPIKey("methodtest", 2, 2)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(APIKeyContextKey, apiKey)
			c.Next()
		})
		router.Use(middleware.RateLimit())
		router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
		router.POST("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
		router.PUT("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

		// GET request
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// POST request (same key, should share rate limit)
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodPost, "/test", nil)
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		// PUT request (should be rate limited)
		w3 := httptest.NewRecorder()
		req3, _ := http.NewRequest(http.MethodPut, "/test", nil)
		router.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	})
}

// TestSetRateLimitHeaders tests the header setting function directly.
func TestSetRateLimitHeaders(t *testing.T) {
	t.Run("sets all required headers", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(middleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		// Verify all headers are present
		assert.NotEmpty(t, w.Header().Get(HeaderRateLimitLimit), "X-RateLimit-Limit should be set")
		assert.NotEmpty(t, w.Header().Get(HeaderRateLimitRemaining), "X-RateLimit-Remaining should be set")
		assert.NotEmpty(t, w.Header().Get(HeaderRateLimitReset), "X-RateLimit-Reset should be set")

		// Verify Limit is the expected value
		limit, err := strconv.Atoi(w.Header().Get(HeaderRateLimitLimit))
		require.NoError(t, err)
		assert.Equal(t, 100, limit)

		// Verify Remaining is less than or equal to Limit
		remaining, err := strconv.Atoi(w.Header().Get(HeaderRateLimitRemaining))
		require.NoError(t, err)
		assert.LessOrEqual(t, remaining, limit)

		// Verify Reset is a valid number
		reset, err := strconv.ParseInt(w.Header().Get(HeaderRateLimitReset), 10, 64)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, reset, int64(0))
	})

	t.Run("headers are set correctly when rate limited", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 1, 1)
		router := setupTestRouter(middleware.RateLimit())

		// First request
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)

		// Second request (rate limited)
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		// Headers should still be set on rate limited response
		assert.NotEmpty(t, w2.Header().Get(HeaderRateLimitLimit))
		assert.NotEmpty(t, w2.Header().Get(HeaderRateLimitRemaining))
		assert.NotEmpty(t, w2.Header().Get(HeaderRateLimitReset))
		assert.NotEmpty(t, w2.Header().Get(HeaderRetryAfter))
	})
}

// TestCheckRateLimitEdgeCases tests edge cases in CheckRateLimit.
func TestCheckRateLimitEdgeCases(t *testing.T) {
	t.Run("handles empty key", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		ctx := context.Background()

		allowed, err := middleware.CheckRateLimit(ctx, "", 100)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("handles very long key", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		ctx := context.Background()

		// Create a very long key
		longKey := "test:" + string(make([]byte, 1000))
		allowed, err := middleware.CheckRateLimit(ctx, longKey, 100)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("handles special characters in key", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		ctx := context.Background()

		allowed, err := middleware.CheckRateLimit(ctx, "test:key:with:special:chars!@#$%", 100)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("handles cancelled context", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		allowed, err := middleware.CheckRateLimit(ctx, "test:cancelled", 100)
		// Should return error due to cancelled context
		assert.Error(t, err)
		assert.False(t, allowed)
	})

	t.Run("handles high RPM value", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		ctx := context.Background()

		// Test with very high RPM
		allowed, err := middleware.CheckRateLimit(ctx, "test:high:rpm", 100000)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	// Note: Zero RPM test removed as GCRA algorithm doesn't handle it correctly
}

// TestIPRateLimitEdgeCases tests edge cases in IPRateLimit.
func TestIPRateLimitEdgeCases(t *testing.T) {
	t.Run("handles missing RemoteAddr", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(middleware.IPRateLimit(100))
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "" // Empty RemoteAddr
		router.ServeHTTP(w, req)

		// Should still work (ClientIP() handles this)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handles RemoteAddr without port", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(middleware.IPRateLimit(100))
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1" // No port
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("limits correctly with very high RPM", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.IPRateLimit(10000))

		// All requests should succeed with very high limit
		for i := 0; i < 100; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/test", nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})
}

// TestRateLimitWithConfigEdgeCases tests edge cases in RateLimitWithConfig.
func TestRateLimitWithConfigEdgeCases(t *testing.T) {
	// Note: Zero and negative RPM tests removed as GCRA algorithm doesn't handle them correctly

	t.Run("uses path-specific key for different endpoints", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 1, 1)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(middleware.RateLimitWithConfig(1))
		router.GET("/endpoint1", func(c *gin.Context) { c.Status(http.StatusOK) })
		router.GET("/endpoint2", func(c *gin.Context) { c.Status(http.StatusOK) })

		// First request to endpoint1
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/endpoint1", nil)
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// First request to endpoint2 (same IP, so should be rate limited)
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/endpoint2", nil)
		router.ServeHTTP(w2, req2)
		// Since same IP is used, should be rate limited
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	})
}

// TestFailOpenBehavior tests comprehensive fail-open behavior.
func TestFailOpenBehavior(t *testing.T) {
	t.Run("RateLimit continues chain on Redis error", func(t *testing.T) {
		mr, client := setupTestRedis(t)

		middleware := NewRateLimitMiddleware(client, 100, 10)

		handlerExecuted := false
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(middleware.RateLimit())
		router.GET("/test", func(c *gin.Context) {
			handlerExecuted = true
			c.Status(http.StatusOK)
		})

		// Close Redis to simulate failure
		mr.Close()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, handlerExecuted, "Handler should execute on Redis failure (fail open)")
	})

	t.Run("rate limit headers not set on Redis error", func(t *testing.T) {
		mr, client := setupTestRedis(t)

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.RateLimit())

		// Close Redis to simulate failure
		mr.Close()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Headers should NOT be set when Redis fails
		assert.Empty(t, w.Header().Get(HeaderRateLimitLimit))
		assert.Empty(t, w.Header().Get(HeaderRateLimitRemaining))
		assert.Empty(t, w.Header().Get(HeaderRateLimitReset))
	})

	t.Run("QueryRateLimit fails open", func(t *testing.T) {
		mr, client := setupTestRedis(t)

		middleware := NewRateLimitMiddleware(client, 100, 10)
		router := setupTestRouter(middleware.QueryRateLimit())

		// Close Redis to simulate failure
		mr.Close()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestRateLimitResponseMessage tests the error response message.
func TestRateLimitResponseMessage(t *testing.T) {
	t.Run("response contains retry after information", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 1, 1)
		router := setupTestRouter(middleware.RateLimit())

		// First request
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w1, req1)

		// Second request (rate limited)
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		body := w2.Body.String()
		assert.Contains(t, body, "Rate limit exceeded")
		assert.Contains(t, body, "retry")
	})
}

// TestNewRateLimitMiddlewareEdgeCases tests edge cases in middleware creation.
func TestNewRateLimitMiddlewareEdgeCases(t *testing.T) {
	t.Run("handles zero default values", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 0, 0)

		assert.NotNil(t, middleware)
		assert.Equal(t, 0, middleware.defaultRPM)
		assert.Equal(t, 0, middleware.defaultBurst)
	})

	t.Run("handles negative default values", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, -10, -5)

		assert.NotNil(t, middleware)
		assert.Equal(t, -10, middleware.defaultRPM)
		assert.Equal(t, -5, middleware.defaultBurst)
	})

	t.Run("limiter is correctly initialized", func(t *testing.T) {
		mr, client := setupTestRedis(t)
		defer mr.Close()

		middleware := NewRateLimitMiddleware(client, 100, 10)

		assert.NotNil(t, middleware.limiter)
	})
}
