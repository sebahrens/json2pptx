package svggen

import (
	"strings"
	"testing"
)

// longCategoryLabels mirrors the consulting-deck repro for
// go-slide-creator-xrmm, where labels rendered as "IT hardware & s…" rotated
// 45° instead of wrapping.
var longCategoryLabels = []string{
	"IT hardware & software",
	"Professional services",
	"Facilities & MRO",
	"Logistics and freight",
	"Marketing services",
	"Lab consumables",
}

func TestAdaptXLabels_PrefersTwoLineWrapOverRotation(t *testing.T) {
	b := NewSVGBuilder(600, 360)
	layout := AdaptXLabels(b, longCategoryLabels, 520, b.StyleGuide().Typography.SizeSmall, false)

	if layout.Rotation != 0 {
		t.Fatalf("rotation = %v, want 0 (two-line wrap should be preferred)", layout.Rotation)
	}
	if layout.MaxLines != 2 {
		t.Fatalf("MaxLines = %d, want 2", layout.MaxLines)
	}
	if len(layout.DisplayLabels) != len(longCategoryLabels) {
		t.Fatalf("DisplayLabels len = %d, want %d", len(layout.DisplayLabels), len(longCategoryLabels))
	}
	for i, cat := range layout.Categories {
		if cat != longCategoryLabels[i] {
			t.Errorf("Categories[%d] = %q, want unmodified %q (scale lookups key on it)", i, cat, longCategoryLabels[i])
		}
	}
	for i, lbl := range layout.DisplayLabels {
		if strings.Contains(lbl, "…") {
			t.Errorf("DisplayLabels[%d] = %q contains an ellipsis", i, lbl)
		}
		if got := strings.ReplaceAll(lbl, "\n", " "); got != longCategoryLabels[i] {
			t.Errorf("DisplayLabels[%d] = %q does not reconstruct %q", i, lbl, longCategoryLabels[i])
		}
		if n := strings.Count(lbl, "\n") + 1; n > 2 {
			t.Errorf("DisplayLabels[%d] = %q has %d lines, want <= 2", i, lbl, n)
		}
	}
	if layout.ExtraBottomMargin <= 0 {
		t.Errorf("ExtraBottomMargin = %v, want > 0 to reserve the second label line", layout.ExtraBottomMargin)
	}
}

func TestAdaptXLabels_RotatesWhenWrapNeedsThreeLines(t *testing.T) {
	cats := []string{
		"Supercalifragilisticexpialidocious", "Pneumonoultramicroscopic", "Hippopotomonstrosesquipedalian",
		"Floccinaucinihilipilification", "Antidisestablishmentarianism", "Incomprehensibilities",
	}
	b := NewSVGBuilder(400, 300)
	layout := AdaptXLabels(b, cats, 300, b.StyleGuide().Typography.SizeSmall, true)
	if layout.Rotation == 0 {
		t.Fatalf("single-word labels that cannot wrap must fall back to rotation, got rotation 0")
	}
	if layout.DisplayLabels != nil {
		t.Errorf("DisplayLabels = %v, want nil when rotating", layout.DisplayLabels)
	}
}

func TestBarChart_LongCategoriesWrapWithoutEllipsisOrRotation(t *testing.T) {
	values := []any{11.2, 9.8, 7.1, 6.4, 5.3, 1.3}
	cats := make([]any, len(longCategoryLabels))
	for i, c := range longCategoryLabels {
		cats[i] = c
	}
	req := &RequestEnvelope{
		Type:   "bar_chart",
		Title:  "Annual savings potential ($M)",
		Output: OutputSpec{Width: 600, Height: 360},
		Data: map[string]any{
			"categories": cats,
			"series":     []any{map[string]any{"name": "Savings", "values": values}},
		},
	}
	doc, err := Render(req)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	svg := doc.String()
	if strings.Contains(svg, "…") {
		t.Errorf("rendered SVG contains an ellipsized label")
	}
	if strings.Contains(svg, "rotate(-45)") || strings.Contains(svg, "rotate(-60)") {
		t.Errorf("rendered SVG contains rotated x-axis labels; want horizontal two-line wrap")
	}
	for _, want := range []string{">IT hardware<", ">&amp; software<", ">Professional<", ">services<"} {
		if !strings.Contains(svg, want) {
			t.Errorf("rendered SVG missing wrapped label line %q", want)
		}
	}
}

func TestAdaptXLabels_TruncationEmitsEllipsizedFinding(t *testing.T) {
	cats := make([]string, 12)
	for i := range cats {
		cats[i] = "Extraordinarilylongcategoryname" + string(rune('A'+i))
	}
	b := NewSVGBuilder(300, 240)
	layout := AdaptXLabels(b, cats, 200, b.StyleGuide().Typography.SizeSmall, true)
	truncated := false
	for _, c := range layout.Categories {
		if strings.HasSuffix(c, "…") {
			truncated = true
		}
	}
	if !truncated {
		t.Fatalf("expected last-resort truncation for 12 unwrappable labels in 200pt")
	}
	found := false
	for _, f := range b.Findings() {
		if f.Code == FindingLabelEllipsized && f.Field == "x_axis.labels" {
			found = true
		}
	}
	if !found {
		t.Errorf("truncation must emit %s finding; got %+v", FindingLabelEllipsized, b.Findings())
	}
}
