package template_test

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
)

// sectionNumberMaxCharsCeiling is the upper bound the validator's
// checkSectionNumberNaming heuristic (cmd/json2pptx/validate_template.go) relies
// on to recognise a decorative number frame. estimateMaxChars must keep the
// reported capacity of a "Section Number" placeholder at or below this ceiling
// once the font size is taken into account.
const sectionNumberMaxCharsCeiling = 5

// TestSectionNumberMaxCharsCorpus iterates every templates/*.pptx file and
// asserts that, after canonical-name normalization, every placeholder resolved
// to the canonical ID "Section Number" reports MaxChars <= 5. These frames are
// rendered at 96-208pt, so their true capacity is a digit or two — the old
// font-agnostic estimator reported 146-297, which corrupted agent planning
// context and silently disabled the section-number naming validator.
//
// Acceptance criteria source: bd go-slide-creator-sjzb.
func TestSectionNumberMaxCharsCorpus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(templatesDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no templates found under %s", templatesDir)
	}
	sort.Strings(files)

	totalFound := 0
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
			// Apply canonical-name normalization in place so decorative number
			// frames resolve to the canonical "Section Number" ID, matching how
			// the validator and agent-facing metadata see them.
			if _, err := template.NormalizeLayoutFiles(reader, layouts); err != nil {
				t.Fatalf("NormalizeLayoutFiles(%s): %v", name, err)
			}

			for _, layout := range layouts {
				for _, ph := range layout.Placeholders {
					if !strings.EqualFold(ph.ID, "Section Number") {
						continue
					}
					totalFound++
					if ph.MaxChars > sectionNumberMaxCharsCeiling {
						t.Errorf(
							"layout %q: Section Number placeholder reports MaxChars=%d (font_size_pt=%.1f, bounds=%dx%d EMU); want <= %d — estimateMaxChars must scale by font size",
							layout.Name, ph.MaxChars, float64(ph.FontSize)/100.0,
							ph.Bounds.Width, ph.Bounds.Height, sectionNumberMaxCharsCeiling,
						)
					}
				}
			}
		})
	}

	// Guard against a vacuous pass: if no Section Number placeholder resolves
	// across the entire corpus, the assertion above never ran and the test would
	// give false confidence.
	if totalFound == 0 {
		t.Fatalf("no Section Number placeholder resolved across %d templates — corpus test is vacuous", len(files))
	}
}
