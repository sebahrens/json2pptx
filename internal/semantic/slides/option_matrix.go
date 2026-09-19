package slides

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// option_matrix -> table-highlight (go-slide-creator-6o1r).
//
// The DeckSpec path compiled to 5 of 43 patterns, and the options × criteria
// evaluation matrix — the slide a recommendation is actually argued on — was not
// one of them. Three reviewers independently reached for it and had to drop to
// kind raw_json2pptx with a hand-written pattern block.

// optionMatrixCriterion mirrors patterns.TableHighlightCriterion for emission.
type optionMatrixCriterion struct {
	Label string `json:"label"`
	Scale string `json:"scale,omitempty"`
}

// optionMatrixOption mirrors patterns.TableHighlightOption for emission.
type optionMatrixOption struct {
	Name   string            `json:"name"`
	Detail string            `json:"detail,omitempty"`
	Scores []json.RawMessage `json:"scores"`
}

// optionMatrixValues mirrors patterns.TableHighlightValues for emission.
type optionMatrixValues struct {
	Criteria       []optionMatrixCriterion `json:"criteria"`
	Options        []optionMatrixOption    `json:"options"`
	Scale          string                  `json:"scale,omitempty"`
	HighlightRow   *int                    `json:"highlight_row,omitempty"`
	HighlightCol   *int                    `json:"highlight_col,omitempty"`
	HighlightLabel string                  `json:"highlight_label,omitempty"`
	CornerLabel    string                  `json:"corner_label,omitempty"`
}

// table-highlight's own bounds, mirrored so a payload that passes here renders.
const (
	optionMatrixMinCriteria  = 2
	optionMatrixMaxCriteria  = 6
	optionMatrixMinOptions   = 2
	optionMatrixMaxOptions   = 6
	optionMatrixCriterionMax = 30
	optionMatrixNameMax      = 40
	optionMatrixDetailMax    = 60
)

