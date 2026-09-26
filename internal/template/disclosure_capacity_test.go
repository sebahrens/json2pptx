package template

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDisclosureDoesNotSupplyOrdinaryTemplateCapacity(t *testing.T) {
	for _, kind := range []types.PlaceholderType{types.PlaceholderSubtitle, types.PlaceholderBody, types.PlaceholderContent} {
		t.Run(string(kind), func(t *testing.T) {
			legal := types.PlaceholderInfo{ID: "legal_disclosure", Type: kind, MaxChars: 400, Bounds: types.BoundingBox{X: 100000, Y: 5000000, Width: 6000000, Height: 1000000}}
			layout := types.LayoutMetadata{ID: "legal-only", Placeholders: []types.PlaceholderInfo{legal}}
			counts := countPlaceholders(layout.Placeholders)
			if counts.body != 0 || counts.usableBody != 0 || counts.subtitle != 0 {
				t.Errorf("legal text advertised ordinary body/subtitle capacity: %+v", counts)
			}
			for _, requirement := range []string{"body", "subtitle"} {
				if matchesPlaceholderRequirement(legal, requirement) {
					t.Errorf("legal text satisfies mandatory ordinary %q requirement", requirement)
				}
			}
			if scanCapabilities([]types.LayoutMetadata{layout}).hasBodyHolder {
				t.Error("legal-only layout advertised generic grid/body capability")
			}
			if _, _, ok := contentSpan(&layout, 12192000); ok {
				t.Error("legal text supplies generic body horizontal bounds")
			}
			signature := layoutSignature(layout.Placeholders, counts)
			if !strings.Contains(signature, "disclosure") || strings.Contains(signature, "subtitle") || strings.Contains(signature, "body") {
				t.Errorf("legal role absent or conflated in structural signature: %q", signature)
			}
			layout.CanonicalType = types.CanonicalLayoutBlank
			rect := BlankContentRect(&layout, 12192000, 6858000)
			if rect.Y+rect.Height > legal.Bounds.Y {
				t.Errorf("blank content rectangle enters required legal text: %+v", rect)
			}
		})
	}
}

func TestOrdinarySubtitleAndBodyCapacityRemainAvailable(t *testing.T) {
	for _, kind := range []types.PlaceholderType{types.PlaceholderSubtitle, types.PlaceholderBody} {
		ph := types.PlaceholderInfo{ID: "ordinary", Type: kind, MaxChars: 400}
		counts := countPlaceholders([]types.PlaceholderInfo{ph})
		if kind == types.PlaceholderSubtitle && counts.subtitle != 1 {
			t.Fatal("ordinary subtitle lost")
		}
		if kind == types.PlaceholderBody && (counts.body != 1 || counts.usableBody != 1) {
			t.Fatal("ordinary body lost")
		}
	}
}
