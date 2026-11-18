package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/asamigentoku/PinguCoin/services/nginx-metrics/internal/exchange"
)

const defaultLogPath = "/var/cache/nginx/exchange-rate-metrics.log"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logPath := envOr("NGINX_EXCHANGE_RATE_LOG", defaultLogPath)
	addr := ":" + envOr("PORT", "9113")
	metrics := exchange.NewMetrics(prometheus.DefaultRegisterer)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go follow(ctx, logPath, metrics, logger)

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info("nginx exchange-rate metrics exporter listening", "addr", addr, "log_path", logPath)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("metrics server stopped", "error", err)
		os.Exit(1)
	}
}

func follow(ctx context.Context, path string, metrics *exchange.Metrics, logger *slog.Logger) {
	for ctx.Err() == nil {
		file, err := os.Open(path)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				continue
			}
		}
		readUntilClosed(ctx, file, metrics, logger)
		_ = file.Close()
	}
}

func readUntilClosed(ctx context.Context, reader io.Reader, metrics *exchange.Metrics, logger *slog.Logger) {
	buffered := bufio.NewReader(reader)
	pending := ""
	for ctx.Err() == nil {
		chunk, err := buffered.ReadString('\n')
		pending += chunk
		if strings.HasSuffix(pending, "\n") {
			if record, parseErr := exchange.ParseLine(pending); parseErr != nil {
				logger.Warn("ignored malformed nginx metrics log", "error", parseErr)
			} else {
				metrics.Observe(record)
			}
			pending = ""
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, io.EOF) {
			logger.Warn("nginx metrics log read failed", "error", err)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
