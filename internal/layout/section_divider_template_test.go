package layout_test

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestModernTemplateAutomaticNumberUsesNumberFrame(t *testing.T) {
	reader, err := template.OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		automatic bool
		want      string
	}{
		{"automatic number", true, "Section Number"},
		{"authored tagline", false, "body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slide := types.SlideDefinition{Type: types.SlideTypeSection, Title: "Strategy", AutoSectionNumber: tc.automatic, Content: types.SlideContent{Body: "01"}}
			result, err := layout.SelectLayout(layout.SelectionRequest{Slide: slide, Layouts: layouts, Context: layout.SelectionContext{Position: 2, TotalSlides: 5}})
			if err != nil {
				t.Fatal(err)
			}
			if result.LayoutID != "slideLayout2" {
				t.Fatalf("selected %s, want divider", result.LayoutID)
			}
			for _, mapping := range result.Mappings {
				if mapping.ContentField == "body" {
					if mapping.PlaceholderID != tc.want {
						t.Errorf("body maps to %q, want %q", mapping.PlaceholderID, tc.want)
					}
					return
				}
			}
			t.Fatal("no body mapping")
		})
	}
}

func TestModernTemplateSelectsDividerOnlyForSections(t *testing.T) {
	reader, err := template.OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name        string
		slide       types.SlideDefinition
		wantDivider bool
	}{
		{"section", types.SlideDefinition{Type: types.SlideTypeSection, Title: "Strategy"}, true},
		{"body", types.SlideDefinition{Type: types.SlideTypeContent, Title: "Results", Content: types.SlideContent{Body: "Details"}}, false},
		{"bullets", types.SlideDefinition{Type: types.SlideTypeContent, Title: "Results", Content: types.SlideContent{Bullets: []string{"First", "Second"}}}, false},
		{"table", types.SlideDefinition{Type: types.SlideTypeContent, Title: "Results", Content: types.SlideContent{TableRaw: "| A | B |"}}, false},
		{"diagram", types.SlideDefinition{Type: types.SlideTypeDiagram, Title: "Results", Content: types.SlideContent{DiagramSpec: &types.DiagramSpec{Type: "bar_chart"}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := layout.SelectLayout(layout.SelectionRequest{Slide: tc.slide, Layouts: layouts, Context: layout.SelectionContext{Position: 2, TotalSlides: 5}})
			if err != nil {
				t.Fatal(err)
			}
			if got := result.LayoutID == "slideLayout2"; got != tc.wantDivider {
				t.Errorf("selected %s; divider = %v, want %v", result.LayoutID, got, tc.wantDivider)
			}
		})
	}
}
