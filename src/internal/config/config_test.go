package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Save original environment
	origEnv := map[string]string{
		"PORT":                   os.Getenv("PORT"),
		"ENV":                    os.Getenv("ENV"),
		"MONGODB_URI":            os.Getenv("MONGODB_URI"),
		"MONGODB_DATABASE":       os.Getenv("MONGODB_DATABASE"),
		"REDIS_URL":              os.Getenv("REDIS_URL"),
		"LOG_LEVEL":              os.Getenv("LOG_LEVEL"),
		"LOG_FORMAT":             os.Getenv("LOG_FORMAT"),
		"SEFIN_ENVIRONMENT":      os.Getenv("SEFIN_ENVIRONMENT"),
		"WORKER_CONCURRENCY":     os.Getenv("WORKER_CONCURRENCY"),
		"RATE_LIMIT_DEFAULT_RPM": os.Getenv("RATE_LIMIT_DEFAULT_RPM"),
	}

	// Restore environment after test
	defer func() {
		for k, v := range origEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	t.Run("loads defaults", func(t *testing.T) {
		// Clear relevant environment variables
		os.Unsetenv("PORT")
		os.Unsetenv("ENV")
		os.Unsetenv("MONGODB_URI")
		os.Unsetenv("MONGODB_DATABASE")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Port != "8080" {
			t.Errorf("Port = %q, want %q", cfg.Port, "8080")
		}
		if cfg.Env != "development" {
			t.Errorf("Env = %q, want %q", cfg.Env, "development")
		}
		if cfg.LogLevel != "info" {
			t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
		}
		if cfg.LogFormat != "json" {
			t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
		}
		if cfg.WorkerConcurrency != 10 {
			t.Errorf("WorkerConcurrency = %d, want %d", cfg.WorkerConcurrency, 10)
		}
		if cfg.RateLimitDefaultRPM != 100 {
			t.Errorf("RateLimitDefaultRPM = %d, want %d", cfg.RateLimitDefaultRPM, 100)
		}
	})

	t.Run("loads from environment", func(t *testing.T) {
		os.Setenv("PORT", "9000")
		os.Setenv("ENV", "production")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("WORKER_CONCURRENCY", "20")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Port != "9000" {
			t.Errorf("Port = %q, want %q", cfg.Port, "9000")
		}
		if cfg.Env != "production" {
			t.Errorf("Env = %q, want %q", cfg.Env, "production")
		}
		if cfg.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
		}
		if cfg.WorkerConcurrency != 20 {
			t.Errorf("WorkerConcurrency = %d, want %d", cfg.WorkerConcurrency, 20)
		}
	})

	t.Run("validates environment", func(t *testing.T) {
		os.Setenv("ENV", "invalid")

		_, err := Load()
		if err == nil {
			t.Error("Load() expected error for invalid ENV")
		}
	})

	t.Run("validates log level", func(t *testing.T) {
		os.Setenv("ENV", "development")
		os.Setenv("LOG_LEVEL", "invalid")

		_, err := Load()
		if err == nil {
			t.Error("Load() expected error for invalid LOG_LEVEL")
		}
	})

	t.Run("validates SEFIN environment", func(t *testing.T) {
		os.Setenv("ENV", "development")
		os.Setenv("LOG_LEVEL", "info")
		os.Setenv("SEFIN_ENVIRONMENT", "invalid")

		_, err := Load()
		if err == nil {
			t.Error("Load() expected error for invalid SEFIN_ENVIRONMENT")
		}
	})
}

func TestConfigHelpers(t *testing.T) {
	t.Run("IsProduction", func(t *testing.T) {
		cfg := &Config{Env: "production"}
		if !cfg.IsProduction() {
			t.Error("IsProduction() = false, want true")
		}

		cfg.Env = "development"
		if cfg.IsProduction() {
			t.Error("IsProduction() = true, want false")
		}
	})

	t.Run("IsDevelopment", func(t *testing.T) {
		cfg := &Config{Env: "development"}
		if !cfg.IsDevelopment() {
			t.Error("IsDevelopment() = false, want true")
		}

		cfg.Env = "production"
		if cfg.IsDevelopment() {
			t.Error("IsDevelopment() = true, want false")
		}
	})
}

func TestParseCORSOrigins(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"single origin", "http://localhost:3000", []string{"http://localhost:3000"}},
		{"multiple origins", "http://localhost:3000,http://localhost:8080", []string{"http://localhost:3000", "http://localhost:8080"}},
		{"with spaces", "http://localhost:3000 , http://localhost:8080", []string{"http://localhost:3000", "http://localhost:8080"}},
		{"empty", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCORSOrigins(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseCORSOrigins(%q) len = %d, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("parseCORSOrigins(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}
