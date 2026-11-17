package app

import (
	"slices"

	"github.com/JasonPKnoll/utanabi/internal/config"
)

// About describes the running app, with the credits the licences of its data ask it to show.
type About struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Platform string   `json:"platform"`
	Credits  []Credit `json:"credits"`
}

// Credit is the attribution one source of the app's data asks for, such as JMdict for word meanings.
type Credit struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// AboutService is the bound service that describes the app. Every exported method here is callable by the frontend.
type AboutService struct {
	about About
}

// NewAboutService returns the service for the given config and credits.
func NewAboutService(cfg config.Config, credits ...Credit) *AboutService {
	return &AboutService{about: About{Name: cfg.AppName, Version: cfg.Version, Platform: cfg.Platform,
		Credits: append([]Credit{}, credits...)}}
}

// Info returns the app's name, version, platform, and credits.
func (s *AboutService) Info() About {
	about := s.about
	about.Credits = slices.Clone(s.about.Credits)
	return about
}
