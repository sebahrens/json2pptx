package slides

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Cycle slides (go-slide-creator-53v5u).
//
// The circular family — cycle-ring, cycle-nodes, cycle-figure-eight,
// radial-hub, concentric-rings — was raw-only: five patterns, five values
// shapes (phases / steps / spokes / layers; highlight as a per-item flag in two
// and a 1-based number in three; a centre two of them do not have). A DeckSpec
// author writes one payload instead — the phases in order, an optional center,
// the one phase to highlight — and names the picture with style. The compiler
// maps that onto the pattern's own values, and a payload the pattern refuses
// becomes a numbered list that keeps every phase.

// Cycle styles, as the payload's style field names them.
const (
	CycleStyleRing        = "ring"
	CycleStyleNodes       = "nodes"
	CycleStyleIntake      = "intake"
	CycleStyleFigureEight = "figure_eight"
	CycleStyleRadial      = "radial"
	CycleStyleConcentric  = "concentric"
)

// CycleStyles lists the styles in the order the contract documents them; ring
// is the default.
var CycleStyles = []string{CycleStyleRing, CycleStyleNodes, CycleStyleIntake, CycleStyleFigureEight, CycleStyleRadial, CycleStyleConcentric}

// cycleStyleInfo is what one style compiles to: the pattern, the values list
// the phases become, and the phase count the pattern holds.
type cycleStyleInfo struct {
	pattern  string
	list     string
	min, max int
	// noun is what one phase is called in a finding for this style.
	noun string
	// tooFew / tooMany say where a payload outside the count belongs, in the
	// kind's own vocabulary.
	tooFew, tooMany string
}

var cycleStyleTable = map[string]cycleStyleInfo{
	CycleStyleRing: {
		pattern: "cycle-ring", list: "phases", min: 4, max: 8, noun: "phase",
		tooFew:  "style nodes draws a loop of 3; two states are a comparison",
		tooMany: "merge related phases or split the loop across two cycle slides",
	},
	CycleStyleNodes: {
		pattern: "cycle-nodes", list: "steps", min: 3, max: 8, noun: "phase",
		tooFew:  "a loop needs at least 3 phases; two states are a comparison",
		tooMany: "merge related phases or split the loop across two cycle slides; a sequence that does not loop back is a process",
	},
	CycleStyleIntake: {
		pattern: "cycle-intake", list: "loop", min: 3, max: 8, noun: "phase",
		tooFew:  "a loop needs at least 3 phases; steps that never loop back are a process",
		tooMany: "merge related phases, or move the first of them into intake when they happen once",
	},
	CycleStyleFigureEight: {
		pattern: "cycle-figure-eight", list: "phases", min: 4, max: 8, noun: "phase",
		tooFew:  "each of the two loops holds 2–4 phases; use style ring or nodes for a single loop",
		tooMany: "each of the two loops holds 2–4 phases; merge related phases or give each loop its own cycle slide",
	},
	CycleStyleRadial: {
		pattern: "radial-hub", list: "spokes", min: 4, max: 8, noun: "item",
		tooFew:  "fewer than 4 peers around one idea read better as pillars or a kpi_snapshot",
		tooMany: "group related items, or use comparison for up to 12 cards",
	},
	CycleStyleConcentric: {
		pattern: "concentric-rings", list: "layers", min: 3, max: 5, noun: "layer",
		tooFew:  "two nested levels are a comparison",
		tooMany: "merge related layers, or use architecture for a stack of up to 6 tiers",
	},
}

// cycleFigureEightDescFourMax is the figure eight's description budget once
// one of its loops carries four phases (the pattern reports the same length as
// BODY_TOO_LONG at render time; validation says it before the render).
const cycleFigureEightDescFourMax = 40

// cyclePhaseFields are the payload keys the phases may be written under,
// canonical first.
var cyclePhaseFields = []string{"phases", "steps", "items"}

// cycleItem is one resolved phase.
type cycleItem struct {
	Label, Description string
	Highlight          bool
	// raw is the entry's index in the authored list; labelKey / descKey the
	// keys it was written with ("" for a plain string entry).
	raw               int
	labelKey, descKey string
}

