// Package secretbox seals small secrets, such as an API key, for keeping in the database: AES-256-GCM, under a key
// the server keeps in a file of its own. A copy of the database alone, such as a backup, then doesn't give them away.
// Each value is sealed for a name, so a sealed value can't be moved to stand for another.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// KeySize is the length of a box's key: 32 bytes, for AES-256.
const KeySize = 32

// ErrOpen means a sealed value didn't open: it was sealed with another key or for another name, or it was changed.
var ErrOpen = errors.New("secretbox: can't open sealed value")

// Box seals and opens values with one key.
type Box struct {
	aead cipher.AEAD
}

// New returns a box with key, which is KeySize bytes.
func New(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("secretbox: key must be %d bytes", KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secretbox: cipher (%v)", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: gcm (%v)", err)
	}
	return &Box{aead: aead}, nil
}

// LoadOrCreate returns the box whose key is in the file at path, making the file with a new random key, readable
// only by its owner, when there's none yet. The key is written in hex.
func LoadOrCreate(path string) (*Box, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return create(path)
	}
	if err != nil {
		return nil, fmt.Errorf("secretbox: read key file (%v)", err)
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != KeySize {
		return nil, fmt.Errorf("secretbox: key file isn't a 32-byte key in hex (%s)", path)
	}
	return New(key)
}

func create(path string) (*Box, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("secretbox: make key (%v)", err)
	}
	// O_EXCL: if another process made the file meanwhile, its key wins, and this one is read instead.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return LoadOrCreate(path)
	}
	if err != nil {
		return nil, fmt.Errorf("secretbox: create key file (%v)", err)
	}
	_, werr := f.WriteString(hex.EncodeToString(key) + "\n")
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return nil, fmt.Errorf("secretbox: write key file (%v)", werr)
	}
	return New(key)
}

// Seal seals plain for name: a fresh random nonce, then the sealed bytes.
func (b *Box) Seal(name string, plain []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("secretbox: make nonce (%v)", err)
	}
	return b.aead.Seal(nonce, nonce, plain, []byte(name)), nil
}

// Open opens a value sealed for name, or returns ErrOpen.
func (b *Box) Open(name string, sealed []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n+b.aead.Overhead() {
		return nil, ErrOpen
	}
	plain, err := b.aead.Open(nil, sealed[:n], sealed[n:], []byte(name))
	if err != nil {
		return nil, ErrOpen
	}
	return plain, nil
}
