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
	// itemKeySchemas overrides the default string schema for selected entry keys.
	itemKeySchemas map[string]any
	itemRequired   []string
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

// execSummaryPointKeys are the keys execSummaryPoints reads from a point entry:
// the bold conclusion and its supporting sentence, plus the aliases the field
// actually accepts (go-slide-creator-ku6t).
var execSummaryPointKeys = []string{
	"lead", "point", "statement", "title", "headline", "text",
	"support", "detail", "description", "evidence", "body",
}

// kpiItemKeys are the keys kpiCells reads from a KPI entry.
var kpiItemKeys = []string{"value", "big", "label", "small", "caption", "delta", "sub", "trend", "change"}

// optionMatrixCriterionKeys are the keys optionMatrixCriteria reads from a
// criterion object; optionMatrixOptionKeys the keys an option row is read by
// (go-slide-creator-6o1r).
var optionMatrixCriterionKeys = []string{"label", "name", "title", "criterion", "scale"}

var optionMatrixOptionKeys = []string{"name", "option", "label", "title", "detail", "description", "summary", "scores", "values"}

// comparisonColumnKeys are the keys a comparison column object is read by.
var comparisonColumnKeys = []string{"header", "title", "label", "name", "items", "pros", "cons"}

// processStepKeys are the keys processSteps reads from a step object.
var processStepKeys = []string{"label", "title", "name", "step", "text", "description", "detail", "summary", "type"}

// timelineStopKeys are the keys a timeline milestone object may carry.
var timelineStopKeys = []string{
	"label", "title", "name", "milestone", "event",
	"date", "date_label", "when", "start", "start_date",
	"end_date", "end", "until",
	"body", "description", "detail", "summary",
}

// matrixQuadrantKeys are the keys a 2x2 quadrant object may carry.
var matrixQuadrantKeys = []string{
	"header", "title", "label", "name",
	"body", "description", "detail", "summary",
}

// imageCaseMetricKeys are the keys a result-metric object may carry.
var imageCaseMetricKeys = []string{"value", "number", "stat", "label", "caption", "name"}

// imageCaseImageKeys are the keys the image object may carry.
var imageCaseImageKeys = []string{"path", "url", "alt"}

// decisionOptionKeys are the keys a decision option object may carry.
var decisionOptionKeys = []string{
	"label", "title", "name", "option",
	"detail", "description", "body", "summary",
}

// archTierKeys are the keys ArchitectureTiers reads from a tier object.
// teamMemberKeys are the keys a team member object may carry.
var teamMemberKeys = []string{
	"name", "title", "person",
	"role", "position", "job_title",
	"bio", "description", "summary",
	"photo_label", "initials",
}

// agendaSectionKeys are the keys an agenda section object may carry.
var agendaSectionKeys = []string{
	"title", "label", "name", "section",
	"subtitle", "description", "detail",
}

var quoteItemKeys = []string{"text", "quote", "name", "attribution", "speaker", "author", "role", "title"}
var bridgeColumnKeys = []string{"label", "value", "type"}
var pillarItemKeys = []string{"title", "body"}

