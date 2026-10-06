package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/policy/placeholder"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// DeckSpec forms and pattern / layout recipes for recommend_visual
// (go-slide-creator-x97m6, go-slide-creator-3ujfq, go-slide-creator-7sqof).
//
// A chart or diagram candidate carried a data contract and a runnable call; a
// named pattern — the first answer for most intents — carried neither, and
// every recipe spoke raw json2pptx although DeckSpec is the default path. Now
// every candidate recommend_visual returns can be authored from the response:
//
//   - When a DeckSpec kind compiles to the candidate (option_matrix →
//     table-highlight, org → org_chart, chart_insight → any chart, title /
//     section / table → the layout), next_tool_call renders one slide of that
//     kind, deckspec names the kind, its fields and aliases, and data_contract
//     (form "deckspec_kind") describes the kind's payload.
//   - When no kind does (the 22 raw-only patterns, most diagrams, a compose
//     envelope), next_tool_call renders one raw_json2pptx slide and
//     data_contract (form "raw_json2pptx") carries the shape inline: keys,
//     item counts and character budgets from the pattern's own schema.
//   - also_as cross-references the other forms of the same visual.
//
// TestRecommendVisualEveryPatternIsAuthorable and
// TestRecommendVisualEvalTopCandidateRenders pin both halves.

// recipeSlidePath is where a recipe's one slide sits in next_tool_call's spec.
const recipeSlidePath = "slides[0]"

// recipeExamplePath is the same slide addressed from next_tool_call.
const recipeExamplePath = "next_tool_call.args_template.spec." + recipeSlidePath

// cycleStyleOfPattern is the cycle kind's style for each circular pattern.
var cycleStyleOfPattern = map[string]string{
	"cycle-ring": "ring", "cycle-nodes": "nodes", "cycle-intake": "intake",
	"cycle-figure-eight": "figure_eight", "radial-hub": "radial", "concentric-rings": "concentric",
}

// kindSlideForPattern returns a DeckSpec slide that compiles to the named
// pattern, or nil when no kind reaches it. Most patterns are their kind's own
// example; a kind that compiles to several patterns (comparison, quote,
// pillars, kpi_snapshot, process, agenda) gets the variant that selects this
// one.
func kindSlideForPattern(pattern string) map[string]any {
	kind, known := semantic.PatternReach(pattern)
	if !known || kind == "" {
		return nil
	}
	ex := semantic.KindExample(kind)
	if ex == nil {
		return nil
	}
	if style, ok := cycleStyleOfPattern[pattern]; ok {
		// One kind draws the circular family; the style picks the pattern.
		return semantic.CycleStyleExample(style)
	}
	switch pattern {
	case "kpi-2up", "kpi-3up", "kpi-4up", "kpi-5up", "kpi-6up":
		n := int(pattern[4] - '0')
		all := []any{
			map[string]any{"value": "$48M", "label": "Annual revenue", "delta": "+41%"},
			map[string]any{"value": "118%", "label": "Net retention"},
			map[string]any{"value": "41d", "label": "Sales cycle", "delta": "-6d"},
			map[string]any{"value": "62", "label": "NPS", "delta": "+5"},
			map[string]any{"value": "68%", "label": "Gross margin"},
			map[string]any{"value": "3.1%", "label": "Monthly churn"},
		}
		ex["kpis"] = all[:n]
	case "card-grid":
		ex["pattern"] = "card-grid"
		ex["title"] = "Four options cover the range from build to buy"
		ex["columns"] = []any{
			map[string]any{"header": "Build in-house", "items": []any{"Full control of the roadmap", "12 months to first release"}},
			map[string]any{"header": "Buy a platform", "items": []any{"Live in 3 months", "Vendor roadmap dependency"}},
			map[string]any{"header": "Partner", "items": []any{"Shared investment", "Slower decisions"}},
			map[string]any{"header": "Acquire", "items": []any{"Team and product at once", "Integration risk"}},
		}
		ex["takeaway"] = "Buying is fastest; building keeps control."
	case "stylish-panels":
		// pillars without objective and foundation is the panel row; with
		// both it is the strategy house.
		ex = map[string]any{
			"kind": "pillars", "title": "Three pillars carry the FY27 plan",
			"pillars": []any{
				map[string]any{"title": "Customer trust", "body": []any{"Transparent pricing", "Operational resilience"}},
				map[string]any{"title": "Product velocity", "body": []any{"Weekly releases", "Shared platform"}},
				map[string]any{"title": "Disciplined growth", "body": []any{"Enterprise focus", "Partner channel"}},
			},
			"takeaway": "Trust, speed and growth carry the plan.",
		}
	case "quote-cluster":
		ex = map[string]any{
			"kind": "quote", "title": "What customers told us",
			"quotes": []any{
				map[string]any{"text": "The new platform cut our cycle time in half.", "name": "J. Lin", "role": "Head of Operations"},
				map[string]any{"text": "Onboarding took days, not months.", "name": "M. Okoro", "role": "CIO"},
				map[string]any{"text": "Support answers within the hour.", "name": "S. Weiss", "role": "Service lead"},
				map[string]any{"text": "Reporting finally matches finance.", "name": "A. Duarte", "role": "Controller"},
			},
			"takeaway": "Speed is the benefit customers name first.",
		}
	case "numbered-step-strip":
		// Steps that each carry a description render as numbered rows.
		ex = map[string]any{
			"kind": "process", "title": "How a deal closes",
			"steps": []any{
				map[string]any{"label": "Qualify", "description": "Confirm budget and sponsor"},
				map[string]any{"label": "Discover", "description": "Map the buying committee"},
				map[string]any{"label": "Propose", "description": "Price against the business case"},
				map[string]any{"label": "Close", "description": "Legal and procurement sign-off"},
			},
			"takeaway": "Discovery is where most deals stall.",
		}
	case "process-flow":
		// Four short labels would take the compact band; name the full flow.
		ex["pattern"] = "process-flow"
	case "process-flow-compact":
		ex["pattern"] = "process-flow-compact"
	case "agenda-with-images":
		ex["pattern"] = "agenda-with-images"
	case "bmc-canvas":
		ex = map[string]any{
			"kind": "framework", "framework": "bmc", "title": "The business model on one page",
			"sections": map[string]any{
				"key_partners":           []any{"Cloud providers", "Systems integrators"},
				"key_activities":         []any{"Platform development", "Customer onboarding"},
				"key_resources":          []any{"Engineering team", "Data pipelines"},
				"value_propositions":     []any{"No-code pipelines", "Live in a week"},
				"customer_relationships": []any{"Self-service", "Named success lead"},
				"channels":               []any{"Direct sales", "Marketplaces"},
				"customer_segments":      []any{"Mid-market IT", "Data teams"},
				"cost_structure":         []any{"Hosting", "Engineering"},
				"revenue_streams":        []any{"Subscriptions", "Usage fees"},
			},
			"takeaway": "Subscriptions fund the platform; usage fees grow with it.",
		}
	}
	return ex
}

