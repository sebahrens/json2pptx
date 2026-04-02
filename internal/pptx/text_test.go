package pptx

import (
	"bytes"
	"strings"
	"testing"
)

func TestTextBody_Empty(t *testing.T) {
	t.Parallel()
	tb := TextBody{}
	var buf bytes.Buffer
	tb.WriteTo(&buf)
	got := buf.String()
	// Empty textbody should have bodyPr, lstStyle, and at least one empty paragraph
	if !strings.Contains(got, `<a:bodyPr>`) {
		t.Error("missing bodyPr")
	}
	if !strings.Contains(got, `<a:lstStyle/>`) {
		t.Error("missing lstStyle")
	}
	if !strings.Contains(got, `<a:p/>`) {
		t.Error("missing empty paragraph")
	}
}

func TestTextBody_WithBodyPrAttributes(t *testing.T) {
	t.Parallel()
	tb := TextBody{
		Wrap:      "square",
		Anchor:    "ctr",
		AnchorCtr: true,
		Insets:    [4]int64{91440, 45720, 91440, 45720},
		AutoFit:   "normAutofit",
	}
	var buf bytes.Buffer
	tb.WriteTo(&buf)
	got := buf.String()

	checks := []string{
		`wrap="square"`,
		`anchor="ctr"`,
		`anchorCtr="1"`,
		`lIns="91440"`,
		`tIns="45720"`,
		`rIns="91440"`,
		`bIns="45720"`,
		`<a:normAutofit/>`,
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("missing %q in:\n%s", c, got)
		}
	}
}

func TestTextBody_SingleRun(t *testing.T) {
	t.Parallel()
	tb := TextBody{
		Paragraphs: []Paragraph{{
			Align: "ctr",
			Runs: []Run{{
				Text:     "Hello World",
				FontSize: 2400,
				Bold:     true,
				Lang:     "en-US",
			}},
		}},
	}
	var buf bytes.Buffer
	tb.WriteTo(&buf)
	got := buf.String()

	checks := []string{
		`<a:pPr algn="ctr"/>`,
		`<a:rPr lang="en-US" sz="2400" b="1"/>`,
		`<a:t>Hello World</a:t>`,
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("missing %q in:\n%s", c, got)
		}
	}
}

func TestTextBody_RunWithColor(t *testing.T) {
	t.Parallel()
	tb := TextBody{
		Paragraphs: []Paragraph{{
			Runs: []Run{{
				Text:  "Colored",
				Color: SolidFill("FF0000"),
			}},
		}},
	}
	var buf bytes.Buffer
	tb.WriteTo(&buf)
	got := buf.String()

	if !strings.Contains(got, `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:rPr>`) {
		t.Errorf("missing color in rPr:\n%s", got)
	}
}

func TestTextBody_Bullets(t *testing.T) {
	t.Parallel()
	tb := TextBody{
		Paragraphs: []Paragraph{{
			Bullet: &BulletDef{
				Char:  "\u2022",
				Font:  "Arial",
				Color: SolidFill("4472C4"),
			},
			MarginL: 342900,
			Indent:  -342900,
			Runs: []Run{{
				Text:     "Bullet item",
				FontSize: 1800,
			}},
		}},
	}
	var buf bytes.Buffer
	tb.WriteTo(&buf)
	got := buf.String()

	checks := []string{
		`marL="342900"`,
		`indent="-342900"`,
		`<a:buClr><a:srgbClr val="4472C4"/></a:buClr>`,
		`<a:buFont typeface="Arial"/>`,
		`<a:buChar char="•"/>`,
		`<a:t>Bullet item</a:t>`,
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("missing %q in:\n%s", c, got)
		}
	}
}

func TestTextBody_MultipleParagraphs(t *testing.T) {
	t.Parallel()
	tb := TextBody{
		Anchor: "t",
		Paragraphs: []Paragraph{
			{Runs: []Run{{Text: "First", Bold: true}}},
			{Runs: []Run{{Text: "Second", Italic: true}}},
		},
	}
	var buf bytes.Buffer
	tb.WriteTo(&buf)
	got := buf.String()

	if strings.Count(got, "<a:p>") != 2 {
		t.Errorf("expected 2 paragraphs, got:\n%s", got)
	}
	if !strings.Contains(got, `b="1"`) {
		t.Error("missing bold")
	}
	if !strings.Contains(got, `i="1"`) {
		t.Error("missing italic")
	}
}

func TestTextBody_EscapesSpecialChars(t *testing.T) {
	t.Parallel()
	tb := TextBody{
		Paragraphs: []Paragraph{{
			Runs: []Run{{Text: "A & B < C > D"}},
		}},
	}
	var buf bytes.Buffer
	tb.WriteTo(&buf)
	got := buf.String()

	if !strings.Contains(got, `A &amp; B &lt; C &gt; D`) {
		t.Errorf("text not properly escaped:\n%s", got)
	}
}