// CompileOptionMatrix compiles an options × criteria evaluation to the
// table-highlight pattern: criteria as columns, options as rows, one score per
// cell, with the recommended option's row and the decisive criterion's column
// highlighted. When the matrix is outside the pattern's 2–6 × 2–6 bounds — or a
// score is not readable on the chosen scale — it degrades to a content slide
// listing each option and its scores, so the slide always renders.
func CompileOptionMatrix(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	values, ok := optionMatrixValuesFrom(in.Body)
	if !ok {
		return compileOptionMatrixFallback(in)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal table-highlight values: %w", err)
	}
	// The pattern's own validator is the last word on the score vocabulary
	// (harvey 0–4 or none/quarter/half/three-quarter/full, rag red/amber/green,
	// text ≤24 chars): rather than reimplement it, ask it, and degrade when it
	// says no. OptionMatrixPatternFeasible asks the same question, so validation
	// warns about the degradation before the render.
	if deckinput.ValidatePattern(&deckinput.PatternInput{Name: "table-highlight", Values: encoded}, patterns.Default()) != nil {
		return compileOptionMatrixFallback(in)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "table-highlight", Values: encoded}
	links = append(links,
		SourceLink{RawPath: in.rawSlide() + ".pattern.values.criteria", SemanticPath: in.semSlide() + ".criteria"},
		SourceLink{RawPath: in.rawSlide() + ".pattern.values.options", SemanticPath: in.semSlide() + ".options"},
	)
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// OptionMatrixPatternFeasible reports whether an option-matrix payload will
// compile to table-highlight rather than degrade to a content slide. The explain
// projection and the validation advisory both consult it.
func OptionMatrixPatternFeasible(body map[string]any) bool {
	values, ok := optionMatrixValuesFrom(body)
	if !ok {
		return false
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return false
	}
	return deckinput.ValidatePattern(&deckinput.PatternInput{Name: "table-highlight", Values: encoded}, patterns.Default()) == nil
}

// UsableOptionMatrixCounts returns the criteria and option counts that survive
// extraction, so validation counts what compile will render rather than raw list
// entries.
func UsableOptionMatrixCounts(body map[string]any) (criteria, options int) {
	return len(optionMatrixCriteria(body)), len(optionMatrixOptions(body, 0))
}

// optionMatrixValuesFrom builds the pattern payload, reporting false when the
// matrix is outside table-highlight's bounds or an option's score count does not
// match the criteria.
func optionMatrixValuesFrom(body map[string]any) (*optionMatrixValues, bool) {
	criteria := optionMatrixCriteria(body)
	if len(criteria) < optionMatrixMinCriteria || len(criteria) > optionMatrixMaxCriteria {
		return nil, false
	}
	options := optionMatrixOptions(body, len(criteria))
	if len(options) < optionMatrixMinOptions || len(options) > optionMatrixMaxOptions {
		return nil, false
	}
	for _, o := range options {
		if len(o.Scores) != len(criteria) {
			return nil, false
		}
	}

	values := &optionMatrixValues{
		Criteria: criteria,
		Options:  options,
		Scale:    strings.ToLower(strField(body, "scale")),
		// A badge over the pattern's 24-char budget is dropped rather than
		// costing the author the whole evaluation matrix: the badge labels a row
		// that is highlighted anyway, so losing the table over it is the wrong
		// trade. Validation names the budget so the drop is not silent.
		HighlightLabel: optionMatrixLabel(firstNonEmpty(strField(body, "highlight_label"), strField(body, "recommended_label"))),
		CornerLabel:    optionMatrixLabel(strField(body, "corner_label")),
	}
	if idx, ok := optionMatrixRowIndex(body, options); ok {
		values.HighlightRow = &idx
	}
	if idx, ok := optionMatrixColIndex(body, criteria); ok {
		values.HighlightCol = &idx
	}
	return values, true
}

// optionMatrixLabelMax is table-highlight's budget for the highlight and corner
// badges.
const optionMatrixLabelMax = 24

// optionMatrixLabel keeps a badge only when it fits the pattern's budget.
func optionMatrixLabel(label string) string {
	if len(label) > optionMatrixLabelMax {
		return ""
	}
	return label
}

// OptionMatrixLabelFits reports whether a badge fits table-highlight's budget,
// so validation can warn about one that will be dropped.
func OptionMatrixLabelFits(label string) bool { return len(label) <= optionMatrixLabelMax }

// optionMatrixCriteria extracts the criteria columns. An entry is either a bare
// label or an object carrying the label and an optional per-column scale.
func optionMatrixCriteria(body map[string]any) []optionMatrixCriterion {
	field := firstPopulatedList(body, "criteria", "columns")
	if field == "" {
		return nil
	}
	raw, _ := body[field].([]any)
	out := make([]optionMatrixCriterion, 0, len(raw))
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				out = append(out, optionMatrixCriterion{Label: s})
			}
		case map[string]any:
			label := firstNonEmpty(strField(t, "label"), strField(t, "name"), strField(t, "title"), strField(t, "criterion"))
			if label == "" {
				continue
			}
			out = append(out, optionMatrixCriterion{Label: label, Scale: strings.ToLower(strField(t, "scale"))})
		}
	}
	return out
}

// optionMatrixOptions extracts the option rows and their scores. criteriaCount
// is used only to size the score slice; a row with the wrong count is kept as-is
// so the caller can detect and report the mismatch.
func optionMatrixOptions(body map[string]any, criteriaCount int) []optionMatrixOption {
	field := firstPopulatedList(body, "options", "rows")
	if field == "" {
		return nil
	}
	out := make([]optionMatrixOption, 0, len(mapList(body, field)))
	for _, m := range mapList(body, field) {
		name := firstNonEmpty(strField(m, "name"), strField(m, "option"), strField(m, "label"), strField(m, "title"))
		if name == "" {
			continue
		}
		out = append(out, optionMatrixOption{
			Name:   name,
			Detail: firstNonEmpty(strField(m, "detail"), strField(m, "description"), strField(m, "summary")),
			Scores: optionMatrixScores(m, criteriaCount),
		})
	}
	return out
}

