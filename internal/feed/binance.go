package feed

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// binanceTickerMsg is the shape of the message Binance sends us
type binanceTickerMsg struct {
	Symbol string `json:"s"` // e.g. "BTCUSDT"
	Price  string `json:"c"` // current price as a string
}

// StreamBinance opens a WebSocket to Binance for a single symbol and calls
// onPrice for every price update. It runs until ctx is cancelled, reconnecting
// automatically on any read/dial error (it never calls log.Fatal — losing one
// symbol's stream must not take down the whole engine).
//
// symbol is case-insensitive (e.g. "BTCUSDT" or "btcusdt"); the Binance stream
// name requires lowercase, which this function handles.
func StreamBinance(ctx context.Context, symbol string, onPrice func(symbol string, price float64)) {
	streamSymbol := toLower(symbol)
	url := "wss://stream.binance.com:9443/ws/" + streamSymbol + "@ticker"

	for {
		if ctx.Err() != nil {
			return
		}

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
		if err != nil {
			log.Printf("Binance dial error for %s: %v (retry in 5s)", symbol, err)
			if sleepCtx(ctx, 5*time.Second) {
				return
			}
			continue
		}
		log.Printf("Connected to Binance stream for %s", symbol)

		readErr := readBinance(ctx, conn, onPrice)
		conn.Close()
		if ctx.Err() != nil {
			return
		}
		log.Printf("Binance stream for %s ended: %v (reconnecting)", symbol, readErr)
		if sleepCtx(ctx, 2*time.Second) {
			return
		}
	}
}

// readBinance pumps messages from a connected socket until an error occurs or
// ctx is cancelled. It returns the error that ended the loop.
func readBinance(ctx context.Context, conn *websocket.Conn, onPrice func(symbol string, price float64)) error {
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

		var price float64
		if _, err := parsePrice(msg.Price, &price); err != nil {
			continue
		}

		onPrice(msg.Symbol, price)
	}
}

// sleepCtx waits for d or until ctx is cancelled. It returns true if ctx was
// cancelled (the caller should stop).
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

// toLower lowercases an ASCII symbol without pulling in strings just for this.
func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
