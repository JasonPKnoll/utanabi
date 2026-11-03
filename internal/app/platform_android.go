//go:build android && !server

package app

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/JasonPKnoll/utanabi/internal/config"
)

// Platform returns the app's private files folder. Logs go to stderr.
func Platform() (config.Platform, error) {
	dir := application.Android.StoragePath()
	if dir == "" {
		return config.Platform{}, errors.New("app: missing Android storage folder")
	}
	return config.Platform{Name: "android", Dirs: config.Dirs{Config: dir, Data: dir}}, nil
}

func platformOptions(*application.Options) {}
