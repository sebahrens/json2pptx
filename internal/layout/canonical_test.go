package layout

import (
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// Sample template layouts for testing
var pwcLayouts = []types.LayoutMetadata{
	{ID: "slideLayout1", Name: "Title Slide", Tags: []string{"title-slide"}},
	{ID: "slideLayout2", Name: "Section Header", Tags: []string{"content", "title-at-bottom", "section-header"}},
	{ID: "slideLayout3", Name: "One Content", Tags: []string{"content"}},
	{ID: "slideLayout4", Name: "Blank", Tags: []string{"title-slide", "blank-title"}},
	{ID: "slideLayout5", Name: "Conclusion 2", Tags: []string{"title-slide", "closing"}},
	{ID: "content-2-50-50", Name: "Two Column 50/50", Tags: []string{"content", "two-column", "comparison"}},
	{ID: "content-2-60-40", Name: "Two Column 60/40", Tags: []string{"content", "two-column", "comparison"}},
	{ID: "content-2-40-60", Name: "Two Column 40/60", Tags: []string{"content", "two-column", "comparison"}},
}

var warmCoralLayouts = []types.LayoutMetadata{
	{ID: "slideLayout1", Name: "Title Slide", Tags: []string{"title-slide"}},
	{ID: "slideLayout2", Name: "Title and Content", Tags: []string{"content"}},
	{ID: "slideLayout3", Name: "Two Content", Tags: []string{"content", "two-column", "comparison"}},
	{ID: "slideLayout4", Name: "Section Header", Tags: []string{"section-header"}},
	{ID: "slideLayout5", Name: "End Slide", Tags: []string{"closing"}},
	{ID: "slideLayout6", Name: "Blank", Tags: []string{"blank-title"}},
}

var modernLayouts = []types.LayoutMetadata{
	{ID: "slideLayout1", Name: "Content", Tags: []string{"content"}},
	{ID: "slideLayout2", Name: "Closing", Tags: []string{"closing"}},
	{ID: "slideLayout3", Name: "Title Slide", Tags: []string{"title-slide"}},
	{ID: "slideLayout4", Name: "Section Header", Tags: []string{"section-header"}},
	{ID: "slideLayout5", Name: "Agenda", Tags: []string{"content", "agenda"}},
}

func TestResolveCanonicalLayoutID_Passthrough(t *testing.T) {
	// slideLayoutN passes through unchanged
	id, ok := ResolveCanonicalLayoutID("slideLayout3", pwcLayouts)
	if !ok || id != "slideLayout3" {
		t.Errorf("slideLayoutN passthrough: got (%q, %v), want (\"slideLayout3\", true)", id, ok)
	}

	// Generated layout IDs pass through unchanged
	id, ok = ResolveCanonicalLayoutID("content-2-50-50", pwcLayouts)
	if !ok || id != "content-2-50-50" {
		t.Errorf("generated layout passthrough: got (%q, %v), want (\"content-2-50-50\", true)", id, ok)
	}

	id, ok = ResolveCanonicalLayoutID("grid-2x2", pwcLayouts)
	if !ok || id != "grid-2x2" {
		t.Errorf("grid layout passthrough: got (%q, %v), want (\"grid-2x2\", true)", id, ok)
	}
}

func TestResolveCanonicalLayoutID_Aliases(t *testing.T) {
	// Layout aliases resolve to generated IDs (no template layouts needed)
	id, ok := ResolveCanonicalLayoutID("2-col", nil)
	if !ok || id != "content-2-50-50" {
		t.Errorf("alias 2-col: got (%q, %v), want (\"content-2-50-50\", true)", id, ok)
	}

	id, ok = ResolveCanonicalLayoutID("sidebar-right", nil)
	if !ok || id != "content-2-70-30" {
		t.Errorf("alias sidebar-right: got (%q, %v), want (\"content-2-70-30\", true)", id, ok)
	}
}

func TestResolveCanonicalLayoutID_PwCTemplate(t *testing.T) {
	tests := []struct {
		name       string
		canonical  string
		wantID     string
		wantOK     bool
	}{
		{"title", "title", "slideLayout1", true},
		{"content", "content", "slideLayout3", true},
		{"section", "section", "slideLayout2", true},
		{"closing", "closing", "slideLayout5", true},
		{"blank", "blank", "slideLayout4", true},
		{"two-column", "two-column", "content-2-50-50", true},
		{"two-column-wide-narrow", "two-column-wide-narrow", "content-2-60-40", true},
		{"two-column-narrow-wide", "two-column-narrow-wide", "content-2-40-60", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := ResolveCanonicalLayoutID(tt.canonical, pwcLayouts)
			if id != tt.wantID || ok != tt.wantOK {
				t.Errorf("ResolveCanonicalLayoutID(%q) = (%q, %v), want (%q, %v)",
					tt.canonical, id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestResolveCanonicalLayoutID_WarmCoral(t *testing.T) {
	tests := []struct {
		name      string
		canonical string
		wantID    string
		wantOK    bool
	}{
		{"title", "title", "slideLayout1", true},
		{"content", "content", "slideLayout2", true},
		{"section", "section", "slideLayout4", true},
		{"closing", "closing", "slideLayout5", true},
		{"blank", "blank", "slideLayout6", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := ResolveCanonicalLayoutID(tt.canonical, warmCoralLayouts)
			if id != tt.wantID || ok != tt.wantOK {
				t.Errorf("ResolveCanonicalLayoutID(%q) = (%q, %v), want (%q, %v)",
					tt.canonical, id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestResolveCanonicalLayoutID_Modern(t *testing.T) {
	tests := []struct {
		name      string
		canonical string
		wantID    string
		wantOK    bool
	}{
		{"title", "title", "slideLayout3", true},
		{"content", "content", "slideLayout1", true},
		{"section", "section", "slideLayout4", true},
		{"closing", "closing", "slideLayout2", true},
		{"agenda", "agenda", "slideLayout5", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := ResolveCanonicalLayoutID(tt.canonical, modernLayouts)
			if id != tt.wantID || ok != tt.wantOK {
				t.Errorf("ResolveCanonicalLayoutID(%q) = (%q, %v), want (%q, %v)",
					tt.canonical, id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestResolveCanonicalLayoutID_CaseInsensitive(t *testing.T) {
	id, ok := ResolveCanonicalLayoutID("Title", pwcLayouts)
	if !ok || id != "slideLayout1" {
		t.Errorf("case insensitive: got (%q, %v), want (\"slideLayout1\", true)", id, ok)
	}

	id, ok = ResolveCanonicalLayoutID("CLOSING", pwcLayouts)
	if !ok || id != "slideLayout5" {
		t.Errorf("case insensitive: got (%q, %v), want (\"slideLayout5\", true)", id, ok)
	}
}

func TestResolveCanonicalLayoutID_Unknown(t *testing.T) {
	id, ok := ResolveCanonicalLayoutID("nonexistent", pwcLayouts)
	if ok || id != "nonexistent" {
		t.Errorf("unknown name: got (%q, %v), want (\"nonexistent\", false)", id, ok)
	}
}

func TestResolveCanonicalLayoutID_NoMatchInTemplate(t *testing.T) {
	// Minimal template with only content — no closing layout
	minimal := []types.LayoutMetadata{
		{ID: "slideLayout1", Name: "Content", Tags: []string{"content"}},
	}

	id, ok := ResolveCanonicalLayoutID("closing", minimal)
	if ok || id != "closing" {
		t.Errorf("no match: got (%q, %v), want (\"closing\", false)", id, ok)
	}
}

func TestMatchesTags(t *testing.T) {
	tests := []struct {
		name        string
		layoutTags  []string
		requireTags []string
		excludeTags []string
		want        bool
	}{
		{"content matches content", []string{"content"}, []string{"content"}, nil, true},
		{"two-column excluded from content", []string{"content", "two-column"}, []string{"content"}, []string{"two-column"}, false},
		{"closing matches closing", []string{"title-slide", "closing"}, []string{"closing"}, nil, true},
		{"no required tag", []string{"content"}, []string{"title-slide"}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesTags(tt.layoutTags, tt.requireTags, tt.excludeTags)
			if got != tt.want {
				t.Errorf("matchesTags(%v, %v, %v) = %v, want %v",
					tt.layoutTags, tt.requireTags, tt.excludeTags, got, tt.want)
			}
		})
	}
}
