package app

import (
	"errors"
	"io"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// closers releases resources when the app quits. It has no exported methods besides
// ServiceShutdown, which Wails never binds, so the frontend cannot call it.
type closers struct {
	list []io.Closer
}

// CloseOnShutdown returns a service that closes each resource, last one first, when the app quits.
func CloseOnShutdown(list ...io.Closer) application.Service {
	return application.NewService(&closers{list: list})
}

func (c *closers) ServiceShutdown() error {
	errs := make([]error, 0, len(c.list))
	for i := len(c.list) - 1; i >= 0; i-- {
		errs = append(errs, c.list[i].Close())
	}
	return errors.Join(errs...)
}
