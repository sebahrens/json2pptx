package semantic

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// TestMapFindingExactMatch covers the acceptance case: an overflow in a compiled
// KPI card maps to a metric-level semantic path like slides[1].metrics[2].label.
func TestMapFindingExactMatch(t *testing.T) {
	sm := NewSourceMap()
	sm.Add("slides[1].pattern.values[2].small", "slides[1].metrics[2].label", 1)

	got := MapFinding(sm, RawFinding{
		Code:     "BODY_TOO_LONG",
		Message:  "metric caption overflows its card",
		Severity: diagnostics.SeverityWarning,
		Action:   "shrink_or_split",
		RawPath:  "slides[1].pattern.values[2].small",
	})

	if !got.Mapped {
		t.Fatal("expected an exact source-map match")
	}
	if got.SemanticPath != "slides[1].metrics[2].label" {
		t.Errorf("SemanticPath = %q, want slides[1].metrics[2].label", got.SemanticPath)
	}
	if got.SlideIndex != 1 {
		t.Errorf("SlideIndex = %d, want 1", got.SlideIndex)
	}
	if got.RawPath != "slides[1].pattern.values[2].small" {
		t.Errorf("RawPath = %q, want it retained as evidence", got.RawPath)
	}
	if got.Edit == nil || got.Edit.Kind != EditShortenText {
		t.Errorf("expected a shorten_text recommended edit, got %+v", got.Edit)
	}
}

// TestMapFindingParentFallback covers a deeper raw path with only a coarse
// (grid-level) mapping: it resolves to the nearest ancestor, still carries the
// slide index and raw path, and still recommends a semantic edit.
func TestMapFindingParentFallback(t *testing.T) {
	sm := NewSourceMap()
	sm.Add("slides[2].pattern.values", "slides[2].metrics", 2)

	got := MapFinding(sm, RawFinding{
		Code:    "TOO_MANY_ITEMS",
		Message: "too many KPI cards",
		Action:  "refuse",
		RawPath: "slides[2].pattern.values[5].big",
	})

	if !got.Mapped {
		t.Fatal("expected a parent-path fallback match")
	}
	if got.SemanticPath != "slides[2].metrics" {
		t.Errorf("SemanticPath = %q, want slides[2].metrics (parent)", got.SemanticPath)
	}
	if got.SlideIndex != 2 {
		t.Errorf("SlideIndex = %d, want 2", got.SlideIndex)
	}
	if got.RawPath != "slides[2].pattern.values[5].big" {
		t.Errorf("RawPath = %q, want the precise generated pointer retained", got.RawPath)
	}
	// A count-style finding on a KPI snapshot recommends a split, not a shorten.
	if got.Edit == nil || got.Edit.Kind != EditSplitSlide {
		t.Errorf("expected a split_slide recommended edit, got %+v", got.Edit)
	}
}

// TestMapFindingUnmappedFallback covers a raw path with no source-map entry at
// all: the finding still recovers the semantic slide index from the raw prefix
// and retains the raw path so it is never silently dropped.
func TestMapFindingUnmappedFallback(t *testing.T) {
	sm := NewSourceMap()
	sm.Add("slides[0].title", "slides[0].title", 0)

	got := MapFinding(sm, RawFinding{
		Code:    "BASELINE_MISALIGN",
		Message: "shape baseline drifts",
		RawPath: "slides[3].shape_grid.cells[7].text",
	})

	if got.Mapped {
		t.Error("expected no source-map match for an unmapped path")
	}
	if got.SemanticPath != "" {
		t.Errorf("SemanticPath = %q, want empty on a full miss", got.SemanticPath)
	}
	if got.SlideIndex != 3 {
		t.Errorf("SlideIndex = %d, want 3 (recovered from raw prefix)", got.SlideIndex)
	}
	if got.RawPath != "slides[3].shape_grid.cells[7].text" {
		t.Errorf("RawPath = %q, want the raw path retained", got.RawPath)
	}
}

