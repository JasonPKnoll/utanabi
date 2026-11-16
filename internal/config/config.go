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
	"strconv"
	"strings"
	"time"
)

// AppName is the product name used for folders and the window title.
const AppName = "Utanabi"

// SettingsFile is the name of the user settings file inside Dirs.Config.
const SettingsFile = "settings.json"

// version is set at build time: -ldflags "-X github.com/JasonPKnoll/utanabi/internal/config.version=1.2.3".
var version = "dev"

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
	Server        Server
}

// Server holds the settings only server mode uses.
type Server struct {
	// AllowedOrigins become Wails's WebSocketOriginPatterns, for a reverse proxy's public origin.
	AllowedOrigins []string
}

// settings mirrors the user settings file. Every field is optional.
type settings struct {
	LogLevel      string `json:"log_level"`
	DBBusyTimeout string `json:"db_busy_timeout"`
}

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
}

func loadServer(getenv func(string) string) (Server, error) {
	// Wails reads WAILS_SERVER_PORT itself but silently ignores a bad value, so check it here.
	if v := getenv("WAILS_SERVER_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return Server{}, fmt.Errorf("config: invalid WAILS_SERVER_PORT (%q is not a port number)", v)
		}
	}

	var srv Server
	for origin := range strings.SplitSeq(getenv("UTANABI_ALLOWED_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			return Server{}, errors.New("config: invalid UTANABI_ALLOWED_ORIGINS (a bare * allows every origin)")
		}
		srv.AllowedOrigins = append(srv.AllowedOrigins, origin)
	}
	return srv, nil
}
