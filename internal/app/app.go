// Package app is the Wails wiring: application options, the main window, the platform folders,
// and the one mapping from domain errors to the errors the frontend sees.
package app

import (
	"context"
	"io/fs"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/JasonPKnoll/utanabi/internal/config"
	"github.com/JasonPKnoll/utanabi/internal/logger"
)

// New builds the Wails application and its main window. assets is the built frontend (frontend/dist).
func New(cfg config.Config, log *slog.Logger, assets fs.FS, services ...application.Service) *application.App {
	opts := application.Options{
		Name:        cfg.AppName,
		Description: "Learn Japanese through song lyrics",
		Services:    services,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Logger:       log.With(slog.String("component", "wails")),
		LogLevel:     cfg.LogLevel,
		MarshalError: marshalError(log),
		PanicHandler: func(d *application.PanicDetails) {
			log.LogAttrs(context.Background(), slog.LevelError, "panic", logger.Panic(d.Error, []byte(d.StackTrace)))
		},
		ErrorHandler: func(err error) {
			log.LogAttrs(context.Background(), slog.LevelError, "app error", logger.Err(err))
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Server: application.ServerOptions{
			Host:                    "localhost",
			WebSocketOriginPatterns: cfg.Server.AllowedOrigins,
		},
	}
	platformOptions(&opts)

	a := application.New(opts)
	a.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            cfg.AppName,
		Width:            1000,
		Height:           700,
		MinWidth:         360,
		MinHeight:        480,
		BackgroundColour: application.NewRGB(250, 248, 245),
		URL:              "/",
	})
	return a
}
