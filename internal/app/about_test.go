package app

import (
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/config"
)

func TestAboutServiceInfo(t *testing.T) {
	t.Parallel()

	s := NewAboutService(config.Config{AppName: "Utanabi", Version: "1.2.3", Platform: "desktop"})
	want := About{Name: "Utanabi", Version: "1.2.3", Platform: "desktop"}
	if got := s.Info(); got != want {
		t.Errorf("Info() = %+v, want %+v", got, want)
	}
}
