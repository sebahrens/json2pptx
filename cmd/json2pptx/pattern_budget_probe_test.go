package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func budgetProbeCopy(length int) string {
	return strings.Repeat("word ", length/5) + strings.Repeat("w", length%5)
}

// matrixFullStackProbeValues deliberately fills every matrix field at the
// current schema limit. A cheap regular test guards this opt-in probe fixture
// against becoming invalid when any budget changes.
func matrixFullStackProbeValues(body string) *patterns.Matrix2x2Values {
	quadrant := patterns.Matrix2x2Quadrant{Header: strings.Repeat("W", 80), Body: body}
	return &patterns.Matrix2x2Values{
		XAxisLabel: strings.Repeat("W", 16), YAxisLabel: strings.Repeat("W", 60),
		XLow: strings.Repeat("W", 11), XHigh: strings.Repeat("W", 11),
		YLow: strings.Repeat("W", 11), YHigh: strings.Repeat("W", 11),
		TopLeft: quadrant, TopRight: quadrant, BottomLeft: quadrant, BottomRight: quadrant,
	}
}

func TestMatrixFullStackBudgetProbeFixtureValid(t *testing.T) {
	pat, ok := patterns.Default().Get("matrix-2x2")
	if !ok {
		t.Fatal("matrix-2x2 pattern missing")
	}
	fixture := matrixFullStackProbeValues(strings.Repeat("W", 200))
	if err := pat.Validate(fixture, nil, nil); err != nil {
		t.Fatalf("full-stack matrix probe must stay valid at schema maxima: %v", err)
	}
	maximum, note := schemaMaximumValues(pat)
	if note != "" {
		t.Fatal(note)
	}
	want := maximum.(*patterns.Matrix2x2Values)
	for _, field := range []struct {
		name     string
		got, max string
	}{
		{"x_axis_label", fixture.XAxisLabel, want.XAxisLabel},
		{"y_axis_label", fixture.YAxisLabel, want.YAxisLabel},
		{"x_low", fixture.XLow, want.XLow},
		{"x_high", fixture.XHigh, want.XHigh},
		{"y_low", fixture.YLow, want.YLow},
		{"y_high", fixture.YHigh, want.YHigh},
		{"quadrant.header", fixture.TopLeft.Header, want.TopLeft.Header},
		{"quadrant.body", fixture.TopLeft.Body, want.TopLeft.Body},
	} {
		if len([]rune(field.got)) != len([]rune(field.max)) {
			t.Errorf("full-stack %s length = %d, schema maximum = %d", field.name, len([]rune(field.got)), len([]rune(field.max)))
		}
	}
}

func bmcProbeValues() *patterns.BMCCanvasValues {
	cell := func(header string) patterns.BMCCell {
		return patterns.BMCCell{Header: header, Bullets: []string{"base"}}
	}
	return &patterns.BMCCanvasValues{
		KeyPartners: cell("Key Partners"), KeyActivities: cell("Key Activities"),
		KeyResources: cell("Key Resources"), ValuePropositions: cell("Value Propositions"),
		CustomerRelations: cell("Customer Relations"), Channels: cell("Channels"),
		CustomerSegments: cell("Customer Segments"), CostStructure: cell("Cost Structure"),
		RevenueStreams: cell("Revenue Streams"),
	}
}
