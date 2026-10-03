package slides

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Every over-budget item of a list, in one pass (go-slide-creator-ipahe).
//
// DecisionOverBudget and ProcessOverBudget name the first offender only, so a
// decision with three over-long labels took three validate round-trips, each
// pointing at the list rather than the label. The functions here return every
// offender with the field it sits in, what was measured and what the visual
// holds, so validation reports each at its own path.

// BudgetItem is one authored field over a text budget.
type BudgetItem struct {
	// Field is the path of the field inside the slide ("options[1].label").
	Field string
	// What names the field for a message ("option 2's label").
	What string
	// Measured and Allowed are character counts. Allowed is the length to
	// write to, which is the readable budget where a visual has one below its
	// hard limit.
	Measured, Allowed int
	// Holds says what the budget belongs to ("a step holds 60").
	Holds string
}

// Message is the item's finding sentence, without the "otherwise it degrades"
// tail the caller adds.
func (b BudgetItem) Message() string {
	return fmt.Sprintf("%s is %d characters; %s", b.What, b.Measured, b.Holds)
}

// authoredKey returns the first of keys that carries text in m.
func authoredKey(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if strField(m, k) != "" {
			return k
		}
	}
	return ""
}

// DecisionBudgetItems lists every option label or detail too long for the
// numbered strip. It is empty when the options take a visual, or lose it for
// a reason other than length (a count, a missing detail).
func DecisionBudgetItems(body map[string]any) []BudgetItem {
	if DecisionOverBudget(body) == "" {
		return nil
	}
	field := ""
	for _, k := range []string{"options", "choices", "alternatives"} {
		if _, ok := body[k].([]any); ok {
			field = k
			break
		}
	}
	raw, _ := body[field].([]any)
	var out []BudgetItem
	n := 0
	for i, e := range raw {
		var label, detail, labelField, detailField string
		switch t := e.(type) {
		case string:
			o, valid := decisionOptionFromString(t)
			if !valid {
				continue
			}
			label, detail = o.Label, o.Detail
			labelField = fmt.Sprintf("%s[%d]", field, i)
			detailField = labelField
		case map[string]any:
			lk := authoredKey(t, "label", "title", "name", "option")
			if lk == "" {
				continue
			}
			label = strField(t, lk)
			labelField = fmt.Sprintf("%s[%d].%s", field, i, lk)
			if dk := authoredKey(t, "detail", "description", "body", "summary"); dk != "" {
				detail = strField(t, dk)
				detailField = fmt.Sprintf("%s[%d].%s", field, i, dk)
			}
		default:
			continue
		}
		n++
		if runeLen(label) > decisionLabelMax {
			out = append(out, BudgetItem{Field: labelField, What: fmt.Sprintf("option %d's label", n),
				Measured: runeLen(label), Allowed: decisionLabelMax, Holds: fmt.Sprintf("a step holds %d", decisionLabelMax)})
		}
		if runeLen(detail) > decisionDetailMax {
			out = append(out, BudgetItem{Field: detailField, What: fmt.Sprintf("option %d's detail", n),
				Measured: runeLen(detail), Allowed: decisionDetailMax, Holds: fmt.Sprintf("a step holds %d", decisionDetailMax)})
		}
	}
	return out
}

