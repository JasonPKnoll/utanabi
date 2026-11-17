package songs

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

var (
	lrcTime     = regexp.MustCompile(`^\[(\d{1,3}):(\d{2})(?:[.:](\d{1,3}))?\]`)
	lrcWordTime = regexp.MustCompile(`<\d{1,3}:\d{2}(?:[.:]\d{1,3})?>`)
)

type timedText struct {
	ms   int64
	text string
}

// parseLRC turns synced lyrics in LRC format into lines with start times, in time order. A line with several
// timestamps (a chorus sung twice) becomes one line per time. A timestamp with no text is a break, and starts a new
// stanza. Tags that aren't times ([ar:...], [offset:...]) and word timings (<00:12.34>) are dropped.
func parseLRC(lrc string) []Line {
	var entries []timedText
	for raw := range strings.SplitSeq(jtext.Normalize(lrc), "\n") {
		rest := strings.TrimSpace(raw)
		var times []int64
		for {
			m := lrcTime.FindStringSubmatch(rest)
			if m == nil {
				break
			}
			times = append(times, lrcMillis(m[1], m[2], m[3]))
			rest = rest[len(m[0]):]
		}
		if len(times) == 0 {
			continue
		}
		text := strings.TrimSpace(lrcWordTime.ReplaceAllString(rest, ""))
		for _, ms := range times {
			entries = append(entries, timedText{ms: ms, text: clip(text, maxLineLen)})
		}
	}
	slices.SortStableFunc(entries, func(a, b timedText) int { return cmp.Compare(a.ms, b.ms) })

	var lines []Line
	stanza, gap := 0, false
	for _, e := range entries {
		if e.text == "" {
			gap = true
			continue
		}
		if gap && len(lines) > 0 {
			stanza++
		}
		gap = false
		start := e.ms
		lines = append(lines, Line{Position: len(lines), Stanza: stanza, Text: e.text, StartMS: &start})
	}
	return lines
}

// lrcMillis reads an LRC time. The fraction is tenths in [00:12.3], hundredths in [00:12.34], and thousandths in
// [00:12.345].
func lrcMillis(minutes, seconds, frac string) int64 {
	ms := (digits(minutes)*60 + digits(seconds)) * 1000
	switch len(frac) {
	case 1:
		ms += digits(frac) * 100
	case 2:
		ms += digits(frac) * 10
	case 3:
		ms += digits(frac)
	}
	return ms
}

// digits reads a short run of ASCII digits, which is all lrcTime matches.
func digits(s string) int64 {
	var n int64
	for _, c := range s {
		n = n*10 + int64(c-'0')
	}
	return n
}
