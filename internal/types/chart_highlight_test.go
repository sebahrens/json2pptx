package types

import (
	"encoding/json"
	"reflect"
	"testing"
)

// go-slide-creator-sdxii: chart_value.highlight reaches svggen as
// data.highlight, and a "highlight" list inside flat data is a directive, not
// a category.
func TestChartSpecHighlightForwarded(t *testing.T) {
	var cs ChartSpec
	if err := json.Unmarshal([]byte(`{"type":"bar","data":{"2024":1,"2025":2},"highlight":["2024"]}`), &cs); err != nil {
		t.Fatal(err)
	}
	ds := cs.ToDiagramSpec()
	if got := ds.Data["highlight"]; !reflect.DeepEqual(got, []any{"2024"}) {
		t.Errorf("data.highlight = %#v", got)
	}
	if cats, _ := ds.Data["categories"].([]string); len(cats) != 2 {
		t.Errorf("categories = %#v", ds.Data["categories"])
	}
}

func TestChartSpecHighlightInsideFlatData(t *testing.T) {
	var cs ChartSpec
	if err := json.Unmarshal([]byte(`{"type":"bar","data":{"A":1,"B":2,"highlight":[1]}}`), &cs); err != nil {
		t.Fatal(err)
	}
	ds := cs.ToDiagramSpec()
	cats, _ := ds.Data["categories"].([]string)
	if !reflect.DeepEqual(cats, []string{"A", "B"}) {
		t.Errorf("highlight leaked into categories: %#v", ds.Data["categories"])
	}
	if !reflect.DeepEqual(ds.Data["highlight"], []any{1.0}) {
		t.Errorf("data.highlight = %#v", ds.Data["highlight"])
	}
}

func TestNormalizeFlatChartDataKeepsHighlight(t *testing.T) {
	raw := []byte(`{"2023":22,"2024":35,"highlight":["2023"]}`)
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	ds := &DiagramSpec{Type: "bar_chart", Data: data}
	if !ds.NormalizeFlatChartData(JSONObjectKeyOrder(raw)) {
		t.Fatal("flat data with a highlight list was not normalized")
	}
	if cats, _ := ds.Data["categories"].([]string); !reflect.DeepEqual(cats, []string{"2023", "2024"}) {
		t.Errorf("categories = %#v", ds.Data["categories"])
	}
	if !reflect.DeepEqual(ds.Data["highlight"], []any{"2023"}) {
		t.Errorf("data.highlight = %#v", ds.Data["highlight"])
	}
}
