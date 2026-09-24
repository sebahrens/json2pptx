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

func TestBlueCorporateTemplateHasCompleteNonOverlappingFooter(t *testing.T) {
	report, err := CheckConformance("../../templates/blue-corporate.pptx")
	if err != nil {
		t.Fatalf("check blue-corporate: %v", err)
	}
	if report.WarnCount() != 0 {
		t.Errorf("blue-corporate has %d conformance warnings, want zero", report.WarnCount())
	}
	foundChromeCheck := false
	for _, check := range report.Checks {
		if check.Category == "chrome" && check.Check == "Footer placeholder set complete or absent" {
			foundChromeCheck = true
			if check.Status != ConformanceStatusPass {
				t.Errorf("blue-corporate footer check = %+v", check)
			}
			break
		}
	}
	if !foundChromeCheck {
		t.Fatal("template-check did not report footer chrome completeness")
	}
	reader, err := OpenTemplate("../../templates/blue-corporate.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	master, err := reader.ReadFile("ppt/slideMasters/slideMaster1.xml")
	if err != nil {
		t.Fatal(err)
	}
	positions, err := ParseSlideMasterPositions(master)
	if err != nil {
		t.Fatal(err)
	}
	date, footer, number := positions["type:dt"], positions["type:ftr"], positions["type:sldNum"]
	if date == nil || footer == nil || number == nil {
		t.Fatalf("master chrome incomplete: date=%+v footer=%+v number=%+v", date, footer, number)
	}
	if date.OffsetX+date.ExtentCX > footer.OffsetX || footer.OffsetX+footer.ExtentCX > number.OffsetX {
		t.Errorf("master chrome overlaps: date=%+v footer=%+v number=%+v", date, footer, number)
	}
	if date.OffsetX != 432000 {
		t.Errorf("date anchor moved from existing left-footer alignment: x=%d", date.OffsetX)
	}
	if positions["type:dt:idx:20"] != date || positions["idx:2"] == date {
		t.Error("date placeholder must use reserved idx 20, not a content-layout idx")
	}
}
