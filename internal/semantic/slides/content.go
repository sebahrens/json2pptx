package slides

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// CompileExecutiveSummary compiles an executive-summary slide as a content
// slide: a title placeholder, a body of the summary points as bullets, and the
// one-line takeaway carried in the slide's Takeaway field (rendered above the
// source note by the generator).
func CompileExecutiveSummary(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	if points, ok := stringList(in.Body, "points"); ok && len(points) > 0 {
		idx := appendContent(slide, bulletsContent("body", points))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".points",
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// CompileDecision compiles a decision slide as a safe content slide: a title,
// the recommendation as a lead-in body paragraph, and the options as supporting
// bullets. It deliberately avoids introducing a new pattern (per the MVP
// contract) — a content slide always validates and renders.
func CompileDecision(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	recommendation := strField(in.Body, "recommendation")
	options, optionsField := decisionOptions(in.Body)

	switch {
	case recommendation != "" && len(options) > 0:
		idx := appendContent(slide, bodyAndBulletsContent("body", recommendation, options))
		links = append(links,
			SourceLink{
				RawPath:      fmt.Sprintf("%s.content[%d].body_and_bullets_value.body", in.rawSlide(), idx),
				SemanticPath: in.semSlide() + ".recommendation",
			},
			SourceLink{
				RawPath:      fmt.Sprintf("%s.content[%d].body_and_bullets_value.bullets", in.rawSlide(), idx),
				SemanticPath: in.semSlide() + "." + optionsField,
			},
		)
	case len(options) > 0:
		idx := appendContent(slide, bulletsContent("body", options))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + "." + optionsField,
		})
	case recommendation != "":
		idx := appendContent(slide, textContent("body", recommendation))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".recommendation",
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
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

// decisionOptions extracts the option labels from a decision payload, accepting
// either a list of strings or a list of {label}/{title} objects. The returned
// string is the semantic field the options came from.
func decisionOptions(body map[string]any) ([]string, string) {
	if opts, ok := stringList(body, "options"); ok && len(opts) > 0 {
		return opts, "options"
	}
	if objs := mapList(body, "options"); len(objs) > 0 {
		out := make([]string, 0, len(objs))
		for _, o := range objs {
			if label := firstNonEmpty(strField(o, "label"), strField(o, "title"), strField(o, "name")); label != "" {
				out = append(out, label)
			}
		}
		if len(out) > 0 {
			return out, "options"
		}
	}
	return nil, "options"
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
