package app

import (
	"reflect"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/config"
)

func TestAboutServiceInfo(t *testing.T) {
	t.Parallel()

	credit := Credit{Name: "JMdict", Text: "Word meanings come from JMdict."}
	s := NewAboutService(config.Config{AppName: "Utanabi", Version: "1.2.3", Platform: "desktop"}, credit)
	want := About{Name: "Utanabi", Version: "1.2.3", Platform: "desktop", Credits: []Credit{credit}}
	got := s.Info()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Info() = %+v, want %+v", got, want)
	}

	got.Credits[0].Name = "changed"
	if again := s.Info(); again.Credits[0].Name != "JMdict" {
		t.Error("changing the returned credits changed the service's own")
	}
}

func TestAboutServiceWithoutCreditsHasAnEmptyList(t *testing.T) {
	t.Parallel()

	got := NewAboutService(config.Config{AppName: "Utanabi"}).Info()
	if got.Credits == nil || len(got.Credits) != 0 {
		t.Errorf("Credits = %#v, want an empty list, so the frontend never gets null", got.Credits)
	}
}
