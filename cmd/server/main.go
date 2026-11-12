package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/api"
	"github.com/ElzaraA/ai-buddy-avito/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// run поднимает сервер и останавливает его корректно (graceful shutdown) при отмене ctx.
func run(parent context.Context) error {
	config.LoadDotEnv(".env")
	cfg := config.Load()

	app, err := api.Build(cfg, cfg.KnowledgeDir)
	if err != nil {
		return err
	}

	// Состояние сохраняется на диск раз в 2 с (если менялось) и ещё раз при любой остановке.
	ctx, cancel := context.WithCancel(parent)
	var bg sync.WaitGroup
	defer bg.Wait() // выполняется после cancel(): ждём финального сохранения
	defer cancel()
	if app.Persister != nil {
		bg.Add(1)
		go func() {
			defer bg.Done()
			app.Persister.Run(ctx, 2*time.Second)
		}()
	}

	mode := "mock"
	if !cfg.LLMMock {
		mode = "real:" + cfg.LLMModel
	}
	slog.Info("starting", "addr", cfg.Addr, "llm", mode, "state_file", cfg.StateFile, "rate_limit_per_min", cfg.RateLimitPerMin)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout не задаём: он оборвёт SSE-поток.
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		slog.Info("shutting down")
		// SSE-потоки живут долго: Shutdown ждёт их до таймаута, затем закрываем принудительно.
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return srv.Close()
		}
		return nil
	}
}
