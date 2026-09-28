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
	values.Steps[2].Label = strings.Repeat("word ", 3) + "w" // 16 characters
	for _, warning := range pat.PostExpandWarnings(testThemeCtx(), values, nil) {
		if strings.Contains(warning, "steps[2].label") && strings.HasPrefix(warning, ErrCodeBodyTooLong) {
			t.Fatalf("at budget warned: %q", warning)
		}
	}
	values.Steps[2].Label += "c"
	warnings := pat.PostExpandWarnings(testThemeCtx(), values, nil)
	found := false
	for _, warning := range warnings {
		if strings.HasPrefix(warning, ErrCodeBodyTooLong+":") && strings.Contains(warning, "steps[2].label") && strings.Contains(warning, "about 16") {
			found = true
		}
	}
	if !found {
		t.Fatalf("six-step budget warning missing: %v", warnings)
	}
	// Six stacked-box rows hold a label and no body: drop the bodies so only
	// the chevron-specific label budget is exercised.
	values.Style = "stacked-box"
	for i := range values.Steps {
		values.Steps[i].Body = ""
	}
	if got := pat.PostExpandWarnings(testThemeCtx(), values, nil); len(got) != 0 {
		t.Fatalf("stacked-box at schema limit warned: %v", got)
	}
	step := pat.Schema().raw.Properties["values"].raw.Properties["steps"].raw.Items
	label := step.raw.Properties["label"]
	if label.raw.MaxLength == nil || *label.raw.MaxLength != 60 || !strings.Contains(label.raw.Description, "six-step chevrons about 16") {
		t.Errorf("schema loses sparse maximum or chevron guidance: %+v", label.raw)
	}
}

func TestNumberedStepFiveChevronLabelBudget(t *testing.T) {
	pat := &numberedStepStrip{}
	values := &NumberedStepStripValues{Style: "chevron"}
	for i := 0; i < 5; i++ {
		values.Steps = append(values.Steps, NumberedStepStripStep{Label: "Step"})
	}
	values.Steps[1].Label = strings.Repeat("word ", 8) + "w" // 41 characters
	for _, warning := range pat.PostExpandWarnings(testThemeCtx(), values, nil) {
		if strings.Contains(warning, "steps[1].label") && strings.HasPrefix(warning, ErrCodeBodyTooLong) {
			t.Fatalf("at budget warned: %q", warning)
		}
	}
	values.Steps[1].Label += "c"
	found := false
	for _, warning := range pat.PostExpandWarnings(testThemeCtx(), values, nil) {
		if strings.HasPrefix(warning, ErrCodeBodyTooLong+":") && strings.Contains(warning, "steps[1].label") && strings.Contains(warning, "about 41") {
			found = true
		}
	}
	if !found {
		t.Fatal("five-step budget warning missing")
	}
}
