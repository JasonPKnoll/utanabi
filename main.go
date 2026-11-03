package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/JasonPKnoll/utanabi/internal/app"
	"github.com/JasonPKnoll/utanabi/internal/config"
	"github.com/JasonPKnoll/utanabi/internal/logger"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "utanabi:", err)
		os.Exit(1)
	}
}

func run() (err error) {
	platform, err := app.Platform()
	if err != nil {
		return err
	}
	cfg, err := config.Load(platform)
	if err != nil {
		return err
	}

	log, logFile, err := logger.Open(cfg.LogDir, cfg.LogLevel)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, logFile.Close()) }()
	slog.SetDefault(log)
	ctx := logger.Into(context.Background(), log)

	db, err := store.Open(ctx, cfg.DBPath, cfg.DBBusyTimeout)
	if err != nil {
		return err
	}

	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return fmt.Errorf("load frontend assets: %w", err)
	}

	a := app.New(cfg, log, dist,
		application.NewService(app.NewAboutService(cfg)),
		app.CloseOnShutdown(db),
	)

	log.InfoContext(ctx, "starting", slog.String("version", cfg.Version), slog.String("platform", cfg.Platform))
	if err := a.Run(); err != nil {
		return fmt.Errorf("run app: %w", err)
	}
	return nil
}
