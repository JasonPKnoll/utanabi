package dictbuild

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// The pinned JMdict release: jmdict-simplified's English-only JSON of all of JMdict, not only the common words, since
// lyrics are full of words the common list leaves out (纏う, 見惚れる, 浮つく). To update, pick a release from
// https://github.com/scriptin/jmdict-simplified/releases, set all three values, and run task dictionary.
const (
	JMdictRelease = "3.6.2+20261116213322"
	JMdictURL     = "https://github.com/scriptin/jmdict-simplified/releases/download/3.6.2%2B20261116213322/" +
		"jmdict-eng-3.6.2%2B20261116213322.json.tgz"
	JMdictSHA256 = "956a1cbd6478b86bc351d26bb33c74deedc8737ebfccc396a95e313199ccbfce"
)

const (
	// maxArchiveSize is far above the 12 MB archive, and stops a wrong URL from filling the disk.
	maxArchiveSize = 200 << 20
	// maxJSONSize bounds the unpacked JSON, which is about 120 MB.
	maxJSONSize = 1 << 30
)

// Fetch makes sure path holds the archive at url with the given SHA-256. A file already there with the right checksum
// is kept, so a rebuild works offline. Otherwise it downloads the archive, and keeps it only when the checksum matches.
func Fetch(ctx context.Context, client *http.Client, url, sum, path string) (err error) {
	if err := Verify(path, sum); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("dictbuild: create download folder failed (%v)", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("dictbuild: build download request failed (%v)", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("dictbuild: download failed (%v)", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dictbuild: download failed (%s from %s)", resp.Status, url)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".download-*")
	if err != nil {
		return fmt.Errorf("dictbuild: create download file failed (%v)", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, remove(tmp.Name()))
		}
	}()

	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(resp.Body, maxArchiveSize+1))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("dictbuild: download failed (%v)", err)
	}
	if n > maxArchiveSize {
		return fmt.Errorf("dictbuild: download is larger than %d MB (%s)", maxArchiveSize>>20, url)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != sum {
		return fmt.Errorf("dictbuild: download checksum does not match (got sha256 %s, want %s)", got, sum)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("dictbuild: save download failed (%v)", err)
	}
	return nil
}

// Verify checks that the file at path has the given SHA-256.
func Verify(path, sum string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("dictbuild: open archive failed (%v)", err)
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return fmt.Errorf("dictbuild: read archive failed (%v)", err)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != sum {
		return fmt.Errorf("dictbuild: archive checksum does not match (got sha256 %s, want %s)", got, sum)
	}
	return nil
}

// OpenArchive returns the JMdict JSON file inside a jmdict-simplified .json.tgz archive. Verify the archive first.
func OpenArchive(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("dictbuild: open archive failed (%v)", err)
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("dictbuild: read archive failed (%v)", err), closeFile(f))
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.Join(errors.New("dictbuild: archive holds no .json file"), closeFile(f))
		}
		if err != nil {
			return nil, errors.Join(fmt.Errorf("dictbuild: read archive failed (%v)", err), closeFile(f))
		}
		if hdr.Typeflag == tar.TypeReg && strings.HasSuffix(hdr.Name, ".json") {
			return archiveFile{Reader: io.LimitReader(tr, maxJSONSize), file: f}, nil
		}
	}
}

type archiveFile struct {
	io.Reader
	file *os.File
}

func (a archiveFile) Close() error {
	return closeFile(a.file)
}

func closeFile(f *os.File) error {
	if err := f.Close(); err != nil {
		return fmt.Errorf("dictbuild: close archive failed (%v)", err)
	}
	return nil
}
