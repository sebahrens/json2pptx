package slides

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// kpiCell is the raw kpi-Nup cell shape: a big number and a short caption.
type kpiCell struct {
	Big   string `json:"big"`
	Small string `json:"small"`
}

// CompileKPISnapshot compiles a KPI-snapshot slide. With 2–6 metrics it emits a
// kpi-Nup pattern (the variant selected by metric count) under a title
// placeholder; with a count outside that range it falls back to a content slide
// listing the metrics as bullets, so the slide always validates.
func CompileKPISnapshot(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	cells, srcField := kpiCells(in.Body)

	// kpi-Nup is registered only for N in 2..6. Outside that range there is no
	// matching pattern, so degrade to a safe content slide.
	if len(cells) < 2 || len(cells) > 6 {
		return compileKPIFallback(in, cells, srcField)
	}

	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	values, err := json.Marshal(cells)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal kpi values: %w", err)
	}
	slide.Pattern = &deckinput.PatternInput{
		Name:   fmt.Sprintf("kpi-%dup", len(cells)),
		Values: values,
	}
	for k := range cells {
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.pattern.values[%d]", in.rawSlide(), k),
			SemanticPath: fmt.Sprintf("%s.%s[%d]", in.semSlide(), srcField, k),
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileKPIFallback renders the metrics as "Big — Small" bullets on a content
// slide when the count is unsupported by the kpi-Nup family.
func compileKPIFallback(in Input, cells []kpiCell, srcField string) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	if len(cells) > 0 {
		bullets := make([]string, len(cells))
		for i, c := range cells {
			bullets[i] = fmt.Sprintf("%s — %s", c.Big, c.Small)
		}
		idx := appendContent(slide, bulletsContent("body", bullets))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + "." + srcField,
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// kpiCells extracts the KPI cells from the payload's "kpis" (or "metrics")
// list, accepting object forms {value,label} or {big,small}. The returned
// string is the semantic field name the cells came from.
func kpiCells(body map[string]any) ([]kpiCell, string) {
	field := "kpis"
	objs := mapList(body, field)
	if objs == nil {
		if alt := mapList(body, "metrics"); alt != nil {
			objs, field = alt, "metrics"
		}
	}
	out := make([]kpiCell, 0, len(objs))
	for _, o := range objs {
		big := firstNonEmpty(strField(o, "big"), strField(o, "value"))
		small := firstNonEmpty(strField(o, "small"), strField(o, "label"), strField(o, "caption"))
		if big == "" && small == "" {
			continue
		}
		out = append(out, kpiCell{Big: big, Small: small})
	}
	return out, field
}