// cyclePayload is a cycle payload resolved against its style.
type cyclePayload struct {
	// Style is the style the slide renders in; Authored what the payload
	// wrote ("" when it left the choice to the compiler).
	Style, Authored string
	// Field is the key the phases were written under.
	Field string
	Items []cycleItem
	// Intake are the linear steps that feed the loop (style intake);
	// intakeSet reports that the payload carries the field.
	Intake    []cycleItem
	intakeSet bool
	// Center* is the centre text; centerObject reports the {label, sublabel}
	// form, centerSet that the payload carries the field at all.
	CenterLabel, CenterSublabel string
	centerObject, centerSet     bool
	LeftLabel, RightLabel       string
	LeftCount                   int
	// highlight is the 0-based index of the highlighted phase, -1 for none;
	// highlights how many phases the payload marks; highlightMiss a slide-level
	// highlight that names no phase.
	highlight, highlights int
	highlightMiss         string
}

// NormalizeCycleStyle spells a style the way the contract lists it:
// "Figure-Eight" and "figure eight" are figure_eight.
func NormalizeCycleStyle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.NewReplacer("-", "_", " ", "_").Replace(s)
}

// CycleStyleKnown reports whether a style is one the kind draws ("" is the
// default and known).
func CycleStyleKnown(style string) bool {
	if strings.TrimSpace(style) == "" {
		return true
	}
	_, ok := cycleStyleTable[NormalizeCycleStyle(style)]
	return ok
}

// cyclePhasesField names the key the phases were written under.
func cyclePhasesField(body map[string]any) string {
	for _, key := range cyclePhaseFields {
		if _, ok := body[key].([]any); ok {
			return key
		}
	}
	return cyclePhaseFields[0]
}

// resolveCycle reads a cycle payload (a slide body or a region).
func resolveCycle(body map[string]any) cyclePayload {
	p := cyclePayload{Authored: NormalizeCycleStyle(strField(body, "style")), Field: cyclePhasesField(body), highlight: -1}
	raw, _ := body[p.Field].([]any)
	p.Items = readCycleItems(raw)
	intake, intakeSet := body["intake"].([]any)
	p.Intake, p.intakeSet = readCycleItems(intake), intakeSet && len(intake) > 0

	switch c := body["center"].(type) {
	case string:
		p.CenterLabel, p.centerSet = strings.TrimSpace(c), strings.TrimSpace(c) != ""
	case map[string]any:
		p.CenterLabel, p.CenterSublabel = strField(c, "label"), strField(c, "sublabel")
		p.centerObject, p.centerSet = true, p.CenterLabel != "" || p.CenterSublabel != ""
	}
	p.LeftLabel, p.RightLabel = strField(body, "left_label"), strField(body, "right_label")
	if n, err := strconv.Atoi(strField(body, "left_count")); err == nil {
		p.LeftCount = n
	}

	marked := map[int]bool{}
	for i, it := range p.Items {
		if it.Highlight {
			marked[i] = true
		}
	}
	if ref := strField(body, "highlight"); ref != "" {
		if i := p.phaseIndex(ref, len(raw)); i >= 0 {
			marked[i] = true
		} else {
			p.highlightMiss = ref
		}
	}
	p.highlights = len(marked)
	for i := range p.Items {
		if marked[i] {
			p.highlight = i
			break
		}
	}

	p.Style = p.Authored
	if p.Style == "" {
		// The default is the ring; it holds 4–8, so a loop of 3 takes nodes.
		p.Style = CycleStyleRing
		if len(p.Items) == 3 {
			p.Style = CycleStyleNodes
		}
		if len(p.Intake) > 0 {
			// Steps that feed the loop name the picture themselves.
			p.Style = CycleStyleIntake
		}
	}
	return p
}

