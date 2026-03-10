package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"veritie.io/internal/config"
	"veritie.io/internal/obs"
	"veritie.io/internal/runtime"
)

func main() {
	// Init core config and deps
	cfg, err := config.LoadFromEnv("worker")
	if err != nil {
		fmt.Fprintf(os.Stderr, "worker startup failed: %v\n", err)
		os.Exit(1)
	}

	logger, err := obs.NewLogger(cfg.Obs.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "worker logger init failed: %v\n", err)
		os.Exit(1)
	}

	metrics := obs.NewNoopMetrics()
	tracing, err := obs.InitTracing(cfg.Obs, logger)
	if err != nil {
		logger.Error("failed to initialize tracing", "error", err)
		os.Exit(1)
	}

	buildInfo := runtime.CurrentBuildInfo()
	logger.Info("worker process startup",
		"service", cfg.Service,
		"mode", cfg.App.Mode,
		"worker_concurrency", cfg.App.WorkerConcurrency,
		"version", buildInfo.Version,
		"commit", buildInfo.Commit,
		"build_time", buildInfo.BuildTime,
	)

	// ctx init
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Log shutdown
	logger.Info("worker process shutdown started")
	if err := tracing.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown tracing", "error", err)
	}
	if err := metrics.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown metrics", "error", err)
	}
	logger.Info("worker process shutdown complete")
}
