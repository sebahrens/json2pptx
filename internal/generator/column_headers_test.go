package generator

import (
	"strings"
	"testing"
)

func TestIsColumnHeaderLine(t *testing.T) {
	for _, tc := range []struct {
		name    string
		bullets []string
		want    bool
	}{
		{"named column", []string{"Open-weight models", "Run on-premise for data-sensitive workloads", "Fine-tuned cheaply for narrow domains", "Lower cost at high, predictable volume"}, true},
		{"colon header", []string{"What changes for the business team:", "Faster close", "Fewer handoffs"}, true},
		{"parallel short points", []string{"Fast", "Cheap", "Reliable"}, false},
		{"first point is a sentence", []string{"Costs fell sharply.", "Revenue grew in every region this year", "Margins widened on mix"}, false},
		{"too few points", []string{"Open-weight models", "Run on-premise for data-sensitive workloads"}, false},
		{"indented first line", []string{"\tOpen-weight models", "Run on-premise for data-sensitive workloads", "Fine-tuned cheaply for narrow domains"}, false},
		{"numbered list", []string{"1. Scope", "2. Build the platform and the data layer", "3. Scale adoption across the business"}, false},
		{"long first line", []string{"A first point that runs on for eight words", "Short", "Also short"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isColumnHeaderLine(tc.bullets); got != tc.want {
				t.Errorf("isColumnHeaderLine = %v, want %v", got, tc.want)
			}
		})
	}
}

func columnShape(name string, x int64) shapeXML {
	var s shapeXML
	s.NonVisualProperties.ConnectionNonVisual.Name = name
	s.ShapeProperties.Transform = &transformXML{Offset: offsetXML{X: x, Y: 1000000}, Extent: extentXML{CX: 5000000, CY: 4000000}}
	return s
}

func TestMarkColumnHeadersNeedsBothSideBySideColumns(t *testing.T) {
	shapes := []shapeXML{columnShape("body", 400000), columnShape("body_2", 6000000)}
	left := []string{"Open-weight models", "Run on-premise for data-sensitive workloads", "Fine-tuned cheaply for narrow domains"}
	right := []string{"Frontier closed models", "Lead on complex reasoning and long context", "Fastest route to new capabilities"}
	content := []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "Open and closed models compete"},
		{PlaceholderID: "body", Type: ContentBullets, Value: left},
		{PlaceholderID: "body_2", Type: ContentBullets, Value: right},
	}
	got := markColumnHeaders(shapes, content)
	h, ok := got[1].Value.(columnHeaderBullets)
	if !ok || h.Header != "Open-weight models" || len(h.Bullets) != 2 {
		t.Fatalf("left column not marked: %#v", got[1].Value)
	}
	if _, ok := got[2].Value.(columnHeaderBullets); !ok {
		t.Fatalf("right column not marked: %#v", got[2].Value)
	}
	if _, ok := content[1].Value.([]string); !ok {
		t.Fatal("input content was modified")
	}

	// One column without a header line: neither column changes.
	content[2].Value = []string{"Fast", "Cheap", "Reliable"}
	for i, item := range markColumnHeaders(shapes, content)[1:] {
		if _, ok := item.Value.([]string); !ok {
			t.Errorf("column %d marked although only one column has a header line", i)
		}
	}

	// Stacked (not side-by-side) bodies: unchanged.
	content[2].Value = right
	stacked := []shapeXML{columnShape("body", 400000), columnShape("body", 400000)}
	stacked[1].NonVisualProperties.ConnectionNonVisual.Name = "body_2"
	stacked[1].ShapeProperties.Transform.Offset.Y = 6000000
	if _, ok := markColumnHeaders(stacked, content)[1].Value.([]string); !ok {
		t.Error("stacked bodies were marked")
	}
}

func TestColumnHeaderRendersBoldWithoutBullet(t *testing.T) {
	shape := &shapeXML{TextBody: &textBodyXML{BodyProperties: &bodyPropertiesXML{}, ListStyle: &listStyleXML{}, Paragraphs: []paragraphXML{emptyParagraph()}}}
	value := columnHeaderBullets{Header: "Open-weight models", Bullets: []string{"Run on-premise", "Fine-tuned cheaply"}}
	if err := setBulletParagraphs(shape, "body", value, 1); err != nil {
		t.Fatal(err)
	}
	paras := shape.TextBody.Paragraphs
	if len(paras) != 3 {
		t.Fatalf("paragraphs = %d, want header + 2 bullets", len(paras))
	}
	head := paras[0]
	if head.Runs[0].Text != "Open-weight models" || head.Runs[0].RunProperties.Bold != "1" {
		t.Errorf("header run = %+v, want bold header text", head.Runs[0])
	}
	if !strings.Contains(head.Properties.Inner, "<a:buNone/>") || !strings.Contains(head.Properties.Inner, `<a:spcAft><a:spcPts val="600"/>`) {
		t.Errorf("header paragraph props = %q, want buNone and 6pt space after", head.Properties.Inner)
	}
	if head.Properties.Level == nil || *head.Properties.Level != 1 || *head.Properties.MarL != 0 {
		t.Errorf("header should sit at the bullet level, flush left: %+v", head.Properties)
	}
	if paras[1].Runs[0].RunProperties != nil && paras[1].Runs[0].RunProperties.Bold == "1" {
		t.Error("bullets after the header must not be bold")
	}
}
