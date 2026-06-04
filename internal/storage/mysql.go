package storage

import (
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/ah2909/crypto-alert-engine/internal/alert"
	"github.com/ah2909/crypto-alert-engine/internal/user"
)

// DB is the shared database connection
var DB *gorm.DB

// InitMySQL connects to MySQL using the DSN from config and auto-migrates tables
func InitMySQL(dsn string) {
	var err error

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}

	// AutoMigrate creates or updates the tables to match the structs
	if err := DB.AutoMigrate(&user.UserTelegram{}, &alert.Alert{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("MySQL connected and migrated successfully")
}

// ---- UserTelegram repository ----
//
// A UserTelegram is a per-user Telegram binding. Its own ID is the row's
// primary key; UserId is the user the binding belongs to. Alerts reference a
// user via alert.UserID, which matches UserTelegram.UserId — so deliveries are
// resolved by UserId, not by the binding's primary key.

// AddTelegramBot inserts a new Telegram binding (and fills u.ID on success)
func AddTelegramBot(u *user.UserTelegram) error {
	return DB.Create(u).Error
}

// GetTelegramByUserID looks up the Telegram binding for a given user
func GetTelegramByUserID(userID uint) (*user.UserTelegram, error) {
	var u user.UserTelegram
	if err := DB.Where("user_id = ?", userID).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// GetTelegramsByUserIDs returns the Telegram bindings for the given user IDs,
// keyed by UserId. Missing IDs are simply absent from the map. Used by the
// engine to resolve which bot to notify for each alert.
func GetTelegramsByUserIDs(userIDs []uint) (map[uint]user.UserTelegram, error) {
	result := make(map[uint]user.UserTelegram, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	var bindings []user.UserTelegram
	if err := DB.Where("user_id IN ?", userIDs).Find(&bindings).Error; err != nil {
		return nil, err
	}
	for _, u := range bindings {
		result[u.UserId] = u
	}
	return result, nil
}

// ---- Alert repository ----

// CreateAlert inserts a new alert (and fills a.ID on success)
func CreateAlert(a *alert.Alert) error {
	return DB.Create(a).Error
}

// ListAlertsByUser returns all alerts belonging to a user, newest first
func ListAlertsByUser(userID uint) ([]alert.Alert, error) {
	var alerts []alert.Alert
	if err := DB.Where("user_id = ?", userID).Order("id DESC").Find(&alerts).Error; err != nil {
		return nil, err
	}
	return alerts, nil
}

// DeleteAlert removes an alert by ID
func DeleteAlert(id uint) error {
	return DB.Delete(&alert.Alert{}, id).Error
}

// GetActiveAlerts returns every alert that has not yet fired. This is the
// source of truth the engine reloads on each tick.
func GetActiveAlerts() ([]alert.Alert, error) {
	var alerts []alert.Alert
	if err := DB.Where("triggered = ?", false).Find(&alerts).Error; err != nil {
		return nil, err
	}
	return alerts, nil
}

// MarkAlertTriggered flags an alert as fired so it won't be reloaded as active
func MarkAlertTriggered(id uint) error {
	return DB.Model(&alert.Alert{}).Where("id = ?", id).Update("triggered", true).Error
}
