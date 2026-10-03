package semantic

import "github.com/sebahrens/json2pptx/internal/semantic/slides"

// FieldBudget is one authored field's fixed text budget — the limit the
// compiler holds the field to on every template. See slides.Budget.
type FieldBudget = slides.Budget

// KindFieldBudgets returns the fixed text budgets of a slide kind, in the
// order its fields are authored, or nil when it states none. The title,
// subtitle and takeaway are not here: they depend on the template and are
// measured (list_slide_kinds budgets).
func KindFieldBudgets(k SlideKind) []FieldBudget {
	return slides.Budgets(string(k))
}
