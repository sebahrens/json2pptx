package patterns

import (
	"strings"
	"testing"
)

func TestNumberedStepSixChevronBudgetAndExistingOverflowSignal(t *testing.T) {
	pat := &numberedStepStrip{}
	values := &NumberedStepStripValues{Style: "chevron"}
	for i := 0; i < 6; i++ {
		values.Steps = append(values.Steps, NumberedStepStripStep{Label: "Step", Body: "Detail"})
	}
	values.Steps[2].Label = strings.Repeat("word ", 9) + "ab" // 47 characters
	for _, warning := range pat.PostExpandWarnings(testThemeCtx(), values, nil) {
		if strings.Contains(warning, "steps[2].label") && strings.HasPrefix(warning, ErrCodeBodyTooLong) {
			t.Fatalf("at budget warned: %q", warning)
		}
	}
	values.Steps[2].Label += "c"
	warnings := pat.PostExpandWarnings(testThemeCtx(), values, nil)
	found := false
	for _, warning := range warnings {
		if strings.HasPrefix(warning, ErrCodeBodyTooLong+":") && strings.Contains(warning, "steps[2].label") && strings.Contains(warning, "about 47") {
			found = true
		}
	}
	if !found {
		t.Fatalf("six-step budget warning missing: %v", warnings)
	}
	values.Style = "stacked-box"
	if got := pat.PostExpandWarnings(testThemeCtx(), values, nil); len(got) != 0 {
		t.Fatalf("stacked-box at schema limit warned: %v", got)
	}
	step := pat.Schema().raw.Properties["values"].raw.Properties["steps"].raw.Items
	label := step.raw.Properties["label"]
	if label.raw.MaxLength == nil || *label.raw.MaxLength != 60 || !strings.Contains(label.raw.Description, "about 47") {
		t.Errorf("schema loses sparse maximum or chevron guidance: %+v", label.raw)
	}
}
