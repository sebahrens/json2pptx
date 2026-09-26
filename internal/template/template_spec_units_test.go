package template

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/placeholderrole"
)

func TestTemplateSpecUnitConversions(t *testing.T) {
	body, err := os.ReadFile("../../docs/TEMPLATE_SPEC.md")
	if err != nil {
		t.Fatal(err)
	}
	number := func(text string) int64 {
		t.Helper()
		value, err := strconv.ParseInt(strings.ReplaceAll(text, ",", ""), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	fonts := regexp.MustCompile(`\*\*(\d+)–(\d+)pt\*\* \(([\d,]+)–([\d,]+) hundredths of a point\)`).FindAllSubmatch(body, -1)
	if len(fonts) != 2 {
		t.Fatalf("expected body and title font range conversions; got %d", len(fonts))
	}
	for _, match := range fonts {
		if number(string(match[1]))*100 != number(string(match[3])) || number(string(match[2]))*100 != number(string(match[4])) {
			t.Errorf("incorrect OOXML font conversion: %s", match[0])
		}
	}
	for _, tc := range []struct {
		pattern       string
		width, height int64
	}{
		{`\*\*16:9\*\* canvas: ([\d,]+) × ([\d,]+) EMU`, 16, 9},
		{`\*\*4:3\*\* \(([\d,]+) × ([\d,]+) EMU\)`, 4, 3},
	} {
		match := regexp.MustCompile(tc.pattern).FindSubmatch(body)
		if len(match) != 3 {
			t.Fatalf("missing documented canvas example for %d:%d", tc.width, tc.height)
		}
		if number(string(match[1]))*tc.height != number(string(match[2]))*tc.width {
			t.Errorf("canvas example does not have declared aspect ratio: %s", match[0])
		}
	}
	role := regexp.MustCompile(`heuristic uses ([\d,]+) \((\d+)pt\)`).FindSubmatch(body)
	if len(role) != 3 || number(string(role[1])) != placeholderrole.SectionNumberMinFontSize || number(string(role[2]))*100 != placeholderrole.SectionNumberMinFontSize {
		t.Fatal("documented section role heuristic differs from actual hundredths-point threshold")
	}
}
