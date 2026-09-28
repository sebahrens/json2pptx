package patterns

import (
	"strings"
	"testing"
)

func tableHighlightBudgetValues(options, criteria, nameChars, detailChars int) *TableHighlightValues {
	v := &TableHighlightValues{Scale: "harvey"}
	for i := 0; i < criteria; i++ {
		v.Criteria = append(v.Criteria, TableHighlightCriterion{Label: "Quality"})
	}
	for i := 0; i < options; i++ {
		option := TableHighlightOption{Name: strings.Repeat("N", nameChars), Detail: strings.Repeat("D", detailChars)}
		for j := 0; j < criteria; j++ {
			option.Scores = append(option.Scores, "3")
		}
		v.Options = append(v.Options, option)
	}
	return v
}

func TestTableHighlightDensePairedCopyWarning(t *testing.T) {
	pat := &tableHighlight{}
	ctx := testThemeCtx()
	dense := tableHighlightBudgetValues(3, 6, 40, 60)
	warnings := pat.PostExpandWarnings(ctx, dense, nil)
	if len(warnings) == 0 || !strings.Contains(warnings[0], ErrCodeBodyTooLong) ||
		!strings.Contains(warnings[0], "options[0].name/detail") ||
		!strings.Contains(warnings[0], "about 30 characters each") {
		t.Fatalf("dense matrix warning: %v", warnings)
	}
	readable := tableHighlightBudgetValues(3, 6, 30, 30)
	if got := pat.PostExpandWarnings(ctx, readable, nil); len(got) != 0 {
		t.Fatalf("measured dense target should fit: %v", got)
	}
	sparse := tableHighlightBudgetValues(2, 2, 40, 60)
	if got := pat.PostExpandWarnings(ctx, sparse, nil); len(got) != 0 {
		t.Fatalf("sparse schema maxima should fit: %v", got)
	}
	mixed := tableHighlightBudgetValues(3, 6, 40, 20)
	if got := pat.PostExpandWarnings(ctx, mixed, nil); len(got) != 0 {
		t.Fatalf("short details leave room for long names: %v", got)
	}
	// From four option rows a row holds its name and no readable detail.
	nameOnly := tableHighlightBudgetValues(5, 6, tableHighlightNameOnlyBudget, 0)
	if got := pat.PostExpandWarnings(ctx, nameOnly, nil); len(got) != 0 {
		t.Fatalf("name-only rows at budget should fit: %v", got)
	}
	nameOnly.Options[1].Detail = "Detail"
	nameOnly.Options[2].Name += "N"
	got := pat.PostExpandWarnings(ctx, nameOnly, nil)
	if len(got) != 2 || !strings.Contains(got[0], "options[1].detail") || !strings.Contains(got[0], "no readable detail") ||
		!strings.Contains(got[1], "options[2].name") || !strings.Contains(got[1], "about 30 name characters") {
		t.Fatalf("name-only matrix warnings: %v", got)
	}
	six := tableHighlightBudgetValues(6, 2, 5, 0)
	if got := pat.PostExpandWarnings(ctx, six, nil); len(got) != 0 {
		t.Fatalf("six name-only option rows should fit: %v", got)
	}
	if got := pat.PostExpandWarnings(ctx, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
	option := pat.Schema().raw.Properties["values"].raw.Properties["options"].raw.Items
	if option.raw.Properties["detail"].raw.MaxLength == nil ||
		*option.raw.Properties["detail"].raw.MaxLength != 80 ||
		!strings.Contains(option.raw.Properties["detail"].raw.Description, "from 4 options a row holds no readable detail") {
		t.Fatalf("schema loses sparse maximum or dense guidance: %+v", option.raw)
	}
}

func TestTableHighlightLongDetailUsesSparseWidth(t *testing.T) {
	pat := &tableHighlight{}
	// Two options keep the sparse 40-per-field paired budget, so a 62-character
	// detail beside a short name is readable copy that widens the option column.
	sparse := tableHighlightBudgetValues(2, 3, 12, 62)
	if err := pat.Validate(sparse, nil, nil); err != nil {
		t.Fatalf("62-character detail should keep a 2×3 table: %v", err)
	}
	layout := newTHLayout(testThemeCtx(), sparse, &TableHighlightOverrides{})
	if got := layout.cols[0]; got != 36 {
		t.Errorf("long-detail option column = %.0f%%, want 36%%", got)
	}
	layout.fit()
	if layout.total() > layout.areaH {
		t.Errorf("long-detail matrix needs %.1fpt but only %.1fpt is available", layout.total(), layout.areaH)
	}
	short := tableHighlightBudgetValues(3, 3, 12, 60)
	if got := newTHLayout(testThemeCtx(), short, &TableHighlightOverrides{}).cols[0]; got != 30 {
		t.Errorf("ordinary option column = %.0f%%, want 30%%", got)
	}
	sparse.Options[0].Detail = strings.Repeat("D", 81)
	if err := pat.Validate(sparse, nil, nil); err == nil || !strings.Contains(err.Error(), "options[0].detail exceeds maxLength 80") {
		t.Errorf("81-character sparse detail should report its 80-character limit: %v", err)
	}
	dense := tableHighlightBudgetValues(5, 5, 12, 62)
	if err := pat.Validate(dense, nil, nil); err == nil || !strings.Contains(err.Error(), "options[0].detail exceeds maxLength 60") {
		t.Errorf("dense matrix detail should retain its 60-character limit: %v", err)
	}
}