// readCycleItems resolves a list of phases (or intake steps): strings, or
// objects carrying a label. Entries with no label are dropped.
func readCycleItems(raw []any) []cycleItem {
	var out []cycleItem
	for i, e := range raw {
		switch t := e.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				out = append(out, cycleItem{Label: s, raw: i})
			}
		case map[string]any:
			lk := authoredKey(t, "label", "name", "title")
			if lk == "" {
				continue
			}
			it := cycleItem{Label: strField(t, lk), raw: i, labelKey: lk, descKey: authoredKey(t, "description", "detail")}
			if it.descKey != "" {
				it.Description = strField(t, it.descKey)
			}
			it.Highlight, _ = t["highlight"].(bool)
			out = append(out, it)
		}
	}
	return out
}

// phaseIndex resolves a slide-level highlight — a 1-based position in the
// authored list, or a phase's label — to the index of the resolved phase, or
// -1 when it names none.
func (p cyclePayload) phaseIndex(ref string, authored int) int {
	if n, err := strconv.Atoi(ref); err == nil {
		if n < 1 || n > authored {
			return -1
		}
		for i, it := range p.Items {
			if it.raw == n-1 {
				return i
			}
		}
		return -1
	}
	for i, it := range p.Items {
		if strings.EqualFold(it.Label, ref) {
			return i
		}
	}
	return -1
}

// info is the resolved style's table entry; ok is false for a style the kind
// does not draw.
func (p cyclePayload) info() (cycleStyleInfo, bool) {
	info, ok := cycleStyleTable[p.Style]
	return info, ok
}

// itemField is the authored path of a phase's label or description.
func (p cyclePayload) itemField(i int, description bool) string {
	return cycleItemField(p.Field, p.Items[i], description)
}

// cycleItemField is the authored path of an entry of the named list.
func cycleItemField(list string, it cycleItem, description bool) string {
	base := fmt.Sprintf("%s[%d]", list, it.raw)
	switch {
	case it.labelKey == "":
		return base
	case description:
		return base + "." + firstNonEmpty(it.descKey, "description")
	default:
		return base + "." + it.labelKey
	}
}

// centerField is the authored path of the centre's label or sublabel.
func (p cyclePayload) centerField(sublabel bool) string {
	switch {
	case !p.centerObject:
		return "center"
	case sublabel:
		return "center.sublabel"
	default:
		return "center.label"
	}
}

// pattern builds the style's pattern block.
func (p cyclePayload) pattern() (deckinput.PatternInput, error) {
	info, ok := p.info()
	if !ok {
		return deckinput.PatternInput{}, fmt.Errorf("unknown cycle style %q", p.Style)
	}
	var values any
	switch p.Style {
	case CycleStyleRing, CycleStyleFigureEight:
		phases := make([]patterns.CycleRingPhase, len(p.Items))
		for i, it := range p.Items {
			phases[i] = patterns.CycleRingPhase{Label: it.Label, Description: it.Description, Highlight: i == p.highlight}
		}
		if p.Style == CycleStyleFigureEight {
			values = patterns.CycleFigureEightValues{Phases: phases, LeftCount: p.LeftCount, LeftLabel: p.LeftLabel, RightLabel: p.RightLabel}
			break
		}
		v := patterns.CycleRingValues{Phases: phases}
		if p.centerSet {
			v.Center = &patterns.CycleRingCenter{Label: p.CenterLabel, Sublabel: p.CenterSublabel}
		}
		values = v
	case CycleStyleIntake:
		v := patterns.CycleIntakeValues{}
		for _, it := range p.Intake {
			v.Intake = append(v.Intake, patterns.CycleIntakeStep{Label: it.Label, Description: it.Description})
		}
		for i, it := range p.Items {
			v.Loop = append(v.Loop, patterns.CycleIntakePhase{Label: it.Label, Description: it.Description, Highlight: i == p.highlight})
		}
		if p.CenterLabel != "" {
			v.Center = &patterns.CycleIntakeCenter{Label: p.CenterLabel}
		}
		values = v
	case CycleStyleNodes:
		v := patterns.CycleNodesValues{Highlight: p.highlight + 1}
		for _, it := range p.Items {
			v.Steps = append(v.Steps, patterns.CycleNodesStep{Label: it.Label, Description: it.Description})
		}
		if p.CenterLabel != "" {
			v.Center = &patterns.CycleNodesCenter{Label: p.CenterLabel}
		}
		values = v
	case CycleStyleRadial:
		v := patterns.RadialHubValues{
			Center:    patterns.RadialHubCenter{Label: p.CenterLabel, Sublabel: p.CenterSublabel},
			Highlight: p.highlight + 1,
		}
		for _, it := range p.Items {
			v.Spokes = append(v.Spokes, patterns.RadialHubSpoke{Label: it.Label, Description: it.Description})
		}
		values = v
	case CycleStyleConcentric:
		v := patterns.ConcentricRingsValues{Highlight: p.highlight + 1}
		for _, it := range p.Items {
			v.Layers = append(v.Layers, patterns.ConcentricRingsLayer{Label: it.Label, Description: it.Description})
		}
		values = v
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return deckinput.PatternInput{}, fmt.Errorf("marshal %s values: %w", info.pattern, err)
	}
	return deckinput.PatternInput{Name: info.pattern, Values: encoded}, nil
}

