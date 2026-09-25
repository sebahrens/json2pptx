package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// kpiCell is the raw kpi-Nup cell shape: a big number, a short caption, and an
// optional delta/trend annotation.
type kpiCell struct {
	Big   string `json:"big"`
	Small string `json:"small"`
	Sub   string `json:"sub,omitempty"`
}

// CompileKPISnapshot compiles a KPI-snapshot slide. With 2–6 metrics it emits a
// kpi-Nup pattern (the variant selected by metric count) under a title
// placeholder; with a count outside that range — or when a metric is too long
// or dense for the compact cards — it falls back to a content slide listing the
// metrics as bullets, so the slide always validates.
func CompileKPISnapshot(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	cells, srcField := kpiCells(in.Body)

	// One decision, shared with the explain planner and validation: a payload
	// that will not fit the compact cards degrades here, and both of those say
	// so up front rather than promising a visual this function will not emit
	// (go-slide-creator-5ok4).
	patternName, _ := kpiPatternPlan(in.Body)
	if patternName == "" {
		return compileKPIFallback(in, cells, srcField)
	}

	values, err := json.Marshal(cells)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal kpi values: %w", err)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	slide.Pattern = &deckinput.PatternInput{
		Name:   patternName,
		Values: values,
	}
	// DeckSpec is a presentation-first surface: use readable hero type instead
	// of the smaller raw-pattern defaults. The pattern's measured fit may still
	// shrink a long value to avoid wrapping in a narrow 5–6-card row.
	slide.Pattern.Overrides, err = json.Marshal(patterns.KPIOverrides{BigSize: 44, SmallSize: 16})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal kpi design defaults: %w", err)
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

// KPIPattern returns the kpi-Nup pattern a payload will actually compile to, or
// "" when it degrades to the bullet fallback. The explain planner advertises
// this rather than the count alone, so plan and result agree.
func KPIPattern(body map[string]any) string {
	name, _ := kpiPatternPlan(body)
	return name
}

// KPIDegradeReason explains why a kpi_snapshot payload will compile to bullets
// instead of kpi-Nup cards, or "" when it compiles to the pattern. Validation
// quotes it so the author learns which value broke which budget before the
// render, rather than from the pixels (go-slide-creator-5ok4).
func KPIDegradeReason(body map[string]any) string {
	_, reason := kpiPatternPlan(body)
	return reason
}

// kpiPatternPlan is the single decision behind compile, explain and validate:
// the pattern a payload compiles to, or the reason it cannot.
func kpiPatternPlan(body map[string]any) (pattern, reason string) {
	cells, _ := kpiCells(body)

	// kpi-Nup is registered only for N in 2..6. Outside that range there is no
	// matching pattern, so the slide degrades to a safe content slide. The count
	// rule reports this case itself, so it needs no reason of its own here.
	if len(cells) < 2 || len(cells) > 6 {
		return "", ""
	}

	values, err := json.Marshal(cells)
	if err != nil {
		return "", "the metrics cannot be encoded as pattern values"
	}
	name := fmt.Sprintf("kpi-%dup", len(cells))

	// A KPI value can be valid semantically yet too long for the compact cards
	// (e.g. "EUR 1,186.42 million" exceeds the big-number budget). Rather than
	// emit raw JSON the renderer will reject, the compiler degrades to the
	// bullet fallback, which always validates. This pre-empts the post-compile
	// raw preflight for the one kind that has a natural fallback (see
	// internal/semantic/preflight.go).
	if err := deckinput.ValidatePattern(&deckinput.PatternInput{Name: name, Values: values}, patterns.Default()); err != nil {
		return "", kpiBudgetReason(name, err)
	}
	return name, ""
}

// kpiBudgetReason turns a pattern validation failure into one readable clause,
// naming the pattern that was lost and the first budget that broke.
func kpiBudgetReason(pattern string, err error) string {
	first := strings.TrimSpace(strings.SplitN(err.Error(), "\n", 2)[0])
	if strings.TrimSpace(strings.TrimPrefix(first, pattern+":")) == "" {
		return fmt.Sprintf("the metrics do not fit %s", pattern)
	}
	return first
}

// compileKPIFallback renders the metrics as "Big — Small" bullets on a content
// slide when the count is unsupported by the kpi-Nup family, or when a metric
// does not fit the compact cards.
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
			if c.Sub != "" {
				bullets[i] = fmt.Sprintf("%s (%s) — %s", c.Big, c.Sub, c.Small)
			} else {
				bullets[i] = fmt.Sprintf("%s — %s", c.Big, c.Small)
			}
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
		sub := firstNonEmpty(strField(o, "sub"), strField(o, "delta"), strField(o, "trend"), strField(o, "change"))
		// delta and trend are usually aliases, but authors may supply both a
		// direction and a magnitude. Keep both rather than silently dropping
		// the direction. If the combined annotation exceeds the pattern's
		// 12-character budget, kpiPatternPlan safely degrades to bullets.
		if strField(o, "sub") == "" {
			delta, trend := strField(o, "delta"), strField(o, "trend")
			if delta != "" && trend != "" && delta != trend {
				switch strings.ToLower(trend) {
				case "up", "rising", "increasing":
					trend = "↑"
				case "down", "falling", "decreasing":
					trend = "↓"
				case "flat", "stable", "unchanged":
					trend = "→"
				}
				if len([]rune(trend)) == 1 {
					sub = trend + " " + delta
				} else {
					sub = trend + " / " + delta
				}
			}
		}
		if big == "" && small == "" {
			continue
		}
		out = append(out, kpiCell{Big: big, Small: small, Sub: sub})
	}
	return out, field
}
