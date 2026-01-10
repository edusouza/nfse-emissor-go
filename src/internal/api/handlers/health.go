// Package handlers provides HTTP request handlers for the NFS-e API.
package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Version information - should be set at build time.
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// HealthStatus represents the overall health status.
type HealthStatus string

const (
	// HealthStatusHealthy indicates all systems are operational.
	HealthStatusHealthy HealthStatus = "healthy"

	// HealthStatusDegraded indicates some systems have issues but the API is functional.
	HealthStatusDegraded HealthStatus = "degraded"

	// HealthStatusUnhealthy indicates critical systems are down.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ComponentStatus represents the health status of a component.
type ComponentStatus struct {
	// Status is the component's health status.
	Status HealthStatus `json:"status"`

	// Message provides additional details about the status.
	Message string `json:"message,omitempty"`

	// Latency is the response time in milliseconds (for connectivity checks).
	LatencyMs int64 `json:"latency_ms,omitempty"`
}

// HealthResponse represents the response from the health check endpoint.
type HealthResponse struct {
	// Status is the overall health status.
	Status HealthStatus `json:"status"`

	// Version is the API version.
	Version string `json:"version"`

	// Timestamp is the current server time.
	Timestamp string `json:"timestamp"`

	// Components contains the health status of individual components.
	Components map[string]ComponentStatus `json:"components,omitempty"`

	// BuildTime is when the binary was built.
	BuildTime string `json:"build_time,omitempty"`

	// GitCommit is the git commit hash.
	GitCommit string `json:"git_commit,omitempty"`
}

// HealthChecker defines the interface for checking component health.
type HealthChecker interface {
	// Ping checks connectivity and returns an error if unhealthy.
	Ping(ctx context.Context) error
}

// HealthHandler handles health check requests.
type HealthHandler struct {
	mongoChecker HealthChecker
	redisChecker HealthChecker
	checkTimeout time.Duration
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(mongoChecker, redisChecker HealthChecker) *HealthHandler {
	return &HealthHandler{
		mongoChecker: mongoChecker,
		redisChecker: redisChecker,
		checkTimeout: 5 * time.Second,
	}
}

// Health handles GET /health requests.
// It returns the overall health status and individual component statuses.
func (h *HealthHandler) Health(c *gin.Context) {
	response := HealthResponse{
		Status:    HealthStatusHealthy,
		Version:   Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		BuildTime: BuildTime,
		GitCommit: GitCommit,
	}

	// If detailed checks are enabled (has dependencies), check components
	if h.mongoChecker != nil || h.redisChecker != nil {
		response.Components = make(map[string]ComponentStatus)

		var wg sync.WaitGroup
		var mu sync.Mutex
		overallHealthy := true

		// Check MongoDB
		if h.mongoChecker != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				status := h.checkComponent(c.Request.Context(), "mongodb", h.mongoChecker)
				mu.Lock()
				response.Components["mongodb"] = status
				if status.Status != HealthStatusHealthy {
					overallHealthy = false
				}
				mu.Unlock()
			}()
		}

		// Check Redis
		if h.redisChecker != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				status := h.checkComponent(c.Request.Context(), "redis", h.redisChecker)
				mu.Lock()
				response.Components["redis"] = status
				if status.Status != HealthStatusHealthy {
					overallHealthy = false
				}
				mu.Unlock()
			}()
		}

		wg.Wait()

		// Set overall status based on components
		if !overallHealthy {
			response.Status = HealthStatusDegraded
		}
	}

	// Return appropriate status code
	statusCode := http.StatusOK
	if response.Status == HealthStatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// checkComponent checks the health of a single component.
func (h *HealthHandler) checkComponent(ctx context.Context, name string, checker HealthChecker) ComponentStatus {
	checkCtx, cancel := context.WithTimeout(ctx, h.checkTimeout)
	defer cancel()

	start := time.Now()
	err := checker.Ping(checkCtx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return ComponentStatus{
			Status:    HealthStatusUnhealthy,
			Message:   err.Error(),
			LatencyMs: latency,
		}
	}

	return ComponentStatus{
		Status:    HealthStatusHealthy,
		LatencyMs: latency,
	}
}

// Liveness handles GET /health/live requests.
// Returns 200 if the server is running, regardless of dependency health.
// Used by Kubernetes liveness probes.
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// Readiness handles GET /health/ready requests.
// Returns 200 only if all dependencies are healthy.
// Used by Kubernetes readiness probes.
func (h *HealthHandler) Readiness(c *gin.Context) {
	// Check all required dependencies
	if h.mongoChecker != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), h.checkTimeout)
		defer cancel()

		if err := h.mongoChecker.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "not_ready",
				"message": "mongodb unavailable",
			})
			return
		}
	}

	if h.redisChecker != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), h.checkTimeout)
		defer cancel()

		if err := h.redisChecker.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "not_ready",
				"message": "redis unavailable",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// SimpleHealth returns a basic health response without dependency checks.
// Useful for load balancer health checks that just need to know the process is running.
func SimpleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    HealthStatusHealthy,
		Version:   Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
