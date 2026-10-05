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
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

// payloadField describes one payload key a kind's compiler reads.
type payloadField struct {
	// typ is the JSON type the compiler reads ("string", "array", "object", "boolean").
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
	// enum restricts a string field to these values.
	enum []string
	// schema, when set, builds the field's whole JSON Schema (desc still
	// supplies its description): the regions list is a union of closed region
	// variants the generic item rendering cannot express.
	schema func() map[string]any
}

// objectTextKeys are the keys stringList reads from an object entry in a
// string-list field (label + detail, rendered as "label — detail").
var objectTextKeys = keyGroups(
	[]string{"title", "label", "name", "heading", "step", "phase", "text"},
	[]string{"description", "detail", "summary", "caption", "body"},
)

// execSummaryPointKeys are the keys execSummaryPoints reads from a point entry:
// the bold conclusion and its supporting sentence, plus the aliases the field
// actually accepts (go-slide-creator-ku6t).
var execSummaryPointKeys = keyGroups(
	[]string{"lead", "point", "statement", "title", "headline", "text"},
	[]string{"support", "detail", "description", "evidence", "body"},
)

// kpiItemKeys are the keys kpiCells reads from a KPI entry.
var kpiItemKeys = keyGroups(
	[]string{"value", "big"}, []string{"label", "small", "caption"},
	// trend is not an alias: beside a delta it is appended to it.
	[]string{"delta", "sub", "change"}, []string{"trend"},
	[]string{"comparator", "vs"},
)

// nextStepsActionKeys are the keys nextStepsPayload reads from an action.
var nextStepsActionKeys = keyGroups(
	[]string{"action", "title", "label", "step", "text"}, []string{"owner", "who"}, []string{"date", "due", "when"},
)

// optionMatrixCriterionKeys are the keys optionMatrixCriteria reads from a
// criterion object; optionMatrixOptionKeys the keys an option row is read by
// (go-slide-creator-6o1r).
var optionMatrixCriterionKeys = keyGroups([]string{"label", "name", "title", "criterion"}, []string{"scale"})

var optionMatrixOptionKeys = keyGroups(
	[]string{"name", "option", "label", "title"}, []string{"detail", "description", "summary"}, []string{"scores", "values"},
)

func optionMatrixDetailKeySchemas() map[string]any {
	field := func() map[string]any {
		return map[string]any{"type": "string", "maxLength": 80,
			"description": "Descriptor under the option name: ≤80 chars in a matrix up to 4×4; ≤60 in a denser matrix."}
	}
	return map[string]any{"detail": field(), "description": field(), "summary": field()}
}

// comparisonColumnKeys are the keys a comparison column object is read by.
var comparisonColumnKeys = keyGroups([]string{"header", "title", "label", "name"}, []string{"items"}, []string{"pros"}, []string{"cons"})

// processStepKeys are the keys processSteps reads from a step object.
var processStepKeys = keyGroups(
	[]string{"label", "title", "name", "step", "text"}, []string{"description", "detail", "summary"}, []string{"type"},
)

// timelineStopKeys are the keys a timeline milestone object may carry.
var timelineStopKeys = keyGroups(
	[]string{"label", "title", "name", "milestone", "event"},
	[]string{"date", "date_label", "when", "start", "start_date"},
	[]string{"end_date", "end", "until"},
	[]string{"body", "description", "detail", "summary"},
)

// matrixQuadrantKeys are the keys a 2x2 quadrant object may carry.
var matrixQuadrantKeys = keyGroups(
	[]string{"header", "title", "label", "name"},
	[]string{"body", "description", "detail", "summary"},
)

// imageCaseMetricKeys are the keys a result-metric object may carry.
var imageCaseMetricKeys = keyGroups([]string{"value", "number", "stat"}, []string{"label", "caption", "name"})

// imageCaseCalloutKeys are the keys a callout on the picture may carry.
var imageCaseCalloutKeys = []string{"label", "x", "y", "units"}

// imageCaseImageKeys are the keys the image object may carry.
var imageCaseImageKeys = []string{"path", "url", "alt", "fit"}

