// Package api provides HTTP routing and request handling for the NFS-e API.
package api

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/eduardo/nfse-nacional/internal/api/handlers"
	"github.com/eduardo/nfse-nacional/internal/api/middleware"
	"github.com/eduardo/nfse-nacional/internal/config"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
	infraredis "github.com/eduardo/nfse-nacional/internal/infrastructure/redis"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/sefin"
)

// RouterConfig contains dependencies needed to configure the router.
type RouterConfig struct {
	// Config is the application configuration.
	Config *config.Config

	// MongoClient is the MongoDB client for health checks and repositories.
	MongoClient *mongodb.Client

	// RedisClient is the Redis client for rate limiting and health checks.
	RedisClient *infraredis.Client

	// APIKeyRepo is the repository for API key lookups.
	APIKeyRepo middleware.APIKeyRepository

	// EmissionRepo is the repository for emission requests.
	EmissionRepo *mongodb.EmissionRepository

	// JobClient is the Asynq job client for enqueueing tasks.
	JobClient *infraredis.JobClient

	// BaseURL is the base URL for constructing status URLs.
	BaseURL string

	// SchemaDir is the directory containing XSD schema files.
	SchemaDir string

	// ValidateCertificate controls whether to validate signer certificate dates.
	// Set to false for testing with expired certificates.
	ValidateCertificate bool

	// SefinClient is the client for communicating with the government SEFIN API.
	// Required for DPS lookup and NFS-e query operations.
	SefinClient sefin.SefinClient
}

// NewRouter creates and configures the Gin router with all middleware and routes.
func NewRouter(cfg RouterConfig) *gin.Engine {
	// Set Gin mode based on environment
	if cfg.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else if cfg.Config.IsDevelopment() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router without default middleware
	router := gin.New()

	// Configure trusted proxies (for accurate client IP detection)
	router.SetTrustedProxies(nil)

	// Create middleware instances
	loggingMiddleware := middleware.NewLoggingMiddleware(middleware.LoggingConfig{
		Format: cfg.Config.LogFormat,
		Level:  cfg.Config.LogLevel,
	})

	// Apply global middleware in order:
	// 1. Request ID - first to ensure all logs have request ID
	// 2. Logging - logs all requests with request ID
	// 3. Recovery - catches panics and logs them
	router.Use(
		middleware.RequestID(),
		loggingMiddleware.Logger(),
		middleware.RecoveryWithLogging(cfg.Config.LogFormat),
	)

	// Health check routes (public, no authentication)
	healthHandler := handlers.NewHealthHandler(cfg.MongoClient, cfg.RedisClient)
	router.GET("/health", healthHandler.Health)
	router.GET("/health/live", healthHandler.Liveness)
	router.GET("/health/ready", healthHandler.Readiness)

	// Metrics endpoint (Prometheus format, no authentication for internal scraping)
	handlers.RegisterMetrics()
	router.GET("/metrics", handlers.MetricsHandler())

	// Determine base URL for status URLs
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", cfg.Config.Port)
	}

	// Create handlers
	var emissionHandler *handlers.EmissionHandler
	var emissionXMLHandler *handlers.EmissionXMLHandler
	var statusHandler *handlers.StatusHandler
	var queryHandler *handlers.QueryHandler
	var dpsHandler *handlers.DPSHandler

	if cfg.EmissionRepo != nil && cfg.JobClient != nil {
		emissionHandler = handlers.NewEmissionHandler(handlers.EmissionHandlerConfig{
			EmissionRepo: cfg.EmissionRepo,
			JobClient:    cfg.JobClient,
			BaseURL:      baseURL,
		})

		// Create emission XML handler for pre-signed XML submissions (Phase 5)
		var err error
		emissionXMLHandler, err = handlers.NewEmissionXMLHandler(handlers.EmissionXMLHandlerConfig{
			EmissionRepo:        cfg.EmissionRepo,
			JobClient:           cfg.JobClient,
			BaseURL:             baseURL,
			SchemaDir:           cfg.SchemaDir,
			ValidateCertificate: cfg.ValidateCertificate,
		})
		if err != nil {
			// Log error but continue - pre-signed XML endpoint will not be available
			fmt.Printf("Warning: Failed to create EmissionXMLHandler: %v\n", err)
		}
	}

	if cfg.EmissionRepo != nil {
		statusHandler = handlers.NewStatusHandler(handlers.StatusHandlerConfig{
			EmissionRepo: cfg.EmissionRepo,
			BaseURL:      baseURL,
		})
	}

	// Create query and DPS handlers for NFS-e query operations (Phase 4 - Query API)
	if cfg.SefinClient != nil {
		queryHandler = handlers.NewQueryHandler(handlers.QueryHandlerConfig{
			SefinClient: cfg.SefinClient,
			BaseURL:     baseURL,
		})

		// Create DPS handler for DPS lookup operations (Phase 4 - User Story 2)
		dpsHandler = handlers.NewDPSHandler(handlers.DPSHandlerConfig{
			SefinClient: cfg.SefinClient,
			BaseURL:     baseURL,
		})
	}

	// API v1 routes (protected)
	v1 := router.Group("/v1")
	{
		// Apply authentication middleware
		if cfg.APIKeyRepo != nil {
			authMiddleware := middleware.NewAuthMiddleware(cfg.APIKeyRepo)
			v1.Use(authMiddleware.Authenticate())

			// Apply rate limiting after authentication
			if cfg.RedisClient != nil {
				rateLimitMiddleware := middleware.NewRateLimitMiddleware(
					cfg.RedisClient.GetClient(),
					cfg.Config.RateLimitDefaultRPM,
					cfg.Config.RateLimitBurst,
				)
				v1.Use(rateLimitMiddleware.RateLimit())
			}
		}

		// Register v1 routes
		registerV1Routes(v1, emissionHandler, emissionXMLHandler, statusHandler, queryHandler, dpsHandler)
	}

	// Handle 404 for undefined routes
	router.NoRoute(func(c *gin.Context) {
		handlers.NotFound(c, "The requested resource was not found")
	})

	// Handle 405 for method not allowed
	router.NoMethod(func(c *gin.Context) {
		handlers.MethodNotAllowed(c, "The HTTP method is not allowed for this resource")
	})

	return router
}

