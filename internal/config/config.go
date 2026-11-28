// Package config builds the one typed Config from build-time values, the user settings file, and,
// in server mode, env vars. Invalid config fails startup with an error that names the setting.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/redact"
)

// AppName is the product name used for folders and the window title.
const AppName = "Utanabi"

// SettingsFile is the name of the user settings file inside Dirs.Config.
const SettingsFile = "settings.json"

// version is set at build time: -ldflags "-X github.com/JasonPKnoll/utanabi/internal/config.version=1.2.3".
// Production builds stamp info.version from build/config.yml through VERSION_LDFLAG in the Taskfiles.
var version = "dev"

// defaultLyricsSources is the comma-separated list of online lyrics sources a build turns on when the user hasn't
// chosen, or "none". A build can ship with them off:
// -ldflags "-X github.com/JasonPKnoll/utanabi/internal/config.defaultLyricsSources=none".
var defaultLyricsSources = "lrclib,amll"

// LyricsSources returns the online lyrics sources this build has an adapter for: lrclib, which song search asks for
// lyrics, and amll, a database of songs timed word by word, which a song's word times are looked up in.
func LyricsSources() []string {
	return []string{"lrclib", "amll"}
}

// Dirs are the OS folders the app writes to. An empty Log means log to stderr.
type Dirs struct {
	Config string
	Data   string
	Log    string
}

// Platform describes where the app runs. internal/app builds it per build target.
type Platform struct {
	Name string
	Dirs Dirs
	// Getenv reads env vars. Only the server build passes os.Getenv; the others return "".
	Getenv func(string) string
}

// Config is the validated config for one run of the app.
type Config struct {
	AppName       string
	Version       string
	Platform      string
	LogLevel      slog.Level
	LogDir        string
	DBPath        string
	DBBusyTimeout time.Duration
	// DictionaryPath is the read-only reference dictionary, dictionary.db. The app runs without it, with word meanings
	// unavailable.
	DictionaryPath string
	// LyricsSources are the online lyrics sources that are on, in order. Empty means online search is off and
	// nothing is sent to any lyrics service.
	LyricsSources []string
	// StudyStreak is how many right answers in a row make a flashcard count as learned.
	StudyStreak int
	Server      Server
}

// Server holds the settings only server mode uses.
type Server struct {
	// AllowedOrigins become Wails's WebSocketOriginPatterns, for a reverse proxy's public origin. The exact hosts
	// among them are also the sites passkeys are made for, as well as localhost.
	AllowedOrigins []string
	// Port is the port the server listens on, which a passkey made on localhost is tied to.
	Port int
	// SignupCode is the code that creating an account asks for. Empty means sign-up is closed: no new accounts.
	SignupCode redact.Secret
	// SpotifyClientID is the app's client ID on Spotify's developer dashboard, for following along with Spotify.
	// Empty means Spotify is off. It isn't a secret: the app signs in with PKCE and has no client secret.
	SpotifyClientID string
	// YouTubeAPIKey is a YouTube Data API key, for finding a song's video by searching. Empty means search is off:
	// videos are pasted as links. It's a secret: it spends the key's quota, so it stays on the server.
	YouTubeAPIKey redact.Secret
	// SongTools turns on the admins' song tools, for testing: finding songs with word times, and removing a song from
	// every library to add it again fresh. Off unless UTANABI_SONG_TOOLS is on.
	SongTools bool
}

// settings mirrors the user settings file. Every field is optional.
type settings struct {
	LogLevel       string `json:"log_level"`
	DBBusyTimeout  string `json:"db_busy_timeout"`
	DictionaryPath string `json:"dictionary_path"`
	// A nil LyricsSources means the user hasn't chosen, so the build's default applies. An empty list turns every
	// online source off.
	LyricsSources *[]string `json:"lyrics_sources"`
	StudyStreak   *int      `json:"study_streak"`
}

// minSignupCode is the shortest sign-up code accepted, so it can't be guessed in a few tries.
const minSignupCode = 12

