package svggen

import (
	"strings"
	"testing"
)

func TestDrawnTextOverlapFindings(t *testing.T) {
	b := NewSVGBuilder(400, 200)
	b.SetFontSize(14)
	b.DrawText("Revenue growth", 30, 40, TextAlignLeft, TextBaselineTop)
	b.DrawText("Margin growth", 32, 42, TextAlignLeft, TextBaselineTop)
	b.DrawText("Source", 30, 170, TextAlignLeft, TextBaselineTop)
	findings := b.textOverlapFindings()
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one overlapping pair", findings)
	}
	if findings[0].Code != FindingDiagramTextOverlap ||
		!strings.Contains(findings[0].Message, "Revenue growth") ||
		!strings.Contains(findings[0].Message, "Margin growth") {
		t.Errorf("finding = %+v, want both colliding strings", findings[0])
	}
}

func TestDrawnTextOverlapQuarterTurnAndCleanLayout(t *testing.T) {
	b := NewSVGBuilder(400, 200)
	b.SetFontSize(12)
	b.DrawText("Legend", 95, 30, TextAlignLeft, TextBaselineTop)
	b.Push()
	b.RotateAround(-90, 105, 40)
	b.DrawText("Axis title", 105, 40, TextAlignCenter, TextBaselineMiddle)
	b.Pop()
	if got := b.textOverlapFindings(); len(got) != 1 {
		t.Fatalf("rotated axis title must intersect legend: %+v boxes=%+v", got, b.textBoxes)
	}
	b = NewSVGBuilder(400, 200)
	b.DrawText("Title", 30, 30, TextAlignLeft, TextBaselineTop)
	b.DrawText("Source", 30, 170, TextAlignLeft, TextBaselineTop)
	if got := b.textOverlapFindings(); len(got) != 0 {
		t.Fatalf("separated labels must be quiet: %+v", got)
	}
}

func TestDryRenderCarriesDrawnTextOverlap(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockDiagramWithBuilder{typeID: "overlap_test", overlapText: true})
	findings, err := RegistryDryRender(registry, &RequestEnvelope{Type: "overlap_test", Data: map[string]any{"x": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Code != FindingDiagramTextOverlap {
		t.Fatalf("DryRender findings = %+v, want drawn-text overlap", findings)
	}
}
