package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/template"
)

// go-slide-creator-4kq5m: shape_grid.columns was parsed as a float64 and
// passed through int() unchecked. 9223372036854775807 / 1e308 panicked in
// makeslice (validate exit 2, MCP RENDER.INTERNAL); 50000000 allocated
// ~900 MB silently. Each must now be refused with the accepted range.
var outOfRangeColumns = []string{"9223372036854775807", "1e308", "50000000", "-5", "0", "2.5", "25"}

func TestResolveColumnsDTORejectsOutOfRange(t *testing.T) {
	rows := []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}}}}
	for _, raw := range outOfRangeColumns {
		cols, err := resolveColumnsDTO(json.RawMessage(raw), rows)
		if err == nil {
			t.Errorf("columns %s: resolved %d columns, want an error", raw, len(cols))
			continue
		}
		if !strings.Contains(err.Error(), "1 to 24") {
			t.Errorf("columns %s: error should state the accepted range: %v", raw, err)
		}
	}
	big := "[" + strings.TrimSuffix(strings.Repeat("1,", shapegrid.MaxColumns+1), ",") + "]"
	if _, err := resolveColumnsDTO(json.RawMessage(big), rows); err == nil {
		t.Error("a 25-entry columns array must be refused")
	}
	if cols, err := resolveColumnsDTO(json.RawMessage("24"), rows); err != nil || len(cols) != 24 {
		t.Errorf("columns 24 must resolve: %v %v", cols, err)
	}
}

func TestValidateInputRefusesOutOfRangeColumns(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	for _, raw := range outOfRangeColumns {
		deck := `{"template":"midnight-blue","slides":[{"slide_type":"blank","content":[
		  {"placeholder_id":"title","type":"text","text_value":"Two cells side by side"}],
		  "shape_grid":{"columns":` + raw + `,"rows":[{"cells":[
		   {"shape":{"geometry":"rect","fill":"accent1","text":"a"}},
		   {"shape":{"geometry":"rect","fill":"accent1","text":"b"}}]}]}}]}`
		result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deck), "fit_report": true,
		}))
		if err != nil || result == nil {
			t.Fatalf("columns %s: validate_input failed: %v", raw, err)
		}
		text := textContent(result)
		if !result.IsError || !strings.Contains(text, "INVALID_GRID") || !strings.Contains(text, "1 to 24") {
			t.Errorf("columns %s: want an INVALID_GRID refusal naming the range, got: %.600s", raw, text)
		}
		if strings.Contains(text, "RENDER.INTERNAL") || strings.Contains(text, "makeslice") {
			t.Errorf("columns %s: validate_input crashed: %.600s", raw, text)
		}
	}
}
