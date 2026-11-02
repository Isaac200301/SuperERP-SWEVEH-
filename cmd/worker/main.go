// Command worker despacha el outbox: avisos de WhatsApp, automatizaciones, agente y webhooks.
// Es la misma base de código que el api, en otro proceso (docs/06 §4.5).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"sweveh/internal/outbox"
	"sweveh/internal/platform/clock"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/logging"
	"sweveh/internal/platform/storage"
	"sweveh/internal/server"
	"sweveh/internal/wire"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadWorker()
	if err != nil {
		return fmt.Errorf("configuración inválida:\n%w", err)
	}
	var lvl slog.Level
	_ = lvl.UnmarshalText([]byte(cfg.LogLevel))
	log := logging.New(os.Stdout, lvl)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, db.PoolConfig{URL: cfg.DatabaseURL, MaxConns: 6})
	if err != nil {
		return err
	}
	defer pool.Close()

	clk := clock.System{}
	store, err := storage.NewLocal(os.Getenv("STORAGE_DIR") + "/../fotos-worker")
	if err != nil {
		return err
	}
	m := server.Build(db.NewTxManager(pool), store, clk, log)
	s := wire.Build(m, cfg.WhatsApp, cfg.AI)
	d := outbox.NewDispatcher(pool, clk, log, cfg.BatchSize)
	wire.Register(d, s)

	log.Info("worker despachando outbox y webhooks", "cada", cfg.PollInterval.String())
	go wire.Loops(ctx, pool, s, log)
	d.Run(ctx, cfg.PollInterval)
	log.Info("worker detenido")
	return nil
}
