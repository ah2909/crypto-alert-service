package notifier

import (
	"fmt"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramNotifier sends messages to a single Telegram chat through one bot.
type TelegramNotifier struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

// NewTelegramNotifier creates a notifier for a user's own bot token and chat ID.
//
// It returns an error (instead of crashing) if the token is invalid, so callers
// can reject bad input — e.g. the API returns 400 on user registration and the
// engine logs and skips. Construction calls Telegram's getMe to validate the
// token, so it is bounded by a 10s HTTP timeout.
func NewTelegramNotifier(token string, chatID int64) (*TelegramNotifier, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
	if err != nil {
		// Note: do not include the token in the error — it is a secret.
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	return &TelegramNotifier{
		bot:    bot,
		chatID: chatID,
	}, nil
}

// SendAlert sends a formatted alert message to the Telegram chat
func (t *TelegramNotifier) SendAlert(symbol string, condition string, targetPrice, currentPrice float64) error {
	text := fmt.Sprintf(
		"🚨 *Price Alert Triggered!*\n\nSymbol: `%s`\nCondition: %s $%.2f\nCurrent Price: $%.2f",
		symbol, condition, targetPrice, currentPrice,
	)

	msg := tgbotapi.NewMessage(t.chatID, text)
	msg.ParseMode = "Markdown"

	_, err := t.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}

	return nil
}
