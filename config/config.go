package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all settings loaded from the .env file.
//
// Telegram bot credentials are no longer global — each user supplies their own
// when they register through the API — so there are no Telegram fields here.
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

	username := getEnv("REDIS_USERNAME", "")
	password := getEnv("REDIS_PASSWORD", "")
	addr := getEnv("REDIS_ADDR", "127.0.0.1:6379")

	// fmt.Sprintf builds a string by substituting values into a template.
	// %s means "insert a string here".
	// Result example: "redis://default:secret@localhost:6379"
	redisAddr := fmt.Sprintf("redis://%s:%s@%s", username, password, addr)

	return &Config{
		APIPort:       getEnv("API_PORT", ":8080"),
		MySQLDSN:      getEnv("MYSQL_DSN", ""),
		RedisAddr:     redisAddr,
		RedisUsername: username,
		RedisPassword: password,
		RedisDB:       0,
	}
}

// getEnv reads an env variable, returning a fallback value if not set
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
