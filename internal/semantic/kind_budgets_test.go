package semantic

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// hasNumber reports whether text states the number n as a whole token ("≤60",
// "2–6", "60 chars"), not as part of a longer number.
func hasNumber(text string, n int) bool {
	return regexp.MustCompile(fmt.Sprintf(`(^|[^0-9])%d([^0-9]|$)`, n)).MatchString(text)
}

// budgetFields splits a budget's field path into the top-level payload fields
// it speaks about: "options[].label" → options, "label+context+source" → all
// three.
func budgetFields(path string) []string {
	var out []string
	for _, part := range strings.Split(path, "+") {
		if i := strings.IndexAny(part, "[."); i >= 0 {
			part = part[:i]
		}
		out = append(out, part)
	}
	return out
}

// TestKindBudgetsAgreeWithSchemaDescriptions holds the schema an agent reads to
// the budget the compiler enforces (go-slide-creator-iubjb): every fixed
// budget's number is stated in the description of the field it limits, so
// list_slide_kinds' item_schema, its budgets and validate_deck_spec's closed
// schema cannot quote different limits.
func TestKindBudgetsAgreeWithSchemaDescriptions(t *testing.T) {
	for _, kind := range AllSlideKinds() {
		fields := kindPayloadFields[kind]
		for _, b := range KindFieldBudgets(kind) {
			var desc strings.Builder
			for _, name := range budgetFields(b.Field) {
				f, ok := fields[name]
				if !ok {
					t.Errorf("%s: budget %q names %q, which is not a payload field", kind, b.Field, name)
					continue
				}
				desc.WriteString(f.desc + "\n")
			}
			for what, n := range map[string]int{"max_chars": b.MaxChars, "min_items": b.MinItems, "max_items": b.MaxItems} {
				if n > 0 && !hasNumber(desc.String(), n) {
					t.Errorf("%s: budget %s %s = %d is not stated in the field's schema description: %q", kind, b.Field, what, n, strings.TrimSpace(desc.String()))
				}
			}
		}
	}
}

// TestKindSummariesStateOnlyBudgetedLengths: a character count a kind summary
// states must be one of that kind's budgets — the catalogue's one-line summary
// may not quote a length the budgets do not carry. (The closing summary used
// to say "~40 characters" where a template's closing title held 20.)
func TestKindSummariesStateOnlyBudgetedLengths(t *testing.T) {
	stated := regexp.MustCompile(`(\d+)[ -]character|≤ ?(\d+) chars`)
	for _, kind := range AllSlideKinds() {
		info, _ := LookupKind(kind)
		budgets := KindFieldBudgets(kind)
		for _, m := range stated.FindAllStringSubmatch(info.Summary, -1) {
			number := m[1] + m[2]
			found := false
			for _, b := range budgets {
				// A budget with two visuals states the second length in its note.
				if fmt.Sprint(b.MaxChars) == number || regexp.MustCompile(`(^|[^0-9])`+number+`([^0-9]|$)`).MatchString(b.Note) {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: summary states %s characters, which is none of its budgets %+v", kind, number, budgets)
			}
		}
	}
}

// overBudgetBody returns the kind's example with the budgeted field one
// character over its limit, or nil when the path form is not one this test
// drives (a combined budget, enforced at render).
func overBudgetBody(kind SlideKind, b FieldBudget) map[string]any {
	body := KindExample(kind)
	delete(body, "kind")
	switch kind {
	case KindProcess:
		// The row budgets are those of steps that carry a description.
		steps, _ := body["steps"].([]any)
		for i, s := range steps {
			steps[i] = map[string]any{"label": fmt.Sprint(s), "description": "What happens in this step."}
		}
	case KindImageCase:
		// Callouts point at a picture.
		body["image"] = "shots/console.png"
		body["callouts"] = []any{map[string]any{"label": "Queue", "x": 0.5, "y": 0.5}}
	}
	long := strings.Repeat("x", b.MaxChars+1)
	list, key, isEntry := strings.Cut(b.Field, "[]")
	switch {
	case strings.Contains(b.Field, "+"):
		return nil
	case strings.HasPrefix(b.Field, "sections.<part>"):
		sections, _ := body["sections"].(map[string]any)
		for name := range sections {
			sections[name] = []any{long}
			return body
		}
		return nil
	case !isEntry:
		body[b.Field] = long
		return body
	}
	entries, _ := body[list].([]any)
	if len(entries) == 0 {
		entries = []any{map[string]any{}}
	}
	key = strings.TrimPrefix(key, ".")
	switch entry := entries[0].(type) {
	case map[string]any:
		if key == "" {
			return nil
		}
		entry[key] = long
	default:
		if key != "" {
			// A string entry is the label alone; an object carries the rest.
			entries[0] = map[string]any{"label": fmt.Sprint(entry), "title": fmt.Sprint(entry), key: long}
		} else {
			entries[0] = long
		}
	}
	body[list] = entries
	return body
}

// TestKindBudgetsAgreeWithFindings holds the findings to the same numbers:
// a field one character over its budget is reported by validation, and the
// finding quotes the budget the catalogue states.
func TestKindBudgetsAgreeWithFindings(t *testing.T) {
	for _, kind := range AllSlideKinds() {
		for _, b := range KindFieldBudgets(kind) {
			if b.MaxChars == 0 {
				continue
			}
			body := overBudgetBody(kind, b)
			if body == nil {
				continue
			}
			ds := Validate(&DeckSpec{Meta: DeckMeta{Title: "Budgets"}, Slides: []SlideSpec{{Kind: kind, Body: body}}}, StrictnessWarn)
			quoted := false
			for _, d := range ds {
				if hasNumber(d.Message, b.MaxChars) {
					quoted = true
				}
			}
			if !quoted {
				t.Errorf("%s: %s at %d characters (budget %d) is not reported with its budget: %v", kind, b.Field, b.MaxChars+1, b.MaxChars, ds)
			}
		}
	}
}
