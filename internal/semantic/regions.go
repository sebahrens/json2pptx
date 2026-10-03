package semantic

// Regions slides: the closed region contract, its JSON Schema, and the
// validator (go-slide-creator-fn2ka). The compiler is slides.CompileRegions.
//
// A region has no fallback to degrade to — a timeline that does not fit the
// third of a slide it was given cannot become a bullet slide without taking
// the chart and the KPI with it — so every region rule here is an error that
// points at the region field to edit, and a regions slide that validates
// compiles to exactly the grid it describes.

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

// regionCommonFields are accepted on every region.
func regionCommonFields() map[string]payloadField {
	return map[string]payloadField{
		"kind":     {typ: "string", desc: "Region kind.", enum: slides.RegionKinds},
		"size_pct": {typ: "number", desc: "This region's share (15–85) along the arrangement's axis; for main_* stacks, of its side. Unset shares split by kind, and a stacked region's is raised to the height its kind reads in on the shortest shipped content area."},
		"heading":  strField("Optional bold label above the region, ≤60 chars."),
		"source":   strField("Source of this region's figures; joins the slide's source line."),
	}
}

// regionPayloadFields is the closed per-region-kind contract: the common
// fields plus the fields of the like-named slide kind the region reads.
var regionPayloadFields = map[string]map[string]payloadField{
	slides.RegionChart: withFields(map[string]payloadField{
		"chart": {typ: "object", objectKeys: chartObjectKeys,
			desc: "Chart: {type, title?, data}. Bar/line/area data is {categories:[…], series:[{name, values:[…]}]}; pie/donut {categories:[…], values:[…]}."},
		"unit": strField("Unit of the chart's values (e.g. \"€m\"), shown after the heading. ≤12 chars."),
		"alt":  strField("Alt text for the chart; defaults to the heading or chart title."),
	}, regionCommonFields()),
	slides.RegionStat: withFields(map[string]payloadField{
		"value":   strField("The number, formatted as it should read (e.g. \"32%\"). ≤20 chars."),
		"label":   strField("The words beneath the number. ≤80 chars."),
		"unit":    strField("Short suffix beside the number. ≤10 chars."),
		"context": strField("One line of context beneath the label. ≤120 chars."),
	}, regionCommonFields()),
	slides.RegionKPIs: withFields(map[string]payloadField{
		"kpis": {typ: "array", itemKeys: kpiItemKeys, desc: "2–4 KPI objects {value, label, delta?, comparator?}."},
	}, regionCommonFields()),
	slides.RegionTable: withFields(map[string]payloadField{
		"headers":           textList("Column headers (≤4)."),
		"rows":              {typ: "array", desc: "Data rows (≤5): a list of cells, or an object keyed by header label."},
		"column_alignments": textList("Per-column alignment: left / center / right."),
	}, regionCommonFields()),
	slides.RegionTimeline: withFields(map[string]payloadField{
		"milestones": {typ: "array", itemStrings: true, itemKeys: timelineStopKeys,
			desc: "3–7 milestones: strings or {label, date?, end_date?, body?}. Label ≤60 chars, date ≤30, body ≤200."},
	}, regionCommonFields()),
	slides.RegionImage: withFields(map[string]payloadField{
		"image":   {typ: "object", objectKeys: imageCaseImageKeys, desc: "The picture: {path|url, alt?, fit?} (fit \"cover\" default, or \"contain\"). A region draws no callouts on it: for callouts pointing at parts of a picture use the image_case kind (callouts [{label, x, y}])."},
		"caption": strField("Italic line under the picture. ≤120 chars."),
	}, regionCommonFields()),
	slides.RegionText: withFields(map[string]payloadField{
		"body":    strField("Narrative text, ≤400 chars; a newline starts a paragraph."),
		"bullets": {typ: "array", itemStrings: true, desc: "Up to 6 bullets."},
	}, regionCommonFields()),
}

// regionRequired lists each region kind's required content field.
var regionRequired = map[string]string{
	slides.RegionChart:    "chart",
	slides.RegionStat:     "value",
	slides.RegionKPIs:     "kpis",
	slides.RegionTable:    "headers",
	slides.RegionTimeline: "milestones",
	slides.RegionImage:    "image",
}

