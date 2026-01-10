// Package middleware provides HTTP middleware for the NFS-e API.
package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger format types.
const (
	LogFormatJSON = "json"
	LogFormatText = "text"
)

// Log levels.
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// LogEntry represents a structured log entry.
type LogEntry struct {
	Timestamp   string `json:"timestamp"`
	Level       string `json:"level"`
	RequestID   string `json:"request_id,omitempty"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Status      int    `json:"status"`
	Latency     string `json:"latency"`
	LatencyMs   int64  `json:"latency_ms"`
	ClientIP    string `json:"client_ip"`
	UserAgent   string `json:"user_agent,omitempty"`
	Error       string `json:"error,omitempty"`
	APIKeyID    string `json:"api_key_id,omitempty"`
	BodySize    int    `json:"body_size,omitempty"`
}

// LoggingMiddleware provides structured request logging.
type LoggingMiddleware struct {
	logger *log.Logger
	format string
	level  string
	output io.Writer
}

// LoggingConfig configures the logging middleware.
type LoggingConfig struct {
	Format string    // "json" or "text"
	Level  string    // "debug", "info", "warn", "error"
	Output io.Writer // Output destination (default: os.Stdout)
}

// EnhancedLoggingConfig controls detailed request/response logging.
type EnhancedLoggingConfig struct {
	// LogRequestBody enables logging of request bodies.
	LogRequestBody bool

	// LogResponseBody enables logging of response bodies (only for errors by default).
	LogResponseBody bool

	// LogResponseBodyOnError logs response body only when status >= 400.
	LogResponseBodyOnError bool

	// MaxBodySize is the maximum bytes to log for bodies (default 4096).
	MaxBodySize int

	// SensitiveFields are field names to mask in logged bodies.
	SensitiveFields []string

	// SensitiveHeaders are header names to mask in logs.
	SensitiveHeaders []string

	// Format is the log output format ("json" or "text").
	Format string

	// Level is the minimum log level.
	Level string

	// Output is the destination for log output.
	Output io.Writer
}

// DefaultEnhancedLoggingConfig returns sensible defaults for enhanced logging.
func DefaultEnhancedLoggingConfig() EnhancedLoggingConfig {
	return EnhancedLoggingConfig{
		LogRequestBody:         true,
		LogResponseBody:        false,
		LogResponseBodyOnError: true,
		MaxBodySize:            4096,
		SensitiveFields: []string{
			"password",
			"senha",
			"secret",
			"segredo",
			"certificate",
			"certificado",
			"pfx_base64",
			"pfx",
			"private_key",
			"chave_privada",
			"api_key",
			"apikey",
			"token",
			"authorization",
			"auth",
			"credential",
			"credencial",
		},
		SensitiveHeaders: []string{
			"Authorization",
			"X-API-Key",
			"X-Auth-Token",
			"Cookie",
			"Set-Cookie",
		},
		Format: LogFormatJSON,
		Level:  LogLevelInfo,
		Output: os.Stdout,
	}
}

// EnhancedLogEntry represents a detailed log entry with body information.
type EnhancedLogEntry struct {
	Timestamp    string            `json:"timestamp"`
	Level        string            `json:"level"`
	RequestID    string            `json:"request_id,omitempty"`
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Query        string            `json:"query,omitempty"`
	Status       int               `json:"status"`
	Latency      string            `json:"latency"`
	LatencyMs    int64             `json:"latency_ms"`
	ClientIP     string            `json:"client_ip"`
	UserAgent    string            `json:"user_agent,omitempty"`
	APIKeyID     string            `json:"api_key_id,omitempty"`
	RequestSize  int               `json:"request_size,omitempty"`
	ResponseSize int               `json:"response_size,omitempty"`
	RequestBody  string            `json:"request_body,omitempty"`
	ResponseBody string            `json:"response_body,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// EnhancedLoggingMiddleware provides detailed request/response logging.
type EnhancedLoggingMiddleware struct {
	logger              *log.Logger
	config              EnhancedLoggingConfig
	sensitiveFieldRegex *regexp.Regexp
}

// NewEnhancedLoggingMiddleware creates a new enhanced logging middleware.
func NewEnhancedLoggingMiddleware(config EnhancedLoggingConfig) *EnhancedLoggingMiddleware {
	if config.MaxBodySize <= 0 {
		config.MaxBodySize = 4096
	}
	if config.Format == "" {
		config.Format = LogFormatJSON
	}
	if config.Level == "" {
		config.Level = LogLevelInfo
	}
	if config.Output == nil {
		config.Output = os.Stdout
	}
	if len(config.SensitiveFields) == 0 {
		config.SensitiveFields = DefaultEnhancedLoggingConfig().SensitiveFields
	}
	if len(config.SensitiveHeaders) == 0 {
		config.SensitiveHeaders = DefaultEnhancedLoggingConfig().SensitiveHeaders
	}

	// Build regex pattern for sensitive fields
	var pattern string
	if len(config.SensitiveFields) > 0 {
		escapedFields := make([]string, len(config.SensitiveFields))
		for i, field := range config.SensitiveFields {
			escapedFields[i] = regexp.QuoteMeta(field)
		}
		// Match JSON key-value pairs: "field": "value" or "field": value
		pattern = fmt.Sprintf(`(?i)"(%s)"\s*:\s*("[^"]*"|[^,}\]]+)`, strings.Join(escapedFields, "|"))
	}

	var regex *regexp.Regexp
	if pattern != "" {
		regex = regexp.MustCompile(pattern)
	}

	return &EnhancedLoggingMiddleware{
		logger:              log.New(config.Output, "", 0),
		config:              config,
		sensitiveFieldRegex: regex,
	}
}

// EnhancedLogger returns a Gin middleware handler that provides detailed logging.
func (m *EnhancedLoggingMiddleware) EnhancedLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Capture request body if enabled
		var requestBody string
		if m.config.LogRequestBody && c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				// Restore the body for subsequent handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				// Truncate and mask sensitive data
				requestBody = m.processBody(bodyBytes)
			}
		}

		// Create response writer wrapper to capture response body
		var responseBody string
		var responseWriter *responseBodyWriter
		if m.config.LogResponseBody || m.config.LogResponseBodyOnError {
			responseWriter = &responseBodyWriter{
				ResponseWriter: c.Writer,
				body:           &bytes.Buffer{},
				maxSize:        m.config.MaxBodySize,
			}
			c.Writer = responseWriter
		}

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		status := c.Writer.Status()

		// Capture response body if needed
		shouldLogResponseBody := m.config.LogResponseBody ||
			(m.config.LogResponseBodyOnError && status >= 400)
		if shouldLogResponseBody && responseWriter != nil {
			responseBody = m.processBody(responseWriter.body.Bytes())
		}

		// Build log entry
		entry := EnhancedLogEntry{
			Timestamp:    start.UTC().Format(time.RFC3339),
			Level:        m.getLogLevel(status),
			RequestID:    GetRequestIDFromContext(c),
			Method:       c.Request.Method,
			Path:         c.Request.URL.Path,
			Query:        c.Request.URL.RawQuery,
			Status:       status,
			Latency:      latency.String(),
			LatencyMs:    latency.Milliseconds(),
			ClientIP:     c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			RequestSize:  int(c.Request.ContentLength),
			ResponseSize: c.Writer.Size(),
			RequestBody:  requestBody,
			ResponseBody: responseBody,
		}

		// Add API key ID if present
		apiKey := GetAPIKeyFromContext(c)
		if apiKey != nil {
			entry.APIKeyID = apiKey.KeyPrefix
		}

		// Add sanitized headers
		entry.Headers = m.sanitizeHeaders(c.Request.Header)

		// Add error if present
		if len(c.Errors) > 0 {
			entry.Error = c.Errors.String()
		}

		// Log based on format
		m.log(entry)
	}
}

