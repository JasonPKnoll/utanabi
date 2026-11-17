// Package dictbuild builds dictionary.db from jmdict-simplified's JMdict JSON and the hand-written grammar list in
// internal/dictionary/grammar.json. It runs only at build time, through task dictionary, and never in the app.
package dictbuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/dictionary"
)

// Stats counts what a build wrote.
type Stats struct {
	Entries       int
	Forms         int
	Senses        int
	Tags          int
	GrammarPoints int
	// JMdictDate is the JMdict release the file was built from, such as 2026-11-16.
	JMdictDate string
}

// Build writes a new dictionary.db at path from a jmdict-simplified JSON file and the grammar list. It builds into a
// temporary file next to path and renames it into place only when every row is in, so a failed build leaves any
// earlier dictionary.db as it was.
func Build(ctx context.Context, path string, jmdict io.Reader, grammar []dictionary.GrammarPoint,
	builtAt time.Time) (stats Stats, err error) {
	if err := checkGrammar(grammar); err != nil {
		return Stats{}, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Stats{}, fmt.Errorf("dictbuild: create output folder failed (%v)", err)
	}
	tmp, err := os.CreateTemp(dir, ".dictionary-*.db")
	if err != nil {
		return Stats{}, fmt.Errorf("dictbuild: create temporary file failed (%v)", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			err = errors.Join(err, remove(tmpPath))
		}
	}()
	if err := tmp.Close(); err != nil {
		return Stats{}, fmt.Errorf("dictbuild: close temporary file failed (%v)", err)
	}

	stats, err = write(ctx, tmpPath, jmdict, grammar, builtAt)
	if err != nil {
		return Stats{}, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return Stats{}, fmt.Errorf("dictbuild: move the new dictionary into place failed (%v)", err)
	}
	return stats, nil
}

func remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("dictbuild: remove temporary file failed (%v)", err)
	}
	return nil
}
