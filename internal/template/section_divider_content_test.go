package template_test

import (
	"slices"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestSectionDividerWithNumberIsNotContent(t *testing.T) {
	reader, err := template.OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, layout := range layouts {
		if layout.Name != "Section Divider" {
			continue
		}
		if !slices.Contains(layout.Tags, "section-header") || slices.Contains(layout.Tags, "content") {
			t.Fatalf("Section Divider tags = %v, want section-header without content", layout.Tags)
		}
		var title, tagline *types.PlaceholderInfo
		for i := range layout.Placeholders {
			ph := &layout.Placeholders[i]
			switch ph.Role {
			case types.PlaceholderRoleTitle:
				title = ph
			case types.PlaceholderRoleBody:
				tagline = ph
			}
		}
		if title == nil || tagline == nil {
			t.Fatalf("Section Divider needs title and optional tagline body, got %v", layout.Placeholders)
		}
		if tagline.Bounds.Y < title.Bounds.Y+title.Bounds.Height {
			t.Fatalf("tagline starts at %d before title ends at %d", tagline.Bounds.Y, title.Bounds.Y+title.Bounds.Height)
		}
		return
	}
	t.Fatal("Section Divider layout not found")
}
