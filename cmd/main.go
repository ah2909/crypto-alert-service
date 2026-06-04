package main

import (
	"context"
	"log"

	"github.com/ah2909/crypto-alert-engine/config"
	"github.com/ah2909/crypto-alert-engine/internal/api"
	"github.com/ah2909/crypto-alert-engine/internal/engine"
	"github.com/ah2909/crypto-alert-engine/internal/notifier"
	"github.com/ah2909/crypto-alert-engine/internal/storage"
)

func main() {
	// 1. Load config and connect to infrastructure.
	cfg := config.Load()
	storage.InitMySQL(cfg.MySQLDSN)
	storage.InitRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)

	// 2. Start the alert engine. It reloads alerts from the database on an
	//    interval, subscribes to the price feed for each symbol that has active
	//    alerts, and delivers notifications through each alert owner's own bot.
	registry := notifier.NewRegistry()
	eng := engine.New(registry)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go eng.Run(ctx)

	// 3. Serve the REST API so users can register themselves and their alerts.
	r := api.NewRouter()
	log.Printf("Starting API server on %s", cfg.APIPort)
	if err := r.Run(cfg.APIPort); err != nil {
		log.Fatalf("API server failed: %v", err)
	}
}