// ProcessBudgetItems lists every step whose label and description together
// overflow a flow box. The description is the field to shorten when there is
// one: its budget is what the box leaves beside the label.
func ProcessBudgetItems(body map[string]any) []BudgetItem {
	if ProcessOverBudget(body) == "" {
		return nil
	}
	raw, _ := body["steps"].([]any)
	count := len(ProcessStepDetails(body))
	if count < processFlowMin || count > processFlowMax {
		return nil
	}
	var out []BudgetItem
	n := 0
	for i, e := range raw {
		var st processStepDetail
		field := fmt.Sprintf("steps[%d]", i)
		descField := ""
		switch t := e.(type) {
		case string:
			if strings.TrimSpace(t) == "" {
				continue
			}
			st.Label = strings.TrimSpace(t)
		case map[string]any:
			lk := authoredKey(t, "label", "title", "name", "step", "text", "description")
			if lk == "" {
				continue
			}
			st.Label = strField(t, lk)
			st.Type = strField(t, "type")
			field = fmt.Sprintf("steps[%d].%s", i, lk)
			if dk := authoredKey(t, "description", "detail", "summary"); dk != "" && strField(t, dk) != st.Label {
				st.Description = strField(t, dk)
				descField = fmt.Sprintf("steps[%d].%s", i, dk)
			}
		default:
			continue
		}
		n++
		total := runeLen(st.flowLabel())
		if total <= processFlowLabelMax {
			continue
		}
		// The box's readable budget shrinks with the step count; it is the
		// length to write to, so the rewrite also clears the fit check.
		box := processFlowLabelMax
		if readable, _ := patterns.ProcessFlowLabelBudget(count, st.Type == "chevron" || st.Type == "arrow"); readable < box {
			box = readable
		}
		const joiner = 3 // " — "
		room := box - runeLen(st.Label) - joiner
		if descField != "" && room >= 10 {
			out = append(out, BudgetItem{Field: descField, What: fmt.Sprintf("step %d's description", n),
				Measured: runeLen(st.Description), Allowed: room,
				Holds: fmt.Sprintf("a flow box of %d steps holds %d with the label, which leaves %d", count, box, room)})
			continue
		}
		out = append(out, BudgetItem{Field: field, What: fmt.Sprintf("step %d", n),
			Measured: total, Allowed: box, Holds: fmt.Sprintf("a flow box of %d steps holds %d with its description", count, box)})
	}
	return out
}

// ExecSummaryBudgetItems lists every lead and support an executive summary
// must shorten to render as exec-summary: those past the pattern's hard limit
// (the cause of a fallback) and, with them, those past the readable budget for
// the point count, so one rewrite clears both. target is the point count the
// budgets are for: the authored count clamped to the pattern's 3–5.
func ExecSummaryBudgetItems(body map[string]any) (items []BudgetItem, target int) {
	field := execSummaryPointsField(body)
	raw, _ := body[field].([]any)
	points, _ := execSummaryPoints(body)
	target = len(points)
	if target < execSummaryMinPoints {
		target = execSummaryMinPoints
	}
	if target > execSummaryMaxPoints {
		target = execSummaryMaxPoints
	}
	bottom := firstNonEmpty(strField(body, "bottom_line"), strField(body, "recommendation"))
	lead, support, _, _ := patterns.ExecSummaryTextBudgets(target, bottom)
	n := 0
	for i, e := range raw {
		var leadText, supportText, leadField, supportField string
		switch t := e.(type) {
		case string:
			leadText = strings.TrimSpace(t)
			leadField = fmt.Sprintf("%s[%d]", field, i)
		case map[string]any:
			lk := authoredKey(t, "lead", "point", "statement", "title", "headline", "text")
			sk := authoredKey(t, "support", "detail", "description", "evidence", "body")
			if lk == "" {
				lk, sk = sk, ""
			}
			if lk == "" {
				continue
			}
			leadText, leadField = strField(t, lk), fmt.Sprintf("%s[%d].%s", field, i, lk)
			if sk != "" {
				supportText, supportField = strField(t, sk), fmt.Sprintf("%s[%d].%s", field, i, sk)
			}
		}
		if leadText == "" {
			continue
		}
		n++
		if runeLen(leadText) > lead {
			holds := fmt.Sprintf("%d points hold about %d per lead", target, lead)
			if supportField == "" {
				holds += fmt.Sprintf(" (a plain string is all lead; {lead, support} keeps %d more as support)", support)
			}
			items = append(items, BudgetItem{Field: leadField, What: fmt.Sprintf("point %d's lead", n), Measured: runeLen(leadText), Allowed: lead, Holds: holds})
		}
		if runeLen(supportText) > support {
			items = append(items, BudgetItem{Field: supportField, What: fmt.Sprintf("point %d's support", n), Measured: runeLen(supportText), Allowed: support,
				Holds: fmt.Sprintf("%d points hold about %d per support", target, support)})
		}
	}
	return items, target
}