// CycleIssue is one reason a cycle payload does not reach its visual.
type CycleIssue struct {
	// Field is the authored path under the slide (or region): "phases",
	// "phases[3].label", "center", "style".
	Field string
	// Message says what is wrong and what to write instead, in the kind's
	// vocabulary.
	Message string
	// Min / Max are the count the style holds, set on a count issue.
	Min, Max int
	// Measured / Allowed are character counts, set on a text-budget issue.
	Measured, Allowed int
	// Required marks content the style needs and the payload lacks.
	Required bool
	// Dropped marks a field the style does not draw: its content is lost
	// rather than the visual.
	Dropped bool
}

// CycleIssues lists every reason a cycle payload (a slide body or a region)
// does not render as its style's visual: the phase count, a missing hub, each
// over-long label at its own path, a highlight that names no phase. It is
// empty when the payload compiles to the pattern. Fields the style does not
// draw are CycleDroppedFields.
func CycleIssues(body map[string]any) []CycleIssue {
	p := resolveCycle(body)
	info, ok := p.info()
	if !ok {
		return []CycleIssue{{Field: "style", Message: fmt.Sprintf("unknown style %q; expected one of %s", strField(body, "style"), strings.Join(CycleStyles, ", "))}}
	}
	var out []CycleIssue
	if n := len(p.Items); n < info.min || n > info.max {
		hint := info.tooMany
		if n < info.min {
			hint = info.tooFew
		}
		out = append(out, CycleIssue{
			Field: p.Field, Min: info.min, Max: info.max,
			Message: fmt.Sprintf("style %s holds %d–%d %ss; found %d — %s", p.Style, info.min, info.max, info.noun, n, hint),
		})
	}
	if n := len(p.Intake); p.Style == CycleStyleIntake && (n < 1 || n > 3) {
		hint := "merge the intake steps, or draw them as a process slide before the cycle"
		if n == 0 {
			hint = "list the one-off steps that feed the loop in intake, or use style ring"
		}
		out = append(out, CycleIssue{Field: "intake", Min: 1, Max: 3, Required: n == 0,
			Message: fmt.Sprintf("style intake holds 1–3 intake steps before the loop; found %d — %s", n, hint)})
	}
	if p.Style == CycleStyleRadial && p.CenterLabel == "" {
		out = append(out, CycleIssue{Field: "center", Required: true,
			Message: "style radial needs center: the idea the items sit around (≤24 chars)"})
	}
	if p.highlightMiss != "" {
		out = append(out, CycleIssue{Field: "highlight",
			Message: fmt.Sprintf("highlight %q names no phase; give a phase's label or its 1-based position (1–%d)", p.highlightMiss, len(p.Items))})
	}
	if p.highlights > 1 {
		out = append(out, CycleIssue{Field: p.Field,
			Message: fmt.Sprintf("%d %ss are highlighted; one is — the single solid accent of the picture", p.highlights, info.noun)})
	}
	if p.Style == CycleStyleFigureEight {
		out = append(out, p.figureEightDescIssues()...)
	}
	return append(out, p.patternIssues(info)...)
}

