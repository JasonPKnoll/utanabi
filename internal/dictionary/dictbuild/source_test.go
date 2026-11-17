package dictbuild_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/dictionary/dictbuild"
)

func tgz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func serve(t *testing.T, body []byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		if _, err := w.Write(body); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestFetchDownloadsVerifiesAndKeepsTheArchive(t *testing.T) {
	t.Parallel()

	archive := tgz(t, map[string]string{"jmdict-eng-common-3.6.2.json": `{"words":[]}`})
	srv, hits := serve(t, archive)
	path := filepath.Join(t.TempDir(), "cache", "jmdict.json.tgz")

	if err := dictbuild.Fetch(t.Context(), srv.Client(), srv.URL, sha(archive), path); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if err := dictbuild.Fetch(t.Context(), srv.Client(), srv.URL, sha(archive), path); err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("server was asked %d times, want 1: a verified archive is reused", got)
	}

	f, err := dictbuild.OpenArchive(path)
	if err != nil {
		t.Fatalf("OpenArchive: %v", err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if got, want := string(body), `{"words":[]}`; got != want {
		t.Errorf("archive JSON = %q, want %q", got, want)
	}
}

func TestFetchRefusesAWrongChecksum(t *testing.T) {
	t.Parallel()

	srv, _ := serve(t, []byte("not the archive"))
	dir := t.TempDir()
	path := filepath.Join(dir, "jmdict.json.tgz")

	err := dictbuild.Fetch(t.Context(), srv.Client(), srv.URL, sha([]byte("the archive")), path)
	if err == nil || !strings.Contains(err.Error(), "checksum does not match") {
		t.Fatalf("Fetch error = %v, want a checksum mismatch", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read folder: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a failed download left %d files behind, want none", len(entries))
	}
}

func TestFetchReplacesACachedFileWithTheWrongChecksum(t *testing.T) {
	t.Parallel()

	archive := tgz(t, map[string]string{"a.json": "{}"})
	srv, hits := serve(t, archive)
	path := filepath.Join(t.TempDir(), "jmdict.json.tgz")
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatalf("write cached file: %v", err)
	}

	if err := dictbuild.Fetch(t.Context(), srv.Client(), srv.URL, sha(archive), path); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("server was asked %d times, want 1", got)
	}
	if err := dictbuild.Verify(path, sha(archive)); err != nil {
		t.Errorf("Verify after Fetch: %v", err)
	}
}

func TestFetchRefusesAnErrorStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	err := dictbuild.Fetch(t.Context(), srv.Client(), srv.URL, sha(nil), filepath.Join(t.TempDir(), "a.tgz"))
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("Fetch error = %v, want one naming the 404", err)
	}
}

func TestOpenArchiveWithoutJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "a.tgz")
	if err := os.WriteFile(path, tgz(t, map[string]string{"README.md": "hi"}), 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, err := dictbuild.OpenArchive(path); err == nil || !strings.Contains(err.Error(), "no .json file") {
		t.Errorf("OpenArchive error = %v, want no .json file", err)
	}
}
