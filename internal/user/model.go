package user

import "time"

type UserTelegram struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserId           uint      `gorm:"not null;index" json:"user_id"`
	TelegramBotToken string    `gorm:"not null" json:"-"` // secret: never serialized
	TelegramChatID   int64     `gorm:"not null" json:"telegram_chat_id"`
	CreatedAt        time.Time `json:"created_at"`
}
