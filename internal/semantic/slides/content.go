package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// execSummaryPoint is the raw exec-summary row shape: a bold lead-in
// conclusion and an optional supporting sentence.
type execSummaryPoint struct {
	Lead    string `json:"lead"`
	Support string `json:"support,omitempty"`
}

// execSummaryValues is the exec-summary pattern's values payload.
type execSummaryValues struct {
	Points     []execSummaryPoint `json:"points"`
	BottomLine string             `json:"bottom_line,omitempty"`
}

// CompileExecutiveSummary compiles an executive-summary slide. With 3–5 points
// it emits the exec-summary pattern — numbered bold conclusions, each with its
// supporting sentence, separated by rules, over an optional bottom-line bar.
// Outside that range, or when a point is too long for the pattern's budgets, it
// falls back to a content slide listing the points as bullets
// (go-slide-creator-ku6t: the kind ALWAYS compiled to bullets, so the pattern
// shipped in round 1 was unreachable from the recommended authoring path).
func CompileExecutiveSummary(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	points, srcField := execSummaryPoints(in.Body)
	values, ok := execSummaryPatternValues(in.Body, points)
	if !ok {
		return compileExecutiveSummaryFallback(in)
	}
	values.BottomLine = FoldConclusion(values.BottomLine, firstNonEmpty(in.Takeaway, strField(in.Body, "takeaway")))

	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal exec-summary values: %w", err)
	}
	if deckinput.ValidatePattern(&deckinput.PatternInput{Name: execSummaryPattern, Values: encoded}, patterns.Default()) != nil {
		// A point can be valid semantically yet over the pattern's char budget.
		// Bullets always validate, so degrade rather than emit JSON the renderer
		// will refuse. ExecSummaryPatternFeasible applies the same test, so
		// validation warns about the degradation before it happens.
		return compileExecutiveSummaryFallback(in)
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

	slide.Pattern = &deckinput.PatternInput{Name: execSummaryPattern, Values: encoded}
	for i := range values.Points {
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.pattern.values.points[%d]", in.rawSlide(), i),
			SemanticPath: fmt.Sprintf("%s.%s[%d]", in.semSlide(), srcField, i),
		})
	}
	if values.BottomLine != "" {
		field := "bottom_line"
		if strField(in.Body, field) == "" {
			if strField(in.Body, "recommendation") != "" {
				field = "recommendation"
			} else {
				field = "takeaway"
			}
		}
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".pattern.values.bottom_line",
			SemanticPath: in.semSlide() + "." + field,
		})
	}

	// The pattern's bottom-line element is already the conclusion band. A
	// separate generator takeaway would put a second band directly below it.
	return slide, links, nil
}

// execSummaryPattern is the named pattern an executive summary renders as.
const execSummaryPattern = "exec-summary"

// execSummaryMinPoints / execSummaryMaxPoints mirror the pattern's own bounds.
// Outside them there is no exec-summary to compile to.
const (
	execSummaryMinPoints = 3
	execSummaryMaxPoints = 5
)

// ExecSummaryPatternFeasible reports whether an executive-summary payload will
// compile to the exec-summary pattern rather than degrade to bullets. The
// explain projection and the validation advisory both consult it, so an agent
// is told which visual it is getting BEFORE the render.
func ExecSummaryPatternFeasible(body map[string]any) bool {
	points, _ := execSummaryPoints(body)
	values, ok := execSummaryPatternValues(body, points)
	if !ok {
		return false
	}
	values.BottomLine = FoldConclusion(values.BottomLine, strField(body, "takeaway"))
	encoded, err := json.Marshal(values)
	if err != nil {
		return false
	}
	return deckinput.ValidatePattern(&deckinput.PatternInput{Name: execSummaryPattern, Values: encoded}, patterns.Default()) == nil
}

// execSummaryPatternValues builds the pattern payload, reporting false when the
// point count is outside the pattern's 3–5 range.
func execSummaryPatternValues(body map[string]any, points []execSummaryPoint) (execSummaryValues, bool) {
	if len(points) < execSummaryMinPoints || len(points) > execSummaryMaxPoints {
		return execSummaryValues{}, false
	}
	return execSummaryValues{
		Points:     points,
		BottomLine: firstNonEmpty(strField(body, "bottom_line"), strField(body, "recommendation")),
	}, true
}

