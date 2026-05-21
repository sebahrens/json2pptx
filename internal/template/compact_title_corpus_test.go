package template_test

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestCompactTitleCorpus iterates every templates/*.pptx file and checks the
// "compact-title" layout tag emitted by ClassifyLayout. The tag flags the
// specific truncation hazard from go-slide-creator-plsd: a small title slot
// placed low on the slide (title-at-bottom geometry) beneath a large decorative
// element. modern-yellow's Section Divider is the canonical example — a 48pt
// title squeezed below a 208pt "Section Number" frame.
//
// Invariants asserted across the corpus:
//   - "compact-title" is always emitted alongside "title-at-bottom" (it is a
//     capacity refinement of that geometry tag, never a standalone signal).
//   - Any compact-title layout has a visible title placeholder whose capacity is
//     below the single-line ceiling, confirming the tag tracks real capacity.
//   - modern-yellow's Section Divider IS tagged compact-title; roomy section
//     dividers (title in the upper half with ample capacity) are NOT.
func TestCompactTitleCorpus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(templatesDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no templates found under %s", templatesDir)
	}
	sort.Strings(files)

	// compactTitleCeiling mirrors classifier.compactTitleMaxChars. Kept as a
	// local literal because the constant is unexported; if the classifier ceiling
	// changes, this corpus expectation must be revisited deliberately.
	const compactTitleCeiling = 35

	totalCompact := 0
	modernYellowSectionTagged := false

	for _, file := range files {
		file := file
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			reader, err := template.OpenTemplate(file)
			if err != nil {
				t.Fatalf("OpenTemplate(%s): %v", name, err)
			}
			defer func() { _ = reader.Close() }()

			layouts, err := template.ParseLayouts(reader)
			if err != nil {
				t.Fatalf("ParseLayouts(%s): %v", name, err)
			}

			for _, layout := range layouts {
				if !slices.Contains(layout.Tags, "compact-title") {
					continue
				}
				totalCompact++

				// Invariant: compact-title is a refinement of title-at-bottom.
				if !slices.Contains(layout.Tags, "title-at-bottom") {
					t.Errorf("layout %q (%s) tagged compact-title without title-at-bottom (tags: %v)",
						layout.Name, name, layout.Tags)
				}

				// Invariant: a visible title placeholder must back the tag with a
				// genuinely small capacity.
				if !hasCompactVisibleTitle(layout, compactTitleCeiling) {
					t.Errorf("layout %q (%s) tagged compact-title but no visible title has 0 < MaxChars < %d (tags: %v)",
						layout.Name, name, compactTitleCeiling, layout.Tags)
				}

				if name == "modern-yellow.pptx" && strings.EqualFold(layout.Name, "Section Divider") {
					modernYellowSectionTagged = true
				}
			}
		})
	}

	if totalCompact == 0 {
		t.Fatalf("no layout tagged compact-title across %d templates — corpus test is vacuous", len(files))
	}
	if !modernYellowSectionTagged {
		t.Errorf("modern-yellow Section Divider was not tagged compact-title; it is the canonical small-title-at-bottom layout (go-slide-creator-plsd)")
	}
}

// hasCompactVisibleTitle reports whether layout has a visible title placeholder
// whose estimated capacity is below ceiling (and known, i.e. > 0).
func hasCompactVisibleTitle(layout types.LayoutMetadata, ceiling int) bool {
	for _, ph := range layout.Placeholders {
		if ph.Type != types.PlaceholderTitle || ph.Bounds.Y < 0 {
			continue
		}
		if ph.MaxChars > 0 && ph.MaxChars < ceiling {
			return true
		}
	}
	return false
}
