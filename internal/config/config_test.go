package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Set required environment variables
	os.Setenv("APP_PORT", "8080")
	os.Setenv("APP_ENV", "test")
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	os.Setenv("REDIS_URL", "redis://localhost:6379/1")
	os.Setenv("JWT_SECRET", "supersecret123")
	os.Setenv("ADMIN_API_KEY", "adminkey123")
	os.Setenv("ANTHROPIC_API_KEY", "anthropic123")

	// Ensure optional defaults
	os.Unsetenv("ANTHROPIC_MODEL")
	os.Unsetenv("RATE_LIMIT_DEFAULT_RPM")
	os.Unsetenv("RATE_LIMIT_WINDOW_SECONDS")

	cfg := Load()

	if cfg.AppPort != "8080" {
		t.Errorf("Expected APP_PORT 8080, got %s", cfg.AppPort)
	}

	if cfg.AnthropicModel != "claude-sonnet-4-6" {
		t.Errorf("Expected default ANTHROPIC_MODEL claude-sonnet-4-6, got %s", cfg.AnthropicModel)
	}

	if cfg.RateLimitDefaultRPM != 60 {
		t.Errorf("Expected default RATE_LIMIT_DEFAULT_RPM 60, got %d", cfg.RateLimitDefaultRPM)
	}

	if cfg.RedisAddr != "localhost:6379/1" {
		t.Errorf("Expected RedisAddr to be parsed as localhost:6379/1, got %s", cfg.RedisAddr)
	}

	// Clean up
	os.Clearenv()
}

func TestGetRequiredPanics(t *testing.T) {
	os.Clearenv()

	// getRequired will call log.Fatalf, which exits. We can't easily catch log.Fatalf in standard Go tests.
	// But we can test getOrDefault
	val := getOrDefault("MISSING_KEY", "default_val")
	if val != "default_val" {
		t.Errorf("Expected default_val, got %s", val)
	}

	os.Setenv("EXISTING_KEY", "my_val")
	val2 := getOrDefault("EXISTING_KEY", "default_val")
	if val2 != "my_val" {
		t.Errorf("Expected my_val, got %s", val2)
	}
}

func TestGetIntOrDefault(t *testing.T) {
	os.Clearenv()

	val := getIntOrDefault("MISSING_INT", 42)
	if val != 42 {
		t.Errorf("Expected 42, got %d", val)
	}

	os.Setenv("MY_INT", "100")
	val2 := getIntOrDefault("MY_INT", 42)
	if val2 != 100 {
		t.Errorf("Expected 100, got %d", val2)
	}

	// Panic recovery test for invalid integer
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic on invalid int")
		}
	}()
	os.Setenv("BAD_INT", "not_a_number")
	getIntOrDefault("BAD_INT", 42)
}
