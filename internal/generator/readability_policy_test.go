package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

func denseBodyShape(paragraphs int, sizeHPt string) *shapeXML {
	var paras []paragraphXML
	for i := 0; i < paragraphs; i++ {
		paras = append(paras, paragraphXML{Runs: []runXML{{
			Text:          "Procure-to-pay cycle time averages 23 days versus a top-quartile benchmark of 7 days, driven by manual approvals and three disconnected ERP instances",
			RunProperties: &runPropertiesXML{Lang: "en-US", FontSize: sizeHPt},
		}}})
	}
	return &shapeXML{
		ShapeProperties: shapePropertiesXML{Transform: &transformXML{Extent: extentXML{CX: 8229600, CY: 2057400}}},
		TextBody:        &textBodyXML{BodyProperties: &bodyPropertiesXML{}, Paragraphs: paras},
	}
}

func readabilityFindings(fs []patterns.FitFinding) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, f := range fs {
		if f.Code == patterns.ErrCodeTextBelowReadableMin {
			out = append(out, f)
		}
	}
	return out
}

// go-slide-creator-vbic: placeholder autofit that shrinks body text below the
// present-mode 12pt floor reports TEXT_BELOW_READABLE_MIN.
func TestPlaceholderAutofit_ReportsBelowReadableMin(t *testing.T) {
	var findings []patterns.FitFinding
	shape := denseBodyShape(6, "1800")
	applySmartAutofitWithOptions(shape,
		withFindingsCollector(&findings, "/slides/1/content/body"),
		withReadabilityPolicy(tokens.ViewingModePresentation, tokens.TextRoleBody))
	if !strings.Contains(shape.TextBody.BodyProperties.Inner, "fontScale") {
		t.Fatalf("expected the dense body to be shrunk: %s", shape.TextBody.BodyProperties.Inner)
	}
	got := readabilityFindings(findings)
	t.Logf("autofit: %s (%d paragraphs kept)", shape.TextBody.BodyProperties.Inner, len(shape.TextBody.Paragraphs))
	if len(got) != 1 {
		t.Fatalf("expected one TEXT_BELOW_READABLE_MIN, got %d (%v)", len(got), findings)
	}
	f := got[0]
	if f.Path != "/slides/1/content/body" || f.Fix == nil || f.Fix.Kind != "reduce_text" {
		t.Errorf("finding = %+v", f)
	}
	if pt := f.Fix.Params["actual_pt"].(float64); pt >= 12 {
		t.Errorf("actual_pt = %v, want < 12", pt)
	}
	if f.Fix.Params["strategy"] != "split" {
		t.Errorf("6 paragraphs should suggest split, got %v", f.Fix.Params["strategy"])
	}
	if f.Fix.Params["measurement_source"] != "generated" {
		t.Errorf("generated autofit source = %v", f.Fix.Params["measurement_source"])
	}

	// read mode floor is 10pt: the same shrink is acceptable.
	findings = nil
	applySmartAutofitWithOptions(denseBodyShape(6, "1800"),
		withFindingsCollector(&findings, "/p"),
		withReadabilityPolicy(tokens.ViewingModeReport, tokens.TextRoleBody))
	if n := len(readabilityFindings(findings)); n != 0 {
		t.Errorf("read mode should accept ~11pt body, got %d findings", n)
	}

	// No role configured: policy not checked (legacy callers).
	findings = nil
	applySmartAutofitWithOptions(denseBodyShape(6, "1800"), withFindingsCollector(&findings, "/p"))
	if n := len(readabilityFindings(findings)); n != 0 {
		t.Errorf("no role -> no readability findings, got %d", n)
	}
}

func TestParseViewingMode(t *testing.T) {
	for in, want := range map[string]tokens.ViewingMode{
		"": tokens.ViewingModePresentation, "present": tokens.ViewingModePresentation,
		"read": tokens.ViewingModeReport, "REPORT": tokens.ViewingModeReport,
	} {
		if got := tokens.ParseViewingMode(in); got != want {
			t.Errorf("ParseViewingMode(%q) = %s, want %s", in, got, want)
		}
	}
	if tokens.MinReadableHPt(tokens.ViewingModePresentation, tokens.TextRoleBody) != 1200 ||
		tokens.MinReadableHPt(tokens.ViewingModePresentation, tokens.TextRoleCaption) != 1000 {
		t.Error("present mode floors must be 12pt body / 10pt caption")
	}
}
