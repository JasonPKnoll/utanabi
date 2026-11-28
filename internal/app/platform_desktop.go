//go:build !server && !ios && !android

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/JasonPKnoll/utanabi/internal/config"
)

// Platform returns the desktop OS folders. Desktop builds read no env vars for config.
func Platform() (config.Platform, error) {
	dirs, err := desktopDirs(runtime.GOOS, os.Getenv)
	if err != nil {
		return config.Platform{}, err
	}
	// Without the executable's path there are no bundled files to look for, which the app runs without.
	if exe, err := os.Executable(); err == nil {
		dirs.Bundled = bundledDir(runtime.GOOS, exe)
	}
	return config.Platform{Name: "desktop", Dirs: dirs}, nil
}

// bundledDir is where the files that ship with the app are: a Mac app's Contents/Resources, beside the executable
// in Contents/MacOS, and elsewhere the executable's own folder.
func bundledDir(goos, exe string) string {
	if goos == "darwin" {
		return filepath.Join(filepath.Dir(filepath.Dir(exe)), "Resources")
	}
	return filepath.Dir(exe)
}

// desktopDirs follows each OS's convention. getenv only resolves the folders, never settings.
func desktopDirs(goos string, getenv func(string) string) (config.Dirs, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config.Dirs{}, fmt.Errorf("app: find home folder failed (%v)", err)
	}
	name := config.AppName

	switch goos {
	case "darwin":
		support := filepath.Join(home, "Library", "Application Support", name)
		return config.Dirs{Config: support, Data: support, Log: filepath.Join(home, "Library", "Logs", name)}, nil
	case "windows":
		roaming := getenv("APPDATA")
		local := getenv("LOCALAPPDATA")
		if roaming == "" || local == "" {
			return config.Dirs{}, fmt.Errorf("app: missing APPDATA or LOCALAPPDATA")
		}
		return config.Dirs{
			Config: filepath.Join(roaming, name),
			Data:   filepath.Join(roaming, name),
			Log:    filepath.Join(local, name, "logs"),
		}, nil
	default:
		lower := strings.ToLower(name)
		return config.Dirs{
			Config: filepath.Join(xdg(getenv, "XDG_CONFIG_HOME", filepath.Join(home, ".config")), lower),
			Data:   filepath.Join(xdg(getenv, "XDG_DATA_HOME", filepath.Join(home, ".local", "share")), lower),
			Log:    filepath.Join(xdg(getenv, "XDG_STATE_HOME", filepath.Join(home, ".local", "state")), lower, "logs"),
		}, nil
	}
}

func xdg(getenv func(string) string, key, fallback string) string {
	if v := getenv(key); filepath.IsAbs(v) {
		return v
	}
	return fallback
}
