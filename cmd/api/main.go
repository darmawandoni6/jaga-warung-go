package main

import (
	"log/slog"
	"os"

	"github.com/labstack/echo/v4"

	"jaga-warung-go/internal/config"
	"jaga-warung-go/internal/database"
	"jaga-warung-go/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect", "err", err)
		os.Exit(1)
	}
	if err := database.Migrate(db); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}
	if err := database.Seed(db, cfg); err != nil {
		slog.Error("seed", "err", err)
		os.Exit(1)
	}

	e := echo.New()
	router.Register(e, db, cfg)
	slog.Info("listening", "port", cfg.Port)
	if err := e.Start(":" + cfg.Port); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
