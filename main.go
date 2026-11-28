package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/JasonPKnoll/utanabi/internal/app"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
	"github.com/JasonPKnoll/utanabi/internal/config"
	"github.com/JasonPKnoll/utanabi/internal/dictionary"
	"github.com/JasonPKnoll/utanabi/internal/features/songs"
	"github.com/JasonPKnoll/utanabi/internal/features/study"
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
	closers := []io.Closer{db}

	dict, err := openDictionary(ctx, log, cfg.DictionaryPath)
	if err != nil {
		return errors.Join(err, db.Close())
	}
	if dict != nil {
		closers = append(closers, dict)
	}

	meanings, err := songs.NewJMdict(dict)
	if err != nil {
		return errors.Join(fmt.Errorf("load grammar list: %w", err), closeAll(closers))
	}

	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return errors.Join(fmt.Errorf("load frontend assets: %w", err), closeAll(closers))
	}

	credits := []app.Credit{
		{Name: "JMdict", Text: dictionary.Attribution()},
		{Name: "IPA dictionary", Text: morph.Attribution()},
	}
	a := app.New(cfg, log, dist,
		application.NewService(app.NewAboutService(cfg, credits...)),
		application.NewService(songs.NewService(songs.NewSQLite(db), songs.NewMorph(morph.New()), meanings,
			sources...)),
		application.NewService(study.NewService(study.NewSQLite(db), study.NewJMdict(dict), cfg.StudyStreak)),
		app.CloseOnShutdown(closers...),
	)

	log.InfoContext(ctx, "starting", slog.String("version", cfg.Version), slog.String("platform", cfg.Platform),
		slog.Any("lyrics_sources", cfg.LyricsSources), slog.Bool("dictionary", dict != nil))
	if err := a.Run(); err != nil {
		return fmt.Errorf("run app: %w", err)
	}
	return nil
}

// openDictionary opens the read-only reference dictionary. Without the file the app still runs, with word meanings
// unavailable, so a missing file is one info line and no error. Any other failure, such as a file built for another
// schema version, stops startup: rebuild it with task dictionary.
func openDictionary(ctx context.Context, log *slog.Logger, path string) (*dictionary.Dictionary, error) {
	dict, err := dictionary.Open(ctx, path)
	if errors.Is(err, dictionary.ErrMissing) {
		log.InfoContext(ctx, "dictionary not found, so word meanings are unavailable", slog.String("path", path))
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open dictionary: %w", err)
	}
	return dict, nil
}

func closeAll(closers []io.Closer) error {
	errs := make([]error, 0, len(closers))
	for i := len(closers) - 1; i >= 0; i-- {
		errs = append(errs, closers[i].Close())
	}
	return errors.Join(errs...)
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
