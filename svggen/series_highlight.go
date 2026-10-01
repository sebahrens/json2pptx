package svggen

import (
	"fmt"
	"math"
	"strings"
)

// A multi-series chart should show the series its slide is about: that series
// keeps its colour and every other one turns neutral grey context (Knaflic's
// preattentive emphasis). data.highlight names the series by 0-based index or
// name (go-slide-creator-kbzu2).
const (
	// contextLineWidthShare is a context (non-highlighted) line's stroke as a
	// share of the highlighted line's.
	contextLineWidthShare = 0.45
	// contextLabelInk is the dk1 share of a context series' direct label —
	// darker than its 38% line so the name stays readable.
	contextLabelInk = 0.6
	// LineMarkersMaxPoints is the most points (categories x series) a line
	// chart draws markers on; denser charts draw plain lines.
	LineMarkersMaxPoints = 12
)

// resolveSeriesHighlight reads data.highlight against the series names. set
// reports whether the key was present; an entry that names no series (by
// exact or case-insensitive name, or a 0-based index) is an error, so the
// caller can fall back to the category reading.
func resolveSeriesHighlight(data map[string]any, series []ChartSeries) ([]int, bool, error) {
	raw, ok := data["highlight"]
	if !ok || raw == nil {
		return nil, false, nil
	}
	entries, isList := toAnySlice(raw)
	if !isList {
		entries = []any{raw}
	}
	names := make([]string, len(series))
	for i, s := range series {
		names[i] = s.Name
	}
	out := []int{}
	seen := map[int]bool{}
	for _, e := range entries {
		idx, ok := highlightIndex(e, names)
		if !ok {
			return nil, true, fmt.Errorf("highlight entry %v names no series", e)
		}
		if !seen[idx] {
			seen[idx] = true
			out = append(out, idx)
		}
	}
	return out, true, nil
}

// seriesHighlighted reports whether series i is emphasised: always when no
// series highlight is set, otherwise only when it is named.
func seriesHighlighted(data ChartData, i int) bool {
	if !data.SeriesHighlightSet {
		return true
	}
	for _, h := range data.SeriesHighlight {
		if h == i {
			return true
		}
	}
	return false
}

// seriesHighlightColors recolours a multi-series palette for a series
// highlight: the highlighted series take the palette's colours in order
// (accent1 first), every other series neutral dk1 at BarNeutralInk.
func seriesHighlightColors(p *Palette, colors []Color, data ChartData) []Color {
	if !data.SeriesHighlightSet || len(colors) == 0 {
		return colors
	}
	out := make([]Color, len(data.Series))
	neutral := NeutralInk(p, BarNeutralInk)
	k := 0
	for i := range data.Series {
		if seriesHighlighted(data, i) {
			out[i] = colors[k%len(colors)]
			k++
			continue
		}
		out[i] = neutral
	}
	return out
}

// seriesLabelColors are the direct-label inks: a highlighted series' own
// colour, a context series a darker neutral than its line.
func seriesLabelColors(p *Palette, colors []Color, data ChartData) []Color {
	if !data.SeriesHighlightSet {
		return colors
	}
	out := make([]Color, len(colors))
	copy(out, colors)
	for i := range out {
		if !seriesHighlighted(data, i) {
			out[i] = NeutralInk(p, contextLabelInk)
		}
	}
	return out
}

// highlightedOnly returns the chart with only its highlighted series (and
// their colours), for labelling them alone when every name does not fit.
func highlightedOnly(data ChartData, colors []Color) (ChartData, []Color) {
	out := data
	out.Series = nil
	var cols []Color
	for i, s := range data.Series {
		if seriesHighlighted(data, i) {
			out.Series = append(out.Series, s)
			cols = append(cols, colors[i%len(colors)])
		}
	}
	return out, cols
}

// seriesDrawOrder lists series indices context-first so highlighted series
// are painted on top.
func seriesDrawOrder(data ChartData) []int {
	order := make([]int, 0, len(data.Series))
	for i := range data.Series {
		if !seriesHighlighted(data, i) {
			order = append(order, i)
		}
	}
	for i := range data.Series {
		if seriesHighlighted(data, i) {
			order = append(order, i)
		}
	}
	return order
}

// contextStrokeWidth is a context series' line width.
func contextStrokeWidth(w float64) float64 {
	return math.Max(1, w*contextLineWidthShare)
}

// lineMarkersByDefault reports whether a line chart with this many series
// and categories draws point markers: only up to LineMarkersMaxPoints.
func lineMarkersByDefault(data ChartData) bool {
	n := 0
	for _, s := range data.Series {
		n += len(s.Values)
	}
	return n <= LineMarkersMaxPoints
}

// seriesNameMatches reports whether name appears in text as whole words
// (case-insensitive) — used to pick a default highlight from a slide title.
func seriesNameMatches(text, name string) bool {
	text, name = strings.ToLower(text), strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for start := 0; ; {
		i := strings.Index(text[start:], name)
		if i < 0 {
			return false
		}
		i += start
		end := i + len(name)
		before := i == 0 || !isWordByte(text[i-1])
		after := end == len(text) || !isWordByte(text[end])
		if before && after {
			return true
		}
		start = i + 1
	}
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_'
}

// DefaultSeriesHighlight returns the index of the one series whose name the
// text (a slide title or takeaway) mentions, or -1 when none or several do.
func DefaultSeriesHighlight(text string, seriesNames []string) int {
	found := -1
	for i, n := range seriesNames {
		if seriesNameMatches(text, n) {
			if found >= 0 {
				return -1
			}
			found = i
		}
	}
	return found
}