// processBody truncates and masks sensitive data in body content.
func (m *EnhancedLoggingMiddleware) processBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// Truncate if necessary
	content := string(body)
	if len(content) > m.config.MaxBodySize {
		content = content[:m.config.MaxBodySize] + "... [TRUNCATED]"
	}

	// Mask sensitive fields
	if m.sensitiveFieldRegex != nil {
		content = m.sensitiveFieldRegex.ReplaceAllStringFunc(content, func(match string) string {
			// Extract the field name and replace value with [REDACTED]
			parts := strings.SplitN(match, ":", 2)
			if len(parts) == 2 {
				return parts[0] + ": \"[REDACTED]\""
			}
			return match
		})
	}

	return content
}

// sanitizeHeaders returns headers with sensitive values masked.
func (m *EnhancedLoggingMiddleware) sanitizeHeaders(headers map[string][]string) map[string]string {
	result := make(map[string]string)

	sensitiveSet := make(map[string]bool)
	for _, h := range m.config.SensitiveHeaders {
		sensitiveSet[strings.ToLower(h)] = true
	}

	for key, values := range headers {
		if sensitiveSet[strings.ToLower(key)] {
			result[key] = "[REDACTED]"
		} else if len(values) > 0 {
			result[key] = values[0]
			if len(values) > 1 {
				result[key] += fmt.Sprintf(" (+%d more)", len(values)-1)
			}
		}
	}

	return result
}

