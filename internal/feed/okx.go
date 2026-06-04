package feed

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/gorilla/websocket"
)

// okxSubscribeMsg is the message we send to OKX to subscribe to a ticker
type okxSubscribeMsg struct {
	Op   string        `json:"op"`
	Args []okxInstArg  `json:"args"`
}

type okxInstArg struct {
	Channel string `json:"channel"`
	InstID  string `json:"instId"` // e.g. "BTC-USDT"
}

// okxResponse is the shape of a ticker update from OKX
type okxResponse struct {
	Data []struct {
		InstID  string `json:"instId"`
		LastPx  string `json:"last"` // last traded price
	} `json:"data"`
}

// ConnectOKX opens a WebSocket to OKX and calls onPrice for every price update.
// instID should be in OKX format, e.g. "BTC-USDT"
//
// Usage:
//
//	go feed.ConnectOKX("BTC-USDT", func(symbol string, price float64) {
//	    fmt.Printf("%s = $%.2f\n", symbol, price)
//	})
func ConnectOKX(instID string, onPrice func(symbol string, price float64)) {
	url := "wss://ws.okx.com:8443/ws/v5/public"

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatalf("OKX WebSocket connection failed: %v", err)
	}
	defer conn.Close()

	// Subscribe to the ticker channel for the given instrument
	sub := okxSubscribeMsg{
		Op: "subscribe",
		Args: []okxInstArg{
			{Channel: "tickers", InstID: instID},
		},
	}
	if err := conn.WriteJSON(sub); err != nil {
		log.Fatalf("OKX subscribe error: %v", err)
	}

	log.Printf("Connected to OKX stream for %s", instID)

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Printf("OKX read error: %v", err)
			return
		}

		var resp okxResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			continue
		}

		// OKX sends multiple data items; we process each one
		for _, d := range resp.Data {
			if d.LastPx == "" {
				continue
			}
			price, err := strconv.ParseFloat(d.LastPx, 64)
			if err != nil {
				continue
			}
			onPrice(d.InstID, price)
		}
	}
}

// parsePrice is a shared helper used by binance.go and okx.go
func parsePrice(s string, out *float64) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price %q: %w", s, err)
	}
	if out != nil {
		*out = v
	}
	return v, nil
}
