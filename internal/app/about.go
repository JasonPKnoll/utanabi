package app

import "github.com/JasonPKnoll/utanabi/internal/config"

// About describes the running app.
type About struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
}

// AboutService is the example bound service. Every exported method here is callable by the frontend.
type AboutService struct {
	about About
}

// NewAboutService returns the service for the given config.
func NewAboutService(cfg config.Config) *AboutService {
	return &AboutService{about: About{Name: cfg.AppName, Version: cfg.Version, Platform: cfg.Platform}}
}

// Info returns the app's name, version, and platform.
func (s *AboutService) Info() About {
	return s.about
}