// getLogLevel determines the log level based on status code.
func (m *EnhancedLoggingMiddleware) getLogLevel(status int) string {
	switch {
	case status >= 500:
		return LogLevelError
	case status >= 400:
		return LogLevelWarn
	default:
		return LogLevelInfo
	}
}

// log outputs the enhanced log entry.
func (m *EnhancedLoggingMiddleware) log(entry EnhancedLogEntry) {
	switch m.config.Format {
	case LogFormatJSON:
		m.logJSON(entry)
	default:
		m.logText(entry)
	}
}

// logJSON outputs the enhanced log entry as JSON.
func (m *EnhancedLoggingMiddleware) logJSON(entry EnhancedLogEntry) {
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		m.logger.Printf(`{"level":"error","message":"failed to marshal enhanced log entry: %v"}`, err)
		return
	}
	m.logger.Println(string(jsonBytes))
}

// logText outputs the enhanced log entry as human-readable text.
func (m *EnhancedLoggingMiddleware) logText(entry EnhancedLogEntry) {
	var reqID string
	if entry.RequestID != "" {
		reqID = fmt.Sprintf("[%s] ", entry.RequestID)
	}

	m.logger.Printf("%s %s%s %s %d %s (req: %d bytes, resp: %d bytes)",
		entry.Timestamp,
		reqID,
		entry.Method,
		entry.Path,
		entry.Status,
		entry.Latency,
		entry.RequestSize,
		entry.ResponseSize,
	)

	// Log request body if present (debug level)
	if entry.RequestBody != "" && m.config.Level == LogLevelDebug {
		m.logger.Printf("  Request body: %s", entry.RequestBody)
	}

	// Log response body if present (for errors or debug)
	if entry.ResponseBody != "" {
		m.logger.Printf("  Response body: %s", entry.ResponseBody)
	}
}

// responseBodyWriter wraps gin.ResponseWriter to capture the response body.
type responseBodyWriter struct {
	gin.ResponseWriter
	body    *bytes.Buffer
	maxSize int
}

