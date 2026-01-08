// Package redis provides Redis connection management for the NFS-e API.
// It supports both the Asynq job queue and the rate limiter.
package redis

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// defaultPingTimeout is the default timeout for ping operations.
	defaultPingTimeout = 5 * time.Second

	// defaultPoolSize is the default connection pool size.
	defaultPoolSize = 10

	// defaultMinIdleConns is the default minimum number of idle connections.
	defaultMinIdleConns = 5
)

// Client wraps the Redis client with additional functionality.
type Client struct {
	client *redis.Client
}

// ClientOptions configures the Redis client.
type ClientOptions struct {
	URL          string
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewClient creates a new Redis client and verifies connectivity.
func NewClient(ctx context.Context, opts ClientOptions) (*Client, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("redis: URL is required")
	}

	// Parse the Redis URL
	redisOpts, err := parseRedisURL(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("redis: invalid URL: %w", err)
	}

	// Apply custom options
	if opts.PoolSize > 0 {
		redisOpts.PoolSize = opts.PoolSize
	} else {
		redisOpts.PoolSize = defaultPoolSize
	}

	if opts.MinIdleConns > 0 {
		redisOpts.MinIdleConns = opts.MinIdleConns
	} else {
		redisOpts.MinIdleConns = defaultMinIdleConns
	}

	if opts.DialTimeout > 0 {
		redisOpts.DialTimeout = opts.DialTimeout
	}

	if opts.ReadTimeout > 0 {
		redisOpts.ReadTimeout = opts.ReadTimeout
	}

	if opts.WriteTimeout > 0 {
		redisOpts.WriteTimeout = opts.WriteTimeout
	}

	// Create the client
	client := redis.NewClient(redisOpts)

	// Verify connectivity with a ping
	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}

	return &Client{
		client: client,
	}, nil
}

// parseRedisURL parses a Redis URL and returns options.
// Supports formats like:
//   - redis://localhost:6379
//   - redis://:password@localhost:6379/0
//   - redis://user:password@localhost:6379/0
func parseRedisURL(redisURL string) (*redis.Options, error) {
	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, err
	}

	opts := &redis.Options{
		Network: "tcp",
	}

	// Parse host and port
	if u.Host != "" {
		opts.Addr = u.Host
	} else {
		opts.Addr = "localhost:6379"
	}

	// Parse password
	if u.User != nil {
		password, _ := u.User.Password()
		opts.Password = password
		// Username is optional in Redis 6+
		if username := u.User.Username(); username != "" && username != "default" {
			opts.Username = username
		}
	}

	// Parse database number from path
	if u.Path != "" && u.Path != "/" {
		dbStr := u.Path[1:] // Remove leading slash
		db, err := strconv.Atoi(dbStr)
		if err != nil {
			return nil, fmt.Errorf("invalid database number: %s", dbStr)
		}
		opts.DB = db
	}

	return opts, nil
}

// GetClient returns the underlying Redis client.
// This is used by Asynq and the rate limiter.
func (c *Client) GetClient() *redis.Client {
	return c.client
}

// Ping checks the connection to Redis.
func (c *Client) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()

	if err := c.client.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("redis: ping failed: %w", err)
	}
	return nil
}

// Close gracefully closes the Redis connection.
func (c *Client) Close() error {
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("redis: close failed: %w", err)
	}
	return nil
}

// Set stores a value with optional expiration.
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value by key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

// Del deletes one or more keys.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// AsynqRedisOpt returns connection options compatible with Asynq.
// Asynq expects a specific format for Redis connection.
type AsynqRedisOpt struct {
	Addr     string
	Password string
	DB       int
	Username string
}

// GetAsynqRedisOpt extracts Asynq-compatible options from the URL.
func GetAsynqRedisOpt(redisURL string) (*AsynqRedisOpt, error) {
	opts, err := parseRedisURL(redisURL)
	if err != nil {
		return nil, err
	}

	return &AsynqRedisOpt{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
		Username: opts.Username,
	}, nil
}