// regionsSchema is the regions field's schema: a list of 2–3 entries, each one
// closed region variant selected by its kind.
func regionsSchema() map[string]any {
	variants := make([]any, 0, len(slides.RegionKinds))
	for _, kind := range slides.RegionKinds {
		fields := regionPayloadFields[kind]
		props := map[string]any{}
		for name, f := range fields {
			props[name] = payloadFieldSchema(f, "")
		}
		props["kind"] = map[string]any{"const": kind}
		props["size_pct"] = map[string]any{"type": "number", "minimum": slides.RegionMinSizePct, "maximum": slides.RegionMaxSizePct}
		required := []any{"kind"}
		if field := regionRequired[kind]; field != "" {
			required = append(required, field)
		}
		v := map[string]any{
			"type": "object", "title": kind + " region",
			"properties": props, "required": required, "additionalProperties": false,
		}
		if kind == slides.RegionText {
			v["anyOf"] = []any{map[string]any{"required": []any{"body"}}, map[string]any{"required": []any{"bullets"}}}
		}
		variants = append(variants, v)
	}
	return map[string]any{
		"type": "array", "minItems": 2, "maxItems": 3,
		"items": map[string]any{"oneOf": variants},
	}
}

// regionFieldNames returns a region kind's sorted field names.
func regionFieldNames(kind string) []string {
	fields := regionPayloadFields[kind]
	out := make([]string, 0, len(fields))
	for name := range fields {
		out = append(out, name)
	}
	return sortedCopy(out)
}

// validateRegions checks a regions slide: the arrangement, the region count
// and shares, and every region against its kind's contract and budgets.
func validateRegions(path string, slide SlideSpec, s *semDiags) {
	arrangement := slides.RegionArrangementOf(slide.Body)
	if !containsString(slides.RegionArrangements, arrangement) {
		s.hard(path+".arrangement", diagnostics.CodeSemanticFieldType,
			fmt.Sprintf("unknown arrangement %q; expected one of %s", arrangement, joinQuoted(slides.RegionArrangements)))
		return
	}
	raw, isList := slide.Body["regions"].([]any)
	if !isList {
		// Absent: the required-field gate reports it; wrong type: the shape check.
		return
	}
	lo, hi := slides.RegionCountBounds(arrangement)
	if len(raw) < lo || len(raw) > hi {
		want := fmt.Sprintf("%d–%d", lo, hi)
		if lo == hi {
			want = fmt.Sprintf("exactly %d", lo)
		}
		s.hard(path+".regions", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("arrangement %q takes %s regions; found %d. Use columns or rows for two regions, main_left / main_right / main_top / main_bottom for one main region beside a stack of two", arrangement, want, len(raw)))
		return
	}

	sharesOK := true
	for i, entry := range raw {
		rpath := fmt.Sprintf("%s.regions[%d]", path, i)
		region, ok := entry.(map[string]any)
		if !ok {
			s.hard(rpath, diagnostics.CodeSemanticFieldType,
				fmt.Sprintf("a region must be an object {kind, …} but is %s", jsonTypeName(entry)))
			sharesOK = false
			continue
		}
		if !validateRegion(rpath, region, s) {
			sharesOK = false
		}
	}
	if !sharesOK {
		return
	}
	if _, _, err := slides.RegionGroupShares(slide.Body); err != nil {
		s.hard(path+".regions", diagnostics.CodeSemanticDensity, fmt.Sprintf("region sizes do not fit: %v", err))
	}
}

// validateRegion checks one region and reports whether its size_pct is usable.
func validateRegion(rpath string, r map[string]any, s *semDiags) bool {
	kind, _ := r["kind"].(string)
	fields, known := regionPayloadFields[kind]
	if !known {
		if strings.TrimSpace(kind) == "" {
			s.hard(rpath+".kind", diagnostics.CodeSemanticRequired,
				fmt.Sprintf("region is missing its \"kind\"; expected one of %s", joinQuoted(slides.RegionKinds)))
		} else {
			s.hard(rpath+".kind", diagnostics.CodeSemanticUnknownKind,
				fmt.Sprintf("unknown region kind %q; expected one of %s", kind, joinQuoted(slides.RegionKinds)))
		}
		return false
	}

	sizeOK := true
	where := kind + " region"
	for _, key := range sortedKeys(r) {
		f, ok := fields[key]
		if !ok {
			// "callouts" is one edit from "caption", so the generic refusal
			// would suggest the wrong field (go-slide-creator-n3j96).
			if key == "callouts" && kind == slides.RegionImage {
				s.hard(rpath+"."+key, diagnostics.CodeSemanticUnknownField,
					"an image region draws no callouts, so these are DROPPED; put the picture on an image_case slide, whose callouts [{label, x, y, units?}] point at parts of it")
				continue
			}
			s.unknownField(rpath+"."+key, key, regionFieldNames(kind), where)
			continue
		}
		if !validateRegionField(rpath, where, key, f, r[key], s) && key == "size_pct" {
			sizeOK = false
		}
	}

	if pct := slides.RegionSizePct(r); sizeOK && r["size_pct"] != nil && (pct < slides.RegionMinSizePct || pct > slides.RegionMaxSizePct) {
		s.hard(rpath+".size_pct", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("size_pct is %g; a region takes %d–%d%% of its axis", pct, slides.RegionMinSizePct, slides.RegionMaxSizePct))
		sizeOK = false
	}
	if n := runeLen(slides.RegionHeading(r)); n > slides.RegionHeadingMax {
		s.hard(rpath+".heading", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("heading is %d characters; a region heading holds %d", n, slides.RegionHeadingMax))
	}
	if rule := regionContentRules[kind]; rule != nil {
		rule(regionCheck{path: rpath, r: r, s: s})
	}
	return sizeOK
}

