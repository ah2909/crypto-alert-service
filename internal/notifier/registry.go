package notifier

import "sync"

// Registry lazily builds and caches one TelegramNotifier per user, so we don't
// re-create (and re-validate over the network) a bot on every alert that fires.
//
// It is safe for concurrent use by the engine's price callbacks.
type Registry struct {
	mu        sync.Mutex
	notifiers map[uint]*TelegramNotifier
}

// NewRegistry creates an empty notifier registry.
func NewRegistry() *Registry {
	return &Registry{
		notifiers: make(map[uint]*TelegramNotifier),
	}
}

// Get returns the cached notifier for userID, creating it from the given token
// and chatID on first use. An invalid token surfaces as an error here.
func (r *Registry) Get(userID uint, token string, chatID int64) (*TelegramNotifier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if n, ok := r.notifiers[userID]; ok {
		return n, nil
	}

	n, err := NewTelegramNotifier(token, chatID)
	if err != nil {
		return nil, err
	}
	r.notifiers[userID] = n
	return n, nil
}
