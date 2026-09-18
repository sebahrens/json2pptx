package types

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNormalizeFlatChartData_ConvertsShorthandPreservingOrder(t *testing.T) {
	raw := json.RawMessage(`{"Q4": 18, "Q1": 12, "Q3": 15.2, "Q2": 14.5}`)
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"bar", "bar_chart"} {
		ds := &DiagramSpec{Type: typ, Data: copyMap(data)}
		if !ds.NormalizeFlatChartData(JSONObjectKeyOrder(raw)) {
			t.Fatalf("%s: expected conversion", typ)
		}
		if ds.Type != "bar_chart" {
			t.Errorf("%s: type = %q, want bar_chart", typ, ds.Type)
		}
		if got, want := ds.Data["categories"], []string{"Q4", "Q1", "Q3", "Q2"}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: categories = %v, want author order %v", typ, got, want)
		}
		series, ok := ds.Data["series"].([]map[string]any)
		if !ok || len(series) != 1 {
			t.Fatalf("%s: series = %#v", typ, ds.Data["series"])
		}
		if got, want := series[0]["values"], []float64{18, 12, 15.2, 14.5}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: values = %v, want %v", typ, got, want)
		}
	}
}

func TestNormalizeFlatChartData_LeavesOtherShapesAlone(t *testing.T) {
	cases := []*DiagramSpec{
		{Type: "bar_chart", Data: map[string]any{"categories": []any{"A"}, "series": []any{}}},
		{Type: "gauge_chart", Data: map[string]any{"value": 40.0, "min": 0.0, "max": 100.0}},
		{Type: "timeline", Data: map[string]any{"A": 1.0}},
		{Type: "bar", Data: map[string]any{"A": "not a number"}},
		nil,
	}
	for i, ds := range cases {
		var before map[string]any
		if ds != nil {
			before = copyMap(ds.Data)
		}
		if ds.NormalizeFlatChartData(nil) {
			t.Errorf("case %d: unexpected conversion", i)
		}
		if ds != nil && !reflect.DeepEqual(ds.Data, before) {
			t.Errorf("case %d: data mutated", i)
		}
	}
}

func TestNormalizeFlatChartData_IncompleteOrderFallsBackToSorted(t *testing.T) {
	ds := &DiagramSpec{Type: "line", Data: map[string]any{"b": 2.0, "a": 1.0}}
	ds.NormalizeFlatChartData([]string{"b"})
	if got, want := ds.Data["categories"], []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("categories = %v, want sorted %v", got, want)
	}
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
