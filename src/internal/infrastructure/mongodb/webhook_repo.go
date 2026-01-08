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
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// webhookDeliveriesCollection is the name of the webhook deliveries collection.
	webhookDeliveriesCollection = "webhook_deliveries"
)

// ErrWebhookDeliveryNotFound is returned when a webhook delivery is not found.
var ErrWebhookDeliveryNotFound = errors.New("webhook delivery not found")

// WebhookDeliveryStatus represents the status of a webhook delivery.
type WebhookDeliveryStatus string

const (
	// WebhookStatusPending indicates the webhook is pending delivery.
	WebhookStatusPending WebhookDeliveryStatus = "pending"

	// WebhookStatusSuccess indicates the webhook was delivered successfully.
	WebhookStatusSuccess WebhookDeliveryStatus = "success"

	// WebhookStatusFailed indicates the webhook delivery failed.
	WebhookStatusFailed WebhookDeliveryStatus = "failed"

	// WebhookStatusRetrying indicates the webhook is being retried.
	WebhookStatusRetrying WebhookDeliveryStatus = "retrying"
)

// WebhookDelivery represents a webhook delivery record in MongoDB.
type WebhookDelivery struct {
	ID        primitive.ObjectID    `bson:"_id,omitempty"`
	RequestID string                `bson:"request_id"`
	APIKeyID  primitive.ObjectID    `bson:"api_key_id"`
	URL       string                `bson:"url"`
	Status    WebhookDeliveryStatus `bson:"status"`
	CreatedAt time.Time             `bson:"created_at"`
	UpdatedAt time.Time             `bson:"updated_at"`

	// Payload is the JSON payload that was/will be sent.
	Payload string `bson:"payload"`

	// Attempts is the number of delivery attempts made.
	Attempts int `bson:"attempts"`

	// LastAttemptAt is when the last delivery attempt was made.
	LastAttemptAt *time.Time `bson:"last_attempt_at,omitempty"`

	// LastStatusCode is the HTTP status code from the last attempt.
	LastStatusCode int `bson:"last_status_code,omitempty"`

	// LastResponse is the response body from the last attempt (truncated).
	LastResponse string `bson:"last_response,omitempty"`

	// LastError is the error message from the last failed attempt.
	LastError string `bson:"last_error,omitempty"`

	// CompletedAt is when the delivery completed (success or final failure).
	CompletedAt *time.Time `bson:"completed_at,omitempty"`

	// Duration is the total time taken for all delivery attempts.
	DurationMs int64 `bson:"duration_ms,omitempty"`
}

// WebhookRepository provides access to webhook delivery data in MongoDB.
type WebhookRepository struct {
	collection *mongo.Collection
}

// NewWebhookRepository creates a new webhook repository.
func NewWebhookRepository(client *Client) *WebhookRepository {
	return &WebhookRepository{
		collection: client.GetCollection(webhookDeliveriesCollection),
	}
}

// Create inserts a new webhook delivery record.
func (r *WebhookRepository) Create(ctx context.Context, delivery *WebhookDelivery) error {
	if delivery == nil {
		return fmt.Errorf("webhook delivery cannot be nil")
	}

	if delivery.RequestID == "" {
		return fmt.Errorf("request ID is required")
	}

	if delivery.URL == "" {
		return fmt.Errorf("URL is required")
	}

	// Set timestamps
	now := time.Now().UTC()
	delivery.CreatedAt = now
	delivery.UpdatedAt = now

	// Set default status
	if delivery.Status == "" {
		delivery.Status = WebhookStatusPending
	}

	result, err := r.collection.InsertOne(ctx, delivery)
	if err != nil {
		return fmt.Errorf("failed to create webhook delivery: %w", err)
	}

	// Set the generated ID
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		delivery.ID = oid
	}

	return nil
}

// FindByID retrieves a webhook delivery by its ID.
func (r *WebhookRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*WebhookDelivery, error) {
	if id.IsZero() {
		return nil, fmt.Errorf("webhook delivery ID cannot be empty")
	}

	filter := bson.M{"_id": id}

	var delivery WebhookDelivery
	err := r.collection.FindOne(ctx, filter).Decode(&delivery)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrWebhookDeliveryNotFound
		}
		return nil, fmt.Errorf("failed to find webhook delivery: %w", err)
	}

	return &delivery, nil
}