var archTierKeys = []string{
	"label", "name", "title", "tier", "layer",
	"description", "detail", "summary", "text",
	"items", "components", "services", "elements",
}

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
		"pattern": strField("Optional composition override: one of this kind's alternative patterns (list_slide_kinds → compositions[], or explain_deck_spec visual.alternatives). Anything else is ignored and reported as SEMANTIC_PATTERN_NOT_AVAILABLE."),
		"layout":  strField("Optional composition override: one of this kind's alternative layouts (list_slide_kinds → compositions[], or explain_deck_spec visual.alternatives). Anything else is ignored and reported as SEMANTIC_PATTERN_NOT_AVAILABLE."),
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
		"title": strField("Slide title."),
		"points": {
			typ: "array", itemStrings: true, itemKeys: execSummaryPointKeys,
			desc: "3–5 key messages, most important first. A string is the conclusion alone; {lead, support} adds the evidence sentence under it. Outside 3–5 the slide degrades to bullets.",
		},
		"takeaways": {
			typ: "array", itemStrings: true, itemKeys: execSummaryPointKeys,
			desc: "Alias for points.",
		},
		"bottom_line": strField("Optional recommendation / ask rendered as a tinted bar under the points."),
		"takeaway":    strField("One-line takeaway footer."),
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
	KindOptionMatrix: withFields(withFields(map[string]payloadField{
		"title": strField("Slide title."),
		"criteria": {
			typ: "array", itemStrings: true, itemKeys: optionMatrixCriterionKeys,
			desc: "2–6 evaluation criteria (table columns): a string label, or {label, scale?} to override the scale for that column.",
		},
		"columns": {
			typ: "array", itemStrings: true, itemKeys: optionMatrixCriterionKeys,
			desc: "Alias for criteria.",
		},
		"options": {
			typ: "array", itemKeys: optionMatrixOptionKeys,
			desc: "2–6 options (table rows): {name, detail?, scores[]} with exactly one score per criterion.",
		},
		"rows": {
			typ: "array", itemKeys: optionMatrixOptionKeys,
			desc: "Alias for options.",
		},
		"scale":              strField("Score vocabulary: harvey (0–4 or none/quarter/half/three-quarter/full, the default), rag (red/amber/green), or text (≤24 chars). Use \"-\" for n/a."),
		"recommended":        strField("The recommended option, named (matched against options[].name) or as a row index — its row is highlighted."),
		"recommended_option": strField("Alias for recommended."),
		"decisive_criterion": strField("The criterion that decides it, named (matched against criteria[].label) or as a column index — its column is highlighted."),
		"highlight_label":    strField("Badge on the highlighted row (e.g. \"Recommended\")."),
		"corner_label":       strField("Label for the table's empty top-left corner cell."),
		"takeaway":           strField("One-line takeaway footer."),
	}, compositionFields()), universalFields()),
	KindTable: withFields(withFields(map[string]payloadField{
		"title":   strField("Slide title."),
		"headers": textList("Column headers (up to 6). The table's first row."),
		"columns": textList("Alias for headers."),
		"rows": {
			typ:  "array",
			desc: "Data rows (up to 6, so the table stays within the 7-row budget including the header). A row is a list of cell values, or an object keyed by header label. Short rows render blank cells.",
		},
		"column_alignments": textList("Per-column alignment: left / center / right (the engine's l / ctr / r are accepted too). Right-align numeric columns."),
		"column_types":      textList("Per-column type hint for the renderer's number formatting, e.g. text / number / currency / percent."),
		"highlight_column":  strField("The column to emphasise, named (matched against a header) or as a 0-based index."),
		"totals_row":        strField("Set true when the last data row is a totals row, so the renderer emphasises it."),
		"takeaway":          strField("One-line takeaway footer."),
	}, compositionFields()), universalFields()),
	KindArchitecture: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"tiers": {
			typ:         "array",
			desc:        "3–6 tiers, top layer first: strings or {label, description?|items?}; items are joined into the tier's detail line. Label ≤60 chars, detail ≤120.",
			itemStrings: true,
			itemKeys:    archTierKeys,
		},
		"layers":     {typ: "array", desc: "Alias for tiers.", itemStrings: true, itemKeys: archTierKeys},
		"rails":      textList("Up to 3 cross-cutting concerns drawn as rails beside the stack, ≤30 chars each."),
		"side_rails": textList("Alias for rails."),
	}, compositionFields()), universalFields()),
	KindAgenda: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title (defaults to the template's own)."),
		"takeaway": strField("One-line takeaway footer."),
		"sections": {
			typ:         "array",
			desc:        "The deck's sections in order: strings, or {title, subtitle?}. A subtitle on any section switches the slide to the agenda-with-images rows (3–6 sections); plain sections render as the numbered list (2–10). Title ≤80 chars, subtitle ≤160.",
			itemStrings: true,
			itemKeys:    agendaSectionKeys,
		},
		"items":           {typ: "array", desc: "Alias for sections.", itemStrings: true, itemKeys: agendaSectionKeys},
		"agenda":          {typ: "array", desc: "Alias for sections.", itemStrings: true, itemKeys: agendaSectionKeys},
		"current":         {typ: "string", desc: "The section the deck is at: its 1-based position or its title. Highlights that row."},
		"current_section": {typ: "string", desc: "Alias for current."},
		"highlight":       {typ: "string", desc: "Alias for current."},
		"active":          {typ: "string", desc: "Alias for current."},
	}, compositionFields()), universalFields()),
	KindQuote: withFields(withFields(map[string]payloadField{
		"title":        strField("Slide title."),
		"takeaway":     strField("One-line takeaway footer."),
		"quotes":       {typ: "array", itemStrings: true, itemKeys: quoteItemKeys, desc: "3–8 attributed quotes for quote-cluster, or one for pull-quote. Each item is {text, name, role?}; un-attributed items use a content fallback."},
		"testimonials": {typ: "array", itemStrings: true, itemKeys: quoteItemKeys, desc: "Alias for quotes."},
		"voices":       {typ: "array", itemStrings: true, itemKeys: quoteItemKeys, desc: "Alias for quotes."},
		"quote":        strField("One-quote shorthand: quotation text."),
		"text":         strField("Alias for quote."),
		"attribution":  strField("Speaker name for the one-quote shorthand."),
		"name":         strField("Alias for attribution."),
		"speaker":      strField("Alias for attribution."),
		"author":       strField("Alias for attribution."),
		"role":         strField("Speaker role for the one-quote shorthand."),
	}, compositionFields()), universalFields()),
	KindBridge: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"columns": {typ: "array", desc: "3–10 ordered {label, type, value?} bars. type is total, delta, or subtotal; subtotal may omit value to use the running total.", itemKeys: bridgeColumnKeys, itemRequired: []string{"label", "type"}, itemKeySchemas: map[string]any{
			"value": map[string]any{"type": "number", "minimum": -1e12, "maximum": 1e12},
			"type":  map[string]any{"type": "string", "enum": []any{"total", "delta", "subtotal"}},
		}},
		"unit":    strField("Short value-label unit, at most 8 characters (for example $m)."),
		"caption": strField("Scale note above the bars, at most 60 characters."),
	}, compositionFields()), universalFields()),
	KindPillars: withFields(withFields(map[string]payloadField{
		"title":       strField("Slide title."),
		"takeaway":    strField("One-line takeaway footer."),
		"pillars":     {typ: "array", desc: "3–5 named pillars; each {title, body?}. Body is a list of short bullet strings.", itemKeys: pillarItemKeys, itemRequired: []string{"title"}, itemKeySchemas: map[string]any{"body": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}},
		"objective":   strField("Strategy-house objective banner; supply with foundation."),
		"foundation":  strField("Strategy-house foundation row; supply with objective."),
		"roof_badges": {typ: "array", itemStrings: true, desc: "Up to three short string badges above a strategy house."},
	}, compositionFields()), universalFields()),
	KindTeam: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"members": {
			typ:         "array",
			desc:        "1–8 people: strings (a bare name) or {name, role, bio?, photo_label?}. Every card needs a role. Name ≤60 chars, role ≤80, bio ≤220 (~2 lines); photo_label is the initials badge, ≤8 chars.",
			itemStrings: true,
			itemKeys:    teamMemberKeys,
		},
		"people": {typ: "array", desc: "Alias for members.", itemStrings: true, itemKeys: teamMemberKeys},
		"team":   {typ: "array", desc: "Alias for members.", itemStrings: true, itemKeys: teamMemberKeys},
	}, compositionFields()), universalFields()),
	KindStat: withFields(withFields(map[string]payloadField{
		"title":       strField("Slide title."),
		"takeaway":    strField("One-line takeaway footer."),
		"value":       strField("The number itself, formatted as it should read (e.g. \"$2.4B\", \"118%\"). ≤20 chars."),
		"stat":        strField("Alias for value."),
		"number":      strField("Alias for value."),
		"metric":      strField("Alias for value."),
		"label":       strField("The words beneath the number — what it measures. ≤80 chars; defaults to the slide title."),
		"caption":     strField("Alias for label."),
		"subtitle":    strField("Alias for label."),
		"unit":        strField("Short suffix beside the number (e.g. \"TAM\", \"MRR\"). ≤10 chars."),
		"suffix":      strField("Alias for unit."),
		"context":     strField("One line of context beneath the label. ≤120 chars."),
		"detail":      strField("Alias for context."),
		"description": strField("Alias for context."),
		"source":      strField("Source footnote. ≤80 chars."),
	}, compositionFields()), universalFields()),
	KindTimeline: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"milestones": {
			typ:         "array",
			desc:        "3–7 milestones: strings (a bare label) or {label, date?, end_date?, body?}. Label ≤60 chars, date ≤30, body ≤200. A milestone with an end_date spans a period, which draws the whole line as range bars.",
			itemStrings: true,
			itemKeys:    timelineStopKeys,
		},
		"stops":    {typ: "array", desc: "Alias for milestones.", itemStrings: true, itemKeys: timelineStopKeys},
		"events":   {typ: "array", desc: "Alias for milestones.", itemStrings: true, itemKeys: timelineStopKeys},
		"timeline": {typ: "array", desc: "Alias for milestones.", itemStrings: true, itemKeys: timelineStopKeys},
	}, compositionFields()), universalFields()),
	KindMatrix2x2: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"quadrants": {
			typ:         "array",
			desc:        "Exactly 4 quadrants, clockwise from the top left: strings (\"Header\" or \"Header: body\") or {header, body?}. Header ≤80 chars, body ≤200.",
			itemStrings: true,
			itemKeys:    matrixQuadrantKeys,
		},
		"cells":        {typ: "array", desc: "Alias for quadrants.", itemStrings: true, itemKeys: matrixQuadrantKeys},
		"boxes":        {typ: "array", desc: "Alias for quadrants.", itemStrings: true, itemKeys: matrixQuadrantKeys},
		"top_left":     {typ: "object", desc: "One quadrant by position, instead of the quadrants list.", itemKeys: matrixQuadrantKeys},
		"top_right":    {typ: "object", desc: "One quadrant by position.", itemKeys: matrixQuadrantKeys},
		"bottom_left":  {typ: "object", desc: "One quadrant by position.", itemKeys: matrixQuadrantKeys},
		"bottom_right": {typ: "object", desc: "One quadrant by position.", itemKeys: matrixQuadrantKeys},
		"x_axis":       strField("The horizontal axis's name (e.g. \"Effort\"). ≤60 chars."),
		"x_axis_label": strField("Alias for x_axis."),
		"y_axis":       strField("The vertical axis's name (e.g. \"Impact\"). ≤60 chars."),
		"y_axis_label": strField("Alias for y_axis."),
		"x_low":        strField("Label for the left end of the horizontal axis (default \"Low\"). ≤20 chars."),
		"x_high":       strField("Label for the right end of the horizontal axis (default \"High\"). ≤20 chars."),
		"y_low":        strField("Label for the bottom end of the vertical axis (default \"Low\"). ≤20 chars."),
		"y_high":       strField("Label for the top end of the vertical axis (default \"High\"). ≤20 chars."),
	}, compositionFields()), universalFields()),
	KindFramework: withFields(withFields(map[string]payloadField{
		"title":     strField("Slide title."),
		"takeaway":  strField("One-line takeaway footer."),
		"framework": strField("Which framework: \"swot\", \"porters_five_forces\" or \"bmc\"."),
		"type":      strField("Alias for framework."),
		"model":     strField("Alias for framework."),
		"sections": {
			typ: "object",
			desc: "The framework's parts, keyed by name, each a list of short items (≤10 per part, ≤200 chars each). " +
				"swot: strengths, weaknesses, opportunities, threats. " +
				"porters_five_forces: rivalry, new_entrants, substitutes, suppliers, buyers. " +
				"bmc: key_partners, key_activities, key_resources, value_propositions, customer_relations, channels, customer_segments, cost_structure, revenue_streams. " +
				"Every part is required; a framework missing one degrades to grouped bullets.",
		},
	}, compositionFields()), universalFields()),
	KindImageCase: withFields(withFields(map[string]payloadField{
		"title":       strField("Slide title."),
		"takeaway":    strField("One-line takeaway footer."),
		"image":       {typ: "object", desc: "The picture: a path or url string, or {path|url, alt}. Omit it to draw a labelled placeholder.", itemKeys: imageCaseImageKeys},
		"eyebrow":     strField("Small kicker above the heading (e.g. \"Case study\"). ≤30 chars."),
		"heading":     strField("The story's headline. ≤80 chars."),
		"body":        strField("The story itself. ≤300 chars; give this or at least one bullet."),
		"text":        strField("Alias for body."),
		"story":       strField("Alias for body."),
		"description": strField("Alias for body."),
		"bullets":     {typ: "array", desc: "Up to 5 supporting points, ≤140 chars each.", itemStrings: true},
		"metrics":     {typ: "array", desc: "Up to 3 result figures: {value (≤10 chars), label (≤40)}. Both are required — a figure with no words is half a claim.", itemKeys: imageCaseMetricKeys},
		"caption":     strField("Italic line under the picture. ≤120 chars."),
		"image_side":  strField("Which side the picture sits on: \"left\" (default) or \"right\"."),
		"image_label": strField("Label for the placeholder when no picture is given. ≤40 chars."),
	}, compositionFields()), universalFields()),
	KindProcess: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"steps": {
			typ: "array",
			desc: "Steps: strings or {label, description?, type?}. A step with a description renders as a numbered row (3–6 steps; label ≤60 chars, description ≤180); " +
				"bare labels and branching steps (type: decision) render as the flow diagram (3–8 steps, ≤80 chars per box — a description there is appended to the label).",
			itemStrings: true,
			itemKeys:    processStepKeys,
		},
	}, compositionFields()), universalFields()),
	KindRoadmap: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"phases":   {typ: "array", desc: "3–6 phases: strings or {name, date_label?, description?, items?, active?, milestone?}.", itemStrings: true, itemKeys: roadmapPhaseKeys},
	}, compositionFields()), universalFields()),
	KindDecision: withFields(withFields(map[string]payloadField{
		"title":          strField("Slide title."),
		"takeaway":       strField("One-line takeaway footer."),
		"recommendation": strField("The ask, rendered in the callout band beneath the options (or as the lead-in paragraph on the content fallback)."),
		"options": {
			typ: "array",
			desc: "2–6 options: strings (\"Label\", or \"Label | detail\") or {label, detail?}. " +
				"3–6 become numbered boxes (label ≤60 chars, detail ≤180); exactly 2, each WITH a detail, become numbered cards side by side (label ≤80, detail ≤300). " +
				"Anything else renders as the recommendation plus option bullets.",
			itemStrings: true,
			itemKeys:    decisionOptionKeys,
		},
		"choices":      {typ: "array", desc: "Alias for options.", itemStrings: true, itemKeys: decisionOptionKeys},
		"alternatives": {typ: "array", desc: "Alias for options.", itemStrings: true, itemKeys: decisionOptionKeys},
	}, compositionFields()), universalFields()),
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
