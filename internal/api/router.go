package api

import (
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/ah2909/crypto-alert-engine/config"
)

func NewRouter() *gin.Engine {
	r := gin.Default()

	// We run without a reverse proxy, so trust no proxy headers: c.ClientIP()
	// uses the real TCP remote address and forged X-Forwarded-For is ignored.
	// If you later deploy behind nginx/a load balancer, set this to its IP(s).
	_ = r.SetTrustedProxies(nil)

	corsCfg := cors.DefaultConfig()
	corsCfg.AllowOrigins = allowedOrigins()
	corsCfg.AllowMethods = []string{"GET", "POST", "DELETE", "OPTIONS"}
	corsCfg.AllowHeaders = []string{"Origin", "Content-Type"}
	r.Use(cors.New(corsCfg))

	r.GET("/health", func(c *gin.Context) {
		respondData(c, 200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	api.Use(AuthMiddleware()) // all /api routes require a valid RS256 bearer token
	{
		api.POST("/telegram-bot", addTelegramBot)
		api.GET("/telegram-bot", getTelegramByUserID)

		api.POST("/alerts", createAlert)
		api.GET("/alerts", listAlerts)
		api.DELETE("/alerts/:id", deleteAlert)
	}

	return r
}

// allowedOrigins reads CORS_ALLOWED_ORIGINS (comma-separated) and falls back to
// "*" when it is unset — convenient for local development.
func allowedOrigins() []string {
	raw := config.GetEnv("CORS_ALLOWED_ORIGINS", "")
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
