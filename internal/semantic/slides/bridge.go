package slides

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// These bounds mirror waterfall-bridge's values schema.
const (
	bridgeMinColumns = 3
	bridgeMaxColumns = 10
	bridgeLabelMax   = 40
	bridgeUnitMax    = 8
	bridgeCaptionMax = 60
)

type bridgeColumn struct {
	Label string   `json:"label"`
	Value *float64 `json:"value,omitempty"`
	Type  string   `json:"type"`
}

type bridgeValues struct {
	Columns []bridgeColumn `json:"columns"`
	Unit    string         `json:"unit,omitempty"`
	Caption string         `json:"caption,omitempty"`
}

// BridgeProblem identifies the first invalid column or pattern budget. The
// semantic validator uses its path; the planner and compiler use the same fit
// decision, so they never promise a visual that compilation drops.
func BridgeProblem(body map[string]any) (string, string) {
	raw, ok := body["columns"].([]any)
	if !ok {
		return "columns", "columns must be an array"
	}
	if len(raw) == 0 {
		return "columns", "provide at least one bridge column"
	}
	running := 0.0
	for i, entry := range raw {
		path := fmt.Sprintf("columns[%d]", i)
		col, ok := entry.(map[string]any)
		if !ok {
			return path, "column must be an object"
		}
		if field, problem := bridgeColumnProblem(col, path, &running); problem != "" {
			return field, problem
		}
	}
	if len(raw) < bridgeMinColumns || len(raw) > bridgeMaxColumns {
		return "columns", fmt.Sprintf("%d columns exceed waterfall-bridge's 3–10 column range", len(raw))
	}
	for _, budget := range []struct {
		key string
		max int
	}{{"unit", bridgeUnitMax}, {"caption", bridgeCaptionMax}} {
		v, present := body[budget.key]
		if !present {
			continue
		}
		value, ok := v.(string)
		if !ok {
			return budget.key, budget.key + " must be a string"
		}
		if utf8.RuneCountInString(value) > budget.max {
			return budget.key, fmt.Sprintf("%s exceeds %d characters", budget.key, budget.max)
		}
	}
	return "", ""
}

func bridgeColumnProblem(col map[string]any, path string, running *float64) (string, string) {
	label, ok := col["label"].(string)
	if !ok || strings.TrimSpace(label) == "" {
		return path + ".label", "label must be a non-empty string"
	}
	if utf8.RuneCountInString(label) > bridgeLabelMax {
		return path + ".label", "label exceeds 40 characters"
	}
	typ, ok := col["type"].(string)
	if !ok || (typ != "total" && typ != "delta" && typ != "subtotal") {
		return path + ".type", "type must be total, delta, or subtotal"
	}
	v, present := col["value"]
	if !present && typ != "subtotal" {
		return path + ".value", "total and delta columns need a numeric value"
	}
	if !present {
		return "", ""
	}
	n, ok := bridgeNumber(v)
	if !ok || math.Abs(n) > 1e12 {
		return path + ".value", "value must be a finite number between -1e12 and 1e12"
	}
	switch typ {
	case "total":
		*running = n
	case "delta":
		*running += n
	case "subtotal":
		if math.Abs(n-*running) > 1e-9 {
			return path + ".value", "subtotal value must match the running total (or omit it for automatic calculation)"
		}
	}
	return "", ""
}

func bridgeNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, !math.IsNaN(n) && !math.IsInf(n, 0)
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
	default:
		return 0, false
	}
}

func BridgePattern(body map[string]any) string {
	if path, _ := BridgeProblem(body); path == "" {
		return "waterfall-bridge"
	}
	return ""
}

func CompileBridge(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	if BridgePattern(in.Body) == "" {
		return compileBridgeFallback(in)
	}
	raw, ok := in.Body["columns"].([]any)
	if !ok {
		return compileBridgeFallback(in)
	}
	values := bridgeValues{Unit: strField(in.Body, "unit"), Caption: strField(in.Body, "caption")}
	for _, entry := range raw {
		obj, ok := entry.(map[string]any)
		if !ok {
			return compileBridgeFallback(in)
		}
		label, labelOK := obj["label"].(string)
		typ, typeOK := obj["type"].(string)
		if !labelOK || !typeOK {
			return compileBridgeFallback(in)
		}
		col := bridgeColumn{Label: strings.TrimSpace(label), Type: typ}
		if rawValue, ok := obj["value"]; ok {
			n, _ := bridgeNumber(rawValue)
			col.Value = &n
		}
		values.Columns = append(values.Columns, col)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal waterfall-bridge values: %w", err)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "waterfall-bridge", Values: encoded}
	links = append(links, SourceLink{RawPath: in.rawSlide() + ".pattern.values.columns", SemanticPath: in.semSlide() + ".columns"})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

func compileBridgeFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	raw, _ := in.Body["columns"].([]any)
	bullets := make([]string, 0, len(raw)+2)
	if c := strField(in.Body, "caption"); c != "" {
		bullets = append(bullets, c)
	}
	for i, entry := range raw {
		obj, ok := entry.(map[string]any)
		if !ok {
			bullets = append(bullets, fmt.Sprintf("Column %d: %v", i+1, entry))
			continue
		}
		label, _ := obj["label"].(string)
		typ, _ := obj["type"].(string)
		line := fmt.Sprintf("%s (%s)", label, typ)
		if v, ok := obj["value"]; ok {
			line += fmt.Sprintf(": %v%s", v, strField(in.Body, "unit"))
		}
		bullets = append(bullets, line)
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "content"}
	links := titleLink(slide, in)
	idx := appendContent(slide, bulletsContent("body", bullets))
	links = append(links, SourceLink{RawPath: fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx), SemanticPath: in.semSlide() + ".columns"})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}
