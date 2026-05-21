package template_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// canonicalTitleID is the single portable name every title-typed placeholder
// must use on disk. Authors and agents reading template metadata can rely on
// it regardless of which template they target.
const canonicalTitleID = "title"

// TestTitlePlaceholdersAreCanonicalOnDisk asserts that every shipped template
// names all of its title-typed placeholders exactly "title" (case-sensitive)
// on disk, without any runtime normalization applied.
//
// PowerPoint hands out default names like "Title 1", "Title 4", and the
// copy-paste stray "Title 123", which leaves JSON authors and agents unable to
// rely on a single placeholder_id for the title slot. The runtime resolver
// papers over this, but agent-facing template evidence (validate-template,
// examine-template) and future CI gates read the on-disk names directly, so the
// canonical name must be baked into the files.
//
// This test deliberately does NOT call NormalizeLayoutFiles: it inspects the
// raw on-disk shape names so it fails if a new or re-exported template ships
// with un-normalized title placeholders.
//
// Acceptance criteria source: bd go-slide-creator-jtlb.
func TestTitlePlaceholdersAreCanonicalOnDisk(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(templatesDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no templates found under %s", templatesDir)
	}
	sort.Strings(files)

	for _, file := range files {
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

			titlesSeen := 0
			for _, layout := range layouts {
				for _, ph := range layout.Placeholders {
					if ph.Type != types.PlaceholderTitle {
						continue
					}
					titlesSeen++
					if ph.ID != canonicalTitleID {
						t.Errorf(
							"template %s layout %q: title placeholder named %q on disk, want %q — "+
								"run NormalizePlaceholderNames over the template before committing it",
							name, layout.Name, ph.ID, canonicalTitleID,
						)
					}
				}
			}

			if titlesSeen == 0 {
				t.Errorf("template %s has no title-typed placeholders in any layout — "+
					"expected at least one title slot", name)
			}
		})
	}
}
