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

func NewTelegramNotifier(token string, chatID int64) (*TelegramNotifier, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
	if err != nil {
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
		"🚨 *Price Alert Triggered!*\n\nSymbol: `%s`\nCondition: %s $%.2f\nCurrent Price: $%.4f",
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
