// Package webhook provides webhook delivery functionality for the NFS-e API.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Sender handles webhook delivery with retry logic.
type Sender struct {
	client       *http.Client
	maxRetries   int
	timeout      time.Duration
	retryDelays  []time.Duration
}

// SenderConfig configures the webhook sender.
type SenderConfig struct {
	// Timeout is the timeout for each webhook request.
	Timeout time.Duration

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// RetryDelays specifies the delay between retry attempts.
	// If not provided, exponential backoff is used.
	RetryDelays []time.Duration
}

// NewSender creates a new webhook sender.
func NewSender(config SenderConfig) *Sender {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if len(config.RetryDelays) == 0 {
		// Exponential backoff: 1s, 2s, 4s
		config.RetryDelays = []time.Duration{
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
		}
	}

	client := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  true,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 5,
		},
	}

	return &Sender{
		client:      client,
		maxRetries:  config.MaxRetries,
		timeout:     config.Timeout,
		retryDelays: config.RetryDelays,
	}
}

// SendResult contains the result of a webhook delivery attempt.
type SendResult struct {
	// Success indicates whether the delivery was successful.
	Success bool

	// StatusCode is the HTTP status code received (if any).
	StatusCode int

	// ResponseBody is the response body (truncated if large).
	ResponseBody string

	// Attempts is the number of delivery attempts made.
	Attempts int

	// Duration is the total time taken for all attempts.
	Duration time.Duration

	// Error contains the error message if delivery failed.
	Error string
}

// Send delivers a webhook payload to the specified URL.
// It includes HMAC-SHA256 signature in the X-Webhook-Signature header.
func (s *Sender) Send(ctx context.Context, url string, payload interface{}, secret string, requestID string) (*SendResult, error) {
	start := time.Now()

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return &SendResult{
			Success:  false,
			Attempts: 0,
			Duration: time.Since(start),
			Error:    fmt.Sprintf("failed to marshal payload: %v", err),
		}, err
	}

	// Generate HMAC signature
	signature := generateSignature(jsonData, secret)

	var lastErr error
	var lastStatusCode int
	var lastResponseBody string

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		// Wait before retry (not on first attempt)
		if attempt > 0 {
			delay := s.getRetryDelay(attempt - 1)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return &SendResult{
					Success:      false,
					StatusCode:   lastStatusCode,
					ResponseBody: lastResponseBody,
					Attempts:     attempt,
					Duration:     time.Since(start),
					Error:        fmt.Sprintf("context cancelled during retry: %v", ctx.Err()),
				}, ctx.Err()
			}
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Signature", signature)
		req.Header.Set("X-Request-ID", requestID)
		req.Header.Set("User-Agent", "NFS-e-Nacional-Webhook/1.0")

		// Send request
		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		// Read response body (limit to 10KB)
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
		resp.Body.Close()

		lastStatusCode = resp.StatusCode
		lastResponseBody = string(body)

		// Check for success (2xx status codes)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &SendResult{
				Success:      true,
				StatusCode:   resp.StatusCode,
				ResponseBody: lastResponseBody,
				Attempts:     attempt + 1,
				Duration:     time.Since(start),
			}, nil
		}

		// Check for non-retryable errors (4xx except 429)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return &SendResult{
				Success:      false,
				StatusCode:   resp.StatusCode,
				ResponseBody: lastResponseBody,
				Attempts:     attempt + 1,
				Duration:     time.Since(start),
				Error:        fmt.Sprintf("non-retryable client error: HTTP %d", resp.StatusCode),
			}, fmt.Errorf("non-retryable client error: HTTP %d", resp.StatusCode)
		}

		lastErr = fmt.Errorf("server error: HTTP %d", resp.StatusCode)
	}

	return &SendResult{
		Success:      false,
		StatusCode:   lastStatusCode,
		ResponseBody: lastResponseBody,
		Attempts:     s.maxRetries + 1,
		Duration:     time.Since(start),
		Error:        fmt.Sprintf("all retry attempts failed: %v", lastErr),
	}, lastErr
}

// getRetryDelay returns the delay for a given retry attempt.
func (s *Sender) getRetryDelay(attempt int) time.Duration {
	if attempt < len(s.retryDelays) {
		return s.retryDelays[attempt]
	}
	// Use the last delay for any additional attempts
	return s.retryDelays[len(s.retryDelays)-1]
}

// generateSignature creates an HMAC-SHA256 signature for the payload.
func generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// VerifySignature verifies an HMAC-SHA256 signature.
func VerifySignature(payload []byte, signature string, secret string) bool {
	expectedSig := generateSignature(payload, secret)
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}