// validateRegionField checks one known region field's JSON type and the keys
// of its list entries / object, reporting whether the type was usable.
func validateRegionField(rpath, where, key string, f payloadField, v any, s *semDiags) bool {
	if v == nil {
		return true
	}
	if !regionValueTypeOK(f.typ, v) {
		s.hard(rpath+"."+key, diagnostics.CodeSemanticFieldType,
			fmt.Sprintf("%q must be %s but is %s; the %s drops it", key, typePhrase(f.typ), jsonTypeName(v), where))
		return false
	}
	switch {
	case f.typ == "array" && len(f.itemKeys) > 0:
		list, _ := v.([]any)
		for j, e := range list {
			m, isMap := e.(map[string]any)
			if !isMap {
				continue
			}
			for _, ik := range sortedKeys(m) {
				if !containsKey(f.itemKeys, ik) {
					s.unknownField(fmt.Sprintf("%s.%s[%d].%s", rpath, key, j, ik), ik, sortedCopy(f.itemKeys), where+" "+key+" entry")
				}
			}
		}
	case f.typ == "object" && len(f.objectKeys) > 0:
		m, _ := v.(map[string]any)
		for _, ck := range sortedKeys(m) {
			if !containsKey(f.objectKeys, ck) {
				s.unknownField(rpath+"."+key+"."+ck, ck, sortedCopy(f.objectKeys), where+" "+key+" object")
			}
		}
	}
	return true
}

// regionCheck is one region under validation.
type regionCheck struct {
	path string
	r    map[string]any
	s    *semDiags
}

func (c regionCheck) required(field, msg string) {
	c.s.hard(c.path+"."+field, diagnostics.CodeSemanticRequired, msg)
}

func (c regionCheck) dense(field, msg string) {
	c.s.hard(c.path+"."+field, diagnostics.CodeSemanticDensity, msg)
}

