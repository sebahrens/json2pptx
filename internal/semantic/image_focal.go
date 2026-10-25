package semantic

import (
	"encoding/json"
	"math"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func imageFocalPointSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"x", "y"}, "properties": map[string]any{
		"x": map[string]any{"type": "number", "minimum": 0, "maximum": 1}, "y": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
	}}
}

func validateImageFocalPoint(path string, image map[string]any, s *semDiags) {
	raw, present := image["focal_point"]
	if !present {
		return
	}
	path += ".focal_point"
	point, ok := raw.(map[string]any)
	if !ok {
		s.hard(path, diagnostics.CodeSemanticFieldType, "focal_point must be {x,y} in source-image fractions")
		return
	}
	for _, key := range sortedKeys(point) {
		if key != "x" && key != "y" {
			s.unknownField(path+"."+key, key, []string{"x", "y"}, "image focal point")
		}
	}
	for _, key := range []string{"x", "y"} {
		encoded, err := json.Marshal(point[key])
		var value float64
		if err != nil || string(encoded) == "null" || json.Unmarshal(encoded, &value) != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			s.hard(path+"."+key, diagnostics.CodeSemanticFieldType, "focal point coordinate must be a finite fraction in [0,1]")
		}
	}
}
