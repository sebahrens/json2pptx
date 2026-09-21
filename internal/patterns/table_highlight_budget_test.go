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
	dense := tableHighlightBudgetValues(6, 6, 40, 60)
	warnings := pat.PostExpandWarnings(ctx, dense, nil)
	if len(warnings) == 0 || !strings.Contains(warnings[0], ErrCodeBodyTooLong) ||
		!strings.Contains(warnings[0], "options[0].name/detail") ||
		!strings.Contains(warnings[0], "about 30 characters each") {
		t.Fatalf("dense matrix warning: %v", warnings)
	}
	readable := tableHighlightBudgetValues(6, 6, 30, 30)
	if got := pat.PostExpandWarnings(ctx, readable, nil); len(got) != 0 {
		t.Fatalf("measured dense target should fit: %v", got)
	}
	sparse := tableHighlightBudgetValues(2, 2, 40, 60)
	if got := pat.PostExpandWarnings(ctx, sparse, nil); len(got) != 0 {
		t.Fatalf("sparse schema maxima should fit: %v", got)
	}
	mixed := tableHighlightBudgetValues(6, 6, 40, 20)
	if got := pat.PostExpandWarnings(ctx, mixed, nil); len(got) != 0 {
		t.Fatalf("short details leave room for long names: %v", got)
	}
	tagged := tableHighlightBudgetValues(5, 6, 30, 30)
	zero := 0
	tagged.HighlightRow = &zero
	tagged.HighlightLabel = strings.Repeat("T", 24)
	tagged.Options[0].Name = strings.Repeat("N", 40)
	tagged.Options[0].Detail = strings.Repeat("D", 60)
	if got := pat.PostExpandWarnings(ctx, tagged, nil); len(got) != 0 {
		t.Fatalf("measured highlighted row should fit: %v", got)
	}
	if got := pat.PostExpandWarnings(ctx, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
	option := pat.Schema().raw.Properties["values"].raw.Properties["options"].raw.Items
	if option.raw.Properties["detail"].raw.MaxLength == nil ||
		*option.raw.Properties["detail"].raw.MaxLength != 60 ||
		!strings.Contains(option.raw.Description, "35/33/30") {
		t.Fatalf("schema loses sparse maximum or dense guidance: %+v", option.raw)
	}
}
