package testrand

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/policy/emoji"
)

// TestGeneratedCorpusHasNoEmoji is the producer-side counterpart to the
// cmd/json2pptx no-emoji enforcer (go-slide-creator-62xl): it asserts the
// random generator's corpus passes the exact validator the engine runs at the
// generate/validate boundary. The generator routes its edge-case corpus
// through emoji.Sanitize in maybeEdge, so this is clean by construction; the
// test guards against future drift between producer and enforcer.
func TestGeneratedCorpusHasNoEmoji(t *testing.T) {
	for seed := uint64(0); seed < 100; seed++ {
		deck := New(seed).Generate()
		if findings := emoji.ValidateNoEmojiInText(deck); len(findings) != 0 {
			for _, f := range findings {
				t.Errorf("seed %d: %s — %s", seed, f.Path, f.Message)
			}
		}
	}
}

// TestVisualDeckHasNoEmoji is the direct regression guard for the class of bug
// in go-slide-creator-6wnl (the visual stress deck violating the no-emoji
// policy). The visual deck is hand-authored from string literals, so rather
// than silently strip emoji it asserts the literals are clean — a loud failure
// here tells the author to fix the source instead of relying on a sanitizer.
func TestVisualDeckHasNoEmoji(t *testing.T) {
	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			deck := NewVisualDeckGenerator(tmpl).Generate()
			if findings := emoji.ValidateNoEmojiInText(deck); len(findings) != 0 {
				for _, f := range findings {
					t.Errorf("%s: %s — %s", tmpl, f.Path, f.Message)
				}
			}
		})
	}
}

// TestMaybeEdgeStripsEmojiFromCorpus proves the by-construction wiring: even if
// the edge-case corpus is polluted with emoji, maybeEdge routes it through the
// shared sanitizer so generated text never carries emoji codepoints.
func TestMaybeEdgeStripsEmojiFromCorpus(t *testing.T) {
	original := edgeCaseStrings
	edgeCaseStrings = append(append([]string{}, original...), "Polluted 🚀 corpus")
	defer func() { edgeCaseStrings = original }()

	g := New(7)
	for i := 0; i < 5000; i++ {
		if emoji.Contains(g.maybeEdge("clean")) {
			t.Fatalf("maybeEdge returned text with emoji despite shared sanitizer")
		}
	}
}
