# crypto-alert-engine

A Go service that watches real-time crypto prices from Binance and delivers Telegram notifications when user-defined price alerts are triggered.

## How it works

```
Binance WebSocket  ──►  Engine  ──►  Telegram Bot
   (@ticker)          (evaluates       (per-user
                        alerts)          bot)
                          ▲
                       MySQL DB
                     (alert store)
```

1. The engine loads active alerts from MySQL on startup and every 60 s.
2. It subscribes to the Binance `@ticker` stream for each symbol that has active alerts.
3. When a price crosses a threshold, the alert fires once and the engine marks it triggered.
4. Redis prevents duplicate notifications across restarts.

## API

All `/api` routes require a valid RS256 bearer token issued by the auth service.

### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness check |

### Telegram binding

Each user links their own Telegram bot. The bot token is validated against the Telegram API before it is saved.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/telegram-bot` | Register a bot |
| `GET` | `/api/telegram-bot` | Get your binding |

**POST body**
```json
{
  "telegram_bot_token": "123456:ABC-...",
  "telegram_chat_id": -1001234567890
}
```

### Alerts

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/alerts` | Create a price alert |
| `GET` | `/api/alerts` | List your alerts |
| `DELETE` | `/api/alerts/:id` | Delete an alert |

**POST body**
```json
{
  "symbol": "BTCUSDT",
  "condition": "above",
  "price": 70000
}
```

`condition` must be `above` or `below`. A Telegram binding must exist before an alert can be created.

## Configuration

Copy `.env.example` to `.env` and fill in the values, or set environment variables directly.

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `:8080` | HTTP listen address |
| `MYSQL_DSN` | — | GORM MySQL DSN |
| `REDIS_ADDR` | `127.0.0.1:6379` | Redis address |
| `REDIS_USERNAME` | — | Redis username (optional) |
| `REDIS_PASSWORD` | — | Redis password (optional) |
| `AUTH_SERVICE_URL` | `http://auth-service:8086` | Auth service base URL (JWKS fetched from `/.well-known/jwks.json`) |
| `CORS_ALLOWED_ORIGINS` | `*` | Comma-separated allowed origins |

## Running

```bash
go run ./cmd
```

## Stack

- **Go** — runtime
- **Gin** — HTTP router
- **GORM + MySQL** — alert persistence
- **Redis** — cross-restart deduplication
- **Binance WebSocket** — real-time price feed
- **Telegram Bot API** — notifications
- **RS256 JWT** — authentication