// Study streaks the settings accept, and the one that applies when none is set.
const (
	defaultStudyStreak = 3
	maxStudyStreak     = 10
)

// Load reads the settings file and env vars for the platform, and validates the result.
func Load(p Platform) (Config, error) {
	if p.Dirs.Data == "" {
		return Config{}, errors.New("config: missing data folder")
	}
	getenv := p.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}

	s, err := readSettings(filepath.Join(p.Dirs.Config, SettingsFile))
	if err != nil {
		return Config{}, err
	}
	overrideFromEnv(&s, getenv)

	cfg := Config{
		AppName:       AppName,
		Version:       version,
		Platform:      p.Name,
		LogLevel:      slog.LevelInfo,
		LogDir:        p.Dirs.Log,
		DBPath:        filepath.Join(p.Dirs.Data, "utanabi.db"),
		DBBusyTimeout: 5 * time.Second,
	}

	if s.LogLevel != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(s.LogLevel)); err != nil {
			return Config{}, fmt.Errorf("config: invalid log_level (%q is not debug, info, warn, or error)", s.LogLevel)
		}
	}
	if s.DBBusyTimeout != "" {
		d, err := time.ParseDuration(s.DBBusyTimeout)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("config: invalid db_busy_timeout (%q is not a positive duration)", s.DBBusyTimeout)
		}
		cfg.DBBusyTimeout = d
	}

	cfg.DictionaryPath, err = dictionaryPath(p.Dirs.Data, s.DictionaryPath)
	if err != nil {
		return Config{}, err
	}

	cfg.LyricsSources, err = lyricsSources(s.LyricsSources)
	if err != nil {
		return Config{}, err
	}

	cfg.StudyStreak, err = studyStreak(s.StudyStreak, getenv("UTANABI_STUDY_STREAK"))
	if err != nil {
		return Config{}, err
	}

	if p.Name == "server" {
		cfg.Server, err = loadServer(getenv)
		if err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func readSettings(path string) (settings, error) {
	var s settings
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("config: read settings file failed (%v)", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return s, fmt.Errorf("config: invalid settings file %s (%v)", path, err)
	}
	return s, nil
}

func overrideFromEnv(s *settings, getenv func(string) string) {
	if v := getenv("UTANABI_LOG_LEVEL"); v != "" {
		s.LogLevel = v
	}
	if v := getenv("UTANABI_DB_BUSY_TIMEOUT"); v != "" {
		s.DBBusyTimeout = v
	}
	if v := getenv("UTANABI_DICTIONARY_PATH"); v != "" {
		s.DictionaryPath = v
	}
	if v := getenv("UTANABI_LYRICS_SOURCES"); v != "" {
		list := splitList(v)
		s.LyricsSources = &list
	}
}

// studyStreak resolves the study_streak setting: the env var in server mode, then settings.json, then 3. It must be a
// whole number from 1 to 10.
func studyStreak(chosen *int, env string) (int, error) {
	bad := func(v any) error {
		return fmt.Errorf("config: invalid study_streak (%v is not a whole number from 1 to %d)", v, maxStudyStreak)
	}
	n := defaultStudyStreak
	switch {
	case env != "":
		parsed, err := strconv.Atoi(strings.TrimSpace(env))
		if err != nil {
			return 0, bad(strconv.Quote(env))
		}
		n = parsed
	case chosen != nil:
		n = *chosen
	}
	if n < 1 || n > maxStudyStreak {
		return 0, bad(n)
	}
	return n, nil
}

// dictionaryPath resolves the dictionary_path setting: dictionary.db in the data folder when unset, and a relative path
// is taken from the data folder, so it means the same thing on every platform.
func dictionaryPath(dataDir, path string) (string, error) {
	if path == "" {
		return filepath.Join(dataDir, "dictionary.db"), nil
	}
	if strings.ContainsRune(path, 0) || strings.TrimSpace(path) != path ||
		strings.HasSuffix(path, "/") || strings.HasSuffix(path, string(filepath.Separator)) {
		return "", fmt.Errorf("config: invalid dictionary_path (%q is not a file path)", path)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dataDir, path)
	}
	return filepath.Clean(path), nil
}