// regionContentRules are each region kind's content rules: what it must carry
// and the budgets its visual holds.
var regionContentRules = map[string]func(regionCheck){
	slides.RegionChart: func(c regionCheck) {
		chart, ok := c.r["chart"].(map[string]any)
		if !ok {
			c.required("chart", "chart region requires a chart {type, data}")
			return
		}
		if slides.RegionChartSpec(c.r) == nil {
			shape, _ := chartDataShape(strPayloadField(chart, "type"))
			c.required("chart.data", fmt.Sprintf("chart region needs chart.type and chart.data as %s", shape))
			return
		}
		data, _ := chart["data"].(map[string]any)
		validateChartSeriesAlignment(c.path+".chart.data", data, c.s)
		if n := runeLen(strPayloadField(c.r, "unit")); n > 12 {
			c.dense("unit", fmt.Sprintf("unit is %d characters; a region unit holds 12", n))
		}
	},
	slides.RegionStat: func(c regionCheck) {
		if strPayloadField(c.r, "value") == "" {
			c.required("value", "stat region requires a value")
			return
		}
		for _, issue := range slides.StatBudgetIssues(c.r) {
			c.dense(issue.Field, "stat region "+issue.Message)
		}
	},
	slides.RegionKPIs: func(c regionCheck) {
		n := slides.RegionKPICount(c.r)
		switch {
		case n == 0:
			c.required("kpis", "kpis region requires 2–4 kpis {value, label}")
		case n < slides.RegionKPIMin || n > slides.RegionKPIMax:
			c.dense("kpis", fmt.Sprintf("kpis region has %d metrics; it holds %d–%d (one number is a stat region)", n, slides.RegionKPIMin, slides.RegionKPIMax))
		default:
			if reason := slides.RegionKPIBudgetReason(c.r); reason != "" {
				c.dense("kpis", "kpis region does not fit its cards: "+reason)
			}
		}
	},
	slides.RegionTable: func(c regionCheck) {
		columns, rows := slides.UsableTableCounts(c.r)
		switch {
		case columns == 0:
			c.required("headers", "table region requires headers")
			return
		case rows == 0:
			c.required("rows", "table region requires at least one row")
			return
		}
		if columns > slides.RegionTableMaxColumns {
			c.dense("headers", fmt.Sprintf("table region has %d columns; a region holds %d — give the table its own slide", columns, slides.RegionTableMaxColumns))
		}
		if rows+1 > slides.RegionTableMaxRows {
			c.dense("rows", fmt.Sprintf("table region has %d rows including the header; a region holds %d — give the table its own slide", rows+1, slides.RegionTableMaxRows))
		}
	},
	slides.RegionTimeline: func(c regionCheck) {
		if slides.UsableTimelineStopCount(c.r) == 0 {
			c.required("milestones", "timeline region requires 3–7 milestones")
			return
		}
		if over := slides.TimelineOverBudget(c.r); over != "" {
			c.dense("milestones", "timeline region "+over)
		}
	},
	slides.RegionImage: func(c regionCheck) {
		if !slides.RegionHasImage(c.r) {
			c.required("image", "image region requires image {path|url}")
		}
		if n := runeLen(strPayloadField(c.r, "caption")); n > 120 {
			c.dense("caption", fmt.Sprintf("caption is %d characters; it holds 120", n))
		}
	},
	slides.RegionText: func(c regionCheck) {
		bodyRunes, bullets := slides.RegionTextCounts(c.r)
		if bodyRunes == 0 && bullets == 0 {
			c.required("body", "text region requires a body or bullets")
			return
		}
		if bodyRunes > slides.RegionTextBodyMax {
			c.dense("body", fmt.Sprintf("body is %d characters; a text region holds %d", bodyRunes, slides.RegionTextBodyMax))
		}
		if bullets > slides.RegionTextBulletsMax {
			c.dense("bullets", fmt.Sprintf("text region has %d bullets; it holds %d", bullets, slides.RegionTextBulletsMax))
		}
	},
}

// regionValueTypeOK reports whether v has the contract's JSON type.
func regionValueTypeOK(typ string, v any) bool {
	switch typ {
	case "string":
		_, ok := v.(string)
		return ok
	case "number":
		switch v.(type) {
		case float64, int, int64:
			return true
		}
		return false
	case "array":
		_, ok := v.([]any)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	}
	return true
}

func typePhrase(typ string) string {
	switch typ {
	case "array", "object":
		return "an " + typ
	default:
		return "a " + typ
	}
}

func runeLen(s string) int { return len([]rune(s)) }

// dottedToPointer spells a dotted raw path ("slides[1].shape_grid.rows[0]")
// as the JSON Pointer render findings use ("/slides/1/shape_grid/rows/0").
func dottedToPointer(p string) string {
	p = strings.NewReplacer("[", ".", "]", "").Replace(normalizePath(p))
	return "/" + strings.ReplaceAll(p, ".", "/")
}

// regionsFamily is the visual family a regions slide counts as for deck
// rhythm: its main (first) region's.
func regionsFamily(body map[string]any) VisualFamily {
	regions := slides.RegionList(body)
	if len(regions) == 0 || regions[0] == nil {
		return FamilyChart
	}
	switch strPayloadField(regions[0], "kind") {
	case slides.RegionStat, slides.RegionKPIs:
		return FamilyKPI
	case slides.RegionTimeline:
		return FamilyTimeline
	case slides.RegionTable, slides.RegionText, slides.RegionImage:
		return FamilyText
	default:
		return FamilyChart
	}
}

// regionsCarryData reports whether a regions slide shows figures (a chart,
// KPIs, a stat or a table), so the deck's evidence checks count it.
func regionsCarryData(body map[string]any) bool {
	for _, r := range slides.RegionList(body) {
		switch strPayloadField(r, "kind") {
		case slides.RegionChart, slides.RegionKPIs, slides.RegionStat, slides.RegionTable:
			return true
		}
	}
	return false
}
