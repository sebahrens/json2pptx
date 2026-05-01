package generator

import (
	"strings"
	"testing"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"slideLayout1", "slideLayout2", 1},
		{"slideLayout1", "slideLayou1", 1},    // deletion
		{"slideLayout1", "slidelayout1", 1},   // case change
		{"Title 1", "Title 2", 1},
		{"Content Placeholder 2", "Content Placeholder 3", 1},
		{"abc", "xyz", 3},
		{"abc", "abcdef", 3},
	}

	for _, tt := range tests {
		got := levenshteinDistance(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestLevenshteinDistance_Symmetry(t *testing.T) {
	pairs := [][2]string{
		{"hello", "hallo"},
		{"slideLayout1", "slideLayout12"},
		{"abc", ""},
	}
	for _, p := range pairs {
		d1 := levenshteinDistance(p[0], p[1])
		d2 := levenshteinDistance(p[1], p[0])
		if d1 != d2 {
			t.Errorf("levenshteinDistance(%q, %q) = %d, but reverse = %d; should be symmetric", p[0], p[1], d1, d2)
		}
	}
}

func TestClosestMatch(t *testing.T) {
	candidates := []string{"slideLayout1", "slideLayout2", "slideLayout3", "slideLayout11"}

	tests := []struct {
		target      string
		maxDist     int
		wantMatch   string
		wantDist    int
		wantNoMatch bool
	}{
		{"slideLayout1", 3, "slideLayout1", 0, false},  // exact match
		{"slideLayout4", 3, "slideLayout1", 1, false},   // 1 char diff
		{"slideLayou1", 3, "slideLayout1", 1, false},    // typo
		{"slideLayout12", 3, "slideLayout1", 1, false},  // extra char
		{"somethingCompletelyDifferent", 3, "", -1, true}, // no close match
	}

	for _, tt := range tests {
		match, dist := ClosestMatch(tt.target, candidates, tt.maxDist)
		if tt.wantNoMatch {
			if match != "" {
				t.Errorf("ClosestMatch(%q) = (%q, %d), want no match", tt.target, match, dist)
			}
		} else {
			if match != tt.wantMatch {
				t.Errorf("ClosestMatch(%q) match = %q, want %q", tt.target, match, tt.wantMatch)
			}
			if dist != tt.wantDist {
				t.Errorf("ClosestMatch(%q) dist = %d, want %d", tt.target, dist, tt.wantDist)
			}
		}
	}
}

func TestClosestMatch_EmptyCandidates(t *testing.T) {
	match, dist := ClosestMatch("anything", nil, 3)
	if match != "" || dist != -1 {
		t.Errorf("ClosestMatch with nil candidates = (%q, %d), want (\"\", -1)", match, dist)
	}

	match, dist = ClosestMatch("anything", []string{}, 3)
	if match != "" || dist != -1 {
		t.Errorf("ClosestMatch with empty candidates = (%q, %d), want (\"\", -1)", match, dist)
	}
}

func TestFormatAvailableIDs(t *testing.T) {
	tests := []struct {
		ids  []string
		want string
	}{
		{[]string{"c", "a", "b"}, "[a, b, c]"},
		{[]string{"slideLayout1"}, "[slideLayout1]"},
		{[]string{}, "[]"},
	}
	for _, tt := range tests {
		got := FormatAvailableIDs(tt.ids)
		if got != tt.want {
			t.Errorf("FormatAvailableIDs(%v) = %q, want %q", tt.ids, got, tt.want)
		}
	}
}

func TestLayoutNotFoundError(t *testing.T) {
	tests := []struct {
		name        string
		layoutID    string
		available   []string
		wantContain []string
	}{
		{
			name:     "with close match",
			layoutID: "slideLayout4",
			available: []string{"slideLayout1", "slideLayout2", "slideLayout3"},
			wantContain: []string{
				`layout_id "slideLayout4" not found in template`,
				"available layouts:",
				"slideLayout1",
				"did you mean",
			},
		},
		{
			name:     "no close match",
			layoutID: "somethingCompletelyDifferent",
			available: []string{"slideLayout1", "slideLayout2"},
			wantContain: []string{
				`layout_id "somethingCompletelyDifferent" not found in template`,
				"available layouts:",
			},
		},
		{
			name:        "no available layouts",
			layoutID:    "slideLayout1",
			available:   nil,
			wantContain: []string{`layout_id "slideLayout1" not found in template`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LayoutNotFoundError(tt.layoutID, tt.available)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("LayoutNotFoundError(%q) = %q, want to contain %q", tt.layoutID, got, want)
				}
			}
		})
	}

	// Verify "did you mean" is NOT present when no close match
	msg := LayoutNotFoundError("somethingCompletelyDifferent", []string{"slideLayout1"})
	if strings.Contains(msg, "did you mean") {
		t.Errorf("should not suggest match for very different string: %q", msg)
	}
}

func TestPlaceholderNotFoundError(t *testing.T) {
	tests := []struct {
		name          string
		placeholderID string
		layoutID      string
		available     []string
		wantContain   []string
	}{
		{
			name:          "with close match",
			placeholderID: "Title 2",
			layoutID:      "slideLayout1",
			available:     []string{"Title 1", "Content Placeholder 2"},
			wantContain: []string{
				`placeholder_id "Title 2" not found in layout "slideLayout1"`,
				"available placeholders:",
				"Title 1",
				`did you mean "Title 1"`,
			},
		},
		{
			name:          "no close match",
			placeholderID: "nonexistent",
			layoutID:      "slideLayout1",
			available:     []string{"Title 1", "Content Placeholder 2"},
			wantContain: []string{
				`placeholder_id "nonexistent" not found in layout "slideLayout1"`,
				"available placeholders:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlaceholderNotFoundError(tt.placeholderID, tt.layoutID, tt.available)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("PlaceholderNotFoundError() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}
