package main

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

const explicitScaleAdvice = "explicit_scale"

var scaleAdviceFactKeys = map[string]bool{
	"field": true, "value": true, "data_min": true, "data_max": true,
	"axis_min": true, "axis_max": true, "range_share": true, "fix_kind": true,
}

// locateScaleAdvice follows the chart's source mapping, including the nearest
// chart ancestor before pattern expansion inserted its generated grid rows.
// Equal candidates remain ambiguous: never suggest editing a different chart.
func locateScaleAdvice(d *semanticDiagnostic, sm *semantic.SourceMap, ir *semantic.DeckIR, f patterns.FitFinding) {
	if f.Fix == nil || f.Fix.Kind != explicitScaleAdvice {
		return
	}
	field, _ := f.Fix.Params["field"].(string)
	if field != "data.y_min" && field != "data.y_max" {
		return
	}
	raw := strings.Split(strings.Trim(specPointer(f.Path), "/"), "/")
	best, path, ambiguous := 0, "", false
	for _, entry := range sm.Entries() {
		if !strings.HasSuffix(entry.SemanticPath, ".chart") && !strings.HasSuffix(entry.SemanticPath, ".chart_value") && !strings.HasSuffix(entry.SemanticPath, ".diagram") {
			continue
		}
		parts := strings.Split(strings.Trim(specPointer(entry.RawPath), "/"), "/")
		same := 0
		for same < len(raw) && same < len(parts) && raw[same] == parts[same] {
			same++
		}
		if same < 2 {
			continue
		}
		switch {
		case same > best:
			best, path, ambiguous = same, entry.SemanticPath, false
		case same == best && path != entry.SemanticPath:
			ambiguous = true
		}
	}
	idx := slidepath.SlideIndex(f.Path)
	if path == "" && ir != nil && idx >= 0 && idx < len(ir.Slides) && ir.Slides[idx].Kind == semantic.KindChartInsight {
		path = sm.SlidePath(idx) + ".chart"
	}
	if path == "" || ambiguous {
		return
	}
	d.SemanticPath = path + "." + field
	if d.diag != nil {
		d.diag.Path = d.SemanticPath
	}
	attachFixParams(d, f.Fix)
}
