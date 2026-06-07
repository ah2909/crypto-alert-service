package engine

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ah2909/crypto-alert-engine/internal/alert"
	"github.com/ah2909/crypto-alert-engine/internal/feed"
	"github.com/ah2909/crypto-alert-engine/internal/notifier"
	"github.com/ah2909/crypto-alert-engine/internal/storage"
	"github.com/ah2909/crypto-alert-engine/internal/user"
)

const defaultReloadInterval = 60 * time.Second

// Engine watches a price feed and fires alerts. It periodically reloads the set
// of active alerts from the database (the source of truth), so alerts created
// through the API are picked up without restarting.
//
// Concurrency: a single mutex guards the in-memory alert cache. Network sends
// happen outside the lock. All symbols share one feed.Stream connection.
type Engine struct {
	registry       *notifier.Registry
	reloadInterval time.Duration
	stream         *feed.Stream

	mu             sync.Mutex
	alertsBySymbol map[string][]alert.Alert   // symbol (upper) -> active alerts
	telegrams      map[uint]user.UserTelegram // userID -> Telegram binding (for bot lookup)
}

// New creates an engine that delivers notifications through the given registry.
func New(registry *notifier.Registry) *Engine {
	e := &Engine{
		registry:       registry,
		reloadInterval: defaultReloadInterval,
		alertsBySymbol: make(map[string][]alert.Alert),
		telegrams:      make(map[uint]user.UserTelegram),
	}
	e.stream = feed.NewStream(e.onPrice)
	return e
}

// Run loads alerts and keeps them refreshed until ctx is cancelled. It blocks,
// so callers typically run it in a goroutine.
func (e *Engine) Run(ctx context.Context) {
	go e.stream.Run(ctx)
	e.reload()

	ticker := time.NewTicker(e.reloadInterval)
	defer ticker.Stop()

	log.Printf("Alert engine running (reload every %s)", e.reloadInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.reload()
		}
	}
}

// reload refreshes the in-memory alert cache from the database and subscribes
// the shared stream to any new symbol. Subscriptions are never dropped — a
// symbol with no active alerts simply gets an empty slice and its ticks become
// no-ops.
func (e *Engine) reload() {
	alerts, err := storage.GetActiveAlerts()
	if err != nil {
		log.Printf("engine: failed to load active alerts: %v", err)
		return
	}

	ids := make([]uint, 0, len(alerts))
	seen := make(map[uint]bool)
	for _, a := range alerts {
		if !seen[a.UserID] {
			seen[a.UserID] = true
			ids = append(ids, a.UserID)
		}
	}
	telegrams, err := storage.GetTelegramsByUserIDs(ids)
	if err != nil {
		log.Printf("engine: failed to load Telegram bindings: %v", err)
		return
	}

	bySymbol := make(map[string][]alert.Alert)
	for _, a := range alerts {
		// Skip alerts whose owner has no Telegram binding — they can never
		// be delivered, so don't subscribe on their behalf.
		if _, ok := telegrams[a.UserID]; !ok {
			continue
		}
		sym := strings.ToUpper(a.Symbol)
		bySymbol[sym] = append(bySymbol[sym], a)
	}

	e.mu.Lock()
	e.alertsBySymbol = bySymbol
	e.telegrams = telegrams
	e.mu.Unlock()

	syms := make([]string, 0, len(bySymbol))
	for sym := range bySymbol {
		syms = append(syms, sym)
	}
	e.stream.Subscribe(syms...)
}

// firedAlert bundles everything needed to deliver one notification outside the
// lock.
type firedAlert struct {
	alert    alert.Alert
	telegram user.UserTelegram
	current  float64
}

// onPrice is the callback invoked by the feed on every price update. It checks
// the symbol's alerts, removes any that fire (so they can't fire twice within
// this process), then delivers them without holding the lock.
func (e *Engine) onPrice(symbol string, price float64) {
	sym := strings.ToUpper(symbol)

	e.mu.Lock()
	alerts := e.alertsBySymbol[sym]
	if len(alerts) == 0 {
		e.mu.Unlock()
		return
	}

	var toFire []firedAlert
	kept := alerts[:0] // reuse the backing array; we overwrite the slice below
	for _, a := range alerts {
		if alert.Evaluate(a, price) {
			tg := e.telegrams[a.UserID] // guaranteed present (filtered in reload)
			toFire = append(toFire, firedAlert{alert: a, telegram: tg, current: price})
		} else {
			kept = append(kept, a)
		}
	}
	e.alertsBySymbol[sym] = kept
	e.mu.Unlock()

	for _, f := range toFire {
		e.fire(f)
	}
}

// fire delivers a single alert through the owner's bot and marks it triggered.
//
// On a transient send failure the alert stays untriggered in the DB, so the
// next reload re-adds it and we retry. The Redis flag is a cross-restart guard
// against re-sending an alert whose DB update was lost.
func (e *Engine) fire(f firedAlert) {
	if storage.IsAlertSent(f.alert.ID) {
		_ = storage.MarkAlertTriggered(f.alert.ID)
		return
	}

	n, err := e.registry.Get(f.telegram.UserId, f.telegram.TelegramBotToken, f.telegram.TelegramChatID)
	if err != nil {
		log.Printf("engine: notifier for user %d unavailable: %v", f.telegram.UserId, err)
		return
	}

	log.Printf("engine: alert %d fired — %s %s $%.2f (current $%.2f) for user %d",
		f.alert.ID, f.alert.Symbol, f.alert.Condition, f.alert.Price, f.current, f.telegram.UserId)

	if err := n.SendAlert(f.alert.Symbol, string(f.alert.Condition), f.alert.Price, f.current); err != nil {
		log.Printf("engine: failed to send alert %d: %v", f.alert.ID, err)
		return
	}

	if err := storage.PublishWSEvent(f.telegram.UserId, "alert-fired", map[string]any{
		"symbol":    f.alert.Symbol,
		"condition": string(f.alert.Condition),
		"target":    f.alert.Price,
		"current":   f.current,
	}); err != nil {
		log.Printf("engine: failed to publish ws event for alert %d: %v", f.alert.ID, err)
	}

	storage.MarkAlertSent(f.alert.ID)
	if err := storage.MarkAlertTriggered(f.alert.ID); err != nil {
		log.Printf("engine: failed to mark alert %d triggered: %v", f.alert.ID, err)
	}
}