// figureEightDescIssues reports descriptions over the tighter budget a figure
// eight has once one of its loops carries four phases.
func (p cyclePayload) figureEightDescIssues() []CycleIssue {
	n := len(p.Items)
	left := p.LeftCount
	if left <= 0 {
		left = (n + 1) / 2
	}
	if n < 4 || n > 8 || max(left, n-left) < 4 {
		return nil
	}
	var out []CycleIssue
	for i, it := range p.Items {
		// Past the pattern's own maximum the pattern's finding reports it.
		if m := runeLen(it.Description); m > cycleFigureEightDescFourMax && m <= 60 {
			out = append(out, CycleIssue{
				Field: p.itemField(i, true), Measured: m, Allowed: cycleFigureEightDescFourMax,
				Message: fmt.Sprintf("phase %d's description is %d characters; a figure_eight loop of four phases holds %d", i+1, m, cycleFigureEightDescFourMax),
			})
		}
	}
	return out
}

// patternIssues runs the pattern's own validation and reports each finding at
// the authored field, so the kind never restates a budget the pattern owns.
func (p cyclePayload) patternIssues(info cycleStyleInfo) []CycleIssue {
	block, err := p.pattern()
	if err != nil {
		return []CycleIssue{{Field: p.Field, Message: err.Error()}}
	}
	verr := deckinput.ValidatePattern(&block, patterns.Default())
	if verr == nil {
		return nil
	}
	findings := deckinput.PatternValidationFindings(verr)
	if len(findings) == 0 {
		return []CycleIssue{{Field: p.Field, Message: strings.TrimSpace(strings.TrimPrefix(firstLine(verr.Error()), info.pattern+":"))}}
	}
	var out []CycleIssue
	for _, f := range findings {
		// The findings arrive rooted at the pattern block ("values.spokes[2].label").
		path := strings.TrimPrefix(f.Path, "values.")
		field, what, value, known := p.authoredPath(info, path)
		switch {
		case path == info.list, path == "intake", path == "highlight", path == "center.label" && f.Code == patterns.ErrCodeRequired:
			// The count, the highlight and the hub are reported above in the
			// kind's own words.
			continue
		case known && f.Code == patterns.ErrCodeMaxLength && f.Fix != nil:
			allowed, _ := f.Fix.Params["max_length"].(int)
			if allowed > 0 {
				out = append(out, CycleIssue{
					Field: field, Measured: runeLen(value), Allowed: allowed,
					Message: fmt.Sprintf("%s is %d characters; style %s holds %d", what, runeLen(value), p.Style, allowed),
				})
				continue
			}
		}
		out = append(out, CycleIssue{Field: field, Message: strings.TrimSpace(strings.TrimPrefix(f.Message, info.pattern+":"))})
	}
	return out
}

// authoredPath maps a path in the pattern's values ("spokes[2].label") to the
// field the author wrote ("phases[2].label"), a name for it in a message, and
// its text. known is false for a path the kind has no field for.
func (p cyclePayload) authoredPath(info cycleStyleInfo, path string) (field, what, value string, known bool) {
	switch path {
	case "center.label":
		return p.centerField(false), "center", p.CenterLabel, true
	case "center.sublabel":
		return p.centerField(true), "center's sublabel", p.CenterSublabel, true
	case "left_label":
		return path, path, p.LeftLabel, true
	case "right_label":
		return path, path, p.RightLabel, true
	case "left_count":
		return path, path, strconv.Itoa(p.LeftCount), true
	}
	if rest, ok := strings.CutPrefix(path, "intake["); ok {
		idx, tail, _ := strings.Cut(rest, "]")
		if i, err := strconv.Atoi(idx); err == nil && i >= 0 && i < len(p.Intake) {
			if tail == ".description" {
				return cycleItemField("intake", p.Intake[i], true), fmt.Sprintf("intake step %d's description", i+1), p.Intake[i].Description, true
			}
			return cycleItemField("intake", p.Intake[i], false), fmt.Sprintf("intake step %d's label", i+1), p.Intake[i].Label, true
		}
	}
	rest, ok := strings.CutPrefix(path, info.list+"[")
	if !ok {
		return p.Field, path, "", false
	}
	idx, tail, _ := strings.Cut(rest, "]")
	i, err := strconv.Atoi(idx)
	if err != nil || i < 0 || i >= len(p.Items) {
		return p.Field, path, "", false
	}
	if tail == ".description" {
		return p.itemField(i, true), fmt.Sprintf("%s %d's description", info.noun, i+1), p.Items[i].Description, true
	}
	return p.itemField(i, false), fmt.Sprintf("%s %d's label", info.noun, i+1), p.Items[i].Label, true
}

