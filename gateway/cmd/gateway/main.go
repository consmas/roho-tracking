package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/roho/telematics/gateway/internal/auth"
	"github.com/roho/telematics/gateway/internal/capture"
	"github.com/roho/telematics/gateway/internal/commands"
	"github.com/roho/telematics/gateway/internal/config"
	"github.com/roho/telematics/gateway/internal/device"
	"github.com/roho/telematics/gateway/internal/observability"
	"github.com/roho/telematics/gateway/internal/protocol"
	"github.com/roho/telematics/gateway/internal/publish"
	"github.com/roho/telematics/gateway/internal/server"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger, err := observability.NewLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer redisClient.Close()

	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)

	registry := device.NewRegistry(redisClient, cfg.SessionTTL)
	manager := device.NewConnectionManager(cfg.GatewayID, registry, metrics, logger)
	authService := auth.NewService(
		redisClient,
		cfg.DeviceLookupURL,
		cfg.InternalAPIToken,
		cfg.AuthCacheTTL,
		cfg.AuthNegativeTTL,
		cfg.AuthLookupTimeout,
		logger,
	)
	captureManager := capture.NewManager(cfg.CaptureEnabled, cfg.CaptureDir, cfg.CaptureFrames, logger)
	parser := protocol.NewParser(cfg.Protocol)
	publisher := publish.NewPublisher(redisClient, cfg.EventsStream)

	tcpServer := server.NewTCPServer(cfg, authService, captureManager, parser, manager, publisher, metrics, logger)
	cmdConsumer := commands.NewConsumer(redisClient, cfg.CommandsStream, cfg.GatewayGroup, cfg.GatewayID, cfg.CommandResults, manager, registry, parser, metrics, logger)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	obsServer := &http.Server{Addr: cfg.MetricsAddr, Handler: mux, ReadHeaderTimeout: 3 * time.Second}

	go func() {
		if err := obsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("metrics server failed", zap.Error(err))
		}
	}()

	go func() {
		if err := cmdConsumer.Run(ctx); err != nil {
			logger.Fatal("command consumer failed", zap.Error(err))
		}
	}()

	if err := tcpServer.ListenAndServe(ctx); err != nil {
		logger.Fatal("tcp server failed", zap.Error(err))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	manager.CloseAll(shutdownCtx)
	_ = obsServer.Shutdown(shutdownCtx)
}
