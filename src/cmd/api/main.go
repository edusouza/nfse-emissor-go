// Package main provides the entry point for the NFS-e Nacional API server.
// It initializes all dependencies, sets up graceful shutdown, and starts the HTTP server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eduardo/nfse-nacional/internal/api"
	"github.com/eduardo/nfse-nacional/internal/api/handlers"
	"github.com/eduardo/nfse-nacional/internal/config"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
	infraredis "github.com/eduardo/nfse-nacional/internal/infrastructure/redis"
)

const (
	// shutdownTimeout is the maximum time to wait for graceful shutdown.
	shutdownTimeout = 30 * time.Second

	// startupTimeout is the maximum time to wait for dependencies to connect.
	startupTimeout = 30 * time.Second

	// drainTimeout is the time to wait for in-flight requests to complete.
	drainTimeout = 15 * time.Second
)

// ShutdownPhase represents a phase in the shutdown sequence.
type ShutdownPhase struct {
	Name     string
	Action   func(ctx context.Context) error
	Critical bool // If true, failure aborts shutdown
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log startup information
	logStartupInfo(cfg)

	// Create context for startup
	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()

	// Initialize MongoDB
	mongoClient, err := initMongoDB(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}
	log.Printf("MongoDB connected successfully to database: %s", cfg.MongoDBDatabase)

	// Initialize Redis
	redisClient, err := initRedis(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	log.Println("Redis connected successfully")

	// Initialize job client for Asynq
	jobClient, err := infraredis.NewJobClientFromURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to initialize job client: %v", err)
	}
	log.Println("Job client initialized successfully")

	// Initialize repositories
	apiKeyRepo := mongodb.NewAPIKeyRepository(mongoClient)
	emissionRepo := mongodb.NewEmissionRepository(mongoClient)

	// Ensure indexes are created
	if err := apiKeyRepo.EnsureIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to ensure API key indexes: %v", err)
	}
	if err := emissionRepo.EnsureIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to ensure emission indexes: %v", err)
	}

	// Determine base URL for status URLs
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", cfg.Port)
	}

	// Setup router with all dependencies
	router := api.NewRouter(api.RouterConfig{
		Config:       cfg,
		MongoClient:  mongoClient,
		RedisClient:  redisClient,
		APIKeyRepo:   apiKeyRepo,
		EmissionRepo: emissionRepo,
		JobClient:    jobClient,
		BaseURL:      baseURL,
	})

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting NFS-e Nacional API server on port %s", cfg.Port)
		log.Printf("Environment: %s", cfg.Env)
		log.Printf("Health check: http://localhost:%s/health", cfg.Port)
		log.Printf("API endpoint: http://localhost:%s/v1/nfse", cfg.Port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	logShutdownEvent("shutdown_initiated", map[string]interface{}{
		"signal": sig.String(),
	})

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Track shutdown start time
	shutdownStart := time.Now()

	// Define shutdown phases in order
	phases := []ShutdownPhase{
		{
			Name: "http_server",
			Action: func(ctx context.Context) error {
				// First, stop accepting new connections
				logShutdownEvent("draining_connections", nil)
				return server.Shutdown(ctx)
			},
			Critical: true,
		},
		{
			Name: "job_client",
			Action: func(ctx context.Context) error {
				return jobClient.Close()
			},
			Critical: false,
		},
		{
			Name: "redis_connection",
			Action: func(ctx context.Context) error {
				return redisClient.Close()
			},
			Critical: false,
		},
		{
			Name: "mongodb_connection",
			Action: func(ctx context.Context) error {
				return mongoClient.Disconnect(ctx)
			},
			Critical: false,
		},
	}

	// Execute shutdown phases
	var shutdownErrors []error
	for _, phase := range phases {
		phaseStart := time.Now()

		select {
		case <-shutdownCtx.Done():
			logShutdownEvent("shutdown_timeout", map[string]interface{}{
				"phase":   phase.Name,
				"elapsed": time.Since(shutdownStart).String(),
			})
			log.Fatal("Shutdown timed out during phase:", phase.Name)
		default:
		}

		err := phase.Action(shutdownCtx)
		phaseDuration := time.Since(phaseStart)

		if err != nil {
			logShutdownEvent("phase_error", map[string]interface{}{
				"phase":    phase.Name,
				"error":    err.Error(),
				"duration": phaseDuration.String(),
			})
			shutdownErrors = append(shutdownErrors, fmt.Errorf("%s: %w", phase.Name, err))

			if phase.Critical {
				log.Printf("Critical shutdown phase failed: %s: %v", phase.Name, err)
			}
		} else {
			logShutdownEvent("phase_complete", map[string]interface{}{
				"phase":    phase.Name,
				"duration": phaseDuration.String(),
			})
		}
	}

	// Log final shutdown status
	totalDuration := time.Since(shutdownStart)
	if len(shutdownErrors) > 0 {
		logShutdownEvent("shutdown_complete_with_errors", map[string]interface{}{
			"duration":    totalDuration.String(),
			"error_count": len(shutdownErrors),
		})
		log.Printf("Server shut down with %d errors in %v", len(shutdownErrors), totalDuration)
	} else {
		logShutdownEvent("shutdown_complete", map[string]interface{}{
			"duration": totalDuration.String(),
		})
		log.Printf("Server exited gracefully in %v", totalDuration)
	}
}

// logShutdownEvent logs a shutdown event in structured format.
func logShutdownEvent(event string, data map[string]interface{}) {
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"level":     "info",
		"event":     event,
		"component": "api_shutdown",
	}
	for k, v := range data {
		entry[k] = v
	}

	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Printf("shutdown event: %s %v", event, data)
		return
	}
	log.Println(string(jsonBytes))
}

// initMongoDB initializes the MongoDB connection.
func initMongoDB(ctx context.Context, cfg *config.Config) (*mongodb.Client, error) {
	client, err := mongodb.NewClient(ctx, mongodb.ClientOptions{
		URI:          cfg.MongoDBURI,
		DatabaseName: cfg.MongoDBDatabase,
	})
	if err != nil {
		return nil, fmt.Errorf("mongodb connection failed: %w", err)
	}
	return client, nil
}

// initRedis initializes the Redis connection.
func initRedis(ctx context.Context, cfg *config.Config) (*infraredis.Client, error) {
	client, err := infraredis.NewClient(ctx, infraredis.ClientOptions{
		URL: cfg.RedisURL,
	})
	if err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}
	return client, nil
}

// logStartupInfo logs application startup information.
func logStartupInfo(cfg *config.Config) {
	log.Println("=================================================")
	log.Println("NFS-e Nacional API")
	log.Printf("Version: %s", handlers.Version)
	log.Printf("Environment: %s", cfg.Env)
	log.Printf("Port: %s", cfg.Port)
	log.Printf("Log Level: %s", cfg.LogLevel)
	log.Printf("Log Format: %s", cfg.LogFormat)
	log.Printf("SEFIN Environment: %s", cfg.SEFINEnvironment)
	log.Printf("Worker Concurrency: %d", cfg.WorkerConcurrency)
	log.Printf("Rate Limit (RPM): %d", cfg.RateLimitDefaultRPM)
	log.Println("=================================================")
}
