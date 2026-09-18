package pipeline

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

func TestAuthoringManifestRoundTripAndStableSlideIDs(t *testing.T) {
	source := []byte("slides:\n  - kind: title\n  - kind: closing\n")
	compiled := json.RawMessage(`{"slides":[{},{}]}`)
	map1 := semantic.NewSourceMap()
	map1.Add("slides[0].content[0]", "slides[0].title", 0)
	a := []json.RawMessage{json.RawMessage(`{"kind":"title"}`), json.RawMessage(`{"kind":"closing"}`)}
	m := NewAuthoringManifest(source, "yaml", "template.pptx", "template-hash", compiled, "deck.pptx", "pptx-hash", map1, a, nil)
	path := filepath.Join(t.TempDir(), "deck.pptx.authoring.json")
	if err := WriteAuthoringManifest(path, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAuthoringManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != string(source) || got.SourceMap["slides[0].content[0]"].SemanticPath != "slides[0].title" {
		t.Fatalf("round trip lost source intent: %+v", got)
	}

	reordered := NewAuthoringManifest(source, "yaml", "template.pptx", "template-hash", compiled, "deck.pptx", "pptx-hash", map1, []json.RawMessage{a[1], a[0]}, nil)
	if reordered.Slides[0].ID != m.Slides[1].ID || reordered.Slides[1].ID != m.Slides[0].ID {
		t.Fatalf("IDs changed on reorder: before=%+v after=%+v", m.Slides, reordered.Slides)
	}
}

func TestAuthoringManifestInvalidatesVisualEvidence(t *testing.T) {
	m := NewAuthoringManifest([]byte("source"), "yaml", "template.pptx", "template-a", json.RawMessage(`{}`), "deck.pptx", "artifact-a", nil, nil, nil)
	m.VisualEvidence = &VisualEvidence{ArtifactSHA256: "artifact-a", ReviewedSlides: []string{"slide-1"}, Verdict: "approved"}
	if !m.EvidenceCurrent(m.SourceSHA256, "template-a", "artifact-a") {
		t.Fatal("matching evidence should be current")
	}
	for _, tc := range []struct{ source, template, artifact string }{
		{"changed", "template-a", "artifact-a"},
		{m.SourceSHA256, "template-b", "artifact-a"},
		{m.SourceSHA256, "template-a", "artifact-b"},
	} {
		if m.EvidenceCurrent(tc.source, tc.template, tc.artifact) {
			t.Fatalf("stale evidence accepted: %+v", tc)
		}
	}
}