// UpdateStatus updates the status of a webhook delivery.
func (r *WebhookRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status WebhookDeliveryStatus, statusCode int, response string) error {
	if id.IsZero() {
		return fmt.Errorf("webhook delivery ID cannot be empty")
	}

	now := time.Now().UTC()
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":           status,
			"last_status_code": statusCode,
			"last_response":    truncateString(response, 10000),
			"last_attempt_at":  now,
			"updated_at":       now,
		},
		"$inc": bson.M{
			"attempts": 1,
		},
	}

	// Mark as completed if success or failed
	if status == WebhookStatusSuccess || status == WebhookStatusFailed {
		update["$set"].(bson.M)["completed_at"] = now
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update webhook delivery status: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrWebhookDeliveryNotFound
	}

	return nil
}

// UpdateError updates a webhook delivery with an error.
func (r *WebhookRepository) UpdateError(ctx context.Context, id primitive.ObjectID, lastError string) error {
	if id.IsZero() {
		return fmt.Errorf("webhook delivery ID cannot be empty")
	}

	now := time.Now().UTC()
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":          WebhookStatusRetrying,
			"last_error":      truncateString(lastError, 1000),
			"last_attempt_at": now,
			"updated_at":      now,
		},
		"$inc": bson.M{
			"attempts": 1,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update webhook delivery error: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrWebhookDeliveryNotFound
	}

	return nil
}

// MarkFailed marks a webhook delivery as permanently failed.
func (r *WebhookRepository) MarkFailed(ctx context.Context, id primitive.ObjectID, lastError string, durationMs int64) error {
	if id.IsZero() {
		return fmt.Errorf("webhook delivery ID cannot be empty")
	}

	now := time.Now().UTC()
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":       WebhookStatusFailed,
			"last_error":   truncateString(lastError, 1000),
			"completed_at": now,
			"updated_at":   now,
			"duration_ms":  durationMs,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to mark webhook delivery as failed: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrWebhookDeliveryNotFound
	}

	return nil
}

// MarkSuccess marks a webhook delivery as successful.
func (r *WebhookRepository) MarkSuccess(ctx context.Context, id primitive.ObjectID, statusCode int, response string, durationMs int64) error {
	if id.IsZero() {
		return fmt.Errorf("webhook delivery ID cannot be empty")
	}

	now := time.Now().UTC()
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":           WebhookStatusSuccess,
			"last_status_code": statusCode,
			"last_response":    truncateString(response, 10000),
			"completed_at":     now,
			"updated_at":       now,
			"duration_ms":      durationMs,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to mark webhook delivery as success: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrWebhookDeliveryNotFound
	}

	return nil
}

// FindByRequestID retrieves all webhook deliveries for a request.
func (r *WebhookRepository) FindByRequestID(ctx context.Context, requestID string) ([]*WebhookDelivery, error) {
	if requestID == "" {
		return nil, fmt.Errorf("request ID cannot be empty")
	}

	filter := bson.M{"request_id": requestID}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find webhook deliveries: %w", err)
	}
	defer cursor.Close(ctx)

	var deliveries []*WebhookDelivery
	if err := cursor.All(ctx, &deliveries); err != nil {
		return nil, fmt.Errorf("failed to decode webhook deliveries: %w", err)
	}

	return deliveries, nil
}

// FindPendingDeliveries retrieves pending webhook deliveries for processing.
func (r *WebhookRepository) FindPendingDeliveries(ctx context.Context, limit int64) ([]*WebhookDelivery, error) {
	if limit < 1 {
		limit = 10
	}

	filter := bson.M{
		"status": bson.M{
			"$in": []WebhookDeliveryStatus{WebhookStatusPending, WebhookStatusRetrying},
		},
	}

	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find pending deliveries: %w", err)
	}
	defer cursor.Close(ctx)

	var deliveries []*WebhookDelivery
	if err := cursor.All(ctx, &deliveries); err != nil {
		return nil, fmt.Errorf("failed to decode pending deliveries: %w", err)
	}

	return deliveries, nil
}

// EnsureIndexes creates the necessary indexes for the webhook deliveries collection.
func (r *WebhookRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "request_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "api_key_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: 1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

// truncateString truncates a string to a maximum length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
