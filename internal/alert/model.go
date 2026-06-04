package alert

import "time"

// Condition describes when an alert should fire
type Condition string

const (
	Above Condition = "above" // fire when price goes above the target
	Below Condition = "below" // fire when price goes below the target
)

// Alert represents a single price alert rule set by a user.
//
// An alert belongs to a User (UserID); when it fires, the notification is sent
// through that user's own Telegram bot.
type Alert struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Symbol    string    `gorm:"not null;index" json:"symbol"`    // e.g. "BTCUSDT"
	Condition Condition `gorm:"not null" json:"condition"`       // "above" or "below"
	Price     float64   `gorm:"not null" json:"price"`           // the target price
	Triggered bool      `gorm:"default:false" json:"triggered"`  // true once the alert has fired
	CreatedAt time.Time `json:"created_at"`
}