// optionMatrixScores renders an option's scores as raw JSON, preserving the
// author's own literal: table-highlight reads a harvey score as a number 0–4 or
// a name, a RAG score as red/amber/green, a text score as a short string.
func optionMatrixScores(option map[string]any, criteriaCount int) []json.RawMessage {
	raw, ok := option[firstPopulatedList(option, "scores", "values")].([]any)
	if !ok {
		return nil
	}
	out := make([]json.RawMessage, 0, max(len(raw), criteriaCount))
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			encoded, err := json.Marshal(strings.TrimSpace(t))
			if err != nil {
				return nil
			}
			out = append(out, encoded)
		case float64:
			out = append(out, json.RawMessage(strconv.FormatFloat(t, 'f', -1, 64)))
		case int:
			out = append(out, json.RawMessage(strconv.Itoa(t)))
		case int64:
			out = append(out, json.RawMessage(strconv.FormatInt(t, 10)))
		case json.Number:
			out = append(out, json.RawMessage(t.String()))
		case bool:
			encoded, _ := json.Marshal(strconv.FormatBool(t))
			out = append(out, encoded)
		case nil:
			out = append(out, json.RawMessage(`"-"`))
		default:
			encoded, err := json.Marshal(fmt.Sprintf("%v", t))
			if err != nil {
				return nil
			}
			out = append(out, encoded)
		}
	}
	return out
}

// optionMatrixRowIndex resolves the recommended option to its row index. An
// author names the option ("recommended: Hub consolidation") far more naturally
// than they count rows, so a name is matched case-insensitively first; an
// explicit index is honoured too.
func optionMatrixRowIndex(body map[string]any, options []optionMatrixOption) (int, bool) {
	for _, field := range []string{"recommended", "recommended_option", "highlight_row"} {
		v, ok := body[field]
		if !ok {
			continue
		}
		if idx, ok := optionMatrixIndexValue(v, len(options)); ok {
			return idx, true
		}
		name, ok := v.(string)
		if !ok {
			continue
		}
		for i, o := range options {
			if strings.EqualFold(strings.TrimSpace(name), o.Name) {
				return i, true
			}
		}
	}
	return 0, false
}

// optionMatrixColIndex resolves the decisive criterion to its column index, by
// label or by index.
func optionMatrixColIndex(body map[string]any, criteria []optionMatrixCriterion) (int, bool) {
	for _, field := range []string{"decisive_criterion", "highlight_col", "highlight_column"} {
		v, ok := body[field]
		if !ok {
			continue
		}
		if idx, ok := optionMatrixIndexValue(v, len(criteria)); ok {
			return idx, true
		}
		label, ok := v.(string)
		if !ok {
			continue
		}
		for i, c := range criteria {
			if strings.EqualFold(strings.TrimSpace(label), c.Label) {
				return i, true
			}
		}
	}
	return 0, false
}

// optionMatrixIndexValue reads a numeric index in range, or reports false.
func optionMatrixIndexValue(v any, n int) (int, bool) {
	var idx int
	switch t := v.(type) {
	case float64:
		idx = int(t)
	case int:
		idx = t
	case json.Number:
		parsed, err := t.Int64()
		if err != nil {
			return 0, false
		}
		idx = int(parsed)
	default:
		return 0, false
	}
	if idx < 0 || idx >= n {
		return 0, false
	}
	return idx, true
}

// firstPopulatedList returns the first of names that holds a non-empty list.
func firstPopulatedList(body map[string]any, names ...string) string {
	for _, n := range names {
		if raw, ok := body[n].([]any); ok && len(raw) > 0 {
			return n
		}
	}
	return ""
}

// compileOptionMatrixFallback renders the matrix as a content slide, one bullet
// per option carrying its scores labelled by criterion, so a matrix outside the
// pattern's bounds still says everything the author wrote.
func compileOptionMatrixFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	criteria := optionMatrixCriteria(in.Body)
	options := optionMatrixOptions(in.Body, len(criteria))

	bullets := make([]string, 0, len(options))
	for _, o := range options {
		parts := make([]string, 0, len(o.Scores))
		for i, score := range o.Scores {
			label := ""
			if i < len(criteria) {
				label = criteria[i].Label + ": "
			}
			parts = append(parts, label+optionMatrixScoreText(score))
		}
		line := o.Name
		if o.Detail != "" {
			line += " — " + o.Detail
		}
		if len(parts) > 0 {
			line += " (" + strings.Join(parts, "; ") + ")"
		}
		bullets = append(bullets, line)
	}
	return contentFallback(in, "options", bullets)
}

// optionMatrixScoreText renders one raw score for the bullet fallback, unquoting
// a string so the reader sees the value rather than its JSON literal.
func optionMatrixScoreText(score json.RawMessage) string {
	var s string
	if err := json.Unmarshal(score, &s); err == nil {
		return s
	}
	return string(score)
}
