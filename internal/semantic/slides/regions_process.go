package slides

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// RegionProcessIssue checks the bounded linear sequence without dropping blank
// or malformed entries. Regions never degrade to another visual.
func RegionProcessIssue(r map[string]any) string {
	raw, ok := r["steps"].([]any)
	if !ok || len(raw) < 3 || len(raw) > 6 {
		return "process region requires 3–6 steps {label, description?}"
	}
	for i, entry := range raw {
		step, ok := entry.(map[string]any)
		if !ok || strField(step, "label") == "" {
			return fmt.Sprintf("process step %d requires a nonblank label", i+1)
		}
		if runeLen(strField(step, "label")) > 40 || runeLen(strField(step, "description")) > 80 {
			return fmt.Sprintf("process step %d holds a label of 40 characters and a description of 80", i+1)
		}
		for _, key := range []string{"label", "description"} {
			if v, exists := step[key]; exists {
				if _, ok := v.(string); !ok {
					return fmt.Sprintf("process step %d %s must be a string", i+1, key)
				}
			}
		}
	}
	return ""
}

// RegionProcessStyle chooses the numbered sequence's axis. Taller regions use two
// columns in reading order; horizontal bands keep the sequence in one row.
func RegionProcessStyle(body map[string]any, idx int) string {
	arrangement := RegionArrangementOf(body)
	if arrangement == ArrangeRows || ((arrangement == ArrangeMainTop || arrangement == ArrangeMainBottom || arrangement == ArrangeMainTop3) && idx == 0) {
		return "horizontal"
	}
	return "vertical"
}

func regionProcess(in Input, idx int, r map[string]any) (regionBuild, error) {
	if reason := RegionProcessIssue(r); reason != "" {
		return regionBuild{}, fmt.Errorf("%s", reason)
	}
	raw, ok := r["steps"].([]any)
	if !ok {
		return regionBuild{}, fmt.Errorf("process steps must be an array")
	}
	cols, rows := 2, (len(raw)+1)/2
	if RegionProcessStyle(in.Body, idx) == "horizontal" {
		cols, rows = len(raw), 1
	}
	cell := &deckinput.GridCellInput{}
	links := []SourceLink{}
	for i, entry := range raw {
		step, ok := entry.(map[string]any)
		if !ok {
			return regionBuild{}, fmt.Errorf("process step %d must be an object", i+1)
		}
		paras := []map[string]any{{"content": fmt.Sprintf("%02d  %s", i+1, strField(step, "label")), "bold": true, "size": 14, "color": "dk1"}}
		if description := strField(step, "description"); description != "" {
			paras = append(paras, map[string]any{"content": description, "size": 12, "color": "dk1"})
		}
		text, _ := json.Marshal(map[string]any{"paragraphs": paras, "align": "l", "vertical_align": "t"})
		cell.Layers = append(cell.Layers, jsonschema.LayerInput{
			Shape: &deckinput.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: text},
			Frame: jsonschema.LayerFrameInput{X: float64(i%cols) / float64(cols), Y: float64(i/cols) / float64(rows), W: 1/float64(cols) - 0.008, H: 1/float64(rows) - 0.012},
		})
		links = append(links, SourceLink{RawPath: fmt.Sprintf(".layers[%d].shape.text", i), SemanticPath: fmt.Sprintf(".steps[%d]", i)})
	}
	return regionBuild{cell: cell, links: links}, nil
}
