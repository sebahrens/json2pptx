package tokens

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestTokensPublishedInRulesMD asserts that the Typography Hierarchy
// table in skills/generate-deck/RULES.md publishes the type-scale steps in
// typography.go, one row per role, and that no row other than the
// engine-rendered source line tells agents to author text below the 10pt
// caption step (go-slide-creator-vmdfm).
//
// The constants are the canonical source. RULES.md is the agent-facing
// surface that documents them. If the constants are changed, this test
// fails until RULES.md is updated to match. If RULES.md is changed
// independently of the constants, this test fails until they agree.
func TestTokensPublishedInRulesMD(t *testing.T) {
	path := findRepoFile(t, "skills/generate-deck/RULES.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read RULES.md: %v", err)
	}
	rules := string(data)
	start := strings.Index(rules, "## Typography Hierarchy")
	if start < 0 {
		t.Fatal("RULES.md has no Typography Hierarchy section")
	}
	section := rules[start:]
	if end := strings.Index(section[1:], "\n## "); end >= 0 {
		section = section[:end+1]
	}

	cases := []struct {
		role string
		size string
	}{
		{"Display / title", fmt.Sprintf("%dpt", TypeScaleDisplayHPt/100)},
		{"Lead / banner", fmt.Sprintf("%dpt", TypeScaleLeadHPt/100)},
		{"Subhead / card title", fmt.Sprintf("%dpt", TypeScaleSubheadHPt/100)},
		{"Body / card body", fmt.Sprintf("%dpt", TypeScaleBodyHPt/100)},
		{"Caption", fmt.Sprintf("%dpt", TypeScaleCaptionHPt/100)},
		{"KPI value", fmt.Sprintf("%d-%dpt", TypeScaleKPIMinHPt/100, TypeScaleKPIMaxHPt/100)},
		{"Source line", fmt.Sprintf("%dpt", SourceLineHPt/100)},
	}
	rows := map[string]string{}
	for _, line := range strings.Split(section, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			continue
		}
		rows[strings.TrimSpace(cells[1])] = strings.TrimSpace(cells[2])
	}
	for _, c := range cases {
		got, ok := rows[c.role]
		if !ok {
			t.Errorf("RULES.md typography table missing row for %q", c.role)
			continue
		}
		if got != c.size {
			t.Errorf("RULES.md row %q publishes %q, tokens say %q", c.role, got, c.size)
		}
	}
	sizeRe := regexp.MustCompile(`(\d+)(?:-\d+)?pt`)
	for role, size := range rows {
		m := sizeRe.FindStringSubmatch(size)
		if m == nil || role == "Source line" {
			continue
		}
		if pt, _ := strconv.Atoi(m[1]); pt*100 < TypeScaleCaptionHPt {
			t.Errorf("RULES.md row %q tells agents to author %s, below the %dpt caption step", role, size, TypeScaleCaptionHPt/100)
		}
	}
}

// Every published step is a type-scale step: authoring at a table size never
// gets snapped to a different size.
func TestPublishedStepsAreScaleSteps(t *testing.T) {
	for _, hpt := range []int{TypeScaleCaptionHPt, TypeScaleBodyHPt, TypeScaleSubheadHPt, TypeScaleLeadHPt, TypeScaleDisplayHPt} {
		if SnapTextHPt(hpt) != hpt {
			t.Errorf("%d is not a type-scale step", hpt)
		}
	}
	if TypeScaleBodyHPt < MinReadableHPt(ViewingModePresentation, TextRoleBody) {
		t.Errorf("body step %d is below the presentation body floor", TypeScaleBodyHPt)
	}
	if TypeScaleCaptionHPt < MinReadableHPt(ViewingModePresentation, TextRoleCaption) {
		t.Errorf("caption step %d is below the presentation caption floor", TypeScaleCaptionHPt)
	}
}

// findRepoFile walks upward from the test working directory until it
// finds the requested repo-relative path. Falls t.Fatal if not found.
func findRepoFile(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate %q above %s", rel, dir)
		}
		dir = parent
	}
}

func TestSnapTextHPtSettlesOnScale(t *testing.T) {
	for _, tc := range []struct{ in, want int }{
		{700, 700}, {950, 950}, {1000, 1000}, {1050, 1000}, {1100, 1100}, {1150, 1100},
		{1200, 1200}, {1300, 1200}, {1500, 1400}, {1600, 1400}, {1700, 1400},
		{1800, 1800}, {2000, 1800}, {2600, 1800}, {2800, 2800}, {3600, 3600}, {4800, 4800},
	} {
		if got := SnapTextHPt(tc.in); got != tc.want {
			t.Errorf("SnapTextHPt(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
	// Readability floors must be scale steps, or snapping down could cross one.
	for _, mode := range []ViewingMode{ViewingModePresentation, ViewingModeReport} {
		for _, role := range []TextRole{TextRoleBody, TextRoleCardTitle, TextRoleCardBody, TextRoleCaption} {
			if floor := MinReadableHPt(mode, role); floor >= TypeScaleCaptionHPt && SnapTextHPt(floor) != floor {
				t.Errorf("%s/%s floor %d is not a scale step", mode, role, floor)
			}
		}
	}
}
