// Package redis provides Redis connection management and job queue functionality
// for the NFS-e API. This file implements the Asynq job queue client.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// Task type constants define the types of background jobs.
const (
	// TypeEmissionProcess is the task type for processing NFS-e emissions.
	TypeEmissionProcess = "emission:process"

	// TypeWebhookDelivery is the task type for delivering webhooks.
	TypeWebhookDelivery = "webhook:delivery"
)

// Queue name constants.
const (
	// QueueCritical is for high-priority tasks that need immediate processing.
	QueueCritical = "critical"

	// QueueDefault is for standard priority tasks.
	QueueDefault = "default"

	// QueueLow is for low-priority tasks that can be delayed.
	QueueLow = "low"
)

// JobClient wraps the Asynq client for enqueuing background jobs.
type JobClient struct {
	client *asynq.Client
}

// JobClientConfig configures the job client.
type JobClientConfig struct {
	// RedisAddr is the Redis server address (host:port).
	RedisAddr string

	// RedisPassword is the Redis password (optional).
	RedisPassword string

	// RedisDB is the Redis database number.
	RedisDB int

	// RedisUsername is the Redis username (optional, for Redis 6+).
	RedisUsername string
}

// NewJobClient creates a new Asynq job client.
func NewJobClient(config JobClientConfig) (*JobClient, error) {
	if config.RedisAddr == "" {
		return nil, fmt.Errorf("redis address is required")
	}

	redisOpt := asynq.RedisClientOpt{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
		Username: config.RedisUsername,
	}

	client := asynq.NewClient(redisOpt)

	return &JobClient{
		client: client,
	}, nil
}

