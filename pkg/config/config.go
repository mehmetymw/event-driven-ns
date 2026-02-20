package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv              string
	AppPort             string
	DatabaseURL         string
	KafkaBrokers        []string
	KafkaConsumerGroup  string
	WebhookURL          string
	JaegerEndpoint      string
	LogLevel            string
	RateLimitPerChannel int
	WorkerConcurrency   int
}

func Load() *Config {
	return &Config{
		AppEnv:              getEnv("APP_ENV", "development"),
		AppPort:             getEnv("APP_PORT", "8080"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://notification_user:notification_pass@localhost:5432/notification_db?sslmode=disable"),
		KafkaBrokers:        strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaConsumerGroup:  getEnv("KAFKA_CONSUMER_GROUP", "notification-worker"),
		WebhookURL:          getEnv("WEBHOOK_URL", "https://webhook.site/test"),
		JaegerEndpoint:      getEnv("JAEGER_ENDPOINT", "http://localhost:4318"),
		LogLevel:            getEnv("LOG_LEVEL", "debug"),
		RateLimitPerChannel: getEnvInt("RATE_LIMIT_PER_CHANNEL", 100),
		WorkerConcurrency:   getEnvInt("WORKER_CONCURRENCY", 20),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return fallback
}
