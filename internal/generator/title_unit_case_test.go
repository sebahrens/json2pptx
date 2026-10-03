package generator

import (
	"encoding/xml"
	"strings"
	"testing"
)

// modern-template renders titles in capitals, turning "€2.2m" into "€2.2M" and
// "6 mo" into "6 MO" (go-slide-creator-3rg5f).
func TestUnitCaseTokens(t *testing.T) {
	cases := []struct {
		title string
		want  []string
	}{
		{"Savings reach €2.2m in 6 mo", []string{"€2.2m", "6 mo"}},
		{"Revenue of $12bn, up 3pp on 40k units", []string{"$12bn", "3pp", "40k"}},
		{"Margin rises 2.5 pp to 12 bn", []string{"2.5 pp", "12 bn"}},
		{"Latency falls to 200ms at 10x the load", []string{"200ms", "10x"}},
		{"Each site draws 40 kWh", []string{"40 kWh"}},
		{"(€2.2m) saved", []string{"(€2.2m"}},
		// Not units: words, ordinals, already-capital tokens, plain numbers.
		{"6 in 10 customers renew", nil},
		{"3 key findings from the 1st review", nil},
		{"5G rollout reaches Q3 targets in B2B", nil},
		{"2024. A big year", nil},
		{"Growth of 12 months", nil},
		{"12months of growth", nil},
		{"Savings reach €2.2M in 6 MO", nil},
		{"", nil},
	}
	for _, tc := range cases {
		got := UnitCaseTokens(tc.title)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("UnitCaseTokens(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestPreserveUnitCase(t *testing.T) {
	base := &runPropertiesXML{Lang: "en-US", FontSize: "2800", Bold: "1"}
	runs := preserveUnitCase([]runXML{{RunProperties: base, Text: "Savings reach €2.2m in 6 mo"}})

	var texts, kept []string
	for _, r := range runs {
		texts = append(texts, r.Text)
		if r.RunProperties.Caps == "none" {
			kept = append(kept, r.Text)
		}
		if r.RunProperties.FontSize != "2800" || r.RunProperties.Bold != "1" {
			t.Errorf("run %q lost its formatting: %+v", r.Text, r.RunProperties)
		}
	}
	if strings.Join(texts, "") != "Savings reach €2.2m in 6 mo" {
		t.Errorf("text changed: %q", texts)
	}
	if strings.Join(kept, "|") != "€2.2m|6 mo" {
		t.Errorf("cap=none runs = %q, want €2.2m and 6 mo", kept)
	}
	if base.Caps != "" {
		t.Error("the template run properties were mutated")
	}

	out, err := xml.Marshal(runs[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `cap="none"`) {
		t.Errorf("unit run does not serialise cap=none: %s", out)
	}

	// A title with no unit is returned as it came: no extra runs.
	plain := []runXML{{RunProperties: base, Text: "Hub consolidation scores best"}}
	if got := preserveUnitCase(plain); len(got) != 1 || got[0].RunProperties != base {
		t.Errorf("a title without units was rewritten: %+v", got)
	}
}

// Through the title paths: a content-slide title and a title-slide title keep
// the unit's case; body text is left alone (its caps are stripped anyway).
func TestTitlePathsPreserveUnitCase(t *testing.T) {
	const title = "Savings reach €2.2m in 6 mo"
	keptUnits := func(shape *shapeXML) string {
		var kept []string
		for _, p := range shape.TextBody.Paragraphs {
			for _, r := range p.Runs {
				if r.RunProperties != nil && r.RunProperties.Caps == "none" {
					kept = append(kept, r.Text)
				}
			}
		}
		return strings.Join(kept, "|")
	}
	newShape := func(phType string) *shapeXML {
		s := &shapeXML{TextBody: &textBodyXML{BodyProperties: &bodyPropertiesXML{}}}
		s.NonVisualProperties.NvPr.Placeholder = &placeholderXML{Type: phType}
		return s
	}

	content := newShape("title")
	if err := setTextParagraph(content, "title", title, 2400, "Arial"); err != nil {
		t.Fatal(err)
	}
	if got := keptUnits(content); got != "€2.2m|6 mo" {
		t.Errorf("content title: cap=none runs = %q", got)
	}

	cover := newShape("ctrTitle")
	if err := setTitleSlideTitle(cover, "title", title, "Arial"); err != nil {
		t.Fatal(err)
	}
	if got := keptUnits(cover); got != "€2.2m|6 mo" {
		t.Errorf("title-slide title: cap=none runs = %q", got)
	}

	body := newShape("body")
	if err := setTextParagraph(body, "body", title, 2400, "Arial"); err != nil {
		t.Fatal(err)
	}
	if got := keptUnits(body); got != "" {
		t.Errorf("body text should not be split: %q", got)
	}
}