// execSummaryPoints extracts the summary points in either authored shape: a
// plain string (the whole conclusion, no supporting sentence) or an object
// carrying the conclusion and its evidence. It returns the payload field the
// points came from so source links address what the author wrote.
func execSummaryPoints(body map[string]any) ([]execSummaryPoint, string) {
	field := execSummaryPointsField(body)
	if field == "" {
		return nil, ""
	}
	raw, _ := body[field].([]any)
	out := make([]execSummaryPoint, 0, len(raw))
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				out = append(out, execSummaryPoint{Lead: s})
			}
		case map[string]any:
			lead := firstNonEmpty(
				strField(t, "lead"), strField(t, "point"), strField(t, "statement"),
				strField(t, "title"), strField(t, "headline"), strField(t, "text"),
			)
			support := firstNonEmpty(
				strField(t, "support"), strField(t, "detail"), strField(t, "description"),
				strField(t, "evidence"), strField(t, "body"),
			)
			if lead == "" {
				// No conclusion, but the evidence line alone still says something.
				lead, support = support, ""
			}
			if lead != "" {
				out = append(out, execSummaryPoint{Lead: lead, Support: support})
			}
		}
	}
	return out, field
}

// execSummaryPointsField names the payload field the points live in. "points"
// is canonical; "takeaways" (the plural array, distinct from the singular
// "takeaway" one-liner) is the common field alias.
func execSummaryPointsField(body map[string]any) string {
	for _, field := range []string{"points", "takeaways"} {
		if raw, ok := body[field].([]any); ok && len(raw) > 0 {
			return field
		}
	}
	return ""
}

// compileExecutiveSummaryFallback compiles an executive summary as a content
// slide: a title placeholder, a body of the summary points as bullets, and the
// one-line takeaway carried in the slide's Takeaway field (rendered above the
// source note by the generator). It is the shape every executive summary used
// to get, and still the shape for a point count the pattern cannot hold.
func compileExecutiveSummaryFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	// Body bullets may be authored as "points" or, more commonly in the field,
	// as "takeaways" (the plural array, distinct from the singular "takeaway"
	// one-liner that becomes the slide footer). Prefer "points" for back-compat;
	// fall back to "takeaways" so the body content is never silently dropped.
	if bulletField, bullets := executiveSummaryBullets(in.Body); len(bullets) > 0 {
		// A bottom_line is the deck's ask. In the pattern it is a tinted bar; in
		// the fallback it rides as the last bullet rather than vanishing.
		if bottom := firstNonEmpty(strField(in.Body, "bottom_line"), strField(in.Body, "recommendation")); bottom != "" {
			bullets = append(append([]string(nil), bullets...), bottom)
		}
		idx := appendContent(slide, bulletsContent("body", bullets))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + "." + bulletField,
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// executiveSummaryBullets selects the body-bullets source for an executive
// summary slide. It prefers the canonical "points" field, then falls back to
// the "takeaways" plural array. The returned field name is "" when neither is
// present so callers can skip the bullets block.
func executiveSummaryBullets(body map[string]any) (string, []string) {
	// Points authored in the object form {lead, support} render as
	// "Lead — support" rather than being dropped by the string extractor.
	if points, field := execSummaryPoints(body); len(points) > 0 {
		out := make([]string, 0, len(points))
		for _, p := range points {
			if p.Support != "" {
				out = append(out, p.Lead+" — "+p.Support)
				continue
			}
			out = append(out, p.Lead)
		}
		return field, out
	}
	if points, ok := stringList(body, "points"); ok && len(points) > 0 {
		return "points", points
	}
	if takeaways, ok := stringList(body, "takeaways"); ok && len(takeaways) > 0 {
		return "takeaways", takeaways
	}
	return "", nil
}

// CompileFallback is the best-effort compiler for content-bearing kinds the MVP
// does not yet model with a bespoke layout (comparison, process, roadmap). It
// emits a content slide with the title, any list-shaped payload as bullets, and
// the takeaway, so the deck still compiles and validates.
func CompileFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	if field, bullets := firstListField(in.Body); len(bullets) > 0 {
		idx := appendContent(slide, bulletsContent("body", bullets))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + "." + field,
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// applyTakeaway sets the slide's Takeaway field when present and returns the
// source link for it.
func applyTakeaway(slide *deckinput.SlideInput, in Input) []SourceLink {
	if in.Takeaway == "" {
		return nil
	}
	slide.Takeaway = in.Takeaway
	// The takeaway may have come from the "insight" alias on chart slides; the
	// planner has already resolved that, so map back to the canonical field.
	field := "takeaway"
	if strField(in.Body, "takeaway") == "" && strField(in.Body, "insight") != "" {
		field = "insight"
	}
	return []SourceLink{{
		RawPath:      in.rawSlide() + ".takeaway",
		SemanticPath: in.semSlide() + "." + field,
	}}
}

// firstListField returns the first string-list payload field (in sorted key
// order for determinism) and its entries, skipping known scalar/structural
// fields. It backs the generic fallback compiler.
func firstListField(body map[string]any) (string, []string) {
	for _, k := range sortedKeys(body) {
		switch k {
		case "title", "subtitle", "takeaway", "insight", "eyebrow":
			continue
		}
		if bullets, ok := stringList(body, k); ok && len(bullets) > 0 {
			return k, bullets
		}
	}
	return "", nil
}
