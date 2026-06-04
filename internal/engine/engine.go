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

// defaultReloadInterval controls how quickly a newly-registered alert (and its
// symbol) is picked up by the running engine.
const defaultReloadInterval = 15 * time.Second

// Engine watches price feeds and fires alerts. It periodically reloads the set
// of active alerts from the database (the source of truth), so alerts created
// through the API are picked up without restarting.
//
// Concurrency: a single mutex guards both the in-memory alert cache and the set
// of active subscriptions. Network sends happen outside the lock.
type Engine struct {
	registry       *notifier.Registry
	reloadInterval time.Duration
	ctx            context.Context

	mu             sync.Mutex
	alertsBySymbol map[string][]alert.Alert   // symbol (upper) -> active alerts
	telegrams      map[uint]user.UserTelegram // userID -> Telegram binding (for bot lookup)
	subscribed     map[string]bool            // symbol (upper) -> stream running
}

// New creates an engine that delivers notifications through the given registry.
func New(registry *notifier.Registry) *Engine {
	return &Engine{
		registry:       registry,
		reloadInterval: defaultReloadInterval,
		alertsBySymbol: make(map[string][]alert.Alert),
		telegrams:      make(map[uint]user.UserTelegram),
		subscribed:     make(map[string]bool),
	}
}

// Run loads alerts and keeps them refreshed until ctx is cancelled. It blocks,
// so callers typically run it in a goroutine.
func (e *Engine) Run(ctx context.Context) {
	e.ctx = ctx
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

// reload refreshes the in-memory alert cache from the database and starts a
// price stream for any symbol that doesn't have one yet. Streams are never torn
// down — a symbol with no active alerts simply gets an empty slice and its
// ticks become no-ops.
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
	for sym := range bySymbol {
		if !e.subscribed[sym] {
			e.subscribed[sym] = true
			log.Printf("engine: subscribing to %s", sym)
			go feed.StreamBinance(e.ctx, sym, e.onPrice)
		}
	}
	e.mu.Unlock()
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
		return // leave untriggered; next reload retries
	}

	storage.MarkAlertSent(f.alert.ID)
	if err := storage.MarkAlertTriggered(f.alert.ID); err != nil {
		log.Printf("engine: failed to mark alert %d triggered: %v", f.alert.ID, err)
	}
}
