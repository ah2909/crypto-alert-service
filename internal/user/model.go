package user

import "time"

// User represents an account that owns alerts and receives notifications
// through their own Telegram bot.
//
// Each user supplies their own bot token + chat ID, so notifications for a
// user are delivered by the bot that user created — not a single shared bot.
type UserTelegram struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserId           uint      `gorm:"not null;index" json:"user_id"`
	TelegramBotToken string    `gorm:"not null" json:"-"` // secret: never serialized
	TelegramChatID   int64     `gorm:"not null" json:"telegram_chat_id"`
	CreatedAt        time.Time `json:"created_at"`
}