// decisionOptionKeys are the keys a decision option object may carry.
var decisionOptionKeys = keyGroups(
	[]string{"label", "title", "name", "option"},
	[]string{"detail", "description", "body", "summary"},
	[]string{"recommended"},
)

// archTierKeys are the keys ArchitectureTiers reads from a tier object.
// teamMemberKeys are the keys a team member object may carry.
var teamMemberKeys = keyGroups(
	[]string{"name", "title", "person"},
	[]string{"role", "position", "job_title"},
	[]string{"bio", "description", "summary"},
	[]string{"photo_label", "initials"}, []string{"photo"},
)

// agendaSectionKeys are the keys an agenda section object may carry.
var agendaSectionKeys = keyGroups(
	[]string{"title", "label", "name", "section"},
	[]string{"subtitle", "description", "detail"},
)

var quoteItemKeys = keyGroups([]string{"text", "quote"}, []string{"name", "attribution", "speaker", "author"}, []string{"role", "title"})
var bridgeColumnKeys = []string{"label", "value", "type"}
var pillarItemKeys = []string{"title", "body"}
var orgNodeKeys = []string{"id", "name", "title", "parent"}

var archTierKeys = keyGroups(
	[]string{"label", "name", "title", "tier", "layer"},
	[]string{"description", "detail", "summary", "text"},
	[]string{"items", "components", "services", "elements"},
)

// roadmapPhaseKeys are the keys roadmapPhases reads from a phase object.
var roadmapPhaseKeys = keyGroups(
	[]string{"name", "title", "label", "phase"}, []string{"date_label", "dates", "date", "period"},
	[]string{"description", "detail", "summary"}, []string{"items", "bullets"}, []string{"active"}, []string{"milestone"},
)

// chartObjectKeys are the keys chartSpec reads from the chart object.
var chartObjectKeys = []string{"type", "title", "data"}

// entryKeyAliases records, per closed entry object, which of its keys are
// aliases: the key set (sorted, comma-joined) maps to alias → canonical. The
// compact item schema lists a canonical key once and names its aliases
// (go-slide-creator-l6mcj).
var entryKeyAliases = map[string]map[string]string{}

// keyGroups declares an entry object's keys as groups — the canonical key
// first, then the aliases the compiler reads for the same value — and returns
// the flat key list the contract and the validator use.
func keyGroups(groups ...[]string) []string {
	var keys []string
	aliases := map[string]string{}
	for _, g := range groups {
		keys = append(keys, g...)
		for _, alias := range g[1:] {
			aliases[alias] = g[0]
		}
	}
	entryKeyAliases[keySetID(keys)] = aliases
	return keys
}

