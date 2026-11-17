package songs

import (
	"errors"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/app"
)

func TestServiceReturnsFrontendErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		call      func(s *Service) error
		wantCode  string
		wantField string
	}{
		{name: "missing song", wantCode: app.CodeNotFound, call: func(s *Service) error {
			_, err := s.Get(t.Context(), 7)
			return err
		}},
		{name: "missing title", wantCode: app.CodeInvalidInput, wantField: "title", call: func(s *Service) error {
			_, err := s.Add(t.Context(), NewSong{Lyrics: "星が光る"})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.call(NewService(newFakeRepo(), fakeAnalyzer{}, noMeanings(t)))

			var fe *app.Error
			if !errors.As(err, &fe) {
				t.Fatalf("err = %T %v, want *app.Error", err, err)
			}
			if fe.Code != tt.wantCode || fe.Field != tt.wantField {
				t.Errorf("got code %q field %q, want code %q field %q", fe.Code, fe.Field, tt.wantCode, tt.wantField)
			}
		})
	}
}

func TestServiceAddReturnsSavedSong(t *testing.T) {
	t.Parallel()

	got, err := NewService(newFakeRepo(), fakeAnalyzer{}, noMeanings(t)).Add(t.Context(), NewSong{Title: "星の歌", Lyrics: "星が光る"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got.ID != 1 || got.Source != SourceUser {
		t.Errorf("song = %+v, want ID 1 from the user", got)
	}
}
