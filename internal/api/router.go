package api

import (
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// NewRouter wires up the HTTP routes.
//
// NOTE ON AUTH: this API is intentionally unauthenticated — callers pass a
// user_id directly and bot tokens are stored per user. That is a deliberate
// choice for a single-operator / learning deployment. Do NOT expose it on a
// public network as-is: anyone could create alerts for any user. Add auth
// (e.g. an API key or JWT scoping requests to the caller's user) before that.
func NewRouter() *gin.Engine {
	r := gin.Default()

	// We run without a reverse proxy, so trust no proxy headers: c.ClientIP()
	// uses the real TCP remote address and forged X-Forwarded-For is ignored.
	// If you later deploy behind nginx/a load balancer, set this to its IP(s).
	_ = r.SetTrustedProxies(nil)

	// CORS: let the frontend call this API from a browser on another origin.
	// Configure allowed origins via CORS_ALLOWED_ORIGINS (comma-separated);
	// defaults to "*" for local development.
	corsCfg := cors.DefaultConfig()
	corsCfg.AllowOrigins = allowedOrigins()
	corsCfg.AllowMethods = []string{"GET", "POST", "DELETE", "OPTIONS"}
	corsCfg.AllowHeaders = []string{"Origin", "Content-Type"}
	r.Use(cors.New(corsCfg))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// need a middleware to auth the request
	api := r.Group("/api")
	{
		api.POST("/telegram-bot", addTelegramBot)
		api.GET("/telegram-bot/:user_id", getTelegramByUserID)

		api.POST("/alerts", createAlert)
		api.GET("/alerts", listAlerts)
		api.DELETE("/alerts/:id", deleteAlert)
	}

	return r
}

// allowedOrigins reads CORS_ALLOWED_ORIGINS (comma-separated) and falls back to
// "*" when it is unset — convenient for local development.
func allowedOrigins() []string {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		return []string{"*"}
	}

	origins := make([]string, 0)
	for p := range strings.SplitSeq(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			origins = append(origins, p)
		}
	}
	if len(origins) == 0 {
		return []string{"*"}
	}
	return origins
}