// TestMapFindingNonSlidePath covers a deck-level raw path that carries no slide
// index: the slide index degrades to -1 and no edit is suggested.
func TestMapFindingNonSlidePath(t *testing.T) {
	got := MapFinding(NewSourceMap(), RawFinding{
		Code:    "ACCENT_OVERLOAD",
		Message: "deck uses too many accents",
		RawPath: "accent_strategy",
	})
	if got.SlideIndex != -1 {
		t.Errorf("SlideIndex = %d, want -1 for a non-slide path", got.SlideIndex)
	}
	if got.Edit != nil {
		t.Errorf("expected no recommended edit for an unmapped, non-length finding, got %+v", got.Edit)
	}
}

// TestSuggestSemanticEdit checks the fix catalog: each common density/overflow
// failure maps to the expected semantic edit kind, and non-length findings get
// none.
func TestSuggestSemanticEdit(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		action   string
		path     string
		wantKind string // "" means expect no suggestion
	}{
		{"takeaway", "BODY_TOO_LONG", "shrink_or_split", "slides[0].takeaway", EditShortenText},
		{"chart insight", "BODY_TOO_LONG", "", "slides[0].insight", EditShortenText},
		{"metric label shorten", "HEADLINE_TOO_LONG", "", "slides[1].metrics[2].label", EditShortenText},
		{"kpi count split", "TOO_MANY_KPIS", "", "slides[1].kpis", EditSplitSlide},
		{"roadmap phases", "BODY_TOO_LONG", "", "slides[2].phases[0]", EditSplitPhases},
		{"comparison side", "BODY_TOO_LONG", "", "slides[3].left", EditSimplifySide},
		{"bullet density", "BODY_TOO_LONG", "", "slides[4].points", EditReduceItems},
		{"title shorten", "HEADLINE_TOO_LONG", "", "slides[5].title", EditShortenText},
		{"non-length finding", "MISSING_ALT_TEXT", "info", "slides[0].metrics[0].label", ""},
		{"unmapped path", "BODY_TOO_LONG", "refuse", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := suggestSemanticEdit(tc.code, tc.action, tc.path)
			if tc.wantKind == "" {
				if got != nil {
					t.Errorf("suggestSemanticEdit(%q,%q,%q) = %+v, want nil", tc.code, tc.action, tc.path, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("suggestSemanticEdit(%q,%q,%q) = nil, want kind %q", tc.code, tc.action, tc.path, tc.wantKind)
			}
			if got.Kind != tc.wantKind {
				t.Errorf("edit kind = %q, want %q", got.Kind, tc.wantKind)
			}
			if got.Hint == "" {
				t.Error("recommended edit is missing a hint")
			}
		})
	}
}

// TestSlideIndexFromRawPath checks slide-index recovery from raw paths.
func TestSlideIndexFromRawPath(t *testing.T) {
	cases := map[string]int{
		"slides[0].title":                0,
		"slides[12].shape_grid.cells[3]": 12,
		"$.slides[4].pattern":            4,
		"accent_strategy":                -1,
		"slides[].title":                 -1,
		"slides[x].title":                -1,
		"":                               -1,
	}
	for path, want := range cases {
		if got := slideIndexFromRawPath(path); got != want {
			t.Errorf("slideIndexFromRawPath(%q) = %d, want %d", path, got, want)
		}
	}
}

// TestMapFindingsBatch checks the batch helper preserves order and count.
func TestMapFindingsBatch(t *testing.T) {
	sm := NewSourceMap()
	sm.Add("slides[0].content[0].text_value", "slides[0].title", 0)

	out := MapFindings(sm, []RawFinding{
		{Code: "A", RawPath: "slides[0].content[0].text_value"},
		{Code: "B", RawPath: "slides[9].body"},
	})
	if len(out) != 2 {
		t.Fatalf("MapFindings returned %d findings, want 2", len(out))
	}
	if out[0].Code != "A" || !out[0].Mapped {
		t.Errorf("first finding = %+v, want code A mapped", out[0])
	}
	if out[1].Code != "B" || out[1].Mapped {
		t.Errorf("second finding = %+v, want code B unmapped", out[1])
	}
	if MapFindings(sm, nil) != nil {
		t.Error("MapFindings(nil) should return nil")
	}
}
