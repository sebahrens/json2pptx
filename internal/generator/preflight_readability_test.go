package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
)

// TestAutofitDensityPolicy pins the one definition of the density thresholds
// the render path and the preflight now share. They used to be two copies, and
// the preflight's copy did not exist at all — it always predicted the default
// floor (go-slide-creator-nlrg).
func TestAutofitDensityPolicy(t *testing.T) {
	tests := []struct {
		paragraphs int
		wantFloor  int
		wantMinPct int
	}{
		{1, readabilityMinScale, 0},
		{9, readabilityMinScale, 0},
		{10, 50000, 50},
		{11, 50000, 50},
		{12, 45000, 45},
		{22, 45000, 45},
	}
	for _, tt := range tests {
		floor, minPct := AutofitDensityPolicy(tt.paragraphs)
		if floor != tt.wantFloor || minPct != tt.wantMinPct {
			t.Errorf("AutofitDensityPolicy(%d) = (%d, %d), want (%d, %d)",
				tt.paragraphs, floor, minPct, tt.wantFloor, tt.wantMinPct)
		}
	}
}

// TestInheritedParagraphSpacingPt covers the spacing rule: a master that
// declares a space-before is believed, one that declares none falls back to the
// typical value the renderer has always assumed.
func TestInheritedParagraphSpacingPt(t *testing.T) {
	if got := InheritedParagraphSpacingPt(8); got != 8 {
		t.Errorf("a declared 8pt space-before should be used, got %.1f", got)
	}
	if got := InheritedParagraphSpacingPt(0); got != defaultParagraphSpacingPt {
		t.Errorf("no declared spacing should fall back to %.1f, got %.1f", defaultParagraphSpacingPt, got)
	}
}

// TestPreflightReportsUnreadableBody is the gap the bead names: validate must
// say TEXT_BELOW_READABLE_MIN wherever generate would. The predictor needs the
// per-paragraph spacing to get there — without it the same content predicted a
// font scale comfortably above the floor.
func TestPreflightReportsUnreadableBody(t *testing.T) {
	paragraphs := make([]string, 22)
	for i := range paragraphs {
		paragraphs[i] = "A long line of body copy that keeps going well past the point where a body placeholder can hold it at a readable size, with several clauses"
	}
	in := TextAutofitPreflightInput{
		Path:           "/slides/0/content/body",
		Paragraphs:     paragraphs,
		WidthEMU:       10515600,
		HeightEMU:      4351338,
		FontSizeHPt:    2000,
		FontName:       "Calibri",
		ViewingMode:    tokens.ViewingModePresentation,
		TextRole:       tokens.TextRoleBody,
		ExtraSpacingPt: 8,
	}

	var codes []string
	for _, f := range DetectTextAutofitPreflight(in) {
		codes = append(codes, f.Code)
	}
	if !hasReadabilityCode(codes) {
		t.Errorf("expected TEXT_BELOW_READABLE_MIN, got %v", codes)
	}

	// And the spacing is decisive, not a refinement: budgeting none for the
	// same 22 paragraphs predicts a font scale comfortably above the floor and
	// reports nothing at all. That is exactly the shape of the bug — validate
	// said the deck was fine while generate rendered it at 10pt.
	in.ExtraSpacingPt = 0
	if got := DetectTextAutofitPreflight(in); len(got) != 0 {
		t.Errorf("without the per-paragraph spacing the prediction should miss it entirely, got %d findings", len(got))
	}
}

func hasReadabilityCode(codes []string) bool {
	for _, c := range codes {
		if c == "TEXT_BELOW_READABLE_MIN" {
			return true
		}
	}
	return false
}