// keySetID identifies a key set regardless of order.
func keySetID(keys []string) string {
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

func strField(desc string) payloadField { return payloadField{typ: "string", desc: desc} }

func textList(desc string) payloadField {
	return payloadField{typ: "array", desc: desc, itemStrings: true, itemKeys: objectTextKeys}
}

// compositionFields are the optional per-slide composition overrides the
// planner reads for kinds that offer alternatives (see compositionCandidates).
func compositionFields() map[string]payloadField {
	return map[string]payloadField{
		"pattern": strField("Optional composition override: one of this kind's alternative patterns (list_slide_kinds fields:[compositions]). Anything else is ignored and reported as SEMANTIC_PATTERN_NOT_AVAILABLE."),
		"layout":  strField("Optional composition override: one of this kind's alternative layouts (list_slide_kinds fields:[compositions]). Anything else is ignored and reported as SEMANTIC_PATTERN_NOT_AVAILABLE."),
	}
}

// universalFields are accepted on EVERY kind: a speaker-notes block and a
// source/footnote line. They are per-slide strings with no layout impact, and
// before go-slide-creator-zmjs only chart_insight could carry a source, so an
// option matrix or financial case had nowhere to cite its numbers.
//
// id is the slide's stable handle (go-slide-creator-1w3uo): it is never
// rendered, and a deck_id patch may use it in place of the slide's index.
func universalFields() map[string]payloadField {
	return map[string]payloadField{
		"id":     strField("Optional stable handle for this slide (a letter, then letters, digits, _ or -; unique in the deck). Never rendered. A stored deck assigns s1, s2, … to slides without one; deck_id patches may address a slide by it (/slides/<id>/title)."),
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
		"title":    strField("Section name. For supporting copy, use a content slide: section layouts reserve body slots for decorative numbering."),
		"appendix": {typ: "boolean", desc: "true marks an appendix / backup divider: no chapter number, and later sections are not renumbered. Dividers titled Appendix, Backup, Annex, Q&A or Thank you are unnumbered automatically."},
	}, universalFields()),
	KindExecutiveSummary: withFields(withFields(map[string]payloadField{
		"title": strField("Slide title."),
		"points": {
			typ: "array", itemStrings: true, itemKeys: execSummaryPointKeys,
			desc: "3–5 key messages, most important first. A string is the conclusion alone; {lead, support} adds the evidence sentence under it (lead ≤90 chars, support ≤200). Outside 3–5 the slide degrades to bullets.",
		},
		"takeaways": {
			typ: "array", itemStrings: true, itemKeys: execSummaryPointKeys,
			desc: "Alias for points.",
		},
		"bottom_line": strField("Optional recommendation / ask rendered as a tinted bar under the points. ≤160 chars."),
		"takeaway":    strField("One-line takeaway footer."),
	}, compositionFields()), universalFields()),
	KindKPISnapshot: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"kpis":     {typ: "array", desc: "2–6 KPI objects {value, label, delta?, comparator?}; value and delta ≤12 chars each (with 5 or 6 KPIs a value holds about 11 or 9 digits on one line on the narrowest template). comparator (alias vs, ≤24 chars) is the reference the number is read against, e.g. \"vs plan +4 pts\".", itemKeys: kpiItemKeys},
		"metrics":  {typ: "array", desc: "Alias for kpis.", itemKeys: kpiItemKeys},
	}, compositionFields()), universalFields()),
	KindChartInsight: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"source":   strField("Data source note."),
		"insight":  strField("Single implication rendered as a so-what callout; when insights bullets are present, keep it distinct from them."),
		"insights": textList("1–6 insight bullets rendered beside the chart; the fit check measures their length against the column."),
		"chart": {typ: "object", desc: "Chart: {type, title?, data}. For bar/line/area charts data is {categories:[…], series:[{name, values:[…]}]}; for pie/donut {categories:[…], values:[…]}; for waterfall {points:[{label, value, type}]}. The value axis starts at zero unless a value is negative; data.y_min / data.y_max zoom it (the axis is then kept visible; a y_min that would cut a bar is rejected).",
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
			typ: "array", itemKeys: optionMatrixOptionKeys, itemKeySchemas: optionMatrixDetailKeySchemas(),
			desc: "2–6 options (table rows): {name, detail?, scores[]} with one score per criterion. detail/description/summary ≤80 chars for up to 4 options × 4 criteria, ≤60 in denser matrices.",
		},
		"rows": {
			typ: "array", itemKeys: optionMatrixOptionKeys, itemKeySchemas: optionMatrixDetailKeySchemas(),
			desc: "Alias for options.",
		},
		"scale": strField("Score vocabulary: harvey (0–4 or none/quarter/half/three-quarter/full, the default), rag (red/amber/green), or text (≤24 chars). Use \"-\" for n/a."),
		"recommended": {typ: "string", schema: recommendedSchema,
			desc: "The recommended option, named (matched against options[].name) or as a 0-based row index — its row is highlighted. A list recommends several options together: each row is highlighted and, with no takeaway, the band reads \"Recommended: A and B\"."},
		"recommended_option": {typ: "string", schema: recommendedSchema, desc: "Alias for recommended."},
		"decisive_criterion": strField("The criterion that decides it, named (matched against criteria[].label) or as a column index — its column is highlighted."),
		"highlight_label":    strField("Badge on the highlighted row (e.g. \"Recommended\"). ≤24 chars."),
		"corner_label":       strField("Label for the table's empty top-left corner cell. ≤24 chars."),
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
		"totals_row":        {typ: "boolean", desc: "Set true when the last data row is a totals row, so the renderer emphasises it."},
		"takeaway":          strField("One-line takeaway footer."),
	}, compositionFields()), universalFields()),
	KindArchitecture: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"tiers": {
			typ:         "array",
			desc:        "3–6 tiers, top layer first: strings or {label, description?|items?}. items (1–12, each ≤40 chars) are drawn as one block each inside the tier band; description is one line of text (≤120 chars). Label ≤60 chars.",
			itemStrings: true,
			itemKeys:    archTierKeys,
		},
		"layers":     {typ: "array", desc: "Alias for tiers.", itemStrings: true, itemKeys: archTierKeys},
		"rails":      textList("Up to 3 cross-cutting concerns drawn as rails beside the stack, ≤30 chars each."),
		"side_rails": textList("Alias for rails."),
	}, compositionFields()), universalFields()),
	KindAgenda: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title (optional; defaults to \"Agenda\")."),
		"takeaway": strField("One-line takeaway footer."),
		"sections": {
			typ:         "array",
			desc:        "The deck's sections in order: strings, or {title, subtitle?}. Subtitles render as a muted line under each title in the numbered list (2–10 sections; title ≤100, subtitle ≤120); a longer subtitle (≤160, 3–6 sections, title ≤80) switches to the agenda-with-images rows.",
			itemStrings: true,
			itemKeys:    agendaSectionKeys,
		},
		"items":           {typ: "array", desc: "Alias for sections.", itemStrings: true, itemKeys: agendaSectionKeys},
		"agenda":          {typ: "array", desc: "Alias for sections.", itemStrings: true, itemKeys: agendaSectionKeys},
		"current":         {typ: "string", schema: agendaCurrentSchema, desc: "The section the deck is at: its 1-based position or its title. Highlights that row."},
		"current_section": {typ: "string", schema: agendaCurrentSchema, desc: "Alias for current."},
		"highlight":       {typ: "string", schema: agendaCurrentSchema, desc: "Alias for current."},
		"active":          {typ: "string", schema: agendaCurrentSchema, desc: "Alias for current."},
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
		"columns": {typ: "array", desc: "3–10 ordered {label, type, value?} bars (label ≤40 chars). type is total, delta, or subtotal; subtotal may omit value to use the running total.", itemKeys: bridgeColumnKeys, itemRequired: []string{"label", "type"}, itemKeySchemas: map[string]any{
			"value": map[string]any{"type": "number", "minimum": -1e12, "maximum": 1e12},
			"type":  map[string]any{"type": "string", "enum": []any{"total", "delta", "subtotal"}},
		}},
		"unit":    strField("Short value-label unit, at most 8 characters (for example $m)."),
		"caption": strField("Scale note above the bars, at most 60 characters."),
	}, compositionFields()), universalFields()),
	KindPillars: withFields(withFields(map[string]payloadField{
		"title":     strField("Slide title."),
		"takeaway":  strField("One-line takeaway footer."),
		"pillars":   {typ: "array", desc: "3–5 named pillars; each {title, body?}. Body is a list of short bullet strings. Take the count from the content: one pillar per independent theme — do not merge a fourth theme or pad to three — and give each pillar the bullets it has (0–5; they need not match).", itemKeys: pillarItemKeys, itemRequired: []string{"title"}, itemKeySchemas: map[string]any{"body": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}},
		"objective": strField("Strategy-house objective, drawn in the gabled roof; supply with foundation."),
		"beam":      strField("Optional cross-cutting band between the roof and the pillars of a strategy house (something every pillar shares)."),
		"foundation": {typ: "string", schema: pillarsFoundationSchema,
			desc: "Strategy-house foundation; supply with objective. A string is one band. A list is 1–3 levels, top to bottom — one level per kind of enabler — where each level is a string (a full-width band) or a list of 2–5 short strings (a row of cells), e.g. [\"Shared platform\", [\"People\", \"Data\", \"Controls\"]]. Separate enablers belong in a split level, not joined into one string."},
		"roof_badges": {typ: "array", itemStrings: true, desc: "Up to three short string badges inside the roof of a strategy house."},
	}, compositionFields()), universalFields()),
	KindOrg: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"nodes":    {typ: "array", desc: "One root plus reporting nodes, in any order. Each node has id, name, optional title, and parent id; exactly one node omits parent.", itemKeys: orgNodeKeys, itemRequired: []string{"id", "name"}},
	}, compositionFields()), universalFields()),
	KindTeam: withFields(withFields(map[string]payloadField{
		"title":    strField("Slide title."),
		"takeaway": strField("One-line takeaway footer."),
		"members": {
			typ:         "array",
			desc:        "1–8 people: strings (a bare name) or {name, role, bio?, photo?, photo_label?}. Every card needs a role. Name ≤60 chars, role ≤80, bio ≤220 (~2 lines); photo is a headshot (a path/url string or {path|url, alt}); photo_label is the initials badge shown when there is no photo, ≤8 chars.",
			itemStrings: true,
			itemKeys:    teamMemberKeys,
		},
		"people": {typ: "array", desc: "Alias for members.", itemStrings: true, itemKeys: teamMemberKeys},
		"team":   {typ: "array", desc: "Alias for members.", itemStrings: true, itemKeys: teamMemberKeys},
	}, compositionFields()), universalFields()),
	KindStat: withFields(withFields(withFields(map[string]payloadField{
		"title":       strField("Slide title."),
		"takeaway":    strField("One-line takeaway footer."),
		"value":       strField("The number itself, formatted as it should read (e.g. \"$2.4B\", \"118%\"). ≤20 chars."),
		"stat":        strField("Alias for value."),
		"number":      strField("Alias for value."),
		"metric":      strField("Alias for value."),
		"label":       strField("The words beneath the number — what it measures. ≤80 chars; defaults to the slide title."),
		"caption":     strField("Alias for label."),
		"subtitle":    strField("Alias for label."),
		"unit":        strField("Short suffix beside the number (e.g. \"TAM\", \"MRR\"), set at 40% of the number's size. ≤10 chars."),
		"suffix":      strField("Alias for unit."),
		"detail":      strField("Alias for context."),
		"description": strField("Alias for context."),
	}, compositionFields()), universalFields()), map[string]payloadField{
		// After the universal fields: the stat's source line has its own budget.
		"source":  strField("Source footnote. ≤80 chars."),
		"context": strField("One line of context beneath the label. ≤120 chars; label, context and source together hold about 120 before the text shrinks."),
	}),
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
		"x_axis":       strField("The horizontal axis's name (e.g. \"Effort\"). ≤16 chars."),
		"x_axis_label": strField("Alias for x_axis."),
		"y_axis":       strField("The vertical axis's name (e.g. \"Impact\"). ≤60 chars."),
		"y_axis_label": strField("Alias for y_axis."),
		"x_low":        strField("Label for the left end of the horizontal axis (default \"Low\"). ≤11 chars."),
		"x_high":       strField("Label for the right end of the horizontal axis (default \"High\"). ≤11 chars."),
		"y_low":        strField("Label for the bottom end of the vertical axis (default \"Low\"). ≤11 chars."),
		"y_high":       strField("Label for the top end of the vertical axis (default \"High\"). ≤11 chars."),
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
		"image":       {typ: "object", desc: "The picture: a path or url string, or {path|url, alt, fit}. fit \"cover\" (default) crops to fill the frame; \"contain\" keeps a whole screenshot or exhibit. Omit it for a draft placeholder; SEMANTIC_IMAGE_MISSING blocks readiness until a picture is supplied.", schema: imageCasePictureSchema, objectKeys: imageCaseImageKeys},
		"eyebrow":     strField("Small kicker above the heading (e.g. \"Case study\"). ≤30 chars."),
		"heading":     strField("The story's headline. ≤80 chars."),
		"body":        strField("The story itself. ≤300 chars; give this or at least one bullet."),
		"text":        strField("Alias for body."),
		"story":       strField("Alias for body."),
		"description": strField("Alias for body."),
		"bullets":     {typ: "array", desc: "Up to 5 supporting points, ≤140 chars each.", itemStrings: true},
		"metrics":     {typ: "array", desc: "Up to 3 result figures: {value (≤10 chars), label (≤40)}. Both are required — a figure with no words is half a claim.", itemKeys: imageCaseMetricKeys},
		"caption":     strField("Italic line under the picture. ≤120 chars."),
		"callouts": {typ: "array", itemKeys: imageCaseCalloutKeys, itemRequired: []string{"label", "x", "y"},
			itemKeySchemas: map[string]any{
				"x":     map[string]any{"type": "number", "minimum": 0},
				"y":     map[string]any{"type": "number", "minimum": 0},
				"units": map[string]any{"type": "string", "enum": []any{"fraction", "px"}},
			},
			desc: "Up to 6 callouts on the picture: {label (≤40 chars), x, y, units?}. x / y are the point the label points at, as fractions of the image (0–1 from its top-left corner) or, with units \"px\", its own pixels; the label is placed beside the point and stays on it under any crop or template. A point the crop hides reports OVERLAY_TARGET_CROPPED at callouts[i] — use image.fit \"contain\"."},
		"image_side":  strField("Which side the picture sits on: \"left\" (default) or \"right\"."),
		"image_label": strField("Label for a draft placeholder when no picture is given; does not clear SEMANTIC_IMAGE_MISSING. ≤40 chars."),
		"placeholder": strField("Alias for image_label."),
		"photo":       {typ: "object", schema: imageCasePictureSchema, objectKeys: imageCaseImageKeys, desc: "Alias for image."},
		"screenshot":  {typ: "object", schema: imageCasePictureSchema, objectKeys: imageCaseImageKeys, desc: "Alias for image."},
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
		"recommended": {typ: "string", schema: recommendedSchema,
			desc: "The recommended option(s) by label or 0-based index, instead of options[].recommended: one value, or a list to recommend several together."},
		"options": {
			typ: "array",
			desc: "2–6 options: strings (\"Label\", or \"Label | detail\") or {label, detail?}. " +
				"3–6 become numbered boxes (label ≤60 chars, detail ≤180); exactly 2, each WITH a detail, become numbered cards side by side (label ≤80, detail ≤300); 7–12, each WITH a detail, become a card grid (label ≤80, detail ≤160; 7 as 4 + 3). " +
				"Mark the recommended option with recommended: true; it gets an accent-filled Recommended badge. Mark two or more to recommend them together: each gets the badge and, with no recommendation text, the band reads \"Recommended: A and B\". Anything else renders as the recommendation plus option bullets.",
			itemStrings:    true,
			itemKeys:       decisionOptionKeys,
			itemKeySchemas: map[string]any{"recommended": map[string]any{"type": "boolean"}},
		},
		"choices":      {typ: "array", desc: "Alias for options.", itemStrings: true, itemKeys: decisionOptionKeys, itemKeySchemas: map[string]any{"recommended": map[string]any{"type": "boolean"}}},
		"alternatives": {typ: "array", desc: "Alias for options.", itemStrings: true, itemKeys: decisionOptionKeys, itemKeySchemas: map[string]any{"recommended": map[string]any{"type": "boolean"}}},
	}, compositionFields()), universalFields()),
	KindNextSteps: withFields(withFields(map[string]payloadField{
		"title": strField("Slide title, e.g. \"Three actions start the pilot in October\"."),
		"actions": {
			typ:         "array",
			desc:        "2–6 actions in order: strings or {action (≤90 chars), owner? (≤30), date? (≤20)}. Owner and date columns drop when no action has one.",
			itemStrings: true, itemKeys: nextStepsActionKeys,
		},
		"next_steps":          {typ: "array", desc: "Alias for actions.", itemStrings: true, itemKeys: nextStepsActionKeys},
		"steps":               {typ: "array", desc: "Alias for actions.", itemStrings: true, itemKeys: nextStepsActionKeys},
		"decisions":           textList("0–3 decisions requested (≤120 chars each), rendered in a band with a left accent rule under the actions."),
		"decisions_requested": textList("Alias for decisions."),
		"asks":                textList("Alias for decisions."),
		"decisions_label":     strField("Band label (default \"Decisions requested\")."),
		"takeaway":            strField("One-line takeaway footer."),
	}, compositionFields()), universalFields()),
	KindClosing: withFields(map[string]payloadField{
		"title":    strField("Closing headline."),
		"subtitle": strField("Subtitle line."),
		"bullets":  textList("Optional closing bullets (renders a content slide)."),
		"points":   textList("Alias for bullets."),
	}, universalFields()),
	KindRegions: withFields(map[string]payloadField{
		"title":    strField("Slide title: the one claim the regions argue together."),
		"takeaway": strField("One-line takeaway footer."),
		"arrangement": {typ: "string", enum: slides.RegionArrangements,
			desc: "columns (default) or rows: 2–3 regions, size_pct is each one's width / height. main_left / main_right / main_top / main_bottom: exactly 3 regions, regions[0] is the main one (size_pct = its share, default 60) and regions[1..2] stack beside it, splitting that side by their own size_pct."},
		"regions": {typ: "array", schema: regionsSchema,
			desc: "2–3 typed regions in reading order: {kind, size_pct?, heading?, source?, …kind fields}. Unset size_pct values share what the set ones leave; every region gets 15–85%."},
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

// PayloadVocabulary returns every key a kind's payload can carry: its fields
// and the keys of their list entries and objects. A finding that advises an
// author of that kind may name these and no other field (go-slide-creator-micna).
func PayloadVocabulary(k SlideKind) []string {
	fields, ok := kindPayloadFields[k]
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	for name, f := range fields {
		seen[name] = true
		for _, key := range f.itemKeys {
			seen[key] = true
		}
		for key := range f.itemKeySchemas {
			seen[key] = true
		}
		for _, key := range f.objectKeys {
			seen[key] = true
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
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
			s.unknownSlideField(path, slide.Kind, key, known)
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

func (s *semDiags) unknownSlideField(path string, kind SlideKind, key string, known []string) {
	if kind == KindSection && key == "subtitle" {
		s.out = append(s.out, diagnostics.Diagnostic{
			Code: diagnostics.CodeSemanticUnknownField, Path: path + ".subtitle", Severity: diagnostics.SeverityError,
			Message: "section.subtitle is unsupported and its content is DROPPED: section body slots may hold decorative numbers; use a content slide or a raw shape_grid text box",
		})
		return
	}
	s.unknownField(path+"."+key, key, known, fmt.Sprintf("%s slide", kind))
}

// unknownField appends a SEMANTIC_UNKNOWN_FIELD finding for a key the compiler
// drops. It is never suppressed (dropping content silently is exactly what it
// guards against): an error under every advisory strictness. A close
// known key yields a rename_field fix with did_you_mean.
func (s *semDiags) unknownField(path, key string, known []string, where string) {
	d := diagnostics.Diagnostic{
		Code:     diagnostics.CodeSemanticUnknownField,
		Path:     path,
		Severity: diagnostics.SeverityError,
		Message: fmt.Sprintf("unknown field %q on %s is ignored by the compiler and its content is DROPPED; expected one of %s",
			key, where, joinQuoted(known)),
	}
	if sug := suggestKey(key, known); sug != "" {
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

// recommendedSchema is a recommendation reference: one option by name or
// 0-based index, or a list of them for a combined recommendation
// (go-slide-creator-3hcw6).
func recommendedSchema() map[string]any {
	ref := map[string]any{"type": []any{"string", "integer"}}
	return map[string]any{"oneOf": []any{
		ref,
		map[string]any{"type": "array", "minItems": 1, "items": ref},
	}}
}

// pillarsFoundationSchema is the pillars foundation: one band, or a list of
// levels that are each a band or a row of cells.
func pillarsFoundationSchema() map[string]any {
	band := map[string]any{"type": "string", "maxLength": 140}
	cells := map[string]any{"type": "array", "minItems": 1, "maxItems": 5, "items": map[string]any{"type": "string", "maxLength": 40}}
	return map[string]any{"oneOf": []any{
		band,
		map[string]any{"type": "array", "minItems": 1, "maxItems": 3, "items": map[string]any{"oneOf": []any{band, cells}}},
	}}
}

// agendaCurrentSchema accepts a section title or a 1-based integer position.
func agendaCurrentSchema() map[string]any {
	return map[string]any{"type": []any{"string", "integer"}, "minimum": 1}
}

func imageCasePictureSchema() map[string]any {
	props := objectKeySchemas(imageCaseImageKeys)
	props["fit"] = map[string]any{"type": "string", "enum": []any{"cover", "contain"}}
	return map[string]any{"anyOf": []any{
		map[string]any{"type": "string"},
		map[string]any{"type": "object", "properties": props, "additionalProperties": false},
	}}
}
