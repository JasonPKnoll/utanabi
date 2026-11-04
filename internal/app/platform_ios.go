//go:build ios && !server

package app

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/JasonPKnoll/utanabi/internal/config"
)

// Platform returns the app's sandbox storage folder. Logs go to stderr, which Xcode shows.
func Platform() (config.Platform, error) {
	dir := application.IOS.StoragePath()
	if dir == "" {
		return config.Platform{}, errors.New("app: missing iOS storage folder")
	}
	return config.Platform{Name: "ios", Dirs: config.Dirs{Config: dir, Data: dir}}, nil
}

func platformOptions(opts *application.Options) {
	// The default signal handler crashes on iOS.
	opts.DisableDefaultSignalHandler = true
}
