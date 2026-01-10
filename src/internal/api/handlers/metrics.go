// Package handlers provides HTTP request handlers for the NFS-e API.
package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics for the NFS-e API.
var (
	// requestsTotal counts total API requests.
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "nfse",
			Subsystem: "api",
			Name:      "requests_total",
			Help:      "Total number of API requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// requestDuration measures request duration in seconds.
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "nfse",
			Subsystem: "api",
			Name:      "request_duration_seconds",
			Help:      "Request duration in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint"},
	)

	// requestSize measures request body size in bytes.
	requestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "nfse",
			Subsystem: "api",
			Name:      "request_size_bytes",
			Help:      "Request body size in bytes",
			Buckets:   []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "endpoint"},
	)

	// responseSize measures response body size in bytes.
	responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "nfse",
			Subsystem: "api",
			Name:      "response_size_bytes",
			Help:      "Response body size in bytes",
			Buckets:   []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "endpoint"},
	)

	// emissionsTotal counts total NFS-e emissions.
	emissionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "nfse",
			Subsystem: "emission",
			Name:      "total",
			Help:      "Total NFS-e emissions",
		},
		[]string{"status", "environment"},
	)

	// emissionDuration measures emission processing duration.
	emissionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "nfse",
			Subsystem: "emission",
			Name:      "duration_seconds",
			Help:      "Emission processing duration in seconds",
			Buckets:   []float64{0.5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"status", "environment"},
	)

	// queueDepth tracks the number of pending emission jobs.
	queueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "nfse",
			Subsystem: "queue",
			Name:      "depth",
			Help:      "Number of pending emission jobs",
		},
	)

	// queueLatency measures time jobs spend in queue.
	queueLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "nfse",
			Subsystem: "queue",
			Name:      "latency_seconds",
			Help:      "Time jobs spend in queue before processing",
			Buckets:   []float64{0.1, 0.5, 1, 5, 10, 30, 60},
		},
	)

	// sefinRequestsTotal counts requests to SEFIN API.
	sefinRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "nfse",
			Subsystem: "sefin",
			Name:      "requests_total",
			Help:      "Total requests to SEFIN API",
		},
		[]string{"status", "error_code"},
	)

	// sefinLatency measures SEFIN API response time.
	sefinLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "nfse",
			Subsystem: "sefin",
			Name:      "latency_seconds",
			Help:      "SEFIN API response time in seconds",
			Buckets:   []float64{0.5, 1, 2, 5, 10, 20, 30, 60},
		},
	)

	// webhookDeliveriesTotal counts webhook delivery attempts.
	webhookDeliveriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "nfse",
			Subsystem: "webhook",
			Name:      "deliveries_total",
			Help:      "Total webhook delivery attempts",
		},
		[]string{"status"},
	)

	// webhookLatency measures webhook delivery time.
	webhookLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "nfse",
			Subsystem: "webhook",
			Name:      "latency_seconds",
			Help:      "Webhook delivery time in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10},
		},
	)

	// rateLimitHits counts rate limit hits.
	rateLimitHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "nfse",
			Subsystem: "api",
			Name:      "rate_limit_hits_total",
			Help:      "Total rate limit hits",
		},
		[]string{"api_key_prefix"},
	)

	// authFailures counts authentication failures.
	authFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "nfse",
			Subsystem: "api",
			Name:      "auth_failures_total",
			Help:      "Total authentication failures",
		},
		[]string{"reason"},
	)

	// activeConnections tracks active HTTP connections.
	activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "nfse",
			Subsystem: "api",
			Name:      "active_connections",
			Help:      "Number of active HTTP connections",
		},
	)

	// dbConnectionsActive tracks active database connections.
	dbConnectionsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "nfse",
			Subsystem: "db",
			Name:      "connections_active",
			Help:      "Number of active database connections",
		},
		[]string{"database"},
	)

	// certificateExpiryDays tracks days until certificate expiry.
	certificateExpiryDays = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "nfse",
			Subsystem: "certificate",
			Name:      "expiry_days",
			Help:      "Days until certificate expiry",
		},
		[]string{"type"},
	)

	// errorsTotal counts errors by type.
	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "nfse",
			Subsystem: "api",
			Name:      "errors_total",
			Help:      "Total errors by type",
		},
		[]string{"type", "endpoint"},
	)
)

// metricsRegistered tracks if metrics have been registered.
var metricsRegistered bool

// RegisterMetrics registers all Prometheus metrics.
// This should be called once during application startup.
func RegisterMetrics() {
	if metricsRegistered {
		return
	}

	prometheus.MustRegister(
		requestsTotal,
		requestDuration,
		requestSize,
		responseSize,
		emissionsTotal,
		emissionDuration,
		queueDepth,
		queueLatency,
		sefinRequestsTotal,
		sefinLatency,
		webhookDeliveriesTotal,
		webhookLatency,
		rateLimitHits,
		authFailures,
		activeConnections,
		dbConnectionsActive,
		certificateExpiryDays,
		errorsTotal,
	)

	metricsRegistered = true
}

