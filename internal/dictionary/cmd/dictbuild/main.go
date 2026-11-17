// Command dictbuild downloads the pinned JMdict release, checks its SHA-256, and builds dictionary.db with the grammar
// list. Run it through task dictionary from the repo root:
//
//	task dictionary                  # writes data/dictionary.db
//	task dictionary OUT=path/to.db   # writes somewhere else
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/dictionary"
	"github.com/JasonPKnoll/utanabi/internal/dictionary/dictbuild"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "dictbuild:", err)
		os.Exit(1)
	}
}

func run() (err error) {
	out := flag.String("out", filepath.Join("data", "dictionary.db"), "where to write dictionary.db")
	grammarPath := flag.String("grammar", filepath.Join("internal", "dictionary", "grammar.json"),
		"the hand-written grammar list")
	cache := flag.String("cache", filepath.Join(".cache", "dictionary"), "folder for the downloaded JMdict archive")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	grammar, err := readGrammar(*grammarPath)
	if err != nil {
		return err
	}

	archive := filepath.Join(*cache, "jmdict-eng-common-"+dictbuild.JMdictRelease+".json.tgz")
	client := &http.Client{Timeout: 5 * time.Minute}
	if err := dictbuild.Fetch(ctx, client, dictbuild.JMdictURL, dictbuild.JMdictSHA256, archive); err != nil {
		return err
	}
	jmdict, err := dictbuild.OpenArchive(archive)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, jmdict.Close()) }()

	stats, err := dictbuild.Build(ctx, *out, jmdict, grammar, time.Now())
	if err != nil {
		return err
	}
	info, err := os.Stat(*out)
	if err != nil {
		return fmt.Errorf("dictbuild: check the new dictionary failed (%v)", err)
	}
	fmt.Printf("wrote %s (%.1f MB): JMdict %s (release %s), %d entries, %d spellings, %d senses, %d tags, "+
		"%d grammar points\n", *out, float64(info.Size())/(1<<20), stats.JMdictDate, dictbuild.JMdictRelease,
		stats.Entries, stats.Forms, stats.Senses, stats.Tags, stats.GrammarPoints)
	return nil
}

func readGrammar(path string) (list []dictionary.GrammarPoint, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("dictbuild: open grammar list failed (%v)", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("dictbuild: close grammar list failed (%v)", closeErr))
		}
	}()
	return dictbuild.ReadGrammar(f)
}
