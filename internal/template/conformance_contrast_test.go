package template

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCheckThemeWarnsOnNearBackgroundAccent1(t *testing.T) {
	tests := []struct {
		name     string
		accent   string
		wantWarn bool
	}{
		{"abstract-like", "#E9E6DF", true},
		{"visible", "#5B9BD5", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checks := checkTheme(types.ThemeInfo{Colors: []types.ThemeColor{
				{Name: "lt1", RGB: "#FFFFFF"},
				{Name: "accent1", RGB: tc.accent},
			}})
			var warned bool
			for _, check := range checks {
				if check.Category == "theme" && strings.Contains(check.Check, "accent1 visible") {
					warned = check.Status == ConformanceStatusWarn
				}
			}
			if warned != tc.wantWarn {
				t.Errorf("accent1 warning = %t, want %t", warned, tc.wantWarn)
			}
		})
	}
}

func TestAbstractTemplateAccentIsVisibleOnLightCanvas(t *testing.T) {
	report, err := CheckConformance("../../templates/abstract.pptx")
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range report.Checks {
		if check.Category == "theme" && strings.Contains(check.Check, "accent1 visible") {
			if check.Status != ConformanceStatusPass {
				t.Errorf("abstract accent1 check = %s, want PASS: %s", check.Status, check.Detail)
			}
			return
		}
	}
	t.Fatal("abstract template has no accent1 visibility check")
}
