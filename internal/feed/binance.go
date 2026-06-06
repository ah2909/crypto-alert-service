package feed

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const binanceWSBase = "wss://stream.binance.com:9443/ws"

type binanceTickerMsg struct {
	Symbol    string          `json:"s"` // e.g. "BTCUSDT"
	Price     json.RawMessage `json:"c"` // last price (string-encoded)
	CloseTime int64           `json:"C"` // stats close time (ms); absorbs the "C" key so it doesn't overwrite Price via Go's case-insensitive match
}

// binanceRequest is a SUBSCRIBE/UNSUBSCRIBE control frame.
type binanceRequest struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int64    `json:"id"`
}

type Stream struct {
	onPrice func(symbol string, price float64)

	mu      sync.Mutex
	streams map[string]bool // "<symbol>@ticker" (lower)
	conn    *websocket.Conn // current connection, nil while disconnected
	id      int64

	writeMu sync.Mutex // gorilla allows only one concurrent writer
}

func NewStream(onPrice func(symbol string, price float64)) *Stream {
	return &Stream{
		onPrice: onPrice,
		streams: make(map[string]bool),
	}
}

func (s *Stream) Subscribe(symbols ...string) {
	s.mu.Lock()
	var added []string
	for _, sym := range symbols {
		stream := toLower(sym) + "@ticker"
		if !s.streams[stream] {
			s.streams[stream] = true
			added = append(added, stream)
		}
	}
	conn := s.conn
	s.mu.Unlock()

	if len(added) == 0 {
		return
	}

	log.Printf("feed: subscribing to %v", added)
	if conn != nil {
		// Not fatal: a full resubscribe happens on the next reconnect.
		if err := s.send(conn, "SUBSCRIBE", added); err != nil {
			log.Printf("feed: subscribe failed: %v", err)
		}
	}
}

func (s *Stream) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, binanceWSBase, nil)
		if err != nil {
			log.Printf("Binance dial error: %v (retry in 5s)", err)
			if sleepCtx(ctx, 5*time.Second) {
				return
			}
			continue
		}
		log.Printf("Connected to Binance stream")

		s.mu.Lock()
		s.conn = conn
		want := make([]string, 0, len(s.streams))
		for st := range s.streams {
			want = append(want, st)
		}
		s.mu.Unlock()

		if len(want) > 0 {
			if err := s.send(conn, "SUBSCRIBE", want); err != nil {
				log.Printf("Binance resubscribe failed: %v", err)
			}
		}

		readErr := s.read(ctx, conn)

		s.mu.Lock()
		s.conn = nil
		s.mu.Unlock()
		conn.Close()

		if ctx.Err() != nil {
			return
		}
		log.Printf("Binance stream ended: %v (reconnecting)", readErr)
		if sleepCtx(ctx, 2*time.Second) {
			return
		}
	}
}

// send writes one control frame. gorilla permits a single concurrent writer, so
// resubscribe (Run) and incremental SUBSCRIBE (Subscribe) are serialized here.
func (s *Stream) send(conn *websocket.Conn, method string, params []string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.id++
	return conn.WriteJSON(binanceRequest{Method: method, Params: params, ID: s.id})
}

// read pumps messages from a connected socket until an error occurs or ctx is
// cancelled. It returns the error that ended the loop.
func (s *Stream) read(ctx context.Context, conn *websocket.Conn) error {
	// Close the connection when ctx is cancelled so the blocking ReadMessage
	// below unblocks promptly.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-stop:
		}
	}()

	for {
		// A healthy @ticker stream pushes about once per second; if we hear
		// nothing for 90s, treat the connection as dead and reconnect.
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var msg binanceTickerMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("Binance JSON parse error: %v", err)
			continue
		}
		// SUBSCRIBE acks carry no symbol; skip them.
		if msg.Symbol == "" {
			continue
		}

		// Strip the surrounding quotes when the price is string-encoded; a
		// number-encoded price has none. parsePrice then handles either form.
		priceStr := strings.Trim(string(msg.Price), `"`)

		var price float64
		if _, err := parsePrice(priceStr, &price); err != nil {
			continue
		}

		s.onPrice(msg.Symbol, price)
	}
}

// sleepCtx waits for d or until ctx is cancelled. It returns true if ctx was cancelled (the caller should stop).
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-t.C:
		return false
	}
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