// NewJobClientFromURL creates a new Asynq job client from a Redis URL.
func NewJobClientFromURL(redisURL string) (*JobClient, error) {
	opts, err := GetAsynqRedisOpt(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	redisOpt := asynq.RedisClientOpt{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
		Username: opts.Username,
	}

	client := asynq.NewClient(redisOpt)

	return &JobClient{
		client: client,
	}, nil
}

// EnqueueOptions configures how a task is enqueued.
type EnqueueOptions struct {
	// Queue specifies which queue to add the task to.
	Queue string

	// MaxRetry specifies the maximum number of retry attempts.
	MaxRetry int

	// Timeout specifies the task processing timeout.
	Timeout time.Duration

	// ProcessAt schedules the task for future processing.
	ProcessAt time.Time

	// ProcessIn schedules the task to be processed after a delay.
	ProcessIn time.Duration

	// TaskID is a unique identifier for the task (for deduplication).
	TaskID string

	// Retention specifies how long to keep the task in the completed queue.
	Retention time.Duration
}

// Enqueue adds a task to the queue for background processing.
func (c *JobClient) Enqueue(ctx context.Context, task *asynq.Task, opts *EnqueueOptions) (*asynq.TaskInfo, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}

	// Build options
	var asynqOpts []asynq.Option

	if opts != nil {
		if opts.Queue != "" {
			asynqOpts = append(asynqOpts, asynq.Queue(opts.Queue))
		}
		if opts.MaxRetry > 0 {
			asynqOpts = append(asynqOpts, asynq.MaxRetry(opts.MaxRetry))
		}
		if opts.Timeout > 0 {
			asynqOpts = append(asynqOpts, asynq.Timeout(opts.Timeout))
		}
		if !opts.ProcessAt.IsZero() {
			asynqOpts = append(asynqOpts, asynq.ProcessAt(opts.ProcessAt))
		}
		if opts.ProcessIn > 0 {
			asynqOpts = append(asynqOpts, asynq.ProcessIn(opts.ProcessIn))
		}
		if opts.TaskID != "" {
			asynqOpts = append(asynqOpts, asynq.TaskID(opts.TaskID))
		}
		if opts.Retention > 0 {
			asynqOpts = append(asynqOpts, asynq.Retention(opts.Retention))
		}
	}

	info, err := c.client.Enqueue(task, asynqOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	return info, nil
}

// EnqueueEmission enqueues an emission processing task.
func (c *JobClient) EnqueueEmission(ctx context.Context, requestID string, opts *EnqueueOptions) (*asynq.TaskInfo, error) {
	task := asynq.NewTask(TypeEmissionProcess, []byte(requestID))

	// Set default options for emission tasks
	if opts == nil {
		opts = &EnqueueOptions{}
	}
	if opts.Queue == "" {
		opts.Queue = QueueDefault
	}
	if opts.MaxRetry == 0 {
		opts.MaxRetry = 3
	}
	if opts.Timeout == 0 {
		opts.Timeout = 2 * time.Minute
	}
	if opts.TaskID == "" {
		opts.TaskID = fmt.Sprintf("emission:%s", requestID)
	}

	return c.Enqueue(ctx, task, opts)
}

// EnqueueWebhook enqueues a webhook delivery task.
func (c *JobClient) EnqueueWebhook(ctx context.Context, deliveryID string, opts *EnqueueOptions) (*asynq.TaskInfo, error) {
	task := asynq.NewTask(TypeWebhookDelivery, []byte(deliveryID))

	// Set default options for webhook tasks
	if opts == nil {
		opts = &EnqueueOptions{}
	}
	if opts.Queue == "" {
		opts.Queue = QueueDefault
	}
	if opts.MaxRetry == 0 {
		opts.MaxRetry = 5
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	return c.Enqueue(ctx, task, opts)
}

// Close closes the job client connection.
func (c *JobClient) Close() error {
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("failed to close job client: %w", err)
	}
	return nil
}

// JobServer wraps the Asynq server for processing background jobs.
type JobServer struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

// JobServerConfig configures the job server.
type JobServerConfig struct {
	// RedisAddr is the Redis server address (host:port).
	RedisAddr string

	// RedisPassword is the Redis password (optional).
	RedisPassword string

	// RedisDB is the Redis database number.
	RedisDB int

	// RedisUsername is the Redis username (optional, for Redis 6+).
	RedisUsername string

	// Concurrency specifies the maximum number of concurrent workers.
	Concurrency int

	// Queues specifies queue priorities (queue name -> priority level).
	Queues map[string]int
}

// NewJobServer creates a new Asynq job server.
func NewJobServer(config JobServerConfig) (*JobServer, error) {
	if config.RedisAddr == "" {
		return nil, fmt.Errorf("redis address is required")
	}

	if config.Concurrency == 0 {
		config.Concurrency = 10
	}

	if config.Queues == nil {
		config.Queues = map[string]int{
			QueueCritical: 6,
			QueueDefault:  3,
			QueueLow:      1,
		}
	}

	redisOpt := asynq.RedisClientOpt{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
		Username: config.RedisUsername,
	}

	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: config.Concurrency,
		Queues:      config.Queues,
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			// Log error - in production, use structured logging
			fmt.Printf("Error processing task %s: %v\n", task.Type(), err)
		}),
		RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
			// Exponential backoff: 10s, 20s, 40s, 80s, 160s...
			return time.Duration(10*(1<<uint(n))) * time.Second
		},
	})

	return &JobServer{
		server: server,
		mux:    asynq.NewServeMux(),
	}, nil
}

// NewJobServerFromURL creates a new Asynq job server from a Redis URL.
func NewJobServerFromURL(redisURL string, concurrency int) (*JobServer, error) {
	opts, err := GetAsynqRedisOpt(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	return NewJobServer(JobServerConfig{
		RedisAddr:     opts.Addr,
		RedisPassword: opts.Password,
		RedisDB:       opts.DB,
		RedisUsername: opts.Username,
		Concurrency:   concurrency,
	})
}

// HandleFunc registers a handler function for a task type.
func (s *JobServer) HandleFunc(taskType string, handler func(context.Context, *asynq.Task) error) {
	s.mux.HandleFunc(taskType, handler)
}

// Handle registers a handler for a task type.
func (s *JobServer) Handle(taskType string, handler asynq.Handler) {
	s.mux.Handle(taskType, handler)
}

// Start starts the job server and begins processing tasks.
func (s *JobServer) Start() error {
	if err := s.server.Start(s.mux); err != nil {
		return fmt.Errorf("failed to start job server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the job server.
func (s *JobServer) Shutdown() {
	s.server.Shutdown()
}

// Stop forcefully stops the job server.
func (s *JobServer) Stop() {
	s.server.Stop()
}