// kindSlideForDiagram returns a DeckSpec slide that compiles to the diagram,
// or nil when no kind draws it.
func kindSlideForDiagram(diagram string) map[string]any {
	switch diagram {
	case "org_chart":
		return semantic.KindExample(semantic.KindOrg)
	case "swot":
		return semantic.KindExample(semantic.KindFramework)
	case "porters_five_forces":
		return map[string]any{
			"kind": "framework", "framework": "porters_five_forces", "title": "Rivalry is the force that sets the price",
			"sections": map[string]any{
				"rivalry":      []any{"Three scaled competitors", "Price-led renewals"},
				"new_entrants": []any{"High capital need", "Licensing takes two years"},
				"substitutes":  []any{"In-house builds", "Manual processing"},
				"suppliers":    []any{"Two cloud providers", "Scarce specialist talent"},
				"buyers":       []any{"Concentrated buyers", "Low switching cost"},
			},
			"takeaway": "Rivalry and buyer power set the price; entry is the smaller threat.",
		}
	}
	return nil
}

// chartInsightCannotDraw lists the chart types the chart_insight kind does
// not draw: it reads chart.data as {categories, series} (or {categories,
// values}) and degrades to bullets on another shape. Those types keep the raw
// chart slide. TestRecommendVisualChartsAndKindDiagramsUseDeckSpecForm fails
// when a type's kind recipe degrades without being listed here.
var chartInsightCannotDraw = map[string]bool{"waterfall": true, "gauge": true, "treemap": true}

// kindSlideForChart returns a chart_insight slide drawing the chart type with
// the recipe's sample data (dataKey selects a variant such as "bar_ranked"),
// or nil when the kind cannot draw that type.
func kindSlideForChart(chartType, dataKey string) map[string]any {
	data, ok := visualRecipeData[dataKey]
	if !ok || chartInsightCannotDraw[chartType] {
		return nil
	}
	label := strings.ReplaceAll(chartType, "_", " ")
	return map[string]any{
		"kind":  "chart_insight",
		"title": placeholder.RecipeActionTitle(label + " chart"),
		"chart": map[string]any{"type": chartType, "data": deepCopyAny(data)},
		"insights": []any{
			placeholder.RecipeCopy(placeholder.SlotChartShows),
			placeholder.RecipeCopy(placeholder.SlotWhyItMatters),
		},
		"source":   placeholder.RecipeSampleSource,
		"takeaway": placeholder.RecipeCopy(placeholder.SlotTakeaway),
	}
}

// kindSlideForLayout returns the DeckSpec slide for a placeholder-layout
// candidate, or nil when the layout is authored raw.
func kindSlideForLayout(slideType string) map[string]any {
	switch slideType {
	case "title":
		return semantic.KindExample(semantic.KindTitle)
	case "section":
		return semantic.KindExample(semantic.KindSection)
	case "table":
		return semantic.KindExample(semantic.KindTable)
	case "image":
		// image_case draws a labelled placeholder until image is set, so the
		// example renders without a picture file.
		return semantic.KindExample(semantic.KindImageCase)
	}
	return nil
}

