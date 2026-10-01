package generator

import (
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
)

// go-slide-creator-q4zjj: body_and_lead follows the body density policy
// instead of hardcoding a 16pt lead over 12pt bullets.
func TestBodyAndLeadUsesDensityPolicy(t *testing.T) {
	for _, source := range []int{1200, 1600, 2000, 2800} {
		shape := &shapeXML{
			NonVisualProperties: nonVisualPropertiesXML{NvPr: nvPrXML{Placeholder: &placeholderXML{Type: "body"}}},
			ShapeProperties:     shapePropertiesXML{Transform: &transformXML{Extent: extentXML{CX: 9000000, CY: 4500000}}},
			TextBody: &textBodyXML{BodyProperties: &bodyPropertiesXML{}, ListStyle: &listStyleXML{},
				Paragraphs: []paragraphXML{emptyParagraph()}},
		}
		err := populateShapeText(shape, ContentItem{PlaceholderID: "body", Type: ContentBodyAndLead,
			Value: BodyAndLeadContent{
				Lead:    "Consolidating support desks cuts cost by a fifth",
				Bullets: []string{"Three regional desks duplicate triage", "Ticket history is split", "Global owner is missing", "Retraining takes one quarter"},
			}}, 0, "Arial",
			withInheritedTextStyle(template.InheritedTextStyle{SizeHPt: source}, "Arial"))
		if err != nil {
			t.Fatal(err)
		}
		paras := shape.TextBody.Paragraphs
		if len(paras) != 5 {
			t.Fatalf("paragraphs = %d, want 5", len(paras))
		}
		bulletHPt := source
		if bulletHPt < 1600 || bulletHPt > 2000 {
			bulletHPt = 1800
		}
		for i, p := range paras {
			for _, r := range p.Runs {
				got := r.RunProperties.FontSize
				switch {
				case i == 0:
					if got != strconv.Itoa(bulletHPt+LeadStepHPt) || r.RunProperties.Bold != "1" {
						t.Errorf("source=%d lead sz=%q b=%q, want %d bold", source, got, r.RunProperties.Bold, bulletHPt+LeadStepHPt)
					}
				case got == "1200":
					t.Errorf("source=%d bullet %d hardcoded at 12pt", source, i)
				case got != "" && got != strconv.Itoa(bulletHPt):
					t.Errorf("source=%d bullet %d sz=%q, want %d", source, i, got, bulletHPt)
				}
			}
		}
	}
}

// An authored font_size keeps the lead one step above it.
func TestBodyAndLeadAuthoredSizeKeepsHierarchy(t *testing.T) {
	shape := &shapeXML{
		NonVisualProperties: nonVisualPropertiesXML{NvPr: nvPrXML{Placeholder: &placeholderXML{Type: "body"}}},
		ShapeProperties:     shapePropertiesXML{Transform: &transformXML{Extent: extentXML{CX: 9000000, CY: 4500000}}},
		TextBody: &textBodyXML{BodyProperties: &bodyPropertiesXML{}, ListStyle: &listStyleXML{},
			Paragraphs: []paragraphXML{emptyParagraph()}},
	}
	if err := populateShapeText(shape, ContentItem{PlaceholderID: "body", Type: ContentBodyAndLead, FontSize: 1400,
		Value: BodyAndLeadContent{Lead: "Thesis", Bullets: []string{"One", "Two"}}}, 0, "Arial"); err != nil {
		t.Fatal(err)
	}
	if got := shape.TextBody.Paragraphs[0].Runs[0].RunProperties.FontSize; got != "1600" {
		t.Errorf("lead sz = %q, want 1600", got)
	}
	if got := shape.TextBody.Paragraphs[1].Runs[0].RunProperties.FontSize; got != "1400" {
		t.Errorf("bullet sz = %q, want 1400", got)
	}
}

// When the bullets inherit an in-range size from the layout's list style for
// their level, the lead is one step above that size, not above level 1's.
func TestBodyAndLeadFollowsListStyleLevelSize(t *testing.T) {
	lvl := 2
	shape := &shapeXML{
		NonVisualProperties: nonVisualPropertiesXML{NvPr: nvPrXML{Placeholder: &placeholderXML{Type: "body"}}},
		ShapeProperties:     shapePropertiesXML{Transform: &transformXML{Extent: extentXML{CX: 9000000, CY: 4500000}}},
		TextBody: &textBodyXML{BodyProperties: &bodyPropertiesXML{},
			ListStyle:  &listStyleXML{Inner: `<a:lvl1pPr><a:defRPr sz="2000"/></a:lvl1pPr><a:lvl2pPr><a:defRPr sz="2000"/></a:lvl2pPr><a:lvl3pPr><a:defRPr sz="1800"/></a:lvl3pPr>`},
			Paragraphs: []paragraphXML{emptyParagraph()}},
	}
	if err := populateShapeText(shape, ContentItem{PlaceholderID: "body", Type: ContentBodyAndLead,
		Value: BodyAndLeadContent{Lead: "Thesis", Bullets: []string{"One", "Two"}}}, lvl, "Arial"); err != nil {
		t.Fatal(err)
	}
	if got := shape.TextBody.Paragraphs[0].Runs[0].RunProperties.FontSize; got != "2000" {
		t.Errorf("lead sz = %q, want 2000 (one step above the 18pt level-3 bullets)", got)
	}
}