func firstLine(s string) string {
	return strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
}

// CycleDroppedFields lists the fields a cycle payload carries that its style
// does not draw — a center on a figure eight, lobe labels on a ring — each
// with where the content belongs. Their content would otherwise vanish.
func CycleDroppedFields(body map[string]any) []CycleIssue {
	p := resolveCycle(body)
	if _, ok := p.info(); !ok {
		return nil
	}
	var out []CycleIssue
	drop := func(field, msg string) {
		out = append(out, CycleIssue{Field: field, Dropped: true, Message: msg})
	}
	switch p.Style {
	case CycleStyleFigureEight:
		if p.centerSet {
			drop("center", "style figure_eight has no centre, so center is DROPPED; name the two loops with left_label and right_label")
		}
	case CycleStyleConcentric:
		if p.centerSet {
			drop("center", "style concentric has no centre, so center is DROPPED; the core is the first of the phases (innermost first)")
		}
	case CycleStyleNodes, CycleStyleIntake:
		if p.CenterSublabel != "" {
			drop("center.sublabel", "style "+p.Style+" draws a one-line centre, so center.sublabel is DROPPED; use style ring, or fold it into center.label (≤24 chars)")
		}
	}
	if p.Style != CycleStyleIntake && p.intakeSet {
		drop("intake", fmt.Sprintf("intake belongs to style intake (steps that feed the loop), so it is DROPPED on style %s; set style: intake or remove it", p.Style))
	}
	if p.Style != CycleStyleFigureEight {
		for _, key := range []string{"left_label", "right_label", "left_count"} {
			if strField(body, key) != "" {
				drop(key, fmt.Sprintf("%s belongs to style figure_eight (two loops), so it is DROPPED on style %s; set style: figure_eight or remove it", key, p.Style))
			}
		}
	}
	return out
}

// CyclePattern returns the pattern a cycle payload compiles to, or "" when it
// degrades to a list; explain and validation read it so neither promises a
// visual compile will not emit.
func CyclePattern(body map[string]any) string {
	if len(CycleIssues(body)) > 0 {
		return ""
	}
	info, _ := resolveCycle(body).info()
	return info.pattern
}

// CycleVisual names the pattern the payload's style stands for, also when the
// payload does not reach it — the visual a degraded slide is losing.
func CycleVisual(body map[string]any) string {
	if info, ok := resolveCycle(body).info(); ok {
		return info.pattern
	}
	return cycleStyleTable[CycleStyleRing].pattern
}

// CyclePhasesField names the key the payload's phases are written under.
func CyclePhasesField(body map[string]any) string { return cyclePhasesField(body) }

// UsableCyclePhaseCount returns how many phases survive extraction.
func UsableCyclePhaseCount(body map[string]any) int { return len(resolveCycle(body).Items) }

// CycleStyleOf returns the style a payload renders in ("ring" unless it says
// otherwise, "nodes" for an unstyled loop of three).
func CycleStyleOf(body map[string]any) string { return resolveCycle(body).Style }

