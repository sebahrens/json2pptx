package shapegrid

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// go-slide-creator-b7qqg.15: comfortable growth filled the spare height of a
// narrow, tall row-label cell and grew a label the pattern had sized to fit
// into a mid-word break ("PRODUCTI / ON"). Growth may never push a word past
// the atomic-token share of its line.
func TestTypeScaleGrowthKeepsLabelWordsWhole(t *testing.T) {
	// A 125pt x 190pt cell: the 12pt label already uses more than the
	// atomic-token share of its ~97pt line, so growth (which the spare height
	// would otherwise push to the 14pt body cap, where PRODUCTION needs 94pt)
	// must leave it alone.
	text := `{"paragraphs":[{"content":"PRODUCTION","size":12,"bold":true}]}`
	if got := firstScaledSize(t, resolvedScaledShape(t, "comfortable", text, 125, 190)); got != 12 {
		t.Errorf("label grown to %.2fpt; want it held at 12pt", got)
	}
	// With room to spare the same label still grows, but only while it keeps
	// inside the atomic-token width.
	const widthPt, heightPt = 180.0, 190.0
	spec := resolvedScaledShape(t, "comfortable", text, widthPt, heightPt)
	got := firstScaledSize(t, spec)
	if got <= 12 {
		t.Errorf("roomy label not grown: %.2fpt", got)
	}
	tb, err := ResolveTextInput(spec.Text)
	if err != nil {
		t.Fatal(err)
	}
	insets := pptx.EffectiveTextInsets(tb, pptx.RectEmu{CX: PtToEMU(widthPt), CY: PtToEMU(heightPt)})
	allowed := float64(PtToEMU(widthPt)-insets[0]-insets[2]) * textfit.AtomicTokenWidthPct / 100
	w, err := textfit.MeasureStyledLineWidth("PRODUCTION", "Liberation Sans", got, true)
	if err != nil {
		t.Fatal(err)
	}
	if float64(w) > allowed+1 {
		t.Errorf("grown to %.2fpt: PRODUCTION is %d EMU wide, over the %.0f EMU atomic-token width", got, w, allowed)
	}
}

// A KPI value is one token even when it contains a space ("12 days").
func TestTokenGrowthCapTreatsKPIValueAsOneToken(t *testing.T) {
	width := PtToEMU(120)
	value := []scaleParagraph{{text: "12 days", fontPt: 20, role: "kpi-value", bold: true}}
	words := []scaleParagraph{{text: "12 days", fontPt: 20, role: "body", bold: true}}
	if kv, body := tokenGrowthCap(value, width), tokenGrowthCap(words, width); kv >= body {
		t.Errorf("kpi-value cap %.2f should be tighter than the per-word cap %.2f", kv, body)
	}
	// A token already wider than its line does not stop growth.
	wide := []scaleParagraph{{text: "Supercalifragilisticexpialidocious", fontPt: 30, role: "body"}}
	if c := tokenGrowthCap(wide, PtToEMU(60)); c < 1e9 {
		t.Errorf("overflowing token capped growth at %.2f", c)
	}
}