// rawSlideForLayout returns the raw slide for a placeholder layout no kind
// stands for, and the path of its content below the slide.
func rawSlideForLayout(slideType string) (slide map[string]any, contentPath string, keys []string, desc string) {
	title := map[string]any{"placeholder_id": "title", "type": "text", "text_value": placeholder.RecipeActionTitle("slide")}
	switch slideType {
	case "content":
		return map[string]any{"slide_type": "content", "content": []any{
				title,
				map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": []any{
					placeholder.RecipeCopy(placeholder.SlotFirstPoint), placeholder.RecipeCopy(placeholder.SlotSecondPoint), placeholder.RecipeCopy(placeholder.SlotThirdPoint)}},
			}}, "content[1].bullets_value", []string{"bullets_value"},
			"Content slide: a title and one body placeholder. The body item is type bullets (bullets_value: 3–6 one-line strings), text (text_value), body_and_bullets, table, chart or diagram."
	case "two-column":
		return map[string]any{"slide_type": "two-column", "content": []any{
				title,
				map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": []any{placeholder.RecipeCopy(placeholder.SlotLeftFirst), placeholder.RecipeCopy(placeholder.SlotLeftSecond)}},
				map[string]any{"placeholder_id": "body_2", "type": "bullets", "bullets_value": []any{placeholder.RecipeCopy(placeholder.SlotRightFirst), placeholder.RecipeCopy(placeholder.SlotRightSecond)}},
			}}, "content", []string{"body", "body_2"},
			"Two-column slide: content items for placeholder_id body (left) and body_2 (right), each bullets, text, table, chart, diagram or image. Keep the two columns balanced."
	case "blank":
		// A title alone is a nearly empty slide (SLIDE_NEARLY_EMPTY), so the
		// recipe carries the smallest custom content: a 2×2 shape_grid.
		return map[string]any{"slide_type": "blank", "layout_id": "blank-title", "content": []any{title}, "shape_grid": recipeShapeGrid()},
			"shape_grid", []string{"rows"},
			"Blank + Title slide: only the title placeholder, with the custom content beside it on the slide — a shape_grid (rows[{cells[{shape {geometry, fill, text {content}}}]}], as here), a pattern {name, values} or a compose block."
	}
	return nil, "", nil, ""
}

// deckSpecFormFor describes a DeckSpec kind for a candidate: its fields under
// their canonical names and the aliases it accepts.
func deckSpecFormFor(kind semantic.SlideKind, differs string) *patterns.VisualDeckSpecForm {
	info, ok := semantic.LookupKind(kind)
	if !ok {
		return nil
	}
	form := &patterns.VisualDeckSpecForm{
		Kind:           string(kind),
		Fields:         patterns.VisualKindFields{Required: append([]string{}, info.RequiredFields...), Optional: append([]string(nil), info.TypicalFields...)},
		Aliases:        kindFieldAliases(info),
		ExamplePath:    recipeExamplePath,
		DiffersFromRaw: differs,
	}
	return form
}

