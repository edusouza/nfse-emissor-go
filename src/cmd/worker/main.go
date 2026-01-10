// Package main provides the entry point for the NFS-e Nacional worker process.
// It processes background jobs for emission processing and webhook delivery.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"github.com/eduardo/nfse-nacional/internal/config"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
	infraredis "github.com/eduardo/nfse-nacional/internal/infrastructure/redis"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/sefin"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/webhook"
	"github.com/eduardo/nfse-nacional/internal/jobs"
)

const (
	// startupTimeout is the maximum time to wait for dependencies to connect.
	startupTimeout = 30 * time.Second

	// shutdownTimeout is the maximum time to wait for graceful shutdown.
	shutdownTimeout = 30 * time.Second

	// jobDrainTimeout is the time to wait for in-progress jobs to complete.
	jobDrainTimeout = 60 * time.Second
)

// workerStats tracks worker statistics for monitoring.
type workerStats struct {
	processedCount int64
	failedCount    int64
	startTime      time.Time
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

	// Initialize repositories
	emissionRepo := mongodb.NewEmissionRepository(mongoClient)
	webhookRepo := mongodb.NewWebhookRepository(mongoClient)
	apiKeyRepo := mongodb.NewAPIKeyRepository(mongoClient)

	// Ensure indexes are created
	if err := emissionRepo.EnsureIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to ensure emission indexes: %v", err)
	}
	if err := webhookRepo.EnsureIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to ensure webhook indexes: %v", err)
	}

	// Initialize SEFIN client (mock for development)
	sefinClient := sefin.NewMockClient()
	log.Println("Using mock SEFIN client for development")

	// Initialize webhook sender
	webhookSender := webhook.NewSender(webhook.SenderConfig{
		Timeout:    10 * time.Second,
		MaxRetries: 3,
	})

	// Create emission processor
	emissionProcessor := jobs.NewEmissionProcessor(jobs.EmissionProcessorConfig{
		EmissionRepo:  emissionRepo,
		WebhookRepo:   webhookRepo,
		SefinClient:   sefinClient,
		WebhookSender: webhookSender,
	})

	// Create webhook processor
	webhookProcessor := jobs.NewWebhookProcessor(jobs.WebhookProcessorConfig{
		WebhookRepo:   webhookRepo,
		WebhookSender: webhookSender,
		APIKeyRepo:    apiKeyRepo,
	})

	// Parse Redis URL for Asynq
	redisOpts, err := infraredis.GetAsynqRedisOpt(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}

	// Create Asynq server
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisOpts.Addr,
			Password: redisOpts.Password,
			DB:       redisOpts.DB,
			Username: redisOpts.Username,
		},
		asynq.Config{
			Concurrency: cfg.WorkerConcurrency,
			Queues: map[string]int{
				infraredis.QueueCritical: 6,
				infraredis.QueueDefault:  3,
				infraredis.QueueLow:      1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(handleError),
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				// Exponential backoff: 10s, 20s, 40s, 80s, 160s...
				delay := time.Duration(10*(1<<uint(n))) * time.Second
				if delay > 5*time.Minute {
					delay = 5 * time.Minute
				}
				return delay
			},
		},
	)

	// Create task handler mux
	mux := asynq.NewServeMux()

	// Register handlers
	mux.HandleFunc(jobs.TypeEmissionProcess, emissionProcessor.ProcessEmission)
	mux.HandleFunc(jobs.TypeWebhookDelivery, webhookProcessor.ProcessWebhook)

	// Initialize worker stats
	stats := &workerStats{
		startTime: time.Now(),
	}

	// Start server in a goroutine
	serverDone := make(chan struct{})
	go func() {
		log.Printf("Starting NFS-e Nacional worker with concurrency: %d", cfg.WorkerConcurrency)
		if err := srv.Start(mux); err != nil {
			log.Fatalf("Worker failed to start: %v", err)
		}
		close(serverDone)
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	logWorkerShutdownEvent("shutdown_initiated", map[string]interface{}{
		"signal":          sig.String(),
		"uptime":          time.Since(stats.startTime).String(),
		"processed_count": atomic.LoadInt64(&stats.processedCount),
		"failed_count":    atomic.LoadInt64(&stats.failedCount),
	})

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Track shutdown start time
	shutdownStart := time.Now()

	// Phase 1: Stop accepting new jobs and wait for in-progress jobs
	logWorkerShutdownEvent("draining_jobs", nil)

	// Create a channel to track shutdown completion
	shutdownComplete := make(chan struct{})
	go func() {
		srv.Shutdown()
		close(shutdownComplete)
	}()

	// Wait for shutdown with timeout
	select {
	case <-shutdownComplete:
		logWorkerShutdownEvent("asynq_shutdown_complete", map[string]interface{}{
			"duration": time.Since(shutdownStart).String(),
		})
	case <-shutdownCtx.Done():
		logWorkerShutdownEvent("asynq_shutdown_timeout", map[string]interface{}{
			"duration": time.Since(shutdownStart).String(),
		})
		log.Println("Warning: Asynq shutdown timed out, some jobs may not have completed")
		// Force stop - Asynq will re-queue unfinished jobs
		srv.Stop()
	}

	// Phase 2: Close MongoDB connection
	mongoShutdownStart := time.Now()
	disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer disconnectCancel()

	if err := mongoClient.Disconnect(disconnectCtx); err != nil {
		logWorkerShutdownEvent("mongodb_disconnect_error", map[string]interface{}{
			"error":    err.Error(),
			"duration": time.Since(mongoShutdownStart).String(),
		})
	} else {
		logWorkerShutdownEvent("mongodb_disconnected", map[string]interface{}{
			"duration": time.Since(mongoShutdownStart).String(),
		})
	}

	// Log final shutdown status
	totalDuration := time.Since(shutdownStart)
	logWorkerShutdownEvent("shutdown_complete", map[string]interface{}{
		"duration":        totalDuration.String(),
		"uptime":          time.Since(stats.startTime).String(),
		"processed_count": atomic.LoadInt64(&stats.processedCount),
		"failed_count":    atomic.LoadInt64(&stats.failedCount),
	})
	log.Printf("Worker exited gracefully in %v", totalDuration)
}

// logWorkerShutdownEvent logs a worker shutdown event in structured format.
func logWorkerShutdownEvent(event string, data map[string]interface{}) {
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"level":     "info",
		"event":     event,
		"component": "worker_shutdown",
	}
	for k, v := range data {
		entry[k] = v
	}

	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Printf("worker shutdown event: %s %v", event, data)
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

// handleError handles task processing errors.
func handleError(ctx context.Context, task *asynq.Task, err error) {
	log.Printf("Error processing task %s: %v", task.Type(), err)
}

// logStartupInfo logs worker startup information.
func logStartupInfo(cfg *config.Config) {
	log.Println("=================================================")
	log.Println("NFS-e Nacional Worker")
	log.Printf("Environment: %s", cfg.Env)
	log.Printf("Log Level: %s", cfg.LogLevel)
	log.Printf("SEFIN Environment: %s", cfg.SEFINEnvironment)
	log.Printf("Worker Concurrency: %d", cfg.WorkerConcurrency)
	log.Printf("Max Retries: %d", cfg.WorkerMaxRetries)
	log.Println("=================================================")
}
