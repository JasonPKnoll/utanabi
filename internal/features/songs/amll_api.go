package songs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

// AMLLURL is the AMLL TTML DB, a community database of songs timed word by word by hand, read as files from its
// GitHub repository. Its lyrics are CC0. https://github.com/amll-dev/amll-ttml-db
const AMLLURL = "https://raw.githubusercontent.com/amll-dev/amll-ttml-db/main"

const (
	amllTimeout       = 20 * time.Second
	amllIndexMaxBytes = 16 << 20
	amllSheetMaxBytes = 4 << 20
	// The index changes as songs are added, a few times a day at most, so it's read again after this long.
	amllIndexFor = 12 * time.Hour
)

// amllFile is the name a sheet has in the database: letters, digits, dashes, and dots, ending in .ttml.
var amllFile = regexp.MustCompile(`^[0-9A-Za-z_-][0-9A-Za-z._-]*\.ttml$`)

// AMLL is the TimingSource for the AMLL TTML DB.
type AMLL struct {
	base      string
	userAgent string
	client    *http.Client
	now       func() time.Time

	mu      sync.Mutex
	index   []SheetRef
	indexAt time.Time
}

// NewAMLL returns the AMLL TTML DB at base (AMLLURL, or a test server).
func NewAMLL(base, version string) *AMLL {
	base = strings.TrimRight(base, "/")
	host := ""
	if u, err := url.Parse(base); err == nil {
		host = u.Host
	}
	return &AMLL{
		base:      base,
		userAgent: "Utanabi " + version + " (https://github.com/JasonPKnoll/utanabi)",
		now:       time.Now,
		client: &http.Client{
			Timeout: amllTimeout,
			// The app reads only the database's files, so a redirect that leaves its host is refused.
			CheckRedirect: func(req *http.Request, _ []*http.Request) error {
				if req.URL.Host != host {
					return fmt.Errorf("songs: amll redirected to another host (%s)", req.URL.Host)
				}
				return nil
			},
		},
	}
}

// Code returns the source code stored with word times from the AMLL TTML DB.
func (a *AMLL) Code() string { return "amll" }

// Name returns the database's name, as shown in the app.
func (a *AMLL) Name() string { return "AMLL TTML DB" }

// Sheets returns every song in the database's index, read again once the copy kept is 12 hours old.
func (a *AMLL) Sheets(ctx context.Context) ([]SheetRef, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.index != nil && a.now().Sub(a.indexAt) < amllIndexFor {
		return a.index, nil
	}
	body, err := a.get(ctx, "/metadata/raw-lyrics-index.jsonl", amllIndexMaxBytes)
	if err != nil {
		return nil, err
	}
	index, err := readAMLLIndex(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	a.index, a.indexAt = index, a.now()
	return index, nil
}

// readAMLLIndex reads the index: one JSON object a line, each a sheet's file and its metadata as [key, [values]]
// pairs. Rows it can't use are skipped.
func readAMLLIndex(r io.Reader) ([]SheetRef, error) {
	var out []SheetRef
	lines := bufio.NewScanner(r)
	lines.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for lines.Scan() {
		var row struct {
			Metadata [][]json.RawMessage `json:"metadata"`
			File     string              `json:"rawLyricFile"`
		}
		if err := json.Unmarshal(lines.Bytes(), &row); err != nil || !amllFile.MatchString(row.File) {
			continue
		}
		ref := SheetRef{ID: row.File}
		for _, pair := range row.Metadata {
			var key string
			var values []string
			if len(pair) != 2 || json.Unmarshal(pair[0], &key) != nil || json.Unmarshal(pair[1], &values) != nil {
				continue
			}
			switch key {
			case "musicName":
				if len(values) > 0 {
					ref.Title = values[0]
				}
			case "artists":
				ref.Artist = strings.Join(values, " ")
			}
		}
		if ref.Title != "" {
			out = append(out, ref)
		}
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("songs: amll index unreadable (%v)", err)
	}
	return out, nil
}

// Sheet reads one song's word times from its TTML file.
func (a *AMLL) Sheet(ctx context.Context, ref SheetRef) (TimedSheet, error) {
	if !amllFile.MatchString(ref.ID) {
		return TimedSheet{}, fault.Invalid("sheet", "is not a file in the database")
	}
	body, err := a.get(ctx, "/raw-lyrics/"+ref.ID, amllSheetMaxBytes)
	if err != nil {
		return TimedSheet{}, err
	}
	return readTTML(bytes.NewReader(body))
}

// readTTML reads the lead vocal's lines from a TTML sheet: each <p> a line, each timed <span> in it a run of text
// with the time it starts. Spans with a role (translations, romaji, backing vocals) are left out.
func readTTML(r io.Reader) (TimedSheet, error) {
	var sheet TimedSheet
	dec := xml.NewDecoder(r)
	var line []TimedSpan
	inLine := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return sheet, nil
		}
		if err != nil {
			return TimedSheet{}, fmt.Errorf("songs: amll sheet isn't TTML (%v)", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Local == "p":
				inLine, line = true, nil
			case t.Name.Local == "span" && inLine && attr(t, "role") != "":
				if err := dec.Skip(); err != nil {
					return TimedSheet{}, fmt.Errorf("songs: amll sheet isn't TTML (%v)", err)
				}
			case t.Name.Local == "span" && inLine && attr(t, "begin") != "":
				start, ok := ttmlTime(attr(t, "begin"))
				var text string
				if err := dec.DecodeElement(&struct {
					Text *string `xml:",chardata"`
				}{&text}, &t); err != nil {
					return TimedSheet{}, fmt.Errorf("songs: amll sheet isn't TTML (%v)", err)
				}
				if ok && strings.TrimSpace(text) != "" {
					line = append(line, TimedSpan{Text: text, StartMS: start})
				}
			}
		case xml.EndElement:
			if t.Name.Local == "p" && inLine {
				if len(line) > 0 {
					sheet.Lines = append(sheet.Lines, line)
				}
				inLine = false
			}
		}
	}
}

func attr(el xml.StartElement, local string) string {
	for _, a := range el.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// ttmlTime reads a TTML clock time, such as 01:02.345, 1:01:02.345, 62.345, or 62.345s, in milliseconds.
func ttmlTime(s string) (int64, bool) {
	s = strings.TrimSuffix(strings.TrimSpace(s), "s")
	var total float64
	for part := range strings.SplitSeq(s, ":") {
		n, err := strconv.ParseFloat(part, 64)
		if err != nil || n < 0 {
			return 0, false
		}
		total = total*60 + n
	}
	return int64(total*1000 + 0.5), true
}

// get fetches a file, at most limit bytes of it, turning network and HTTP failures into the project's errors.
func (a *AMLL) get(ctx context.Context, path string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.base+path, nil)
	if err != nil {
		return nil, fmt.Errorf("songs: build amll request failed (%v)", err)
	}
	req.Header.Set("User-Agent", a.userAgent)

	resp, err := a.client.Do(req)
	if err != nil {
		var netErr net.Error
		switch {
		case errors.Is(err, context.Canceled):
			return nil, fault.ErrCanceled
		case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
			return nil, fmt.Errorf("songs: amll took too long (%v); %w", err, fault.ErrTimeout)
		default:
			return nil, fmt.Errorf("songs: amll unreachable (%v)", err)
		}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fault.ErrNotFound
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return nil, fmt.Errorf("songs: amll is busy (status %d); %w", resp.StatusCode, fault.ErrTimeout)
	default:
		return nil, fmt.Errorf("songs: amll request failed (status %d)", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, fmt.Errorf("songs: amll response unreadable (%v)", err)
	}
	return body, nil
}
