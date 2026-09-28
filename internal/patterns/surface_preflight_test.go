package patterns

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TestExpandContextResolveSurface_PreflightMatchesGenerate guards
// go-slide-creator-yjap4: the generate path hands patterns the template
// metadata, while validate / fit-report preflight only carries the ThemeInfo.
// Both must resolve the same surface fill, or contrast preflight predicts a
// swap against a background the generated slide never paints.
func TestExpandContextResolveSurface_PreflightMatchesGenerate(t *testing.T) {
	tints := map[string]string{"subtle": "lt2", "paper": "lt1"}
	generate := ExpandContext{Metadata: &types.TemplateMetadata{SurfaceTints: tints}}
	preflight := ExpandContext{Theme: types.ThemeInfo{SurfaceTints: tints}}

	for _, role := range []string{"subtle", "paper", "undeclared"} {
		want := generate.ResolveSurface(role, "dk1-default")
		if got := preflight.ResolveSurface(role, "dk1-default"); got != want {
			t.Errorf("role %q: preflight resolved %q, generate resolved %q", role, got, want)
		}
	}
	if got := preflight.ResolveSurface("subtle", "dk1-default"); got != "lt2" {
		t.Errorf("preflight subtle = %q, want lt2 from mirrored surface_tints", got)
	}

	// Metadata, when present, stays authoritative over the theme mirror.
	both := ExpandContext{
		Metadata: &types.TemplateMetadata{SurfaceTints: map[string]string{"subtle": "lt1"}},
		Theme:    types.ThemeInfo{SurfaceTints: tints},
	}
	if got := both.ResolveSurface("subtle", "x"); got != "lt1" {
		t.Errorf("metadata should win over theme mirror: got %q, want lt1", got)
	}
	// No tints anywhere -> default.
	if got := (ExpandContext{}).ResolveSurface("subtle", "x"); got != "x" {
		t.Errorf("empty context: got %q, want default", got)
	}
}
