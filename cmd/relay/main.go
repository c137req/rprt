package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/reverseproxy/internal/relay"
)

func main() {
	configPath := flag.String("config", "configs/relay.yaml", "path to relay configuration file")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := relay.LoadConfig(*configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	server := relay.NewServer(cfg)
	if err := server.Run(); err != nil {
		slog.Error("relay server exited with error", "err", err)
		os.Exit(1)
	}
}
