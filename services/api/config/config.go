package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	JWTSecret            string
	DatabaseURL          string
	SubmissionServiceURL string
	APIPort              string
	Env                  string // "production" or "development"
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found, using environment variables")
	}

	cfg := &Config{
		JWTSecret:            getEnv("JWT_SECRET", ""),
		DatabaseURL:          getEnv("DATABASE_URL", ""),
		SubmissionServiceURL: getEnv("SUBMISSION_SERVICE_URL", "http://localhost:8081"),
		APIPort:              getEnv("API_PORT", "8080"),
		Env:                  getEnv("APP_ENV", "development"),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
