package alert

import "time"

type Condition string

const (
	Above Condition = "above"
	Below Condition = "below"
)

type Alert struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Symbol    string    `gorm:"not null;index" json:"symbol"`  // e.g. "BTCUSDT"
	Condition Condition `gorm:"not null" json:"condition"`
	Price     float64   `gorm:"not null" json:"price"`
	Triggered bool      `gorm:"default:false" json:"triggered"`
	CreatedAt time.Time `json:"created_at"`
}
