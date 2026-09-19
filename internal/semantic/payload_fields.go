package semantic

// This file is the closed per-kind payload-field contract: every key each slide
// kind's compiler actually reads (canonical names plus the aliases it accepts),
// including the keys of list-entry objects and of the chart object. It backs
// two surfaces so they cannot drift:
//
//   - Schema(): each Slide_<kind> variant lists exactly these properties with
//     additionalProperties:false (and closed item schemas), so the MCP input
//     schema for render_deck_spec / validate_deck_spec tells an agent the real
//     payload shape instead of an open object.
//   - validateUnknownFields: a payload key outside the contract would be dropped
//     silently by the compiler, so validation reports it as SEMANTIC_UNKNOWN_FIELD
//     (a warning; an error under strict) naming the dropped path, with a
//     did-you-mean rename when a known key is close.

import (
	"fmt"
	"sort"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// payloadField describes one payload key a kind's compiler reads.
type payloadField struct {
	// typ is the JSON type the compiler reads ("string", "array", "object").
	typ string
	// desc is a short authoring hint for the schema.
	desc string
	// itemStrings reports whether array entries may be plain strings.
	itemStrings bool
	// itemKeys lists the keys the compiler reads from object array entries.
	// Nil means object entries are not read (only strings are).
	itemKeys []string
	// objectKeys lists the keys the compiler reads from an object-valued field.
	// Nil for an object field means the object is open (validated elsewhere).
	objectKeys []string
}

// objectTextKeys are the keys stringList reads from an object entry in a
// string-list field (label + detail, rendered as "label — detail").
var objectTextKeys = []string{
	"title", "label", "name", "heading", "step", "phase", "text",
	"description", "detail", "summary", "caption", "body",
}

// kpiItemKeys are the keys kpiCells reads from a KPI entry.
var kpiItemKeys = []string{"value", "big", "label", "small", "caption", "delta", "sub", "trend", "change"}

// comparisonColumnKeys are the keys a comparison column object is read by.
var comparisonColumnKeys = []string{"header", "title", "label", "name", "items", "pros", "cons"}

// processStepKeys are the keys processSteps reads from a step object.
var processStepKeys = []string{"label", "title", "name", "step", "text", "description", "detail", "summary", "type"}

// roadmapPhaseKeys are the keys roadmapPhases reads from a phase object.
var roadmapPhaseKeys = []string{
	"name", "title", "label", "phase", "date_label", "dates", "date", "period",
	"description", "detail", "summary", "items", "bullets", "active", "milestone",
}

// chartObjectKeys are the keys chartSpec reads from the chart object.
var chartObjectKeys = []string{"type", "title", "data"}

func strField(desc string) payloadField { return payloadField{typ: "string", desc: desc} }

func textList(desc string) payloadField {
	return payloadField{typ: "array", desc: desc, itemStrings: true, itemKeys: objectTextKeys}
}

// compositionFields are the optional per-slide composition overrides the
// planner reads for kinds that offer alternatives (see compositionCandidates).
func compositionFields() map[string]payloadField {
	return map[string]payloadField{
		"pattern": strField("Optional composition override: one of this kind's alternative patterns (see explain_deck_spec visual.alternatives)."),
		"layout":  strField("Optional composition override: one of this kind's alternative layouts (see explain_deck_spec visual.alternatives)."),
	}
}

// universalFields are accepted on EVERY kind: a speaker-notes block and a
// source/footnote line. They are per-slide strings with no layout impact, and
// before go-slide-creator-zmjs only chart_insight could carry a source, so an
// option matrix or financial case had nowhere to cite its numbers.
func universalFields() map[string]payloadField {
	return map[string]payloadField{
		"notes":  strField("Speaker notes for this slide (rendered into the PPTX notes slide, never shown on the slide)."),
		"source": strField("Source / footnote line shown under the slide content."),
	}
}

func withFields(base map[string]payloadField, extra map[string]payloadField) map[string]payloadField {
	for k, v := range extra {
		base[k] = v
	}
	return base
}

// kindPayloadFields is the closed payload contract per kind.
var kindPayloadFields = map[SlideKind]map[string]payloadField{
	KindTitle: withFields(map[string]payloadField{
		"title":    strField("Headline."),
		"subtitle": strField("Subtitle line."),
		"eyebrow":  strField("Small kicker text above the title."),
	}, universalFields()),
	KindSection: withFields(map[string]payloadField{
		"title":    strField("Section name."),
		"subtitle": strField("Optional subtitle (not rendered on shipped section layouts)."),
	}, universalFields()),
	KindExecutiveSummary: withFields(map[string]payloadField{
		"title":     strField("Slide title."),
		"points":    textList("Body bullets (3–5 recommended)."),
		"takeaways": textList("Alias for points (body bullets)."),
		"takeaway":  strField("One-line takeaway footer."),
	}, universalFields()),
	KindKPISnapshot: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"kpis":     {typ: "array", desc: "2–6 KPI objects {value, label, delta?}.", itemKeys: kpiItemKeys},
		"metrics":  {typ: "array", desc: "Alias for kpis.", itemKeys: kpiItemKeys},
	}, compositionFields()), universalFields()),
	KindChartInsight: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"source":   strField("Data source note."),
		"insight":  strField("Single insight (alias for a one-item insights list)."),
		"insights": textList("1–6 insight bullets rendered beside the chart."),
		"chart": {typ: "object", desc: "Chart: {type, title?, data}. For bar/line/area charts data is {categories:[…], series:[{name, values:[…]}]}; for pie/donut {categories:[…], values:[…]}.",
			objectKeys: chartObjectKeys},
	}, compositionFields()), universalFields()),
	KindComparison: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"columns":  {typ: "array", desc: "Exactly 2 balanced columns {header, items[]} (or {header, pros[], cons[]}).", itemKeys: comparisonColumnKeys},
	}, compositionFields()), universalFields()),
	KindProcess: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"steps":    {typ: "array", desc: "3–8 steps: strings or {label, description?, type?}.", itemStrings: true, itemKeys: processStepKeys},
	}, compositionFields()), universalFields()),
	KindRoadmap: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"phases":   {typ: "array", desc: "3–6 phases: strings or {name, date_label?, description?, items?, active?, milestone?}.", itemStrings: true, itemKeys: roadmapPhaseKeys},
	}, compositionFields()), universalFields()),
	KindDecision: withFields(map[string]payloadField{
		"title":          strField("Slide title."),
		"takeaway":       strField("One-line takeaway footer."),
		"recommendation": strField("Recommendation lead-in paragraph."),
		"options":        textList("Options: strings or {label}."),
	}, universalFields()),
	KindClosing: withFields(map[string]payloadField{
		"title":    strField("Closing headline."),
		"subtitle": strField("Subtitle line."),
		"bullets":  textList("Optional closing bullets (renders a content slide)."),
		"points":   textList("Alias for bullets."),
	}, universalFields()),
	KindRawJSON2pptx: withFields(map[string]payloadField{
		"slide": {typ: "object", desc: "A raw json2pptx slide object (validated strictly as PresentationInput.slides[])."},
	}, universalFields()),
}

