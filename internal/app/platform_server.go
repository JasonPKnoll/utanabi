//go:build server

package app

import (
	"os"

	"github.com/JasonPKnoll/utanabi/internal/config"
)

// Platform returns the server-mode folders. Server mode reads env vars and logs to stderr.
func Platform() (config.Platform, error) {
	data := os.Getenv("UTANABI_DATA_DIR")
	if data == "" {
		data = "data"
	}
	cfgDir := os.Getenv("UTANABI_CONFIG_DIR")
	if cfgDir == "" {
		cfgDir = data
	}
	return config.Platform{
		Name:   "server",
		Dirs:   config.Dirs{Config: cfgDir, Data: data},
		Getenv: os.Getenv,
	}, nil
}
