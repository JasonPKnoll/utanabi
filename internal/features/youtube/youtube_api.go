package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/redact"
)

// APIURL is YouTube's Data API.
const APIURL = "https://www.googleapis.com/youtube/v3"

const (
	apiTimeout  = 10 * time.Second
	apiMaxBytes = 1 << 20
	// searchSize is how many videos a search reads: the most relevant few, among which one may be the length of the
	// lyrics. A search costs 100 of the key's 10,000 daily units, and reading their lengths 1 more.
	searchSize = 5
)

// DataAPI is the Search backed by YouTube's Data API, with the server's API key.
type DataAPI struct {
	base   string
	key    redact.Secret
	client *http.Client
}

// NewDataAPI returns the API at base (APIURL, or a test server), with key.
func NewDataAPI(base string, key redact.Secret) *DataAPI {
	base = strings.TrimRight(base, "/")
	host := ""
	if u, err := url.Parse(base); err == nil {
		host = u.Host
	}
	return &DataAPI{base: base, key: key, client: &http.Client{
		Timeout: apiTimeout,
		// The app talks only to YouTube's API, so a redirect elsewhere is refused rather than followed.
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if req.URL.Host != host {
				return fmt.Errorf("youtube: redirected to another host (%s)", req.URL.Host)
			}
			return nil
		},
	}}
}

type snippetJSON struct {
	Title        string `json:"title"`
	ChannelTitle string `json:"channelTitle"`
}

func (a *DataAPI) Videos(ctx context.Context, query string) ([]Video, error) {
	var found struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
		} `json:"items"`
	}
	err := a.get(ctx, "/search", url.Values{"part": {"id"}, "type": {"video"}, "videoEmbeddable": {"true"},
		"maxResults": {strconv.Itoa(searchSize)}, "q": {query}}, &found)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, item := range found.Items {
		if videoID.MatchString(item.ID.VideoID) {
			ids = append(ids, item.ID.VideoID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return a.details(ctx, ids)
}

func (a *DataAPI) Video(ctx context.Context, id string) (Video, error) {
	videos, err := a.details(ctx, []string{id})
	if err != nil {
		return Video{}, err
	}
	if len(videos) == 0 {
		return Video{}, fault.ErrNotFound
	}
	return videos[0], nil
}

// details reads the videos' titles, channels, and lengths, in the order of ids.
func (a *DataAPI) details(ctx context.Context, ids []string) ([]Video, error) {
	var found struct {
		Items []struct {
			ID             string      `json:"id"`
			Snippet        snippetJSON `json:"snippet"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	err := a.get(ctx, "/videos", url.Values{"part": {"snippet,contentDetails"}, "id": {strings.Join(ids, ",")}},
		&found)
	if err != nil {
		return nil, err
	}
	byID := map[string]Video{}
	for _, item := range found.Items {
		byID[item.ID] = Video{ID: item.ID, Title: clip(html.UnescapeString(item.Snippet.Title), 300),
			Channel:    clip(html.UnescapeString(item.Snippet.ChannelTitle), 200),
			DurationMS: isoMillis(item.ContentDetails.Duration)}
	}
	var videos []Video
	for _, id := range ids {
		if v, ok := byID[id]; ok {
			videos = append(videos, v)
		}
	}
	return videos, nil
}

// get asks the API for path with query, and reads its answer into out. The key goes in a header, so it's never in a
// URL that could end up in a log.
func (a *DataAPI) get(ctx context.Context, path string, query url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.base+path+"?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("youtube: build request (%v)", err)
	}
	req.Header.Set("X-Goog-Api-Key", a.key.Reveal())
	req.Header.Set("Accept", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("youtube: can't reach YouTube (%v)", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxBytes))
	if err != nil {
		return fmt.Errorf("youtube: read answer (%v)", err)
	}
	if resp.StatusCode != http.StatusOK {
		var failed struct {
			Error struct {
				Errors []struct {
					Reason string `json:"reason"`
				} `json:"errors"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &failed) == nil {
			for _, e := range failed.Error.Errors {
				if e.Reason == "quotaExceeded" || e.Reason == "dailyLimitExceeded" {
					return ErrQuota
				}
			}
		}
		return fmt.Errorf("youtube: request refused (GET %s, status %d)", path, resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("youtube: answer isn't JSON (GET %s, %v)", path, err)
	}
	return nil
}

var isoDuration = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// isoMillis reads a video's length as YouTube writes it (PT3M46S), or 0 when it can't.
func isoMillis(d string) int64 {
	m := isoDuration.FindStringSubmatch(d)
	if m == nil {
		return 0
	}
	var ms int64
	for i, unit := range []int64{3600000, 60000, 1000} {
		if n, err := strconv.ParseInt(m[i+1], 10, 64); err == nil {
			ms += n * unit
		}
	}
	return ms
}

// clip cuts s to at most n runes, so a long title fits the database.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
