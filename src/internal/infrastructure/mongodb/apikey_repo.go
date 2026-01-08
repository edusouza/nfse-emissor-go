// Package mongodb provides MongoDB repository implementations for the NFS-e API.
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	// apiKeysCollection is the name of the API keys collection.
	apiKeysCollection = "api_keys"
)

// ErrAPIKeyNotFound is returned when an API key is not found.
var ErrAPIKeyNotFound = errors.New("api key not found")

// RateLimitConfig defines rate limiting parameters for an API key.
type RateLimitConfig struct {
	RequestsPerMinute int `bson:"requests_per_minute" json:"requests_per_minute"`
	Burst             int `bson:"burst" json:"burst"`
}

// APIKey represents an API key for authenticating integrators.
type APIKey struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	KeyHash        string             `bson:"key_hash" json:"-"`                 // SHA-256 hash of the key (never expose)
	KeyPrefix      string             `bson:"key_prefix" json:"key_prefix"`      // First 8 chars for identification
	IntegratorName string             `bson:"integrator_name" json:"integrator_name"`
	WebhookURL     string             `bson:"webhook_url" json:"webhook_url"`
	WebhookSecret  string             `bson:"webhook_secret" json:"-"`           // Secret for webhook signatures (never expose)
	Environment    string             `bson:"environment" json:"environment"`    // production or homologation
	RateLimit      RateLimitConfig    `bson:"rate_limit" json:"rate_limit"`
	Active         bool               `bson:"active" json:"active"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at"`
}

// APIKeyRepository provides access to API key data in MongoDB.
type APIKeyRepository struct {
	collection *mongo.Collection
}

// NewAPIKeyRepository creates a new API key repository.
func NewAPIKeyRepository(client *Client) *APIKeyRepository {
	return &APIKeyRepository{
		collection: client.GetCollection(apiKeysCollection),
	}
}

// FindByKeyHash retrieves an API key by its hash.
// Returns ErrAPIKeyNotFound if the key does not exist.
func (r *APIKeyRepository) FindByKeyHash(ctx context.Context, keyHash string) (*APIKey, error) {
	if keyHash == "" {
		return nil, fmt.Errorf("api key hash cannot be empty")
	}

	filter := bson.M{"key_hash": keyHash}

	var apiKey APIKey
	err := r.collection.FindOne(ctx, filter).Decode(&apiKey)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to find API key: %w", err)
	}

	return &apiKey, nil
}

// FindByPrefix retrieves an API key by its prefix.
// This is useful for debugging and identifying keys without exposing the full hash.
func (r *APIKeyRepository) FindByPrefix(ctx context.Context, prefix string) (*APIKey, error) {
	if prefix == "" {
		return nil, fmt.Errorf("api key prefix cannot be empty")
	}

	filter := bson.M{"key_prefix": prefix}

	var apiKey APIKey
	err := r.collection.FindOne(ctx, filter).Decode(&apiKey)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to find API key by prefix: %w", err)
	}

	return &apiKey, nil
}

// Create inserts a new API key into the database.
func (r *APIKeyRepository) Create(ctx context.Context, apiKey *APIKey) error {
	if apiKey == nil {
		return fmt.Errorf("api key cannot be nil")
	}

	if apiKey.KeyHash == "" {
		return fmt.Errorf("api key hash is required")
	}

	if apiKey.KeyPrefix == "" {
		return fmt.Errorf("api key prefix is required")
	}

	// Set timestamps
	now := time.Now().UTC()
	apiKey.CreatedAt = now
	apiKey.UpdatedAt = now

	// Set defaults if not provided
	if apiKey.Environment == "" {
		apiKey.Environment = "homologation"
	}

	if apiKey.RateLimit.RequestsPerMinute == 0 {
		apiKey.RateLimit.RequestsPerMinute = 100
	}

	if apiKey.RateLimit.Burst == 0 {
		apiKey.RateLimit.Burst = 20
	}

	result, err := r.collection.InsertOne(ctx, apiKey)
	if err != nil {
		// Check for duplicate key error
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("api key already exists")
		}
		return fmt.Errorf("failed to create API key: %w", err)
	}

	// Set the generated ID
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		apiKey.ID = oid
	}

	return nil
}

// Update modifies an existing API key.
func (r *APIKeyRepository) Update(ctx context.Context, apiKey *APIKey) error {
	if apiKey == nil {
		return fmt.Errorf("api key cannot be nil")
	}

	if apiKey.ID.IsZero() {
		return fmt.Errorf("api key ID is required for update")
	}

	// Update timestamp
	apiKey.UpdatedAt = time.Now().UTC()

	filter := bson.M{"_id": apiKey.ID}
	update := bson.M{
		"$set": bson.M{
			"integrator_name": apiKey.IntegratorName,
			"webhook_url":     apiKey.WebhookURL,
			"webhook_secret":  apiKey.WebhookSecret,
			"environment":     apiKey.Environment,
			"rate_limit":      apiKey.RateLimit,
			"active":          apiKey.Active,
			"updated_at":      apiKey.UpdatedAt,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrAPIKeyNotFound
	}

	return nil
}

// Delete removes an API key from the database.
func (r *APIKeyRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	if id.IsZero() {
		return fmt.Errorf("api key ID is required for deletion")
	}

	filter := bson.M{"_id": id}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	if result.DeletedCount == 0 {
		return ErrAPIKeyNotFound
	}

	return nil
}

// SetActive updates the active status of an API key.
func (r *APIKeyRepository) SetActive(ctx context.Context, id primitive.ObjectID, active bool) error {
	if id.IsZero() {
		return fmt.Errorf("api key ID is required")
	}

	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"active":     active,
			"updated_at": time.Now().UTC(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update API key status: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrAPIKeyNotFound
	}

	return nil
}

// ListActive returns all active API keys.
func (r *APIKeyRepository) ListActive(ctx context.Context) ([]*APIKey, error) {
	filter := bson.M{"active": true}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list active API keys: %w", err)
	}
	defer cursor.Close(ctx)

	var apiKeys []*APIKey
	if err := cursor.All(ctx, &apiKeys); err != nil {
		return nil, fmt.Errorf("failed to decode API keys: %w", err)
	}

	return apiKeys, nil
}

// EnsureIndexes creates the necessary indexes for the API keys collection.
// This should be called during application startup.
func (r *APIKeyRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "key_hash", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "key_prefix", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "active", Value: 1}},
		},
		{
			Keys: bson.D{
				{Key: "environment", Value: 1},
				{Key: "active", Value: 1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}
