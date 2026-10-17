package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dborovsky/paygate/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger.Info("config loaded", "addr", cfg.HTTPAddr, "shutdown_timeout", cfg.ShutdownTimeout)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create the pool. It does not connect yet. Connections are opened lazily.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}

	defer pool.Close()

	// Give the startup DB check at most 5 seconds (also cancelled by Ctrl+C via ctx).
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	// Ping is the real check. Fail fast if the DB is unreachable.
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	logger.Info("Connected to postgres")

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		err := pool.Ping(r.Context())
		if err != nil {
			http.Error(w, "db not ready", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ready"))
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Buffered: the server goroutine can send its error and exit
	// even if nobody reads errCh anymore (no goroutine leak).
	errCh := make(chan error, 1)

	go func() {
		logger.Info("server started", "addr", srv.Addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// Wait for whichever comes first: a shutdown signal or a server crash.
	select {
	case <-ctx.Done(): // Ctrl+C / SIGTERM
		// continue to graceful shutdown below
	case err := <-errCh: // server failed to start or crashed
		return fmt.Errorf("http server: %w", err)
	}

	stop()
	logger.Info("shutdown signal received")

	ctxShutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("server stopped gracefully")
	return nil
}
