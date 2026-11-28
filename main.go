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
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/JasonPKnoll/utanabi/internal/app"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
	"github.com/JasonPKnoll/utanabi/internal/common/secretbox"
	"github.com/JasonPKnoll/utanabi/internal/config"
	"github.com/JasonPKnoll/utanabi/internal/dictionary"
	"github.com/JasonPKnoll/utanabi/internal/dictionary/bundled"
	"github.com/JasonPKnoll/utanabi/internal/features/accounts"
	"github.com/JasonPKnoll/utanabi/internal/features/songs"
	"github.com/JasonPKnoll/utanabi/internal/features/spotify"
	"github.com/JasonPKnoll/utanabi/internal/features/study"
	"github.com/JasonPKnoll/utanabi/internal/features/youtube"
	"github.com/JasonPKnoll/utanabi/internal/identity"
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
	sources, timings, err := lyricsSources(cfg)
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

	dict, err := openDictionary(ctx, log, cfg)
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

	// In server mode each person signs in with a passkey and sees only their own songs and cards. The desktop and
	// phone apps have no accounts: everything there is the device's.
	owners := identity.OnDevice()
	var signIn application.Middleware
	var serverOnly []application.Service
	if cfg.Platform == "server" {
		owners = identity.Accounts()
		sites := accounts.SitesFor(cfg.Server.AllowedOrigins, cfg.Server.Port)
		ceremonies, err := accounts.NewWebAuthn(sites)
		if err != nil {
			return errors.Join(err, closeAll(closers))
		}
		users := accounts.NewSQLite(db)
		signIn = accounts.NewHTTP(users, ceremonies, sites, cfg.Server.SignupCode).Middleware
		serverOnly = append(serverOnly, application.NewService(accounts.NewService(users)))
	}

	// Spotify, for following along, needs a client ID and server mode: its sign-in sends people back to the
	// server's own address, and its tokens stay on the server. Its pages run inside sign-in, so they know who's
	// signed in.
	spotifyOn := cfg.Platform == "server" && cfg.Server.SpotifyClientID != ""
	spotifyRepo := spotify.NewSQLite(db, owners)
	spotifyAPI := spotify.NewWebAPI(spotify.AccountsURL, spotify.APIURL, cfg.Server.SpotifyClientID)
	if spotifyOn {
		bases := []string{fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)}
		for _, origin := range cfg.Server.AllowedOrigins {
			if !strings.Contains(origin, "*") {
				bases = append(bases, "https://"+origin)
			}
		}
		connect := spotify.NewHTTP(spotifyRepo, spotifyAPI, cfg.Server.SpotifyClientID, bases).Middleware
		signIn = application.ChainMiddleware(signIn, connect)
	}

	// YouTube plays in the page in server mode, where the page is served over the web. With an API key it can also
	// search for a song's video; without one, videos are pasted as links. A nil Search means no key.
	youtubeOn := cfg.Platform == "server"
	// The server's own key for secrets it keeps in the database, such as the YouTube API key an admin enters: made on
	// first start, next to the database, readable only by the server's user.
	secrets, err := secretbox.LoadOrCreate(filepath.Join(filepath.Dir(cfg.DBPath), "secret.key"))
	if err != nil {
		return fmt.Errorf("open secret key: %w", err)
	}
	youtubeRepo := youtube.NewSQLite(db, owners, secrets)

	// One analyzer for every slice that reads Japanese, so its dictionary loads once.
	analyzer := morph.New()
	credits := []app.Credit{
		{Name: "JMdict", Text: dictionary.Attribution()},
		{Name: "IPA dictionary", Text: morph.Attribution()},
	}
	if len(timings) > 0 {
		// CC0 asks for no credit, but the people who timed the songs earned one.
		credits = append(credits, app.Credit{Name: "AMLL TTML DB",
			Text: "Word times from the AMLL TTML DB (github.com/amll-dev/amll-ttml-db), timed by hand by its " +
				"contributors and shared under CC0 1.0."})
	}
	songsRepo, songsAnalyzer := songs.NewSQLite(db, owners), songs.NewMorph(analyzer)
	services := append([]application.Service{
		application.NewService(app.NewAboutService(cfg, credits...)),
		application.NewService(songs.NewService(songsRepo, songsAnalyzer, meanings, timings, sources...)),
		application.NewService(songs.NewTools(songs.Toolkit{Repo: songsRepo, Analyzer: songsAnalyzer, Dict: meanings,
			Sources: sources, Timings: timings}, cfg.Platform == "server" && cfg.Server.SongTools)),
		application.NewService(study.NewService(study.NewSQLite(db, owners), study.NewJMdict(dict), cfg.StudyStreak)),
		application.NewService(spotify.NewService(spotifyRepo, spotifyAPI, spotify.NewMorph(analyzer), spotifyOn)),
		application.NewService(youtube.NewService(youtubeRepo, youtubeRepo, cfg.Server.YouTubeAPIKey,
			youtube.NewDataAPI(youtube.APIURL), youtubeOn)),
	}, serverOnly...)
	a := app.New(cfg, log, dist, signIn, append(services, app.CloseOnShutdown(closers...))...)

	log.InfoContext(ctx, "starting", slog.String("version", cfg.Version), slog.String("platform", cfg.Platform),
		slog.Any("lyrics_sources", cfg.LyricsSources), slog.Bool("dictionary", dict != nil),
		slog.Bool("accounts", signIn != nil), slog.Bool("signup_open", !cfg.Server.SignupCode.IsZero()),
		slog.Bool("spotify", spotifyOn), slog.Bool("youtube", youtubeOn), slog.Bool("song_tools", cfg.Server.SongTools))
	if err := a.Run(); err != nil {
		return fmt.Errorf("run app: %w", err)
	}
	return nil
}

