package template

import (
	"slices"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestLayoutClassification(t *testing.T) {
	// AC7: Layout Classification
	// Test that layouts are classified with appropriate tags during parsing
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	layouts, err := ParseLayouts(reader)
	if err != nil {
		t.Fatalf("ParseLayouts() error = %v", err)
	}

	if len(layouts) == 0 {
		t.Fatal("ParseLayouts() returned no layouts")
	}

	// Verify that at least one layout has classification tags
	foundTags := false
	for _, layout := range layouts {
		if len(layout.Tags) > 0 {
			foundTags = true
			t.Logf("Layout %q has tags: %v", layout.Name, layout.Tags)
		}
	}

	if !foundTags {
		t.Error("No layouts have classification tags - ClassifyLayout may not be called")
	}

	// Verify specific layouts have expected tag patterns
	for _, layout := range layouts {
		// If layout has title only (no body/image/chart), should be title-slide
		hasTitle := false
		hasOtherContent := false
		for _, ph := range layout.Placeholders {
			if ph.Type == types.PlaceholderTitle {
				hasTitle = true
			}
			if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderImage ||
				ph.Type == types.PlaceholderChart || ph.Type == types.PlaceholderContent {
				hasOtherContent = true
			}
		}

		if hasTitle && !hasOtherContent && len(layout.Placeholders) > 0 {
			if !slices.Contains(layout.Tags, "title-slide") {
				t.Errorf("Layout %q with title only should have 'title-slide' tag, got: %v", layout.Name, layout.Tags)
			}
		}
	}
}