// PayloadFieldNames returns the sorted payload keys a kind's compiler reads
// (excluding the "kind" discriminator), or nil for an unknown kind.
func PayloadFieldNames(k SlideKind) []string {
	fields, ok := kindPayloadFields[k]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(fields))
	for name := range fields {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// validateUnknownFields reports payload keys (and list-entry / chart keys) the
// kind's compiler never reads. Without it such content vanishes silently.
func validateUnknownFields(path string, slide SlideSpec, s *semDiags) {
	fields, ok := kindPayloadFields[slide.Kind]
	if !ok {
		return
	}
	known := PayloadFieldNames(slide.Kind)
	for _, key := range sortedKeys(slide.Body) {
		f, ok := fields[key]
		if !ok {
			s.unknownField(path+"."+key, key, known,
				fmt.Sprintf("%s slide", slide.Kind))
			continue
		}
		v := slide.Body[key]
		switch {
		case f.typ == "array" && len(f.itemKeys) > 0:
			list, _ := v.([]any)
			for i, e := range list {
				m, isMap := e.(map[string]any)
				if !isMap {
					continue
				}
				for _, ik := range sortedKeys(m) {
					if !containsKey(f.itemKeys, ik) {
						s.unknownField(fmt.Sprintf("%s.%s[%d].%s", path, key, i, ik), ik, sortedCopy(f.itemKeys),
							fmt.Sprintf("%s.%s entry", slide.Kind, key))
					}
				}
			}
		case f.typ == "object" && len(f.objectKeys) > 0:
			m, _ := v.(map[string]any)
			for _, ck := range sortedKeys(m) {
				if !containsKey(f.objectKeys, ck) {
					s.unknownField(path+"."+key+"."+ck, ck, sortedCopy(f.objectKeys),
						fmt.Sprintf("%s.%s object", slide.Kind, key))
				}
			}
		}
	}
}

// unknownField appends a SEMANTIC_UNKNOWN_FIELD finding for a key the compiler
// drops. It is never suppressed (dropping content silently is exactly what it
// guards against): a warning under off/warn, an error under strict. A close
// known key yields a rename_field fix with did_you_mean.
func (s *semDiags) unknownField(path, key string, known []string, where string) {
	sev := diagnostics.SeverityWarning
	if s.strict == StrictnessStrict {
		sev = diagnostics.SeverityError
	}
	d := diagnostics.Diagnostic{
		Code:     diagnostics.CodeSemanticUnknownField,
		Path:     path,
		Severity: sev,
		Message: fmt.Sprintf("unknown field %q on %s is ignored by the compiler and its content is DROPPED; expected one of %s",
			key, where, joinQuoted(known)),
	}
	if sug := closestKey(key, known); sug != "" {
		d.Message = fmt.Sprintf("unknown field %q on %s is ignored by the compiler and its content is DROPPED; did you mean %q?", key, where, sug)
		d.Fix = &diagnostics.Fix{Kind: "rename_field", Params: map[string]any{"from": key, "to": sug, "did_you_mean": sug}}
	}
	s.out = append(s.out, d)
}

// closestKey returns the known key nearest to key within a small edit
// distance, or "" when nothing is close.
func closestKey(key string, known []string) string {
	best, bestDist := "", -1
	for _, k := range known {
		d := editDistance(key, k)
		if bestDist < 0 || d < bestDist {
			best, bestDist = k, d
		}
	}
	limit := 2
	if len(key) >= 8 {
		limit = 3
	}
	if bestDist >= 0 && bestDist <= limit {
		return best
	}
	return ""
}

func containsKey(list []string, k string) bool {
	for _, v := range list {
		if v == k {
			return true
		}
	}
	return false
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func joinQuoted(keys []string) string {
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%q", k)
	}
	return out
}
