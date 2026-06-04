package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ah2909/crypto-alert-engine/internal/alert"
	"github.com/ah2909/crypto-alert-engine/internal/notifier"
	"github.com/ah2909/crypto-alert-engine/internal/storage"
	"github.com/ah2909/crypto-alert-engine/internal/user"
)

// ---- Telegram bindings ----

type addTelegramBotReq struct {
	UserID           uint   `json:"user_id" binding:"required"`
	TelegramBotToken string `json:"telegram_bot_token" binding:"required"`
	TelegramChatID   int64  `json:"telegram_chat_id" binding:"required"`
}

// addTelegramBot registers a user's own Telegram bot binding. The bot token is
// validated against Telegram before the binding is stored, so a bad token is
// rejected with 400 rather than failing later when an alert fires.
func addTelegramBot(c *gin.Context) {
	var req addTelegramBotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate the bot token by constructing a notifier (calls Telegram getMe).
	if _, err := notifier.NewTelegramNotifier(req.TelegramBotToken, req.TelegramChatID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid telegram bot token or chat id"})
		return
	}

	u := &user.UserTelegram{
		UserId:           req.UserID,
		TelegramBotToken: req.TelegramBotToken,
		TelegramChatID:   req.TelegramChatID,
	}
	if err := storage.AddTelegramBot(u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create telegram binding"})
		return
	}

	// UserTelegram serializes without the token (json:"-"), so the secret is not echoed.
	c.JSON(http.StatusCreated, u)
}

// getTelegramByUserID returns a user's Telegram binding, looked up by user_id.
func getTelegramByUserID(c *gin.Context) {
	user_id, ok := parseID(c, "user_id")
	if !ok {
		return
	}

	u, err := storage.GetTelegramByUserID(user_id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "telegram binding not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch telegram binding"})
		return
	}
	c.JSON(http.StatusOK, u)
}

// ---- Alerts ----

type createAlertReq struct {
	UserID    uint    `json:"user_id" binding:"required"`
	Symbol    string  `json:"symbol" binding:"required"`
	Condition string  `json:"condition" binding:"required,oneof=above below"`
	Price     float64 `json:"price" binding:"required,gt=0"`
}

// createAlert registers a price alert for an existing user.
func createAlert(c *gin.Context) {
	var req createAlertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The alert is undeliverable unless the user has registered a Telegram
	// binding first (POST /api/telegram-bot).
	if _, err := storage.GetTelegramByUserID(req.UserID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id has no telegram binding; register one via POST /api/telegram-bot first"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate user"})
		return
	}

	a := &alert.Alert{
		UserID:    req.UserID,
		Symbol:    strings.ToUpper(strings.TrimSpace(req.Symbol)),
		Condition: alert.Condition(req.Condition),
		Price:     req.Price,
	}
	if err := storage.CreateAlert(a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create alert"})
		return
	}

	// Picked up by the engine on its next reload (within ~15s).
	c.JSON(http.StatusCreated, a)
}

// listAlerts returns alerts, optionally filtered by ?user_id=.
func listAlerts(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id query parameter is required"})
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id must be a number"})
		return
	}

	alerts, err := storage.ListAlertsByUser(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list alerts"})
		return
	}
	c.JSON(http.StatusOK, alerts)
}

// deleteAlert removes an alert by ID.
func deleteAlert(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := storage.DeleteAlert(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete alert"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// parseID reads a uint path parameter, writing a 400 and returning false on error.
func parseID(c *gin.Context, name string) (uint, bool) {
	raw := c.Param(name)
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": name + " must be a number"})
		return 0, false
	}
	return uint(v), true
}
