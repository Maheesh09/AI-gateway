package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort  string
	AppEnv   string
	DBUrl     string
	RedisURL  string // full redis://host:port URL — used by go-redis
	RedisAddr string // bare host:port — used by asynq

	JWTSecret   string
	AdminAPIKey string

	AnthropicAPIKey string
	AnthropicModel  string

	RateLimitDefaultRPM    int
	RateLimitWindowSeconds int
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	cfg := &Config{
		AppPort:         getRequired("APP_PORT"),
		AppEnv:          getRequired("APP_ENV"),
		DBUrl:    getRequired("DATABASE_URL"),
		RedisURL: getRequired("REDIS_URL"),
		RedisAddr: strings.TrimPrefix(
			strings.TrimPrefix(getRequired("REDIS_URL"), "redis://"),
			"rediss://",
		),
		JWTSecret:       getRequired("JWT_SECRET"),
		AdminAPIKey:     getRequired("ADMIN_API_KEY"),
		AnthropicAPIKey: getRequired("ANTHROPIC_API_KEY"),
		AnthropicModel:  getOrDefault("ANTHROPIC_MODEL", "claude-sonnet-4-6"),

		RateLimitDefaultRPM:    getIntOrDefault("RATE_LIMIT_DEFAULT_RPM", 60),
		RateLimitWindowSeconds: getIntOrDefault("RATE_LIMIT_WINDOW_SECONDS", 60),
	}
	return cfg
}

func getRequired(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("FATAL: required environment variable %s is not set", key)
	}
	return v
}

func getOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getIntOrDefault(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		panic(fmt.Sprintf("config: %s must be an integer, got %q", key, v))
	}
	return n
}
