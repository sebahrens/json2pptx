package template_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
)

// TestLayoutTagsCorpus_NoShippedTemplateHasUntaggedLayout loads every shipped
// templates/*.pptx file and fails if any layout's Tags slice is empty.
//
// An untagged layout is unreachable by the tag-based auto-selection used
// throughout cmd/json2pptx and internal/pipeline: it can only be hit by an
// explicit layout_id pin, which makes it dead weight at best and a source of
// cross-template index collisions at worst. The fix for such a layout is to
// classify/tag it (give it real placeholders or a canonical name) or to remove
// it from the shipped template — never to ship it with an empty tag set.
//
// Acceptance criteria source: bd go-slide-creator-an1n.
func TestLayoutTagsCorpus_NoShippedTemplateHasUntaggedLayout(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(templatesDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no templates found under %s", templatesDir)
	}
	sort.Strings(files)

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

			for _, l := range layouts {
				if len(l.Tags) == 0 {
					t.Errorf(
						"layout %q (%s) in %s has an empty Tags slice — it is unreachable by tag-based selection; "+
							"classify/tag it or remove it from the template",
						l.Name, l.ID, name,
					)
				}
			}
		})
	}
}
