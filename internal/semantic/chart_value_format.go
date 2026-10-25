package semantic

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/types"
)

func chartValueFormatSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"style":    map[string]any{"type": "string", "enum": []string{"plain", "compact", "percent", "currency"}},
		"decimals": map[string]any{"type": "integer", "minimum": 0, "maximum": 3},
		"prefix":   map[string]any{"type": "string"}, "suffix": map[string]any{"type": "string"},
		"thousands_sep": map[string]any{"type": "boolean"},
		"input_scale":   map[string]any{"type": "string", "enum": []string{"auto", "fraction", "percentage_points"}},
	}, "description": "Shared notation for value-axis ticks and data labels."}
}

func validateChartValueFormat(path string, chart map[string]any, s *semDiags) {
	raw, present := chart["value_format"]
	if !present {
		return
	}
	path += ".value_format"
	obj, ok := raw.(map[string]any)
	if !ok {
		s.hard(path, diagnostics.CodeSemanticFieldType, "chart.value_format must be an object")
		return
	}
	props, _ := chartValueFormatSchema()["properties"].(map[string]any)
	for _, key := range sortedKeys(obj) {
		field, known := props[key]
		if !known {
			s.unknownField(path+"."+key, key, sortedKeys(props), "chart number format")
			continue
		}
		schema, _ := field.(map[string]any)
		valid := false
		switch schema["type"] {
		case "string":
			value, isString := obj[key].(string)
			valid = isString
			if values, enum := schema["enum"].([]string); enum {
				valid = isString && containsKey(values, value)
			}
		case "boolean":
			_, valid = obj[key].(bool)
		case "integer":
			encoded, err := json.Marshal(obj[key])
			var value float64
			valid = err == nil && json.Unmarshal(encoded, &value) == nil && value == math.Trunc(value) && value >= 0 && value <= 3
		}
		if !valid {
			s.hard(path+"."+key, diagnostics.CodeSemanticFieldType, fmt.Sprintf("invalid chart number format %s", key))
		}
	}
	encoded, err := json.Marshal(obj)
	var format types.ValueFormatSpec
	if err != nil || json.Unmarshal(encoded, &format) != nil {
		return
	}
	if format.InputScale != "" && format.InputScale != "auto" && format.Style != "percent" {
		s.hard(path+".input_scale", diagnostics.CodeSemanticFieldType, "input_scale applies only to percent formatting")
	}
}
