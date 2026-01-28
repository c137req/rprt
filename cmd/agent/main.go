package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/reverseproxy/internal/agent"
)

func main() {
	configPath := flag.String("config", "configs/agent.yaml", "path to agent configuration file")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	a, err := agent.New(cfg)
	if err != nil {
		slog.Error("failed to create agent", "err", err)
		os.Exit(1)
	}

	slog.Info("agent starting")
	if err := a.Run(ctx); err != nil && ctx.Err() == nil {
		slog.Error("agent exited with error", "err", err)
		os.Exit(1)
	}
	slog.Info("agent stopped")
}