// registerV1Routes registers all v1 API routes.
// These routes are protected by authentication and rate limiting.
func registerV1Routes(v1 *gin.RouterGroup, emissionHandler *handlers.EmissionHandler, emissionXMLHandler *handlers.EmissionXMLHandler, statusHandler *handlers.StatusHandler, queryHandler *handlers.QueryHandler, dpsHandler *handlers.DPSHandler) {
	// API info endpoint
	v1.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "NFS-e Nacional API v1",
			"status":  "operational",
			"version": handlers.Version,
		})
	})

	// Emission endpoints (Phase 3)
	if emissionHandler != nil {
		v1.POST("/nfse", emissionHandler.Create)
	}

	// Pre-signed XML emission endpoint (Phase 5 - User Story 3)
	// Accepts pre-signed DPS XML documents for submission
	if emissionXMLHandler != nil {
		v1.POST("/nfse/xml", emissionXMLHandler.Create)
	}

	// Status endpoints (Phase 3)
	if statusHandler != nil {
		v1.GET("/nfse/status/:requestId", statusHandler.Get)
		v1.GET("/nfse/status", statusHandler.List)
	}

	// Query endpoints (Phase 4 - User Story 1: Query NFS-e by Access Key)
	// Allows integrators to retrieve complete NFS-e documents using the access key
	if queryHandler != nil {
		v1.GET("/nfse/:chaveAcesso", queryHandler.GetNFSe)

		// Events endpoint (Phase 4 - User Story 5: Query Events by Access Key)
		// Allows integrators to retrieve events (cancellations, substitutions, etc.) for an NFS-e
		// Supports optional filtering by event type via ?tipo=e101101 query parameter
		v1.GET("/nfse/:chaveAcesso/eventos", queryHandler.GetEvents)
	}

	// DPS Lookup endpoints (Phase 4 - User Story 2: Lookup Access Key by DPS Identifier)
	// Allows integrators to recover NFS-e access key using the DPS identifier
	if dpsHandler != nil {
		v1.GET("/dps/:id", dpsHandler.Lookup)
		v1.HEAD("/dps/:id", dpsHandler.CheckExists)
	}

	// Event endpoints (Phase 5)
	// v1.POST("/nfse/:key/cancel", eventHandler.Cancel)
	// v1.POST("/nfse/:key/replace", eventHandler.Replace)
}

// NewRouterSimple creates a minimal router for testing or simple deployments.
// It does not require MongoDB or Redis connections.
func NewRouterSimple(cfg *config.Config) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Apply basic middleware
	loggingMiddleware := middleware.NewLoggingMiddleware(middleware.LoggingConfig{
		Format: cfg.LogFormat,
		Level:  cfg.LogLevel,
	})

	router.Use(
		middleware.RequestID(),
		loggingMiddleware.Logger(),
		middleware.RecoveryWithLogging(cfg.LogFormat),
	)

	// Simple health check
	router.GET("/health", handlers.SimpleHealth)

	return router
}
