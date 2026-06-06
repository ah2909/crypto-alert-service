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
	TelegramBotToken string `json:"telegram_bot_token" binding:"required"`
	TelegramChatID   int64  `json:"telegram_chat_id" binding:"required"`
}

func addTelegramBot(c *gin.Context) {
	var req addTelegramBotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate the bot token by constructing a notifier (calls Telegram getMe).
	if _, err := notifier.NewTelegramNotifier(req.TelegramBotToken, req.TelegramChatID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid telegram bot token or chat id")
		return
	}

	u := &user.UserTelegram{
		UserId:           authUserID(c),
		TelegramBotToken: req.TelegramBotToken,
		TelegramChatID:   req.TelegramChatID,
	}
	if err := storage.AddTelegramBot(u); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create telegram binding")
		return
	}

	respondData(c, http.StatusCreated, u)
}

// getTelegramByUserID returns the authenticated user's Telegram binding.
func getTelegramByUserID(c *gin.Context) {
	uid := authUserID(c)

	u, err := storage.GetTelegramByUserID(uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "telegram binding not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "failed to fetch telegram binding")
		return
	}
	respondData(c, http.StatusOK, u)
}

// ---- Alerts ----

type createAlertReq struct {
	Symbol    string  `json:"symbol" binding:"required"`
	Condition string  `json:"condition" binding:"required,oneof=above below"`
	Price     float64 `json:"price" binding:"required,gt=0"`
}

// createAlert registers a price alert for the authenticated user.
func createAlert(c *gin.Context) {
	var req createAlertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	uid := authUserID(c)

	// The alert is undeliverable unless the user has registered a Telegram
	// binding first (POST /api/telegram-bot).
	if _, err := storage.GetTelegramByUserID(uid); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusBadRequest, "you have no telegram binding; register one via POST /api/telegram-bot first")
			return
		}
		respondError(c, http.StatusInternalServerError, "failed to validate user")
		return
	}

	a := &alert.Alert{
		UserID:    uid,
		Symbol:    strings.ToUpper(strings.TrimSpace(req.Symbol)),
		Condition: alert.Condition(req.Condition),
		Price:     req.Price,
	}
	if err := storage.CreateAlert(a); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create alert")
		return
	}

	respondData(c, http.StatusCreated, a)
}

// listAlerts returns the authenticated user's alerts, newest first.
func listAlerts(c *gin.Context) {
	alerts, err := storage.ListAlertsByUser(authUserID(c))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to list alerts")
		return
	}
	respondData(c, http.StatusOK, alerts)
}

// deleteAlert removes one of the authenticated user's alerts by ID. The delete
// is scoped to the owner, so a user cannot delete another user's alert.
func deleteAlert(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	deleted, err := storage.DeleteAlert(id, authUserID(c))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to delete alert")
		return
	}
	if !deleted {
		respondError(c, http.StatusNotFound, "alert not found")
		return
	}
	respondData(c, http.StatusOK, gin.H{"deleted": id})
}

// parseID reads a uint path parameter, writing a 400 and returning false on error.
func parseID(c *gin.Context, name string) (uint, bool) {
	raw := c.Param(name)
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, name+" must be a number")
		return 0, false
	}
	return uint(v), true
}
