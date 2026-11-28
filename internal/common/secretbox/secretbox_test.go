package secretbox

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSealThenOpen(t *testing.T) {
	t.Parallel()

	box, err := New(bytes.Repeat([]byte{7}, KeySize))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sealed, err := box.Seal("youtube_api_key", []byte("AIza-secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(sealed, []byte("AIza-secret")) {
		t.Error("the sealed value shows the secret")
	}
	again, err := box.Seal("youtube_api_key", []byte("AIza-secret"))
	if err != nil || bytes.Equal(sealed, again) {
		t.Error("sealing the same value twice gave the same bytes; each needs a fresh nonce")
	}
	got, err := box.Open("youtube_api_key", sealed)
	if err != nil || string(got) != "AIza-secret" {
		t.Errorf("Open = %q, %v", got, err)
	}
}

func TestOpenRefuses(t *testing.T) {
	t.Parallel()

	box, _ := New(bytes.Repeat([]byte{7}, KeySize))
	other, _ := New(bytes.Repeat([]byte{8}, KeySize))
	sealed, err := box.Seal("youtube_api_key", []byte("AIza-secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	changed := bytes.Clone(sealed)
	changed[len(changed)-1] ^= 1
	tests := map[string]func() ([]byte, error){
		"another name":   func() ([]byte, error) { return box.Open("spotify_token", sealed) },
		"another key":    func() ([]byte, error) { return other.Open("youtube_api_key", sealed) },
		"a changed byte": func() ([]byte, error) { return box.Open("youtube_api_key", changed) },
		"too short":      func() ([]byte, error) { return box.Open("youtube_api_key", sealed[:5]) },
	}
	for name, open := range tests {
		if _, err := open(); !errors.Is(err, ErrOpen) {
			t.Errorf("%s: err = %v, want ErrOpen", name, err)
		}
	}
}

func TestLoadOrCreateMakesTheKeyOnceAndPrivately(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "secret.key")
	first, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key file mode = %v, want readable by its owner only", info.Mode().Perm())
	}
	sealed, err := first.Seal("n", []byte("v"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	second, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate again: %v", err)
	}
	if got, err := second.Open("n", sealed); err != nil || string(got) != "v" {
		t.Errorf("the key read back doesn't open what the first sealed: %q, %v", got, err)
	}
}

func TestLoadOrCreateRefusesABadKeyFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "secret.key")
	if err := os.WriteFile(path, []byte("not a key\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadOrCreate(path); err == nil {
		t.Error("LoadOrCreate took a key file that isn't a key")
	}
}

func TestNewRefusesAShortKey(t *testing.T) {
	t.Parallel()

	if _, err := New([]byte("short")); err == nil {
		t.Error("New took a 5-byte key")
	}
}
