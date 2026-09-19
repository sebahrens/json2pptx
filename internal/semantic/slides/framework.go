package slides

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Framework slides (go-slide-creator-anzx).
//
// SWOT, Porter's five forces and the Business Model Canvas are named things
// with fixed parts: what goes in them is not a design choice, and an author
// writing one should not have to know whether the engine draws it as a diagram
// or as a shape grid. One kind takes all three — `framework` names which, and
// `sections` carries the parts by their own names — and the compiler routes it.

const (
	// frameworkSWOT, frameworkPorters and frameworkBMC are the frameworks this
	// kind compiles.
	frameworkSWOT    = "swot"
	frameworkPorters = "porters_five_forces"
	frameworkBMC     = "bmc"

	// frameworkItemMax is the per-item character budget. It is bmc-canvas's own
	// bullet budget, applied to all three so an author is not told a different
	// number per framework.
	frameworkItemMax = 200
	// frameworkSectionItemsMax is the per-section item cap. bmc-canvas enforces
	// it; for the two diagrams it is the point past which the renderer shrinks
	// the text rather than the layout.
	frameworkSectionItemsMax = 10
)

// frameworkSpellings maps what an author writes onto the framework it names.
var frameworkSpellings = map[string]string{
	"swot":                  frameworkSWOT,
	"swot_analysis":         frameworkSWOT,
	"porters":               frameworkPorters,
	"porters_five_forces":   frameworkPorters,
	"porter_five_forces":    frameworkPorters,
	"five_forces":           frameworkPorters,
	"bmc":                   frameworkBMC,
	"business_model_canvas": frameworkBMC,
	"bmc_canvas":            frameworkBMC,
	"business_model":        frameworkBMC,
}

// frameworkSection is one part of a framework: the key the renderer wants, the
// heading a reader sees, and the spellings an author may have used.
type frameworkSection struct {
	key      string
	heading  string
	synonyms []string
}

// frameworkSections lists each framework's parts, in the order they are read.
var frameworkSections = map[string][]frameworkSection{
	frameworkSWOT: {
		{key: "strengths", heading: "Strengths", synonyms: []string{"strength", "s"}},
		{key: "weaknesses", heading: "Weaknesses", synonyms: []string{"weakness", "w"}},
		{key: "opportunities", heading: "Opportunities", synonyms: []string{"opportunity", "o"}},
		{key: "threats", heading: "Threats", synonyms: []string{"threat", "t"}},
	},
	frameworkPorters: {
		{key: "rivalry", heading: "Competitive rivalry", synonyms: []string{"competitive_rivalry", "competition"}},
		{key: "new_entrants", heading: "Threat of new entrants", synonyms: []string{"threat_of_new_entrants", "entrants"}},
		{key: "substitutes", heading: "Threat of substitutes", synonyms: []string{"threat_of_substitutes", "substitution"}},
		{key: "suppliers", heading: "Bargaining power of suppliers", synonyms: []string{"supplier_power", "bargaining_power_of_suppliers"}},
		{key: "buyers", heading: "Bargaining power of buyers", synonyms: []string{"buyer_power", "bargaining_power_of_buyers", "customers"}},
	},
	frameworkBMC: {
		{key: "key_partners", heading: "Key partners", synonyms: []string{"partners"}},
		{key: "key_activities", heading: "Key activities", synonyms: []string{"activities"}},
		{key: "key_resources", heading: "Key resources", synonyms: []string{"resources"}},
		{key: "value_propositions", heading: "Value propositions", synonyms: []string{"value_proposition", "value"}},
		{key: "customer_relations", heading: "Customer relationships", synonyms: []string{"customer_relationships", "relationships"}},
		{key: "channels", heading: "Channels", synonyms: []string{"channel"}},
		{key: "customer_segments", heading: "Customer segments", synonyms: []string{"segments", "customers"}},
		{key: "cost_structure", heading: "Cost structure", synonyms: []string{"costs"}},
		{key: "revenue_streams", heading: "Revenue streams", synonyms: []string{"revenue", "revenues"}},
	},
}

// bmcCell is one cell of the bmc-canvas pattern.
type bmcCell struct {
	Header  string   `json:"header"`
	Bullets []string `json:"bullets"`
}

// CompileFramework compiles a named framework onto the visual that draws it,
// falling back to grouped bullets when the payload is not complete enough.
func CompileFramework(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	name := FrameworkName(in.Body)
	sections := FrameworkContent(in.Body)
	if !frameworkComplete(name, sections) || FrameworkOverBudget(in.Body) != "" {
		return compileFrameworkFallback(in, name, sections)
	}

	switch name {
	case frameworkBMC:
		return compileBMC(in, sections)
	case frameworkSWOT, frameworkPorters:
		return compileFrameworkDiagram(in, name, sections)
	default:
		return compileFrameworkFallback(in, name, sections)
	}
}

