package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// go-slide-creator-wvr0. Storing the shrink makes a previously invisible fact
// visible: a SWOT quadrant holding twelve bullets is shrunk to 28% — about
// 3.4pt — and nobody will read it. The shrink is honest; the deck is not fine.

func slideWithAutofit(scale string, runSizes ...string) string {
	var sb strings.Builder
	sb.WriteString(`<p:sp><p:nvSpPr><p:cNvPr id="1" name="Body"/></p:nvSpPr><p:spPr/>`)
	sb.WriteString(`<p:txBody><a:bodyPr>`)
	if scale == "" {
		sb.WriteString(`<a:normAutofit/>`)
	} else {
		sb.WriteString(`<a:normAutofit fontScale="` + scale + `"/>`)
	}
	sb.WriteString(`</a:bodyPr>`)
	for _, sz := range runSizes {
		sb.WriteString(`<a:p><a:r><a:rPr sz="` + sz + `"/><a:t>Some bullet text</a:t></a:r></a:p>`)
	}
	sb.WriteString(`</p:txBody></p:sp>`)
	return sb.String()
}

func readabilityFindingsFor(t *testing.T, slideXML string) []patterns.FitFinding {
	t.Helper()
	ctx := &singlePassContext{}
	ctx.viewingMode = tokens.ParseViewingMode("present")
	ctx.reportUnreadableAutofit([]byte(slideXML), ctx.calculateStartingSlideNum())
	return ctx.fitFindings
}

func TestUnreadableAutofitIsReported(t *testing.T) {
	// 12pt shrunk to 28% is 3.4pt, far under the 12pt present-mode floor.
	got := readabilityFindingsFor(t, slideWithAutofit("28000", "1200", "1200", "1200", "1200"))
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(got), got)
	}
	f := got[0]
	if f.Code != patterns.ErrCodeTextBelowReadableMin {
		t.Errorf("code = %q", f.Code)
	}
	if !strings.Contains(f.Message, "3.4pt") {
		t.Errorf("message does not report the effective size: %q", f.Message)
	}
	if !strings.Contains(f.Message, "autofit 28%") {
		t.Errorf("message does not say the shrink caused it: %q", f.Message)
	}
	if f.Action != "review" {
		t.Errorf("action = %q, want review — the prediction is advisory", f.Action)
	}
	if f.Path != "/slides/0/rendered_shapes/1" {
		t.Errorf("path = %q, want exact rendered shape", f.Path)
	}
	if got := f.Fix.Params["rendered_shape_text"]; got != "Some bullet text Some bullet text Some bullet text Some bullet text" {
		t.Errorf("rendered shape text = %v", got)
	}
}

func TestReadableAutofitIsNotReported(t *testing.T) {
	// 24pt shrunk to 90% is 21.6pt, well above the floor.
	if got := readabilityFindingsFor(t, slideWithAutofit("90000", "2400")); len(got) != 0 {
		t.Errorf("a legible shrink was reported: %+v", got)
	}
	// A bare normAutofit carries no prediction to judge.
	if got := readabilityFindingsFor(t, slideWithAutofit("", "1200")); len(got) != 0 {
		t.Errorf("a shape with no stored scale was reported: %+v", got)
	}
	// No runs at all: nothing to measure.
	if got := readabilityFindingsFor(t, slideWithAutofit("28000")); len(got) != 0 {
		t.Errorf("a shape with no text was reported: %+v", got)
	}
}

func TestSmallestRunSizeHPt(t *testing.T) {
	shape := slideWithAutofit("50000", "2400", "900", "1800")
	if got := smallestRunSizeHPt(shape); got != 900 {
		t.Errorf("smallest = %d, want 900", got)
	}
	if got := smallestRunSizeHPt(slideWithAutofit("50000")); got != 0 {
		t.Errorf("no runs should report 0, got %d", got)
	}
}
