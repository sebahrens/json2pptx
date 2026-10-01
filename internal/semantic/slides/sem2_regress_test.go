package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

// An over-budget image case kept only "Image: <caption>"; it now keeps the
// picture beside the text (go-slide-creator-csclk.52).
func TestImageCaseDegradeKeepsThePicture(t *testing.T) {
	body := imageCaseBody(map[string]any{
		"body":       strings.Repeat("a", 400),
		"bullets":    []any{"b1"},
		"image":      "https://example.com/a.png",
		"caption":    "CAP",
		"image_side": "right",
	})
	slide, _, err := CompileImageCase(Input{Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern != nil || slide.LayoutID != "two-column" || len(slide.Content) != 2 {
		t.Fatalf("want a two-column fallback with 2 items, got %+v", slide)
	}
	img := slide.Content[1]
	if img.Type != "image" || img.PlaceholderID != "body_2" || img.ImageValue == nil || img.ImageValue.URL != "https://example.com/a.png" || img.ImageValue.Alt != "CAP" {
		t.Fatalf("picture not kept on the right: %+v", img)
	}
	encoded, _ := json.Marshal(slide.Content[0])
	for _, want := range []string{strings.Repeat("a", 400), "b1", "CAP"} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("text column dropped %q", want)
		}
	}
}

// Team members accept a headshot (go-slide-creator-csclk.92).
func TestTeamMemberPhoto(t *testing.T) {
	members := TeamMembers(map[string]any{"members": []any{
		map[string]any{"name": "Ada", "role": "CTO", "photo": "/img/ada.png"},
		map[string]any{"name": "Bo", "role": "CFO", "photo": map[string]any{"url": "https://x/bo.png", "alt": "Bo"}},
		map[string]any{"name": "Cy", "role": "COO"},
	}})
	if len(members) != 3 || members[0].Photo == nil || members[0].Photo.Path != "/img/ada.png" ||
		members[1].Photo == nil || members[1].Photo.URL != "https://x/bo.png" || members[1].Photo.Alt != "Bo" || members[2].Photo != nil {
		t.Fatalf("photos not mapped: %+v", members)
	}
}

// Sparse bare-label flows compile to the compact flow, a content-sized band
// top-anchored under the title, so SPARSE_SINGLE_ROW_FLOW does not fire
// against the compiler's own choice (go-slide-creator-csclk.56) and the boxes
// do not stretch over a capped half-slide band (go-slide-creator-xb06p).
func TestSparseProcessFlowIsCompact(t *testing.T) {
	slide, _, err := CompileProcess(Input{Body: map[string]any{
		"title": "How a deal closes", "steps": []any{"Qualify", "Discover", "Propose", "Close"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "process-flow-compact" || slide.Pattern.MaxHeightPct != 0 {
		t.Fatalf("want an uncapped process-flow-compact, got %+v", slide.Pattern)
	}
}
