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
		logger.Error("load config", "error", err)
	}

	logger.Info("config loaded", "addr", cfg.HTTPAddr, "shutdown_timeout", cfg.ShutdownTimeout)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// TODO 1 создать пулл соеденений/ пока к бд не подключаемся.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}

	defer pool.Close()

	//шаг 3 - отдедбный конекст на проверку бд даем максимум 5 секунд
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	//реально идем в БД
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	//   - залогировать "connected to postgres" (без URL!)
	logger.Info("Connected to postgкes")

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

	// TODO 2: создать http.Server с Addr, Handler, ReadHeaderTimeout
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)

	// TODO 3: запустить srv.ListenAndServe() в отдельной горутине
	//         (http.ErrServerClosed — нормальное завершение, остальные ошибки передаем в канал)
	go func() {
		logger.Info("server started", "addr", srv.Addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// TODO 3: select — ждём ctx.Done() ИЛИ ошибку из errCh
	//   - ошибка → return fmt.Errorf("http server: %w", err)
	select {
	case <-ctx.Done(): // Ctrl+C / SIGTERM
		// идём в graceful shutdown
	case err := <-errCh: // ListenAndServe вернул ошибку
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
