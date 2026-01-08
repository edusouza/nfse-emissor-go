// Package jobs provides background job definitions and handlers for the NFS-e API.
package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// Task type constants.
const (
	// TypeEmissionProcess is the task type for processing NFS-e emissions.
	TypeEmissionProcess = "emission:process"
)

// EmissionTaskPayload contains the data needed to process an emission.
type EmissionTaskPayload struct {
	// RequestID is the unique identifier of the emission request.
	RequestID string `json:"request_id"`
}

// NewEmissionTask creates a new emission processing task.
func NewEmissionTask(requestID string) (*asynq.Task, error) {
	if requestID == "" {
		return nil, fmt.Errorf("request ID is required")
	}

	payload := EmissionTaskPayload{
		RequestID: requestID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal emission task payload: %w", err)
	}

	return asynq.NewTask(TypeEmissionProcess, data), nil
}

// ParseEmissionTask parses an emission task and returns its payload.
func ParseEmissionTask(task *asynq.Task) (*EmissionTaskPayload, error) {
	if task == nil {
		return nil, fmt.Errorf("task is nil")
	}

	if task.Type() != TypeEmissionProcess {
		return nil, fmt.Errorf("unexpected task type: %s (expected %s)", task.Type(), TypeEmissionProcess)
	}

	var payload EmissionTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal emission task payload: %w", err)
	}

	if payload.RequestID == "" {
		return nil, fmt.Errorf("task payload is missing request_id")
	}

	return &payload, nil
}

// WebhookTaskPayload contains the data needed to deliver a webhook.
type WebhookTaskPayload struct {
	// DeliveryID is the unique identifier of the webhook delivery.
	DeliveryID string `json:"delivery_id"`

	// RequestID is the emission request ID (for correlation).
	RequestID string `json:"request_id"`
}

// TypeWebhookDelivery is the task type for webhook delivery.
const TypeWebhookDelivery = "webhook:delivery"

// NewWebhookTask creates a new webhook delivery task.
func NewWebhookTask(deliveryID, requestID string) (*asynq.Task, error) {
	if deliveryID == "" {
		return nil, fmt.Errorf("delivery ID is required")
	}

	payload := WebhookTaskPayload{
		DeliveryID: deliveryID,
		RequestID:  requestID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook task payload: %w", err)
	}

	return asynq.NewTask(TypeWebhookDelivery, data), nil
}

// ParseWebhookTask parses a webhook task and returns its payload.
func ParseWebhookTask(task *asynq.Task) (*WebhookTaskPayload, error) {
	if task == nil {
		return nil, fmt.Errorf("task is nil")
	}

	if task.Type() != TypeWebhookDelivery {
		return nil, fmt.Errorf("unexpected task type: %s (expected %s)", task.Type(), TypeWebhookDelivery)
	}

	var payload WebhookTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal webhook task payload: %w", err)
	}

	if payload.DeliveryID == "" {
		return nil, fmt.Errorf("task payload is missing delivery_id")
	}

	return &payload, nil
}
