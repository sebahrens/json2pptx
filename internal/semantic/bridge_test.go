package semantic

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

func bridgeSpec(body map[string]any) *DeckSpec {
	return &DeckSpec{Meta: DeckMeta{Title: "P&L walk"}, Slides: []SlideSpec{{Kind: KindBridge, Body: body}}}
}

func bridgeColumns(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = map[string]any{"label": fmt.Sprintf("Driver %d", i), "type": "delta", "value": -1}
	}
	out[0] = map[string]any{"label": "Revenue", "type": "total", "value": 120}
	return out
}

func TestBridgeCompilePatternAndFallback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   map[string]any
		visual bool
	}{
		{"valid", map[string]any{"title": "Walk", "columns": []any{
			map[string]any{"label": "Revenue", "type": "total", "value": 120},
			map[string]any{"label": "COGS", "type": "delta", "value": -45},
			map[string]any{"label": "Gross profit", "type": "subtotal"},
		}, "unit": "$m", "caption": "FY26, USD millions"}, true},
		{"eleven columns", map[string]any{"title": "Walk", "columns": bridgeColumns(11)}, false},
		{"long label", map[string]any{"title": "Walk", "columns": []any{
			map[string]any{"label": strings.Repeat("A", 41), "type": "total", "value": 120},
			map[string]any{"label": "COGS", "type": "delta", "value": -45},
			map[string]any{"label": "End", "type": "subtotal"},
		}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ir := Normalize(bridgeSpec(tc.body))
			if got := ir.Slides[0].Visual.Pattern; (got == "waterfall-bridge") != tc.visual {
				t.Fatalf("planned pattern = %q", got)
			}
			input, result, err := Compile(bridgeSpec(tc.body), CompileOptions{})
			if err != nil {
				t.Fatalf("compile: %v; %+v", err, result.Diagnostics)
			}
			slide := input.Slides[0]
			if tc.visual {
				if slide.Pattern == nil || slide.Pattern.Name != "waterfall-bridge" {
					t.Fatalf("pattern = %+v", slide.Pattern)
				}
				pat, ok := patterns.Default().Get(slide.Pattern.Name)
				if !ok {
					t.Fatal("unregistered pattern")
				}
				values := pat.NewValues()
				if err := json.Unmarshal(slide.Pattern.Values, values); err != nil {
					t.Fatal(err)
				}
				if err := pat.Validate(values, nil, nil); err != nil {
					t.Fatalf("pattern values: %v", err)
				}
				var got struct {
					Columns []struct {
						Label, Type string
						Value       *float64
					}
				}
				if err := json.Unmarshal(slide.Pattern.Values, &got); err != nil {
					t.Fatal(err)
				}
				if len(got.Columns) != 3 || got.Columns[1].Value == nil || *got.Columns[1].Value != -45 || got.Columns[2].Value != nil {
					t.Fatalf("value mismatch: %+v", got)
				}
				if hasCode(result.Diagnostics, diagnostics.CodeSemanticPatternDegraded) {
					t.Fatalf("unexpected degrade: %+v", result.Diagnostics)
				}
			} else {
				if slide.Pattern != nil {
					t.Fatalf("unexpected pattern: %+v", slide.Pattern)
				}
				found := false
				for _, c := range slide.Content {
					if c.PlaceholderID == "body" && c.BulletsValue != nil && len(*c.BulletsValue) == len(tc.body["columns"].([]any)) {
						found = true
					}
				}
				if !found {
					t.Fatalf("fallback lost columns: %+v", slide.Content)
				}
				if !hasCode(result.Diagnostics, diagnostics.CodeSemanticPatternDegraded) {
					t.Fatalf("missing degrade: %+v", result.Diagnostics)
				}
			}
		})
	}
}

func TestBridgeMalformedColumnHasSemanticPath(t *testing.T) {
	for _, tc := range []struct {
		key   string
		value any
		path  string
	}{
		{"value", "-45", "slides[0].columns[1].value"},
		{"type", "adjustment", "slides[0].columns[1].type"},
		{"label", "", "slides[0].columns[1].label"},
		{"subtotal", 999, "slides[0].columns[2].value"},
	} {
		cols := bridgeColumns(3)
		if tc.key == "subtotal" {
			cols[2] = map[string]any{"label": "End", "type": "subtotal", "value": tc.value}
		} else {
			cols[1].(map[string]any)[tc.key] = tc.value
		}
		input, result, err := Compile(bridgeSpec(map[string]any{"columns": cols}), CompileOptions{})
		if err == nil || input != nil {
			t.Fatalf("%s compiled malformed input", tc.key)
		}
		found := false
		for _, d := range result.Diagnostics {
			if d.Path == tc.path && d.Severity == diagnostics.SeverityError {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: missing error at %s: %+v", tc.key, tc.path, result.Diagnostics)
		}
	}
}

func TestBridgeWrongColumnsTypeIsBlocking(t *testing.T) {
	input, result, err := Compile(bridgeSpec(map[string]any{"columns": "Revenue 120, COGS -45"}), CompileOptions{})
	if err == nil || input != nil {
		t.Fatalf("wrong-typed columns compiled: %+v", input)
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Path == "slides[0].columns" && d.Severity == diagnostics.SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing blocking semantic path: %+v", result.Diagnostics)
	}
}

func TestBridgeWrongUnitTypeIsBlocking(t *testing.T) {
	input, result, err := Compile(bridgeSpec(map[string]any{"columns": bridgeColumns(3), "unit": 7}), CompileOptions{})
	if err == nil || input != nil {
		t.Fatalf("wrong-typed unit compiled: %+v", input)
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Path == "slides[0].unit" && d.Severity == diagnostics.SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing blocking semantic path: %+v", result.Diagnostics)
	}
}

func TestBridgeDiscoverySchema(t *testing.T) {
	ex := KindExample(KindBridge)
	if ex == nil || ex["kind"] != "bridge" {
		t.Fatalf("example: %+v", ex)
	}
	item := KindItemSchema(KindBridge)
	if item["additionalProperties"] != false {
		t.Fatalf("open bridge schema: %+v", item)
	}
	props := item["properties"].(map[string]any)
	cols := props["columns"].(map[string]any)["items"].(map[string]any)
	keys := cols["properties"].(map[string]any)
	if keys["value"].(map[string]any)["type"] != "number" {
		t.Fatalf("value schema: %+v", keys["value"])
	}
	if cols["additionalProperties"] != false {
		t.Fatal("open column schema")
	}
}
