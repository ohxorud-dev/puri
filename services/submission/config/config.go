package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	RabbitMQURL    string
	SubmissionPort string
	ProblemsPath   string
	TestcasesPath  string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found, using environment variables")
	}

	cfg := &Config{
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		RabbitMQURL:    getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		SubmissionPort: getEnv("SUBMISSION_PORT", "8081"),
		ProblemsPath:   getEnv("PROBLEMS_PATH", "./problems"),
		TestcasesPath:  getEnv("TESTCASES_PATH", "./testcases"),
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
