package httpx

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func NewLogger(level string) *zap.Logger {
	cfg := zap.NewProductionConfig()
	if level == "debug" {
		cfg = zap.NewDevelopmentConfig()
	}
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

func NewNopLogger() *zap.Logger { return zap.NewNop() }

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r)
	})
}

func Recover(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						zap.Any("panic", rec),
						zap.String("path", r.URL.Path),
					)
					http.Error(w, "internal error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func RequestLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Info("http request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Duration("elapsed", time.Since(start)),
			)
		})
	}
}
