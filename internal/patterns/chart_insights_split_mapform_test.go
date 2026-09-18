package patterns

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestChartInsightsSplitValues_MapFormChart covers go-slide-creator-yzbo: the
// {label: value} chart shorthand must be normalized to svggen's
// categories/series payload at decode time, keeping the author's key order.
func TestChartInsightsSplitValues_MapFormChart(t *testing.T) {
	raw := `{"chart": {"type": "bar", "data": {"Travel": 3.9, "IT": 11.2, "Logistics": 6.4}}, "insights": ["a"]}`
	var v ChartInsightsSplitValues
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Chart == nil || v.Chart.Type != "bar_chart" {
		t.Fatalf("chart = %+v, want normalized bar_chart", v.Chart)
	}
	if got, want := v.Chart.Data["categories"], []string{"Travel", "IT", "Logistics"}; !reflect.DeepEqual(got, want) {
		t.Errorf("categories = %v, want %v", got, want)
	}
	if len(v.Insights) != 1 {
		t.Errorf("insights lost during custom unmarshal: %v", v.Insights)
	}
}

func TestChartInsightsSplitValues_NativeChartUntouched(t *testing.T) {
	raw := `{"chart": {"type": "bar_chart", "data": {"categories": ["A", "B"], "series": [{"name": "S", "values": [1, 2]}]}}, "insights": ["a"]}`
	var v ChartInsightsSplitValues
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := v.Chart.Data["series"].([]any); !ok {
		t.Errorf("native series payload should pass through unchanged, got %#v", v.Chart.Data["series"])
	}
}
