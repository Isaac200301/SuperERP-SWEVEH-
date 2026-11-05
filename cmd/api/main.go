// Command api es el proceso HTTP de SWEVEH (panel, piso y API).
package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sweveh/internal/identity/adapters/out/otp"
	identityports "sweveh/internal/identity/ports"
	"sweveh/internal/platform/authn"
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
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadAPI()
	if err != nil {
		return fmt.Errorf("configuración inválida:\n%w", err)
	}
	log := logging.New(os.Stdout, parseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, db.PoolConfig{
		URL: cfg.DatabaseURL, MaxConns: cfg.DBMaxConns, MaxConnLifetime: cfg.DBMaxLifetime, SimpleProtocol: cfg.DBSimpleProtocol,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	key, err := signingKey(cfg, log)
	if err != nil {
		return err
	}
	clk := clock.System{}
	signer := authn.NewSigner(key, cfg.JWTKeyID, clk)
	verifier := authn.NewVerifier(map[string]*ecdsa.PublicKey{cfg.JWTKeyID: &key.PublicKey}, clk)

	var sender identityports.OTPSender
	switch cfg.OTPSender {
	case "consola":
		sender = otp.NewConsole(os.Stderr)
	case "memoria":
		sender = otp.NewMemory()
	case "waha": // código por el WhatsApp de prueba (ADR-0034, solo demo)
		sender = wire.NewWAHAOTP(cfg.WhatsApp.WAHA)
	}

	store, err := storage.NewLocal(cfg.StorageDir)
	if err != nil {
		return fmt.Errorf("almacenamiento de fotos: %w", err)
	}
	if cfg.RunWorker { // ADR-0036: el worker en el mismo proceso, para la demo en un solo servicio
		stopWorker, err := startWorker(ctx, cfg, store, clk, log)
		if err != nil {
			return err
		}
		defer stopWorker()
	}
	handler := server.New(server.Deps{
		Storage: store, Extra: wire.Extra(cfg), Public: wire.Public(pool, cfg, log),
		Pool: pool, Log: log, Clock: clk, Signer: signer, Verifier: verifier, OTPSender: sender,
		OTPPepper: []byte(cfg.OTPPepper), AllowedOrigins: cfg.AllowedOrigins, TrustProxy: cfg.TrustProxy,
		HSTS: cfg.Env != config.EnvLocal, WebDir: cfg.WebDir, LoginPerMinute: cfg.LoginPerMinute,
		PlatformAdmins: cfg.Platform.Admins,
	})
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           http.TimeoutHandler(handler, 15*time.Second, `{"code":"tiempo_agotado"}`),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api escuchando", "addr", cfg.HTTPAddr, "entorno", string(cfg.Env))
		errCh <- srv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	log.Info("apagando api")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// startWorker abre el pool del rol sweveh_worker (pocas conexiones, para no pasarse del pooler) y corre el worker en
// segundo plano con el mismo almacenamiento del api. La función que devuelve lo detiene y espera a que termine antes de
// cerrar su pool.
func startWorker(ctx context.Context, cfg config.API, store storage.Store, clk clock.Clock, log *slog.Logger) (func(), error) {
	pool, err := db.NewPool(ctx, db.PoolConfig{URL: cfg.WorkerDatabaseURL, MaxConns: 4, MaxConnLifetime: cfg.DBMaxLifetime,
		SimpleProtocol: cfg.DBSimpleProtocol})
	if err != nil {
		return nil, fmt.Errorf("base del worker: %w", err)
	}
	wctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		wire.StartWorker(wctx, pool, store, clk, log.With("proceso", "worker"),
			wire.WorkerConfig{WhatsApp: cfg.WhatsApp, AI: cfg.AI, Poll: cfg.PollInterval, Batch: cfg.BatchSize})
	}()
	return func() {
		cancel()
		<-done
		pool.Close()
	}, nil
}

func signingKey(cfg config.API, log *slog.Logger) (*ecdsa.PrivateKey, error) {
	if cfg.JWTPrivateKeyPEM != "" {
		return authn.ParsePrivateKeyPEM([]byte(cfg.JWTPrivateKeyPEM))
	}
	// Solo llega aquí en local (config lo exige fuera de local): las sesiones mueren al reiniciar.
	log.Warn("JWT_PRIVATE_KEY_PEM vacía: se usa una clave efímera de desarrollo")
	return authn.GenerateDevKey()
}

func parseLevel(s string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return l
}
