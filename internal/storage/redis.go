package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RDB is the shared Redis client
var RDB *redis.Client

// InitRedis creates a Redis client and verifies the connection.
// addr accepts either "host:port" or a full URL like "redis://user:pass@host:port"
func InitRedis(addr, password string, db int) {
	// ParseURL handles the full redis:// URL format (with username/password baked in)
	opt, err := redis.ParseURL(addr)
	if err != nil {
		// Fallback: plain host:port without credentials
		opt = &redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}
	}

	RDB = redis.NewClient(opt)

	// Ping Redis to make sure the connection works
	if err := RDB.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Println("Redis connected successfully")
}

// IsAlertSent checks if an alert was already sent (prevents duplicate notifications)
func IsAlertSent(alertID uint) bool {
	key := fmt.Sprintf("alert:sent:%d", alertID)
	val, _ := RDB.Get(context.Background(), key).Result()
	return val == "1"
}

// MarkAlertSent records that an alert was sent in Redis
func MarkAlertSent(alertID uint) {
	key := fmt.Sprintf("alert:sent:%d", alertID)
	RDB.Set(context.Background(), key, "1", 24*time.Hour)
}
