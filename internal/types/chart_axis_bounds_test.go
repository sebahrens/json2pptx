package types

import (
	"encoding/json"
	"testing"
)

// go-slide-creator-929jm: data.y_min / data.y_max are value-axis directives.
// Inside a flat {label: value} map they are numbers like any category value,
// so they are recognised by name and forwarded to svggen rather than drawn as
// two more bars.
func TestChartSpecAxisBoundsInsideFlatData(t *testing.T) {
	var cs ChartSpec
	if err := json.Unmarshal([]byte(`{"type":"line","data":{"2024":11,"2025":9,"2026":7,"y_min":5,"y_max":12}}`), &cs); err != nil {
		t.Fatal(err)
	}
	ds := cs.ToDiagramSpec()
	cats, _ := ds.Data["categories"].([]string)
	if len(cats) != 3 {
		t.Errorf("categories = %#v, want the three years only", ds.Data["categories"])
	}
	if got, _ := ds.Data["y_min"].(float64); got != 5 {
		t.Errorf("data.y_min = %#v, want 5", ds.Data["y_min"])
	}
	if got, _ := ds.Data["y_max"].(float64); got != 12 {
		t.Errorf("data.y_max = %#v, want 12", ds.Data["y_max"])
	}
}

// A y_min that is not a number is not a directive: it is left where it was
// so the renderer reports it rather than a category silently vanishing.
func TestChartSpecAxisBoundsNonNumericIsNotADirective(t *testing.T) {
	_, directives := splitChartDirectives(map[string]any{"A": 1.0, "y_min": "low"})
	if _, ok := directives["y_min"]; ok {
		t.Error("a string y_min was taken as a directive")
	}
	_, directives = splitChartDirectives(map[string]any{"A": 1.0, "y_min": 2})
	if _, ok := directives["y_min"]; !ok {
		t.Error("an integer y_min was not taken as a directive")
	}
}

// Structured data passes the bounds through untouched.
func TestChartSpecAxisBoundsInStructuredData(t *testing.T) {
	var cs ChartSpec
	if err := json.Unmarshal([]byte(`{"type":"waterfall","data":{"points":[{"label":"Start","value":24,"type":"total"}],"y_min":20}}`), &cs); err != nil {
		t.Fatal(err)
	}
	ds := cs.ToDiagramSpec()
	if got, _ := ds.Data["y_min"].(float64); got != 20 {
		t.Errorf("data.y_min = %#v, want 20", ds.Data["y_min"])
	}
}