// Write captures the response body up to maxSize.
func (w *responseBodyWriter) Write(b []byte) (int, error) {
	// Capture up to maxSize bytes
	if w.body.Len() < w.maxSize {
		remaining := w.maxSize - w.body.Len()
		if len(b) > remaining {
			w.body.Write(b[:remaining])
		} else {
			w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

// WriteString captures the response body string.
func (w *responseBodyWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// NewLoggingMiddleware creates a new logging middleware.
func NewLoggingMiddleware(config LoggingConfig) *LoggingMiddleware {
	if config.Format == "" {
		config.Format = LogFormatJSON
	}
	if config.Level == "" {
		config.Level = LogLevelInfo
	}
	if config.Output == nil {
		config.Output = os.Stdout
	}

	return &LoggingMiddleware{
		logger: log.New(config.Output, "", 0),
		format: config.Format,
		level:  config.Level,
		output: config.Output,
	}
}

// Logger returns a Gin middleware handler that logs requests.
func (m *LoggingMiddleware) Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Get request path
		path := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			path = path + "?" + c.Request.URL.RawQuery
		}

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Build log entry
		entry := LogEntry{
			Timestamp: start.UTC().Format(time.RFC3339),
			Level:     m.getLogLevel(c.Writer.Status()),
			RequestID: GetRequestIDFromContext(c),
			Method:    c.Request.Method,
			Path:      path,
			Status:    c.Writer.Status(),
			Latency:   latency.String(),
			LatencyMs: latency.Milliseconds(),
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			BodySize:  c.Writer.Size(),
		}

		// Add API key ID if present
		apiKey := GetAPIKeyFromContext(c)
		if apiKey != nil {
			entry.APIKeyID = apiKey.KeyPrefix
		}

		// Add error if present
		if len(c.Errors) > 0 {
			entry.Error = c.Errors.String()
		}

		// Log based on format
		m.log(entry)
	}
}

// getLogLevel determines the log level based on status code.
func (m *LoggingMiddleware) getLogLevel(status int) string {
	switch {
	case status >= 500:
		return LogLevelError
	case status >= 400:
		return LogLevelWarn
	default:
		return LogLevelInfo
	}
}

// log outputs the log entry in the configured format.
func (m *LoggingMiddleware) log(entry LogEntry) {
	switch m.format {
	case LogFormatJSON:
		m.logJSON(entry)
	default:
		m.logText(entry)
	}
}

// logJSON outputs the log entry as JSON.
func (m *LoggingMiddleware) logJSON(entry LogEntry) {
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		m.logger.Printf(`{"level":"error","message":"failed to marshal log entry: %v"}`, err)
		return
	}
	m.logger.Println(string(jsonBytes))
}

// logText outputs the log entry as human-readable text.
func (m *LoggingMiddleware) logText(entry LogEntry) {
	var reqID string
	if entry.RequestID != "" {
		reqID = fmt.Sprintf("[%s] ", entry.RequestID)
	}

	m.logger.Printf("%s %s%s %s %d %s",
		entry.Timestamp,
		reqID,
		entry.Method,
		entry.Path,
		entry.Status,
		entry.Latency,
	)
}

// RecoveryWithLogging returns a Gin middleware that recovers from panics and logs them.
func RecoveryWithLogging(format string) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				entry := map[string]interface{}{
					"timestamp":  time.Now().UTC().Format(time.RFC3339),
					"level":      "error",
					"request_id": GetRequestIDFromContext(c),
					"method":     c.Request.Method,
					"path":       c.Request.URL.Path,
					"client_ip":  c.ClientIP(),
					"error":      fmt.Sprintf("panic recovered: %v", err),
				}

				if format == LogFormatJSON {
					jsonBytes, _ := json.Marshal(entry)
					log.Println(string(jsonBytes))
				} else {
					log.Printf("[PANIC] %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
				}

				// Return 500 error
				c.AbortWithStatusJSON(500, gin.H{
					"type":   "https://api.nfse.gov.br/problems/internal-error",
					"title":  "Internal Server Error",
					"status": 500,
					"detail": "An unexpected error occurred",
				})
			}
		}()

		c.Next()
	}
}

// SkipPaths configures paths to skip logging (e.g., health checks).
type SkipPaths struct {
	paths map[string]bool
}

// NewSkipPaths creates a new SkipPaths configuration.
func NewSkipPaths(paths ...string) *SkipPaths {
	sp := &SkipPaths{
		paths: make(map[string]bool),
	}
	for _, p := range paths {
		sp.paths[p] = true
	}
	return sp
}

// ShouldSkip returns true if the path should be skipped.
func (sp *SkipPaths) ShouldSkip(path string) bool {
	return sp.paths[path]
}

// LoggerWithSkip returns a logging middleware that skips certain paths.
func (m *LoggingMiddleware) LoggerWithSkip(skip *SkipPaths) gin.HandlerFunc {
	return func(c *gin.Context) {
		if skip != nil && skip.ShouldSkip(c.Request.URL.Path) {
			c.Next()
			return
		}

		m.Logger()(c)
	}
}
