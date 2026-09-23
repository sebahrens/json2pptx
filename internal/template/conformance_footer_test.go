package template

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCheckFooterChromeCompleteness(t *testing.T) {
	region := func(typ string) types.ChromeRegion {
		return types.ChromeRegion{Type: typ, X: 1, Y: 1, Width: 10, Height: 2}
	}
	for _, tc := range []struct {
		name    string
		layout  types.LayoutMetadata
		status  ConformanceStatus
		missing string
	}{
		{"complete", types.LayoutMetadata{Name: "Content", FooterRegions: []types.ChromeRegion{region("dt"), region("ftr"), region("sldNum")}}, ConformanceStatusPass, ""},
		{"absent", types.LayoutMetadata{Name: "Blank"}, ConformanceStatusPass, ""},
		{"partial", types.LayoutMetadata{Name: "Content", FooterRegions: []types.ChromeRegion{region("ftr"), region("sldNum")}}, ConformanceStatusWarn, "missing dt"},
		{"missing page number", types.LayoutMetadata{Name: "Content", FooterRegions: []types.ChromeRegion{region("dt"), region("ftr")}}, ConformanceStatusWarn, "missing sldNum"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checkFooterChromeCompleteness([]types.LayoutMetadata{tc.layout})
			if got.Status != tc.status || got.Category != "chrome" {
				t.Errorf("check = %+v, want chrome/%s", got, tc.status)
			}
			if tc.missing != "" && !strings.Contains(got.Detail, tc.missing) {
				t.Errorf("detail %q does not identify %q", got.Detail, tc.missing)
			}
		})
	}
}

func TestBlueCorporateTemplateCheckWarnsOnPartialFooter(t *testing.T) {
	report, err := CheckConformance("../../templates/blue-corporate.pptx")
	if err != nil {
		t.Fatalf("check blue-corporate: %v", err)
	}
	for _, check := range report.Checks {
		if check.Category == "chrome" && check.Check == "Footer placeholder set complete or absent" {
			if check.Status != ConformanceStatusWarn || !strings.Contains(check.Detail, "missing dt") {
				t.Errorf("blue-corporate footer check = %+v", check)
			}
			return
		}
	}
	t.Fatal("template-check did not report footer chrome completeness")
}
