package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const wsEventsChannel = "app:events"

// RDB is the shared Redis client
var RDB *redis.Client

func InitRedis(addr, password string, db int) {
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

// PublishWSEvent publishes a realtime event to Redis pub/sub
// Socket.io server emits to a user's FE.
func PublishWSEvent(userID uint, event string, data any) error {
	payload, err := json.Marshal(map[string]any{
		"event":  event,
		"userId": userID,
		"data":   data,
	})
	if err != nil {
		return err
	}
	return RDB.Publish(context.Background(), wsEventsChannel, payload).Err()
}