// lyricsSources resolves the lyrics_sources setting: the build's default when unset, nothing for "none" or an
// empty list, and otherwise the named sources, which must be ones this build has. Names are trimmed and lowercased,
// and blanks and repeats are ignored.
func lyricsSources(chosen *[]string) ([]string, error) {
	names := splitList(defaultLyricsSources)
	if chosen != nil {
		names = *chosen
	}

	var picked []string
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" && !slices.Contains(picked, name) {
			picked = append(picked, name)
		}
	}

	if slices.Contains(picked, "none") {
		if len(picked) > 1 {
			return nil, errors.New("config: invalid lyrics_sources (none can't be combined with a source)")
		}
		return []string{}, nil
	}
	known := LyricsSources()
	for _, name := range picked {
		if !slices.Contains(known, name) {
			return nil, fmt.Errorf("config: invalid lyrics_sources (%q is not one of %s, or none)",
				name, strings.Join(known, ", "))
		}
	}
	if picked == nil {
		return []string{}, nil
	}
	return picked, nil
}

func splitList(s string) []string {
	var list []string
	for item := range strings.SplitSeq(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			list = append(list, item)
		}
	}
	return list
}

// youtubeAPIKey is the shape of a Google API key.
var youtubeAPIKey = regexp.MustCompile(`^[0-9A-Za-z_-]{30,100}$`)

// spotifyClientID is the shape of a client ID on Spotify's developer dashboard.
var spotifyClientID = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

func loadServer(getenv func(string) string) (Server, error) {
	srv := Server{Port: 8080}
	// Wails reads WAILS_SERVER_PORT itself but silently ignores a bad value, so check it here.
	if v := getenv("WAILS_SERVER_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return Server{}, fmt.Errorf("config: invalid WAILS_SERVER_PORT (%q is not a port number)", v)
		}
		srv.Port = port
	}
	if code := getenv("UTANABI_SIGNUP_CODE"); code != "" {
		if len(code) < minSignupCode {
			return Server{}, fmt.Errorf("config: invalid UTANABI_SIGNUP_CODE (it must be at least %d characters)",
				minSignupCode)
		}
		srv.SignupCode = redact.New(code)
	}

	if id := getenv("UTANABI_SPOTIFY_CLIENT_ID"); id != "" {
		if !spotifyClientID.MatchString(id) {
			return Server{}, errors.New("config: invalid UTANABI_SPOTIFY_CLIENT_ID (it must be the 32-character " +
				"client ID from Spotify's developer dashboard)")
		}
		srv.SpotifyClientID = id
	}

	if key := getenv("UTANABI_YOUTUBE_API_KEY"); key != "" {
		if !youtubeAPIKey.MatchString(key) {
			return Server{}, errors.New("config: invalid UTANABI_YOUTUBE_API_KEY (it must be an API key from " +
				"Google Cloud's credentials page)")
		}
		srv.YouTubeAPIKey = redact.New(key)
	}

	switch v := strings.ToLower(strings.TrimSpace(getenv("UTANABI_SONG_TOOLS"))); v {
	case "", "off", "false", "0":
	case "on", "true", "1":
		srv.SongTools = true
	default:
		return Server{}, fmt.Errorf("config: invalid UTANABI_SONG_TOOLS (%q is not on or off)", v)
	}

	for _, origin := range splitList(getenv("UTANABI_ALLOWED_ORIGINS")) {
		if origin == "*" {
			return Server{}, errors.New("config: invalid UTANABI_ALLOWED_ORIGINS (a bare * allows every origin)")
		}
		srv.AllowedOrigins = append(srv.AllowedOrigins, origin)
	}
	return srv, nil
}