// kindFieldAliases maps every alias a kind accepts for a top-level field to
// the canonical field name: the registered required-field aliases plus the
// payload fields documented as "Alias for <field>.".
func kindFieldAliases(info semantic.KindInfo) map[string]string {
	out := map[string]string{}
	for canonical, aliases := range info.RequiredAliases {
		for _, a := range aliases {
			out[a] = canonical
		}
	}
	props, _ := semantic.KindItemSchema(info.Kind)["properties"].(map[string]any)
	for name, raw := range props {
		p, _ := raw.(map[string]any)
		desc, _ := p["description"].(string)
		if rest, ok := strings.CutPrefix(desc, "Alias for "); ok {
			if canonical, _, found := strings.Cut(rest, "."); found && canonical != "" {
				out[name] = canonical
			}
		}
	}
	// An alias of an alias resolves to the final canonical name.
	for a, c := range out {
		for i := 0; i < 4; i++ {
			next, ok := out[c]
			if !ok {
				break
			}
			c = next
		}
		out[a] = c
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// kindDataContract is the data contract of a candidate authored as a DeckSpec
// kind: the kind's required and typical fields, and what each required field
// holds (its counts and budgets are in that sentence).
func kindDataContract(kind semantic.SlideKind) *patterns.VisualDataContract {
	info, ok := semantic.LookupKind(kind)
	if !ok {
		return nil
	}
	schema := semantic.KindItemSchema(kind)
	props, _ := schema["properties"].(map[string]any)
	var parts []string
	for _, f := range info.RequiredFields {
		p, _ := props[f].(map[string]any)
		desc, _ := p["description"].(string)
		desc = strings.TrimSpace(strings.TrimPrefix(desc, "Required payload field."))
		if desc != "" && f != "title" {
			parts = append(parts, f+": "+desc)
		}
	}
	// What each required field holds carries the counts and budgets; the
	// kind's summary stands in when no field says more than its name.
	desc := strings.Join(parts, " ")
	if desc == "" {
		desc = info.Summary
	}
	if kind == semantic.KindRegions {
		desc = regionsContractDescription
	}
	if kind == semantic.KindOptionMatrix {
		desc += " " + optionMatrixStatusBoardNote
	}
	return &patterns.VisualDataContract{
		Form:         patterns.ContractFormKind,
		RequiredKeys: append([]string{}, info.RequiredFields...),
		OptionalKeys: append([]string(nil), info.TypicalFields...),
		Limits:       schemaLimits(schema, info.RequiredFields, info.TypicalFields),
		Description:  desc,
		FieldPath:    recipeSlidePath,
	}
}

// optionMatrixStatusBoardNote maps a status board onto the option_matrix
// kind, which the journeys could not work out from the options × criteria
// vocabulary alone (go-slide-creator-ux1fl).
const optionMatrixStatusBoardNote = "A status board (risk appetite dashboard, RAG results by domain, controls tested with exceptions) is this kind: " +
	"options are the rows (one per risk type / domain), criteria the columns (e.g. Metric, Limit, Status), scale \"rag\" scores a column red / amber / green " +
	"(set scale on the Status criterion alone as {label: \"Status\", scale: \"rag\"} and keep figures as text in the others), " +
	"and the breached / red rows go in recommended with highlight_label \"Breached\" so they are highlighted."

// cardGridContractDescription replaces the comparison kind's two-column
// description when the kind compiles to card-grid (go-slide-creator-ux1fl).
const cardGridContractDescription = "columns: 2–12 titled cards {header, items[]} (each card's items join as its body; 7 cards arrange as 4 + 3) " +
	"under the comparison kind with pattern: \"card-grid\" — the kind's two-balanced-columns rule applies to comparison-2col, not here."

// regionsContractDescription is the regions kind's shape in one paragraph;
// the kind's own summary runs to several.
const regionsContractDescription = "2–3 regions under one title. arrangement: columns | rows (size_pct = each region's share) " +
	"or main_left | main_right | main_top | main_bottom (3 regions: regions[0] main, [1..2] stacked beside it). " +
	"Region = {kind, size_pct, heading, source} + its kind's fields: chart {chart {type, data}, unit}, stat {value, label, unit, context}, " +
	"kpis {kpis 2–4}, table {headers, rows ≤4×5}, timeline {milestones 3–7}, image {image, caption}, text {body, bullets}."

// rawPatternSlide hosts a pattern's exemplar values on a Blank + Title slide.
func rawPatternSlide(reg *patterns.Registry, name string) (map[string]any, *patterns.VisualDataContract, error) {
	values, contract, err := patternRegionRecipe(reg, name)
	if err != nil {
		return nil, nil, err
	}
	pat, _ := reg.Get(name)
	contract.Form = patterns.ContractFormRaw
	contract.FieldPath = recipeSlidePath + ".slide.pattern.values"
	contract.Limits = patternValuesLimits(pat)
	shape := "an object with the keys listed"
	if len(contract.Limits) > 0 && contract.Limits[0].Path == "values" {
		shape = "a list; each item takes the keys listed"
	}
	contract.Description = "Raw-only: no DeckSpec kind compiles to " + name + ", so author it as the raw_json2pptx slide in next_tool_call. pattern.values is " +
		shape + " (cells: " + pat.CellsHint() + "); limits gives the item counts and character budgets."
	label := strings.ReplaceAll(name, "-", " ")
	slide := map[string]any{
		"slide_type": "content",
		"layout_id":  composeRecipeLayoutID,
		"content": []any{
			map[string]any{"placeholder_id": "title", "type": "text", "text_value": placeholder.RecipeActionTitle(label)},
		},
		"pattern": map[string]any{"name": name, "values": values},
	}
	return map[string]any{"kind": "raw_json2pptx", "slide": slide}, contract, nil
}

// recipeSpec wraps one slide in a complete DeckSpec.
func recipeSpec(label, templateName string, slide map[string]any) map[string]any {
	if templateName == "" {
		templateName = visualRecipeTemplate
	}
	return map[string]any{
		"meta":   map[string]any{"title": "Recommended " + label, "template": templateName},
		"slides": []any{slide},
	}
}

// setKindRecipe makes a candidate's runnable call one DeckSpec slide of kind.
func setKindRecipe(c *patterns.VisualCandidate, slide map[string]any, templateName, differs string) bool {
	kindName, _ := slide["kind"].(string)
	kind := semantic.SlideKind(kindName)
	form := deckSpecFormFor(kind, differs)
	contract := kindDataContract(kind)
	if form == nil || contract == nil {
		return false
	}
	c.DeckSpec = form
	c.DataContract = contract
	label := strings.ReplaceAll(c.Name, "_", " ")
	// A kind's example reads as a finished slide. The recipe is scaffolding,
	// so its title is the registered placeholder: rendered verbatim it is
	// refused as exemplar content, like every other recipe
	// (go-slide-creator-327g6).
	if title, _ := slide["title"].(string); !isRecipePlaceholder(title) {
		slide["title"] = placeholder.RecipeActionTitle(recipeVisualLabel(c))
	}
	c.NextToolCall = &patterns.ToolCallSuggestion{
		Tool:         "render_deck_spec",
		ArgsTemplate: map[string]any{"spec": recipeSpec(label, templateName, slide)},
	}
	return true
}

// isRecipePlaceholder reports whether text is registered placeholder copy.
func isRecipePlaceholder(text string) bool {
	_, ok := placeholder.Detect(text)
	return ok
}

// recipeVisualLabel names a candidate's visual inside a recipe title ("kpi
// 3up", "org chart", "title slide").
func recipeVisualLabel(c *patterns.VisualCandidate) string {
	label := strings.NewReplacer("_", " ", "-", " ").Replace(c.Name)
	if c.Category == patterns.VisualCategoryPlaceholder {
		label += " slide"
	}
	return label
}

// kindVersusRaw says, for the kinds whose field names differ from the raw
// form of the same visual, what differs — the one place the two vocabularies
// are set side by side (go-slide-creator-7sqof).
var kindVersusRaw = map[string]string{
	"matrix-2x2":          "the kind takes x_axis / y_axis and quadrants[4] clockwise from the top left (top-left, top-right, bottom-right, bottom-left); the raw pattern takes x_axis_label / y_axis_label and top_left / top_right / bottom_left / bottom_right",
	"table-highlight":     "the kind names the highlight (recommended: an option name, decisive_criterion: a criterion label); the raw pattern takes indices (highlight_row, highlight_col)",
	"quote-cluster":       "the kind takes quotes[{text, name, role}]; the raw pattern takes quotes[{text, name, title}]",
	"pull-quote":          "the kind takes quote / attribution / role as slide fields (or one quotes[] entry); the raw pattern takes values {quote, attribution, role}",
	"comparison-2col":     "the kind takes columns[{header, items[]}]; the raw pattern takes headers[2] and rows[{left, right}]",
	"card-grid":           "the kind takes columns[{header, items[]}] (2–12) with pattern: \"card-grid\"; the raw pattern takes cells[{header, body}] (or \"Header | Body\" strings) with optional columns / rows counts",
	"kpi":                 "the kind takes kpis[{value, label, delta, comparator}]; the raw pattern takes a list of {big, small, sub, comparator}",
	"strategy-house":      "the kind takes pillars[{title, body[]}] with objective and foundation; the raw pattern takes the same keys under pattern.values",
	"org_chart":           "the kind takes a flat nodes[{id, name, title, parent}] list; the raw diagram takes a nested root {name, title, children[]}",
	"swot":                "the kind takes framework: swot and sections {strengths, weaknesses, opportunities, threats}; the raw diagram takes those four lists as data keys",
	"porters_five_forces": "the kind takes framework: porters_five_forces and sections {rivalry, new_entrants, substitutes, suppliers, buyers} as string lists; the raw diagram takes forces[{type, intensity}]",
	"chart":               "the kind takes chart {type, data} beside insights[]; the raw form is a chart content item (chart_value {type, data}) in the body placeholder, or diagram {type, data} inside a compose segment",
}

func kindVersusRawFor(c *patterns.VisualCandidate) string {
	switch {
	case c.Category == patterns.VisualCategoryChart:
		return kindVersusRaw["chart"]
	case strings.HasPrefix(c.Name, "kpi-") && strings.HasSuffix(c.Name, "up"):
		return kindVersusRaw["kpi"]
	}
	return kindVersusRaw[c.Name]
}

// attachCandidateRecipe gives one non-compose candidate its runnable call,
// data contract, DeckSpec form and cross-references.
func attachCandidateRecipe(c *patterns.VisualCandidate, templateName string, reg *patterns.Registry, hints map[string]skillDataFormat, ranking bool) {
	c.AlsoAs = patterns.VisualAlsoAs(c.Category, c.Name)
	switch c.Category {
	case patterns.VisualCategoryPattern:
		if setPatternKindRecipe(c, templateName) {
			return
		}
		slide, contract, err := rawPatternSlide(reg, c.Name)
		if err != nil {
			return
		}
		c.DataContract = contract
		c.NextToolCall = &patterns.ToolCallSuggestion{
			Tool:         "render_deck_spec",
			ArgsTemplate: map[string]any{"spec": recipeSpec(strings.ReplaceAll(c.Name, "-", " "), templateName, slide)},
		}
	case patterns.VisualCategoryChart:
		dataKey := c.Name
		// A ranking ("largest", "top 5") is the textbook horizontal bar chart:
		// names in a column, bars sorted largest first (go-slide-creator-oocqj).
		if c.Name == "bar" && ranking {
			dataKey = "bar_ranked"
			c.Rationale += "; a ranking reads best as horizontal bars sorted largest first (data.orientation \"horizontal\")"
		}
		slide := kindSlideForChart(c.Name, dataKey)
		if slide == nil || !setKindRecipe(c, slide, templateName, kindVersusRawFor(c)) {
			attachRawChartOrDiagram(c, dataKey, templateName, hints)
			return
		}
		// The chart's own data keys are what the agent fills in.
		if h, ok := hints[c.Name]; ok {
			c.DataContract.RequiredKeys = h.RequiredKeys
			c.DataContract.OptionalKeys = h.OptionalKeys
			c.DataContract.Description = h.Description + " Slide fields: title, chart {type, data}, insights[] (1–6 bullets beside the chart), source, takeaway."
			c.DataContract.Limits = nil
			c.DataContract.FieldPath = recipeSlidePath + ".chart.data"
		}
	case patterns.VisualCategoryDiagram:
		if slide := kindSlideForDiagram(c.Name); slide != nil && setKindRecipe(c, slide, templateName, kindVersusRawFor(c)) {
			return
		}
		attachRawChartOrDiagram(c, c.Name, templateName, hints)
		// The pattern twin may be reachable through a kind even though this
		// diagram is not: say so.
		for _, p := range patterns.VisualTwinPatterns(c.Category, c.Name) {
			if kind, ok := semantic.PatternReach(p); ok && kind != "" {
				c.AlsoAs = append(c.AlsoAs, patterns.VisualFormRef{
					Form: "kind", Name: string(kind),
					Differs: "compiles to the " + p + " pattern, not to this diagram",
				})
			}
		}
	case patterns.VisualCategoryPlaceholder:
		if slide := kindSlideForLayout(c.Name); slide != nil && setKindRecipe(c, slide, templateName, "") {
			if c.Name == "image" {
				c.DataContract.Description += " For a picture alone in the body placeholder, write a raw_json2pptx content slide with the item {placeholder_id: body, type: image, image_value: {path, alt}}."
			}
			return
		}
		slide, contentPath, keys, desc := rawSlideForLayout(c.Name)
		if slide == nil {
			return
		}
		if keys == nil {
			keys = []string{}
		}
		c.DataContract = &patterns.VisualDataContract{
			Form:         patterns.ContractFormRaw,
			RequiredKeys: keys,
			Description:  desc,
			FieldPath:    recipeSlidePath + ".slide." + contentPath,
		}
		c.NextToolCall = &patterns.ToolCallSuggestion{
			Tool:         "render_deck_spec",
			ArgsTemplate: map[string]any{"spec": recipeSpec(c.Name+" slide", templateName, map[string]any{"kind": "raw_json2pptx", "slide": slide})},
		}
	case patterns.VisualCategoryKind:
		attachRegionsRecipe(c, templateName)
	case patterns.VisualCategoryShapeGrid:
		attachShapeGridRecipe(c, templateName)
	}
}

// setPatternKindRecipe gives a pattern candidate the DeckSpec kind that
// compiles to it, reporting false when no kind does. card-grid reaches the
// comparison kind with pattern: card-grid, whose contract would otherwise
// describe comparison-2col's two balanced columns (go-slide-creator-ux1fl).
func setPatternKindRecipe(c *patterns.VisualCandidate, templateName string) bool {
	slide := kindSlideForPattern(c.Name)
	if slide == nil || !setKindRecipe(c, slide, templateName, kindVersusRawFor(c)) {
		return false
	}
	if c.Name == "card-grid" {
		c.DataContract.Description = cardGridContractDescription
		c.DataContract.OptionalKeys = append(c.DataContract.OptionalKeys, "pattern")
	}
	return true
}

// attachRawChartOrDiagram is the raw recipe: one raw_json2pptx slide hosting
// the chart / diagram in the body placeholder.
func attachRawChartOrDiagram(c *patterns.VisualCandidate, dataKey, templateName string, hints map[string]skillDataFormat) {
	spec, valueKey := visualRecipeDeckSpecFrom(c.Category, c.Name, dataKey, templateName)
	if spec == nil {
		return
	}
	if h, ok := hints[c.Name]; ok {
		c.DataContract = &patterns.VisualDataContract{
			Form:         patterns.ContractFormRaw,
			RequiredKeys: h.RequiredKeys,
			OptionalKeys: h.OptionalKeys,
			Description:  h.Description,
			FieldPath:    "slides[0].slide.content[1]." + valueKey + ".data",
		}
	}
	c.NextToolCall = &patterns.ToolCallSuggestion{
		Tool:         "render_deck_spec",
		ArgsTemplate: map[string]any{"spec": spec},
	}
}

// recipeShapeGrid is the minimal 2×2 grid the shape_grid and blank recipes
// start from.
func recipeShapeGrid() map[string]any {
	cell := func(text, fill string) map[string]any {
		return map[string]any{"shape": map[string]any{"geometry": "roundRect", "fill": fill, "text": map[string]any{"content": text}}}
	}
	return map[string]any{
		"columns": 2,
		"gap":     8,
		"rows": []any{
			// Neutral fills: default text ink reads on lt2 on every template,
			// which it does not on an accent fill.
			map[string]any{"cells": []any{cell(placeholder.RecipeCopy(placeholder.SlotFirstBlock), "lt2"), cell(placeholder.RecipeCopy(placeholder.SlotSecondBlock), "lt2")}},
			map[string]any{"cells": []any{cell(placeholder.RecipeCopy(placeholder.SlotThirdBlock), "lt2"), cell(placeholder.RecipeCopy(placeholder.SlotFourthBlock), "lt2")}},
		},
	}
}

// attachShapeGridRecipe gives the raw_shape_grid fallback a minimal grid to
// start from.
func attachShapeGridRecipe(c *patterns.VisualCandidate, templateName string) {
	slide := map[string]any{
		"slide_type": "content",
		"layout_id":  composeRecipeLayoutID,
		"content": []any{
			map[string]any{"placeholder_id": "title", "type": "text", "text_value": placeholder.RecipeActionTitle("slide")},
		},
		"shape_grid": recipeShapeGrid(),
	}
	c.DataContract = &patterns.VisualDataContract{
		Form:         patterns.ContractFormRaw,
		RequiredKeys: []string{"rows"},
		OptionalKeys: []string{"columns", "gap", "bounds"},
		Description:  "Raw shape_grid: rows[{cells[{shape {geometry, fill, text {content}}}]}]; columns is the column count, gap the spacing in points. Fills are theme colour names (accent1, lt2, dk2), never hex; text sizes come from the template.",
		FieldPath:    recipeSlidePath + ".slide.shape_grid",
	}
	c.NextToolCall = &patterns.ToolCallSuggestion{
		Tool:         "render_deck_spec",
		ArgsTemplate: map[string]any{"spec": recipeSpec("custom grid", templateName, map[string]any{"kind": "raw_json2pptx", "slide": slide})},
	}
}

// ---------------------------------------------------------------------------
// regions
// ---------------------------------------------------------------------------

// regionChartData is single-series sample data for a chart region: one
// series needs no legend, which a third of a slide has no room for.
var regionChartData = map[string]any{
	"categories": []any{"Q1", "Q2", "Q3", "Q4"},
	"series":     []any{map[string]any{"name": "Revenue", "values": []any{12, 14, 17, 21}}},
}

// regionSlideFor builds one region of a regions slide from a composition leaf.
func regionSlideFor(r *patterns.VisualRegion) map[string]any {
	switch patterns.RegionKindFor(r) {
	case "chart":
		data := regionChartData
		switch r.Name {
		case "bar", "line", "area":
		default:
			if d, ok := visualRecipeData[r.Name]; ok {
				data = d
			}
		}
		return map[string]any{
			"kind": "chart", "heading": "Quarterly revenue", "unit": "€m",
			"chart": map[string]any{"type": r.Name, "data": deepCopyAny(data)},
		}
	case "stat":
		return map[string]any{"kind": "stat", "value": "32%", "label": "Gross margin, Q4"}
	case "kpis":
		n := int(r.Name[4] - '0')
		all := []any{
			map[string]any{"value": "$48M", "label": "Revenue"},
			map[string]any{"value": "118%", "label": "Net retention"},
			map[string]any{"value": "41d", "label": "Sales cycle"},
			map[string]any{"value": "62", "label": "NPS"},
		}
		return map[string]any{"kind": "kpis", "kpis": all[:n]}
	case "timeline":
		return map[string]any{"kind": "timeline", "heading": "Launch plan, Oct–Dec", "milestones": []any{
			map[string]any{"label": "Design"},
			map[string]any{"label": "Pilot"},
			map[string]any{"label": "Rollout"},
		}}
	case "text":
		return map[string]any{
			"kind": "text", "heading": "What this means",
			"body": placeholder.RecipeCopy(placeholder.SlotWhyItMatters),
			"bullets": []any{
				placeholder.RecipeCopy(placeholder.SlotFirstPoint),
				placeholder.RecipeCopy(placeholder.SlotSecondPoint),
			},
		}
	}
	return nil
}

// regionContractNote says what one region of a recommended regions slide
// takes, so the contract names the region kinds this intent asked for
// (go-slide-creator-ux1fl).
func regionContractNote(r *patterns.VisualRegion, i int) string {
	at := fmt.Sprintf("regions[%d] ", i)
	switch patterns.RegionKindFor(r) {
	case "chart":
		if r.Name == "waterfall" {
			return at + "is a chart region {kind: chart, chart {type: waterfall, data {points: [{label, value, type: increase | decrease | total | subtotal}]}}, unit?}"
		}
		shape := "{categories, series: [{name, values}]}"
		if r.Name == "pie" || r.Name == "donut" {
			shape = "{categories, values}"
		}
		return at + "is a chart region {kind: chart, chart {type: " + r.Name + ", data " + shape + "}, unit?}"
	case "stat":
		return at + "is a stat region {kind: stat, value, label, context?}"
	case "kpis":
		return at + "is a kpis region {kind: kpis, kpis: [{value, label, delta?}] (2–4)}"
	case "timeline":
		return at + "is a timeline region {kind: timeline, milestones: [{label, date?}] (3–7)}"
	case "text":
		return at + "is a text region {kind: text, heading?, body (≤400) and / or bullets (≤6)}"
	}
	return ""
}

// clampSharePct keeps a share inside the bounds the regions kind accepts.
func clampSharePct(v, lo, hi float64) float64 {
	switch {
	case v < lo:
		return lo
	case v > hi:
		return hi
	}
	return v
}

// attachRegionsRecipe turns a regions candidate's composition into one
// DeckSpec regions slide: the arrangement, a typed region per view with its
// share, and sample content in each.
func attachRegionsRecipe(c *patterns.VisualCandidate, templateName string) {
	slide := semantic.KindExample(semantic.KindRegions)
	if c.Composition != nil {
		arrangement, ordered, ok := patterns.RegionsArrangement(c.Composition)
		if !ok {
			return
		}
		regions := make([]any, 0, len(ordered))
		for i, r := range ordered {
			region := regionSlideFor(r)
			if region == nil {
				return
			}
			pct := r.SizePct
			if semanticIsMainArrangement(arrangement) {
				if i == 0 {
					// The main region keeps the share the intent gave it.
					pct = clampSharePct(pct, 50, 70)
				} else {
					// A stat over a timeline reads on every shipped template
					// from a 45/55 to a 60/40 split (go-slide-creator-fn2ka).
					pct = 50
				}
			} else {
				pct = clampSharePct(pct, 15, 85)
			}
			region["size_pct"] = pct
			regions = append(regions, region)
			r.SegmentPath = fmt.Sprintf("%s.regions[%d]", recipeSlidePath, i)
		}
		slide = map[string]any{
			"kind":        "regions",
			"title":       placeholder.RecipeActionTitle("set of views"),
			"arrangement": arrangement,
			"regions":     regions,
			"source":      placeholder.RecipeSampleSource,
			"takeaway":    placeholder.RecipeCopy(placeholder.SlotTakeaway),
		}
		c.Composition.Instructions = "Copy " + recipeExamplePath + " into your DeckSpec slides: one regions slide, arrangement " + arrangement +
			". Each composition region's segment_path is its entry in regions[]; replace the sample content there and keep kind, arrangement and size_pct."
		if setKindRecipe(c, slide, templateName, "") {
			var notes []string
			for i, r := range ordered {
				if n := regionContractNote(r, i); n != "" {
					notes = append(notes, n)
				}
			}
			if len(notes) > 0 {
				c.DataContract.Description += " This slide: " + strings.Join(notes, "; ") + "."
			}
		}
		return
	}
	setKindRecipe(c, slide, templateName, "")
}

func semanticIsMainArrangement(arrangement string) bool {
	return strings.HasPrefix(arrangement, "main_")
}

// ---------------------------------------------------------------------------
// Limits from a JSON Schema
// ---------------------------------------------------------------------------

// maxContractLimits bounds a contract's limits list; the example in
// next_tool_call shows the rest of the shape.
const maxContractLimits = 14

// patternValuesLimits lists the item counts and character budgets of a
// pattern's values from its schema.
func patternValuesLimits(pat patterns.Pattern) []patterns.VisualFieldLimit {
	var root map[string]any
	if err := json.Unmarshal(patterns.SchemaJSON(pat), &root); err != nil {
		return nil
	}
	props, _ := root["properties"].(map[string]any)
	values, _ := props["values"].(map[string]any)
	if values == nil {
		return nil
	}
	defs, _ := root["$defs"].(map[string]any)
	w := limitWalker{defs: defs}
	// A pattern whose values are a list of cells (kpi-3up, icon-row) has its
	// count on values itself.
	path := ""
	if values["type"] == "array" {
		path = "values"
	}
	w.walk(values, path, 0)
	return w.result()
}

// schemaLimits lists the limits of the named top-level fields of an object
// schema (a DeckSpec kind's item schema).
func schemaLimits(schema map[string]any, fieldGroups ...[]string) []patterns.VisualFieldLimit {
	props, _ := schema["properties"].(map[string]any)
	w := limitWalker{}
	for _, fields := range fieldGroups {
		for _, f := range fields {
			if p, ok := props[f].(map[string]any); ok {
				w.walk(p, f, 1)
			}
		}
	}
	return w.result()
}

type limitWalker struct {
	defs map[string]any
	// byDepth keeps shallow fields first when the list is cut.
	byDepth [5][]patterns.VisualFieldLimit
}

func (w *limitWalker) result() []patterns.VisualFieldLimit {
	var out []patterns.VisualFieldLimit
	for _, level := range w.byDepth {
		out = append(out, level...)
	}
	if len(out) > maxContractLimits {
		out = out[:maxContractLimits]
	}
	return out
}

func schemaInt(s map[string]any, key string) int {
	if f, ok := s[key].(float64); ok {
		return int(f)
	}
	if n, ok := s[key].(int); ok {
		return n
	}
	return 0
}

// limitSkipKeys are value keys whose sub-shape is tooling detail (an icon or
// photo reference), not content budget.
var limitSkipKeys = map[string]bool{"icon": true, "photo": true, "image": true, "secondary_chart": true}

func (w *limitWalker) walk(s map[string]any, path string, depth int) {
	if s == nil || depth >= len(w.byDepth) {
		return
	}
	s, ok := w.resolve(s)
	if !ok {
		return
	}
	// A tolerated shorthand sits in a oneOf / anyOf beside the full form:
	// describe the richest branch (an object, else an array, else the first).
	if pick, ok := w.richestBranch(s); ok {
		w.walk(pick, path, depth)
		return
	}
	lim := patterns.VisualFieldLimit{Path: path}
	switch s["type"] {
	case "array":
		lim.MinItems, lim.MaxItems = schemaInt(s, "minItems"), schemaInt(s, "maxItems")
		if path != "" && (lim.MinItems > 0 || lim.MaxItems > 0) {
			w.byDepth[depth] = append(w.byDepth[depth], lim)
		}
		items, _ := s["items"].(map[string]any)
		w.walk(items, path+"[]", depth+1)
	case "object":
		props, _ := s["properties"].(map[string]any)
		names := make([]string, 0, len(props))
		for n := range props {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if limitSkipKeys[n] {
				continue
			}
			child, _ := props[n].(map[string]any)
			p := n
			if path != "" {
				p = path + "." + n
			}
			w.walk(child, p, depth+1)
		}
	case "string":
		if lim.MaxChars = schemaInt(s, "maxLength"); lim.MaxChars > 0 && path != "" {
			w.byDepth[depth] = append(w.byDepth[depth], lim)
		}
	}
}

// resolve follows a "#/$defs/<name>" reference; ok is false when s is a
// reference that does not resolve.
func (w *limitWalker) resolve(s map[string]any) (map[string]any, bool) {
	ref, isRef := s["$ref"].(string)
	if !isRef {
		return s, true
	}
	name, isDef := strings.CutPrefix(ref, "#/$defs/")
	target, _ := w.defs[name].(map[string]any)
	return target, isDef && target != nil
}

// richestBranch picks the branch of a oneOf / anyOf to describe: an object,
// else an array, else the first. ok is false when s has no such branches.
func (w *limitWalker) richestBranch(s map[string]any) (map[string]any, bool) {
	for _, key := range []string{"oneOf", "anyOf"} {
		branches, _ := s[key].([]any)
		if len(branches) == 0 {
			continue
		}
		var pick map[string]any
		rank := -1
		for _, b := range branches {
			bm, _ := b.(map[string]any)
			if bm == nil {
				continue
			}
			if target, ok := w.resolve(bm); ok {
				bm = target
			}
			r := 0
			switch bm["type"] {
			case "object":
				r = 2
			case "array":
				r = 1
			}
			if r > rank {
				pick, rank = bm, r
			}
		}
		return pick, true
	}
	return nil, false
}
