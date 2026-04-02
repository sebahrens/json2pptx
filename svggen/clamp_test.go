package svggen

import (
	"math"
	"testing"
)

func TestClampFloat64(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{"zero", 0, 0},
		{"positive normal", 42.5, 42.5},
		{"negative normal", -100, -100},
		{"at positive boundary", maxSafeValue, maxSafeValue},
		{"at negative boundary", -maxSafeValue, -maxSafeValue},
		{"above positive boundary", 2e15, maxSafeValue},
		{"below negative boundary", -2e15, -maxSafeValue},
		{"float64 max", math.MaxFloat64, maxSafeValue},
		{"float64 negative max", -math.MaxFloat64, -maxSafeValue},
		{"positive inf", math.Inf(1), maxSafeValue},
		{"negative inf", math.Inf(-1), -maxSafeValue},
		{"NaN", math.NaN(), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampFloat64(tt.in)
			if got != tt.want {
				t.Errorf("clampFloat64(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestClampDataValues(t *testing.T) {
	data := map[string]any{
		"normal":    42.0,
		"huge":      math.MaxFloat64,
		"neg_huge":  -math.MaxFloat64,
		"string":    "hello",
		"int_val":   123,
		"nil_val":   nil,
		"nested": map[string]any{
			"deep_huge": 2e16,
			"deep_ok":   7.0,
		},
		"list": []any{
			1e20,
			-1e20,
			"text",
			map[string]any{
				"inner": math.Inf(1),
			},
		},
	}

	clampDataValues(data)

	// Check top-level values
	if v := data["normal"].(float64); v != 42.0 {
		t.Errorf("normal: got %v, want 42.0", v)
	}
	if v := data["huge"].(float64); v != maxSafeValue {
		t.Errorf("huge: got %v, want %v", v, maxSafeValue)
	}
	if v := data["neg_huge"].(float64); v != -maxSafeValue {
		t.Errorf("neg_huge: got %v, want %v", v, -maxSafeValue)
	}
	if v := data["string"].(string); v != "hello" {
		t.Errorf("string: got %v, want hello", v)
	}
	if v := data["int_val"].(int); v != 123 {
		t.Errorf("int_val: got %v, want 123", v)
	}

	// Check nested map
	nested := data["nested"].(map[string]any)
	if v := nested["deep_huge"].(float64); v != maxSafeValue {
		t.Errorf("nested.deep_huge: got %v, want %v", v, maxSafeValue)
	}
	if v := nested["deep_ok"].(float64); v != 7.0 {
		t.Errorf("nested.deep_ok: got %v, want 7.0", v)
	}

	// Check list
	list := data["list"].([]any)
	if v := list[0].(float64); v != maxSafeValue {
		t.Errorf("list[0]: got %v, want %v", v, maxSafeValue)
	}
	if v := list[1].(float64); v != -maxSafeValue {
		t.Errorf("list[1]: got %v, want %v", v, -maxSafeValue)
	}
	if v := list[2].(string); v != "text" {
		t.Errorf("list[2]: got %v, want text", v)
	}
	innerMap := list[3].(map[string]any)
	if v := innerMap["inner"].(float64); v != maxSafeValue {
		t.Errorf("list[3].inner: got %v, want %v", v, maxSafeValue)
	}
}
