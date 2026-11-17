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
	"github.com/JasonPKnoll/utanabi/internal/features/songs"
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
	sources, err := lyricsSources(cfg)
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
	if err := db.Migrate(ctx, store.Migrations()); err != nil {
		return errors.Join(err, db.Close())
	}

	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return fmt.Errorf("load frontend assets: %w", err)
	}

	a := app.New(cfg, log, dist,
		application.NewService(app.NewAboutService(cfg)),
		application.NewService(songs.NewService(songs.NewSQLite(db), sources...)),
		app.CloseOnShutdown(db),
	)

	log.InfoContext(ctx, "starting", slog.String("version", cfg.Version), slog.String("platform", cfg.Platform),
		slog.Any("lyrics_sources", cfg.LyricsSources))
	if err := a.Run(); err != nil {
		return fmt.Errorf("run app: %w", err)
	}
	return nil
}

// lyricsSources builds the online lyrics sources that config turns on, in order. The lyrics_sources setting is the
// switch: with it empty, the app sends nothing to any lyrics service.
func lyricsSources(cfg config.Config) ([]songs.LyricsSource, error) {
	sources := make([]songs.LyricsSource, 0, len(cfg.LyricsSources))
	for _, name := range cfg.LyricsSources {
		switch name {
		case "lrclib":
			sources = append(sources, songs.NewLRCLIB(songs.LRCLIBURL, cfg.Version))
		default:
			return nil, fmt.Errorf("build lyrics sources: config allows %q but main.go has no adapter for it", name)
		}
	}
	return sources, nil
}