// MetricsHandler returns the Prometheus metrics handler as a Gin handler.
// This handler should be registered at GET /metrics endpoint.
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// MetricsMiddleware returns a Gin middleware that records request metrics.
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Track active connections
		activeConnections.Inc()
		defer activeConnections.Dec()

		// Get endpoint pattern (use route path for grouping)
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = "unknown"
		}

		// Record request size
		if c.Request.ContentLength > 0 {
			requestSize.WithLabelValues(c.Request.Method, endpoint).Observe(float64(c.Request.ContentLength))
		}

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start).Seconds()

		// Get status code
		status := strconv.Itoa(c.Writer.Status())

		// Record metrics
		requestsTotal.WithLabelValues(c.Request.Method, endpoint, status).Inc()
		requestDuration.WithLabelValues(c.Request.Method, endpoint).Observe(duration)

		// Record response size
		if c.Writer.Size() > 0 {
			responseSize.WithLabelValues(c.Request.Method, endpoint).Observe(float64(c.Writer.Size()))
		}

		// Track errors
		if c.Writer.Status() >= 400 {
			errorType := "client_error"
			if c.Writer.Status() >= 500 {
				errorType = "server_error"
			}
			errorsTotal.WithLabelValues(errorType, endpoint).Inc()
		}
	}
}

// RecordEmission records emission metrics.
func RecordEmission(status string, environment string, duration time.Duration) {
	emissionsTotal.WithLabelValues(status, environment).Inc()
	emissionDuration.WithLabelValues(status, environment).Observe(duration.Seconds())
}

// RecordSefinRequest records SEFIN API request metrics.
func RecordSefinRequest(success bool, errorCode string, latency time.Duration) {
	status := "success"
	if !success {
		status = "failure"
	}
	if errorCode == "" {
		errorCode = "none"
	}

	sefinRequestsTotal.WithLabelValues(status, errorCode).Inc()
	sefinLatency.Observe(latency.Seconds())
}

// RecordWebhookDelivery records webhook delivery metrics.
func RecordWebhookDelivery(success bool, latency time.Duration) {
	status := "success"
	if !success {
		status = "failure"
	}

	webhookDeliveriesTotal.WithLabelValues(status).Inc()
	webhookLatency.Observe(latency.Seconds())
}

// SetQueueDepth sets the current queue depth metric.
func SetQueueDepth(depth float64) {
	queueDepth.Set(depth)
}

// RecordQueueLatency records the time a job spent in queue.
func RecordQueueLatency(latency time.Duration) {
	queueLatency.Observe(latency.Seconds())
}

// RecordRateLimitHit records a rate limit hit.
func RecordRateLimitHit(apiKeyPrefix string) {
	rateLimitHits.WithLabelValues(apiKeyPrefix).Inc()
}

// RecordAuthFailure records an authentication failure.
func RecordAuthFailure(reason string) {
	authFailures.WithLabelValues(reason).Inc()
}

// SetDBConnections sets the database connection count.
func SetDBConnections(database string, count float64) {
	dbConnectionsActive.WithLabelValues(database).Set(count)
}

// SetCertificateExpiryDays sets the certificate expiry days metric.
func SetCertificateExpiryDays(certType string, days float64) {
	certificateExpiryDays.WithLabelValues(certType).Set(days)
}

// RecordError records an error metric.
func RecordError(errorType, endpoint string) {
	errorsTotal.WithLabelValues(errorType, endpoint).Inc()
}

// MetricsCollector provides an interface for collecting custom metrics.
type MetricsCollector interface {
	// CollectMetrics is called periodically to update custom metrics.
	CollectMetrics()
}

// QueueMetricsCollector collects queue-related metrics from Asynq.
type QueueMetricsCollector struct {
	// GetQueueSize returns the current queue size.
	GetQueueSize func() (int, error)
}

// CollectMetrics updates queue metrics.
func (c *QueueMetricsCollector) CollectMetrics() {
	if c.GetQueueSize != nil {
		size, err := c.GetQueueSize()
		if err == nil {
			SetQueueDepth(float64(size))
		}
	}
}

// StartMetricsCollection starts periodic metrics collection.
func StartMetricsCollection(collectors []MetricsCollector, interval time.Duration) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				for _, collector := range collectors {
					collector.CollectMetrics()
				}
			case <-stop:
				return
			}
		}
	}()

	return stop
}

// GetMetricsSummary returns a summary of key metrics for health checks.
type MetricsSummary struct {
	RequestsPerSecond   float64 `json:"requests_per_second"`
	AvgLatencyMs        float64 `json:"avg_latency_ms"`
	ErrorRate           float64 `json:"error_rate"`
	QueueDepth          int     `json:"queue_depth"`
	ActiveConnections   int     `json:"active_connections"`
	EmissionsSuccess    int64   `json:"emissions_success"`
	EmissionsFailure    int64   `json:"emissions_failure"`
	SefinAvgLatencyMs   float64 `json:"sefin_avg_latency_ms"`
	WebhookSuccessRate  float64 `json:"webhook_success_rate"`
}
