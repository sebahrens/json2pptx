package generator

import (
	"fmt"
	"strings"
	"testing"
)

// assertNoLiteralLatin fails if a run carries a latin typeface other than the
// theme minor-font token (+mn-lt). Template fidelity requires generated runs
// to inherit the template's fonts (go-slide-creator-tjd0).
func assertNoLiteralLatin(t *testing.T, where string, runs []runXML) {
	t.Helper()
	for _, r := range runs {
		if r.RunProperties == nil {
			continue
		}
		inner := r.RunProperties.Inner
		if !strings.Contains(inner, "<a:latin") {
			continue
		}
		if !strings.Contains(inner, `typeface="+mn-lt"`) {
			t.Errorf("%s: run %q has literal latin typeface: %s", where, r.Text, inner)
		}
	}
}

func TestBodyAndBulletsLeadParagraph_InheritsTemplateFont(t *testing.T) {
	shape := &shapeXML{TextBody: &textBodyXML{Paragraphs: []paragraphXML{{}}}}
	err := setBodyAndBulletsParagraphs(shape, "body", BodyAndBulletsContent{
		Body:    "Lead paragraph acting as header",
		Bullets: []string{"one", "two"},
	}, -1)
	if err != nil {
		t.Fatal(err)
	}
	lead := shape.TextBody.Paragraphs[0]
	if len(lead.Runs) == 0 {
		t.Fatal("lead paragraph has no runs")
	}
	for _, r := range lead.Runs {
		if r.RunProperties == nil || r.RunProperties.Bold != "1" {
			t.Errorf("lead run %q should stay bold", r.Text)
		}
	}
	for i, p := range shape.TextBody.Paragraphs {
		assertNoLiteralLatin(t, fmt.Sprintf("paragraph %d", i), p.Runs)
	}
}

func TestGroupLabel_InheritsTemplateFont(t *testing.T) {
	paras := buildGroupParagraphs(BulletGroup{GroupLabel: "Phase 1", Header: "Header", Bullets: []string{"a"}}, false, "600", 0, nil)
	for _, p := range paras {
		assertNoLiteralLatin(t, "group", p.Runs)
	}
}

func TestEyebrow_UsesThemeMinorFont(t *testing.T) {
	shape := &shapeXML{TextBody: &textBodyXML{Paragraphs: []paragraphXML{{Runs: []runXML{{Text: "Title"}}}}}}
	prependEyebrowParagraph(shape, "CONTEXT", 1200)
	eb := shape.TextBody.Paragraphs[0].Runs[0]
	if !strings.Contains(eb.RunProperties.Inner, `typeface="+mn-lt"`) {
		t.Errorf("eyebrow should use +mn-lt, got %s", eb.RunProperties.Inner)
	}
	assertNoLiteralLatin(t, "eyebrow", shape.TextBody.Paragraphs[0].Runs)
}

func TestEyebrow_FollowsTitleAlignmentAndReadableSize(t *testing.T) {
	for _, tt := range []struct {
		name      string
		titleAlgn string
		margin    int
		fontSize  int
		wantSize  string
	}{
		{"center", "ctr", 0, 1400, "1400"},
		{"right with margin", "r", 250000, 1800, "1800"},
		{"inherited alignment", "", 0, 1000, "1200"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var margin *int
			if tt.margin > 0 {
				margin = &tt.margin
			}
			shape := &shapeXML{TextBody: &textBodyXML{Paragraphs: []paragraphXML{{
				Properties: &paragraphPropertiesXML{Algn: tt.titleAlgn, MarL: margin},
				Runs:       []runXML{{Text: "Title"}},
			}}}}
			prependEyebrowParagraph(shape, "CONTEXT", tt.fontSize)
			eb := shape.TextBody.Paragraphs[0]
			if eb.Properties == nil || eb.Properties.Algn != tt.titleAlgn {
				t.Errorf("eyebrow alignment = %+v, want %q", eb.Properties, tt.titleAlgn)
			}
			if tt.margin > 0 && (eb.Properties.MarL == nil || *eb.Properties.MarL != tt.margin) {
				t.Errorf("eyebrow margin = %+v, want %d", eb.Properties.MarL, tt.margin)
			}
			if got := eb.Runs[0].RunProperties.FontSize; got != tt.wantSize {
				t.Errorf("eyebrow font size = %q, want %q", got, tt.wantSize)
			}
			if shape.TextBody.Paragraphs[1].Runs[0].Text != "Title" {
				t.Error("eyebrow replaced the title paragraph")
			}
		})
	}
}

func TestEyebrowFontSize_FollowsSubtitle(t *testing.T) {
	if got := eyebrowFontSize(nil); got != 1200 {
		t.Errorf("eyebrowFontSize(nil) = %d, want 1200", got)
	}
	for _, tt := range []struct {
		name     string
		subtitle int
		want     int
	}{
		{"missing", 0, 1200},
		{"small subtitle", 1400, 1200},
		{"medium subtitle", 2000, 1400},
		{"large subtitle", 3000, 1800},
	} {
		t.Run(tt.name, func(t *testing.T) {
			slide := &slideXML{}
			if tt.subtitle > 0 {
				slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{{
					NonVisualProperties: nonVisualPropertiesXML{NvPr: nvPrXML{Placeholder: &placeholderXML{Type: "subTitle"}}},
					TextBody: &textBodyXML{Paragraphs: []paragraphXML{{Runs: []runXML{{
						RunProperties: &runPropertiesXML{FontSize: fmt.Sprintf("%d", tt.subtitle)},
					}}}}},
				}}
			}
			if got := eyebrowFontSize(slide); got != tt.want {
				t.Errorf("eyebrowFontSize() = %d, want %d", got, tt.want)
			}
		})
	}
	bodyOnly := &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: []shapeXML{{
		NonVisualProperties: nonVisualPropertiesXML{NvPr: nvPrXML{Placeholder: &placeholderXML{Type: "body"}}},
		TextBody: &textBodyXML{Paragraphs: []paragraphXML{{Runs: []runXML{{
			RunProperties: &runPropertiesXML{FontSize: "3000"},
		}}}}},
	}}}}}
	if got := eyebrowFontSize(bodyOnly); got != 1200 {
		t.Errorf("eyebrowFontSize(body-only slide) = %d, want 1200", got)
	}
}