// CompileCycle compiles a cycle payload onto its style's pattern, falling back
// to a numbered list when the pattern cannot take it.
func CompileCycle(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	p := resolveCycle(in.Body)
	if in.wantsContent() || CyclePattern(in.Body) == "" {
		return compileCycleFallback(in, p)
	}
	block, err := p.pattern()
	if err != nil {
		return nil, nil, err
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title", Pattern: &block}
	links := titleLink(slide, in)
	for _, l := range p.links() {
		links = append(links, SourceLink{RawPath: in.rawSlide() + l.RawPath, SemanticPath: in.semSlide() + l.SemanticPath})
	}
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// links are the pattern block's source links, relative to the slide (or the
// region's cell): the list, the centre and the lobe labels.
func (p cyclePayload) links() []SourceLink {
	info, _ := p.info()
	links := []SourceLink{{RawPath: ".pattern.values." + info.list, SemanticPath: "." + p.Field}}
	if p.centerSet && (p.Style == CycleStyleRing || p.Style == CycleStyleNodes || p.Style == CycleStyleRadial || p.Style == CycleStyleIntake) {
		links = append(links, SourceLink{RawPath: ".pattern.values.center.label", SemanticPath: "." + p.centerField(false)})
		if p.CenterSublabel != "" && p.Style != CycleStyleNodes && p.Style != CycleStyleIntake {
			links = append(links, SourceLink{RawPath: ".pattern.values.center.sublabel", SemanticPath: ".center.sublabel"})
		}
	}
	if p.Style == CycleStyleIntake {
		links = append(links, SourceLink{RawPath: ".pattern.values.intake", SemanticPath: ".intake"})
	}
	if p.Style == CycleStyleFigureEight {
		for _, key := range []string{"left_label", "right_label"} {
			links = append(links, SourceLink{RawPath: ".pattern.values." + key, SemanticPath: "." + key})
		}
	}
	return links
}

// compileCycleFallback lists the phases in order, the centre first, so a
// payload the visual refuses loses its picture and none of its words.
func compileCycleFallback(in Input, p cyclePayload) (*deckinput.SlideInput, []SourceLink, error) {
	var bullets []string
	if p.CenterLabel != "" {
		bullets = append(bullets, strings.TrimSpace(p.CenterLabel+" "+p.CenterSublabel))
	}
	if p.Style == CycleStyleFigureEight && (p.LeftLabel != "" || p.RightLabel != "") {
		bullets = append(bullets, strings.Trim(p.LeftLabel+" ↔ "+p.RightLabel, " ↔"))
	}
	for _, it := range p.Intake {
		line := "First: " + it.Label
		if it.Description != "" {
			line += " — " + it.Description
		}
		bullets = append(bullets, line)
	}
	for i, it := range p.Items {
		line := fmt.Sprintf("%d. %s", i+1, it.Label)
		if it.Description != "" {
			line += " — " + it.Description
		}
		bullets = append(bullets, line)
	}
	if len(p.Items) == 0 {
		return CompileFallback(in)
	}
	return contentFallback(in, p.Field, bullets)
}

// --- Regions ---

// cycleRegionMinHeightPct is the least share of a stacked group a cycle region
// reads in: a ring is as tall as it is wide, so it needs more of the height
// than any other region kind.
const cycleRegionMinHeightPct = 60

// regionCycle hosts the style's pattern in a region's cell.
func regionCycle(r map[string]any) (regionBuild, error) {
	if issues := CycleIssues(r); len(issues) > 0 {
		return regionBuild{}, fmt.Errorf("cycle region %s", issues[0].Message)
	}
	p := resolveCycle(r)
	block, err := p.pattern()
	if err != nil {
		return regionBuild{}, err
	}
	return regionBuild{cell: patternCell(block), links: p.links()}, nil
}

// CycleRegionWidthIssue reports a figure eight placed in a region that does
// not span the slide: two loops and their label columns need the full width.
// index is the region's position.
func CycleRegionWidthIssue(body map[string]any, index int) string {
	regions := RegionList(body)
	if index >= len(regions) || regions[index] == nil || strField(regions[index], "kind") != RegionCycle {
		return ""
	}
	if resolveCycle(regions[index]).Style != CycleStyleFigureEight {
		return ""
	}
	if regionSpansWidth(RegionArrangementOf(body), index) {
		return ""
	}
	return "style figure_eight needs the full slide width, and this region shares it; use style ring here (it draws a legend when the region is narrow), or stack the regions with arrangement rows"
}

// regionSpansWidth reports whether the region at index runs the full width of
// the content area in the arrangement.
func regionSpansWidth(arrangement string, index int) bool {
	switch arrangement {
	case ArrangeRows:
		return true
	case ArrangeMainTop, ArrangeMainBottom:
		return index == 0
	}
	return false
}
