//go:build !ios && !android

package app

import "github.com/wailsapp/wails/v3/pkg/application"

func platformOptions(*application.Options) {}
