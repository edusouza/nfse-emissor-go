// Package mongodb provides MongoDB connection management for the NFS-e API.
// It implements connection pooling, health checks, and graceful shutdown.
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	// defaultConnectTimeout is the default timeout for establishing a connection.
	defaultConnectTimeout = 10 * time.Second

	// defaultPingTimeout is the default timeout for ping operations.
	defaultPingTimeout = 5 * time.Second

	// defaultMaxPoolSize is the default maximum number of connections in the pool.
	defaultMaxPoolSize = 100

	// defaultMinPoolSize is the default minimum number of connections in the pool.
	defaultMinPoolSize = 10

	// defaultMaxIdleTime is the default maximum time a connection can remain idle.
	defaultMaxIdleTime = 30 * time.Second
)

// Client wraps the MongoDB client with additional functionality.
type Client struct {
	client       *mongo.Client
	databaseName string
}

// ClientOptions configures the MongoDB client.
type ClientOptions struct {
	URI            string
	DatabaseName   string
	ConnectTimeout time.Duration
	MaxPoolSize    uint64
	MinPoolSize    uint64
	MaxIdleTime    time.Duration
}

// NewClient creates a new MongoDB client with connection pooling.
// It establishes a connection and verifies connectivity with a ping.
func NewClient(ctx context.Context, opts ClientOptions) (*Client, error) {
	if opts.URI == "" {
		return nil, fmt.Errorf("mongodb: URI is required")
	}
	if opts.DatabaseName == "" {
		return nil, fmt.Errorf("mongodb: database name is required")
	}

	// Apply defaults
	if opts.ConnectTimeout == 0 {
		opts.ConnectTimeout = defaultConnectTimeout
	}
	if opts.MaxPoolSize == 0 {
		opts.MaxPoolSize = defaultMaxPoolSize
	}
	if opts.MinPoolSize == 0 {
		opts.MinPoolSize = defaultMinPoolSize
	}
	if opts.MaxIdleTime == 0 {
		opts.MaxIdleTime = defaultMaxIdleTime
	}

	// Configure client options
	clientOpts := options.Client().
		ApplyURI(opts.URI).
		SetMaxPoolSize(opts.MaxPoolSize).
		SetMinPoolSize(opts.MinPoolSize).
		SetMaxConnIdleTime(opts.MaxIdleTime).
		SetServerSelectionTimeout(opts.ConnectTimeout)

	// Create connection context with timeout
	connectCtx, cancel := context.WithTimeout(ctx, opts.ConnectTimeout)
	defer cancel()

	// Connect to MongoDB
	client, err := mongo.Connect(connectCtx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("mongodb: failed to connect: %w", err)
	}

	// Verify connectivity with a ping
	pingCtx, pingCancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer pingCancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		// Attempt to disconnect on ping failure
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongodb: ping failed: %w", err)
	}

	return &Client{
		client:       client,
		databaseName: opts.DatabaseName,
	}, nil
}

// GetDatabase returns the configured database.
func (c *Client) GetDatabase() *mongo.Database {
	return c.client.Database(c.databaseName)
}

// GetCollection returns a collection from the configured database.
func (c *Client) GetCollection(name string) *mongo.Collection {
	return c.GetDatabase().Collection(name)
}

// Ping checks the connection to MongoDB.
func (c *Client) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()

	if err := c.client.Ping(pingCtx, readpref.Primary()); err != nil {
		return fmt.Errorf("mongodb: ping failed: %w", err)
	}
	return nil
}

// Disconnect gracefully closes the MongoDB connection.
func (c *Client) Disconnect(ctx context.Context) error {
	if err := c.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("mongodb: disconnect failed: %w", err)
	}
	return nil
}

// Client returns the underlying mongo.Client for advanced operations.
func (c *Client) Client() *mongo.Client {
	return c.client
}

// DatabaseName returns the configured database name.
func (c *Client) DatabaseName() string {
	return c.databaseName
}