// compileBMC fills the bmc-canvas pattern's nine cells, supplying each cell's
// canonical heading so the canvas reads as the canvas rather than as whatever
// the author happened to call its boxes.
func compileBMC(in Input, sections map[string][]string) (*deckinput.SlideInput, []SourceLink, error) {
	cells := map[string]bmcCell{}
	for _, s := range frameworkSections[frameworkBMC] {
		cells[s.key] = bmcCell{Header: s.heading, Bullets: sections[s.key]}
	}
	encoded, err := json.Marshal(cells)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal bmc-canvas values: %w", err)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "bmc-canvas", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values",
		SemanticPath: in.semSlide() + ".sections",
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileFrameworkDiagram emits the native diagram that draws SWOT or the five
// forces, in the data shape that renderer reads.
func compileFrameworkDiagram(in Input, name string, sections map[string][]string) (*deckinput.SlideInput, []SourceLink, error) {
	data := map[string]any{}
	for _, s := range frameworkSections[name] {
		items := sections[s.key]
		if name == frameworkPorters {
			// The five-forces renderer reads each force as an object carrying
			// its supporting factors, not as a bare list. An intensity the
			// author stated is passed through; one they did not is left to the
			// renderer's own default rather than invented here.
			force := map[string]any{"label": s.heading, "factors": items}
			if intensity, ok := frameworkIntensity(in.Body, s); ok {
				force["intensity"] = intensity
			}
			data[s.key] = force
			continue
		}
		data[s.key] = items
	}

	// A native diagram needs a body placeholder to land in; blank-title has
	// none, and the content would be dropped on the floor with a warning.
	slide := &deckinput.SlideInput{SlideType: "diagram"}
	links := titleLink(slide, in)
	idx := appendContent(slide, diagramContent("body", &types.DiagramSpec{Type: name, Data: data}))
	links = append(links, SourceLink{
		RawPath:      fmt.Sprintf("%s.content[%d].diagram_value", in.rawSlide(), idx),
		SemanticPath: in.semSlide() + ".sections",
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileFrameworkFallback groups each section under its own heading, so an
// incomplete framework still reads as the framework rather than as a pile of
// bullets.
func compileFrameworkFallback(in Input, name string, sections map[string][]string) (*deckinput.SlideInput, []SourceLink, error) {
	order := frameworkSections[name]
	if len(order) == 0 {
		// An unnamed framework — or one this kind does not draw, like a PESTEL —
		// still has everything the author wrote. Losing it to a title-only slide
		// would be the silent drop this kind exists to avoid.
		if bullets := unknownFrameworkBullets(in.Body); len(bullets) > 0 {
			return contentFallback(in, "sections", bullets)
		}
		return CompileFallback(in)
	}

	var bullets []string
	for _, s := range order {
		items := sections[s.key]
		if len(items) == 0 {
			continue
		}
		bullets = append(bullets, s.heading+" — "+strings.Join(items, "; "))
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}
	return contentFallback(in, "sections", bullets)
}

// frameworkComplete reports whether every part of the named framework carries
// content. A SWOT missing its threats is not a SWOT, so a partial one is drawn
// as text rather than as a visual with an empty quadrant.
func frameworkComplete(name string, sections map[string][]string) bool {
	order := frameworkSections[name]
	if len(order) == 0 {
		return false
	}
	for _, s := range order {
		if len(sections[s.key]) == 0 {
			return false
		}
	}
	return true
}

// unknownFrameworkBullets groups whatever sections a payload carries, under
// headings made from their own keys, in a stable order.
func unknownFrameworkBullets(body map[string]any) []string {
	sections, ok := body["sections"].(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(sections))
	for key := range sections {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var bullets []string
	for _, key := range keys {
		items := frameworkItems(sections[key])
		if len(items) == 0 {
			continue
		}
		bullets = append(bullets, humaniseSectionKey(key)+" — "+strings.Join(items, "; "))
	}
	return bullets
}

// humaniseSectionKey turns a payload key into a heading a reader can read.
func humaniseSectionKey(key string) string {
	words := strings.ReplaceAll(strings.TrimSpace(key), "_", " ")
	if words == "" {
		return key
	}
	return strings.ToUpper(words[:1]) + words[1:]
}

// FrameworkName resolves which framework the slide names, or "" when it names
// none this kind knows.
func FrameworkName(body map[string]any) string {
	raw := firstNonEmpty(strField(body, "framework"), strField(body, "type"), strField(body, "model"))
	key := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(raw)), " ", "_")
	key = strings.ReplaceAll(key, "'", "")
	return frameworkSpellings[key]
}

// FrameworkContent resolves the framework's parts, keyed by the canonical
// section name. Sections live under "sections" or at the top level, and a
// section is a list of strings or a single string.
func FrameworkContent(body map[string]any) map[string][]string {
	name := FrameworkName(body)
	order := frameworkSections[name]
	if len(order) == 0 {
		return nil
	}
	scope := body
	if nested, ok := body["sections"].(map[string]any); ok {
		scope = nested
	}

	out := map[string][]string{}
	for _, s := range order {
		for _, key := range append([]string{s.key}, s.synonyms...) {
			if items := frameworkItems(scope[key]); len(items) > 0 {
				out[s.key] = items
				break
			}
		}
	}
	return out
}

// frameworkIntensity reads a force's stated intensity (0.0-1.0), for the five
// forces. "high" / "medium" / "low" are accepted as the numbers they stand for,
// because that is how an author says it.
func frameworkIntensity(body map[string]any, section frameworkSection) (float64, bool) {
	scope := body
	if nested, ok := body["sections"].(map[string]any); ok {
		scope = nested
	}
	for _, key := range append([]string{section.key}, section.synonyms...) {
		m, ok := scope[key].(map[string]any)
		if !ok {
			continue
		}
		switch v := m["intensity"].(type) {
		case float64:
			return math.Max(0, math.Min(1, v)), true
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "high":
				return 0.85, true
			case "medium", "moderate":
				return 0.5, true
			case "low":
				return 0.15, true
			}
		}
	}
	return 0, false
}

// frameworkItems reads one section: a list of strings, a single string, or an
// object carrying items.
func frameworkItems(v any) []string {
	switch t := v.(type) {
	case string:
		if s := strings.TrimSpace(t); s != "" {
			return []string{s}
		}
	case []any:
		var out []string
		for _, e := range t {
			if s, ok := e.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return out
	case map[string]any:
		for _, key := range []string{"items", "bullets", "factors", "points"} {
			if items := frameworkItems(t[key]); len(items) > 0 {
				return items
			}
		}
		if s := firstNonEmpty(strField(t, "description"), strField(t, "detail"), strField(t, "summary")); s != "" {
			return []string{s}
		}
	}
	return nil
}

// FrameworkOverBudget explains why a framework cannot take its visual, or ""
// when it fits. A framework is a named thing with fixed parts: a SWOT missing
// its threats is not a SWOT, so an incomplete one degrades rather than
// rendering an empty quadrant.
func FrameworkOverBudget(body map[string]any) string {
	name := FrameworkName(body)
	if name == "" {
		if raw := firstNonEmpty(strField(body, "framework"), strField(body, "type"), strField(body, "model")); raw != "" {
			return fmt.Sprintf("names %q, which is not a framework this kind draws (swot, porters_five_forces, bmc)", raw)
		}
		return "does not say which framework it is (framework: swot, porters_five_forces or bmc)"
	}
	order := frameworkSections[name]
	sections := FrameworkContent(body)
	if len(sections) == 0 {
		return ""
	}

	var missing []string
	for _, s := range order {
		if len(sections[s.key]) == 0 {
			missing = append(missing, s.key)
		}
	}
	if len(missing) > 0 {
		return fmt.Sprintf("is missing %s; the visual draws every part or none of it", strings.Join(missing, ", "))
	}
	for _, s := range order {
		items := sections[s.key]
		if len(items) > frameworkSectionItemsMax {
			return fmt.Sprintf("has %d items under %s; a section holds %d", len(items), s.key, frameworkSectionItemsMax)
		}
		for _, item := range items {
			if runeLen(item) > frameworkItemMax {
				return fmt.Sprintf("has a %d-character item under %s; a section's items hold %d", runeLen(item), s.key, frameworkItemMax)
			}
		}
	}
	return ""
}

// FrameworkPattern returns the pattern a framework compiles to. SWOT and the
// five forces are native diagrams rather than patterns, so they report "" —
// the plan advertises a layout only, and explain matches what compile emits.
func FrameworkPattern(body map[string]any) string {
	name := FrameworkName(body)
	if name != frameworkBMC {
		return ""
	}
	if frameworkComplete(name, FrameworkContent(body)) && FrameworkOverBudget(body) == "" {
		return "bmc-canvas"
	}
	return ""
}

// UsableFrameworkSectionCount returns how many of the framework's parts carry
// content, so validation counts what compile will draw.
func UsableFrameworkSectionCount(body map[string]any) int { return len(FrameworkContent(body)) }
