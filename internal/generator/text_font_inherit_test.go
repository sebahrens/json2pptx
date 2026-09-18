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
	prependEyebrowParagraph(shape, "CONTEXT")
	eb := shape.TextBody.Paragraphs[0].Runs[0]
	if !strings.Contains(eb.RunProperties.Inner, `typeface="+mn-lt"`) {
		t.Errorf("eyebrow should use +mn-lt, got %s", eb.RunProperties.Inner)
	}
	assertNoLiteralLatin(t, "eyebrow", shape.TextBody.Paragraphs[0].Runs)
}
