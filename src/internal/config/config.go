// Package config provides configuration loading and management for the NFS-e API.
// It loads environment variables with sensible defaults for development environments.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration values.
type Config struct {
	// Server configuration
	Port string
	Env  string

	// MongoDB configuration
	MongoDBURI      string
	MongoDBDatabase string

	// Redis configuration
	RedisURL string

	// SEFIN (Government API) configuration
	SEFINApiURL     string
	SEFINEnvironment string
	SEFINTimeout    time.Duration

	// Logging configuration
	LogLevel  string
	LogFormat string

	// Worker configuration
	WorkerConcurrency int
	WorkerMaxRetries  int

	// Rate limiting configuration
	RateLimitDefaultRPM int
	RateLimitBurst      int

	// Certificate configuration
	CertPath     string
	CertPassword string

	// CORS configuration
	CORSOrigins []string
}

// Load reads configuration from environment variables with defaults.
// It validates required configurations and returns an error if critical values are missing.
func Load() (*Config, error) {
	cfg := &Config{
		// Server defaults
		Port: getEnvOrDefault("PORT", "8080"),
		Env:  getEnvOrDefault("ENV", "development"),

		// MongoDB defaults
		MongoDBURI:      getEnvOrDefault("MONGODB_URI", getEnvOrDefault("MONGO_URI", "mongodb://localhost:27017")),
		MongoDBDatabase: getEnvOrDefault("MONGODB_DATABASE", getEnvOrDefault("MONGO_DATABASE", "nfse")),

		// Redis defaults
		RedisURL: getEnvOrDefault("REDIS_URL", "redis://localhost:6379"),

		// SEFIN defaults (homologation by default for safety)
		SEFINApiURL:      getEnvOrDefault("SEFIN_API_URL", "https://hom.nfse.gov.br/api"),
		SEFINEnvironment: getEnvOrDefault("SEFIN_ENVIRONMENT", "homologacao"),
		SEFINTimeout:     time.Duration(getEnvOrDefaultInt("SEFIN_TIMEOUT", 30)) * time.Second,

		// Logging defaults
		LogLevel:  getEnvOrDefault("LOG_LEVEL", "info"),
		LogFormat: getEnvOrDefault("LOG_FORMAT", "json"),

		// Worker defaults
		WorkerConcurrency: getEnvOrDefaultInt("WORKER_CONCURRENCY", 10),
		WorkerMaxRetries:  getEnvOrDefaultInt("WORKER_MAX_RETRIES", 3),

		// Rate limiting defaults
		RateLimitDefaultRPM: getEnvOrDefaultInt("RATE_LIMIT_DEFAULT_RPM", 100),
		RateLimitBurst:      getEnvOrDefaultInt("RATE_LIMIT_BURST", 20),

		// Certificate configuration
		CertPath:     getEnvOrDefault("CERT_PATH", ""),
		CertPassword: getEnvOrDefault("CERT_PASSWORD", ""),

		// CORS configuration
		CORSOrigins: parseCORSOrigins(getEnvOrDefault("CORS_ORIGINS", "http://localhost:3000,http://localhost:8080")),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// validate checks that required configuration values are present and valid.
func (c *Config) validate() error {
	// Validate environment
	validEnvs := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvs[c.Env] {
		return fmt.Errorf("invalid ENV value: %s (must be development, staging, or production)", c.Env)
	}

	// Validate log level
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid LOG_LEVEL value: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	// Validate log format
	validLogFormats := map[string]bool{"json": true, "text": true}
	if !validLogFormats[c.LogFormat] {
		return fmt.Errorf("invalid LOG_FORMAT value: %s (must be json or text)", c.LogFormat)
	}

	// Validate SEFIN environment
	validSEFINEnvs := map[string]bool{"producao": true, "homologacao": true}
	if !validSEFINEnvs[c.SEFINEnvironment] {
		return fmt.Errorf("invalid SEFIN_ENVIRONMENT value: %s (must be producao or homologacao)", c.SEFINEnvironment)
	}

	// Validate numeric values
	if c.WorkerConcurrency < 1 {
		return fmt.Errorf("WORKER_CONCURRENCY must be at least 1")
	}

	if c.RateLimitDefaultRPM < 1 {
		return fmt.Errorf("RATE_LIMIT_DEFAULT_RPM must be at least 1")
	}

	return nil
}

// IsProduction returns true if the environment is production.
func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

// IsDevelopment returns true if the environment is development.
func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

// getEnvOrDefault returns the value of an environment variable or a default value if not set.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvOrDefaultInt returns the integer value of an environment variable or a default value.
func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// parseCORSOrigins parses a comma-separated list of origins.
func parseCORSOrigins(origins string) []string {
	if origins == "" {
		return nil
	}
	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
