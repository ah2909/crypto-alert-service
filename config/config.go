package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	APIPort       string
	MySQLDSN      string
	RedisAddr     string
	RedisUsername string
	RedisPassword string
	RedisDB       int
}

// Load reads the .env file and returns a Config struct
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	username := GetEnv("REDIS_USERNAME", "")
	password := GetEnv("REDIS_PASSWORD", "")
	addr := GetEnv("REDIS_ADDR", "127.0.0.1:6379")

	redisAddr := fmt.Sprintf("redis://%s:%s@%s", username, password, addr)

	return &Config{
		APIPort:       GetEnv("API_PORT", ":8080"),
		MySQLDSN:      GetEnv("MYSQL_DSN", ""),
		RedisAddr:     redisAddr,
		RedisUsername: username,
		RedisPassword: password,
		RedisDB:       0,
	}
}

// GetEnv returns the value of the environment variable key, or fallback when it
// is unset or empty. It is the single entry point for reading env vars so the
// rest of the codebase doesn't call os.Getenv directly.
func GetEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
