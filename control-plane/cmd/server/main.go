package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"tenara/control-plane/internal/config"
	"tenara/control-plane/internal/httpapi"
	"tenara/control-plane/internal/httpx"
	"tenara/control-plane/internal/pgstore"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := httpx.NewLogger(cfg.LogLevel)
	defer func() {
		//nolint:errcheck // sync on shutdown; nothing to do with the error
		_ = log.Sync()
	}()

	pool, err := pgstore.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("database unavailable", zap.Error(err))
	}
	defer pool.Close()

	router := chi.NewRouter()
	router.Use(httpx.RequestID)
	router.Use(httpx.Recover(log))
	router.Use(httpx.RequestLogger(log))

	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if pingErr := pool.Ping(pingCtx); pingErr != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	router.NotFound(httpapi.Handler().ServeHTTP)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		//nolint:contextcheck // parent ctx is already cancelled; shutdown needs a fresh deadline
		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Error("graceful shutdown failed", zap.Error(shutdownErr))
		}
	}()

	log.Info("control plane listening", zap.String("port", cfg.Port))
	if listenErr := srv.ListenAndServe(); !errors.Is(listenErr, http.ErrServerClosed) {
		log.Fatal("server error", zap.Error(listenErr))
	}
}