// openDictionary opens the read-only reference dictionary. Without the file the app still runs, with word meanings
// unavailable, so a missing file is one info line and no error. Any other failure, such as a file built for another
// schema version, stops startup: rebuild it with task dictionary.
// openDictionary opens the reference dictionary: the one at its path, else the copy in the app's bundle folder, else
// the one built into the app, written to its path first. A dictionary from an older version of the app is replaced by
// the one built in. Without any, the app runs with word meanings unavailable.
func openDictionary(ctx context.Context, log *slog.Logger, cfg config.Config) (*dictionary.Dictionary, error) {
	path := cfg.DictionaryPath
	dict, err := dictionary.Open(ctx, path)
	if errors.Is(err, dictionary.ErrMissing) && cfg.BundledDictionaryPath != "" {
		path = cfg.BundledDictionaryPath
		dict, err = dictionary.Open(ctx, path)
	}
	built := bundled.Data()
	if (errors.Is(err, dictionary.ErrMissing) || errors.Is(err, dictionary.ErrVersion)) && built != nil &&
		!cfg.DictionaryPathSet {
		path = cfg.DictionaryPath
		if err := writeFileAtomic(path, built); err != nil {
			return nil, fmt.Errorf("install the built-in dictionary: %w", err)
		}
		log.InfoContext(ctx, "installed the built-in dictionary", slog.String("path", path))
		dict, err = dictionary.Open(ctx, path)
	}
	if errors.Is(err, dictionary.ErrMissing) {
		log.InfoContext(ctx, "dictionary not found, so word meanings are unavailable", slog.String("path", path))
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open dictionary: %w", err)
	}
	return dict, nil
}

// writeFileAtomic writes data to path through a temporary file beside it, so a crash halfway leaves no half-written
// file for the next start to open.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dictionary-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		return errors.Join(err, os.Remove(tmp.Name()))
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return errors.Join(err, os.Remove(tmp.Name()))
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return errors.Join(err, os.Remove(tmp.Name()))
	}
	return nil
}

func closeAll(closers []io.Closer) error {
	errs := make([]error, 0, len(closers))
	for i := len(closers) - 1; i >= 0; i-- {
		errs = append(errs, closers[i].Close())
	}
	return errors.Join(errs...)
}

// lyricsSources builds the online lyrics sources that config turns on: the ones song search asks, in order, and the
// word timing databases. The lyrics_sources setting is the switch: with it empty, the app sends nothing to any lyrics
// service.
func lyricsSources(cfg config.Config) ([]songs.LyricsSource, []songs.TimingSource, error) {
	sources := make([]songs.LyricsSource, 0, len(cfg.LyricsSources))
	var timings []songs.TimingSource
	for _, name := range cfg.LyricsSources {
		switch name {
		case "lrclib":
			sources = append(sources, songs.NewLRCLIB(songs.LRCLIBURL, cfg.Version))
		case "amll":
			timings = append(timings, songs.NewAMLL(songs.AMLLURL, cfg.Version))
		default:
			return nil, nil, fmt.Errorf("build lyrics sources: config allows %q but main.go has no adapter for it", name)
		}
	}
	return sources, timings, nil
}
