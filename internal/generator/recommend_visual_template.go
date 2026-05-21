package generator

import (
	"fmt"
	"sort"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// This file makes recommend_visual (and, via the same helper, plan_deck)
// template-aware. Given a parsed *types.TemplateAnalysis, it reports whether
// each recommended visual candidate is supported, risky, or unsupported for that
// specific template — grounded in the same canonical-layout taxonomy, derivable-
// layout analysis, and font-aware placeholder capacities that generation and
// preflight share. The recommendation scorer in internal/patterns stays
// template-agnostic (it cannot import internal/template, which imports patterns);
// this enrichment lives in internal/generator, the single layer that already
// reaches both.

// minUsableBodyCharsForRecommend mirrors the body-capacity floor used by the
// canonical layout classifier (internal/template.minUsableBodyChars). A
// content-bearing layout whose largest body placeholder holds fewer characters
// than this cannot host substantial prose, so text-heavy candidates are flagged
// risky against it.
const minUsableBodyCharsForRecommend = 100

// TemplateSupportContext precomputes a template's canonical-layout coverage,
// derivable capabilities, grid-base readiness, body capacity, and palette so the
// support of any number of candidates can be evaluated cheaply. Build it once per
// template and reuse it across every candidate (recommend_visual) or planned
// slide (plan_deck).
type TemplateSupportContext struct {
	reg *patterns.Registry

	// familyLayouts maps each content-bearing canonical family to the names of
	// the native layouts that cover it.
	familyLayouts map[types.CanonicalLayoutFamily][]string
	// derivable maps a derivable-layout name to its readiness/missing analysis.
	derivable map[string]template.DerivableLayout

	nativeTwoContent bool // a layout classifies as Two Content
	nativeLargeImage bool // a layout carries a large image placeholder
	hasBlank         bool // a Blank canonical layout exists
	hasBlankTitle    bool // a Blank + Title canonical layout exists

	// gridBaseReady is true when the template can host an overlaid shape_grid
	// (patterns, charts, diagrams, compose, raw grids) on some title/body canvas.
	gridBaseReady   bool
	gridBaseMissing []string

	// maxBodyChars is the largest font-aware character capacity among body /
	// content placeholders across all layouts (0 when none carry text capacity).
	maxBodyChars int

	dataPalette      []string
	accentUsageGuide map[string]string
}

// NewTemplateSupportContext analyses a parsed template once for repeated
// per-candidate support evaluation. reg may be nil; when nil the default
// registry is used for pattern-density lookups.
func NewTemplateSupportContext(analysis *types.TemplateAnalysis, reg *patterns.Registry) *TemplateSupportContext {
	if reg == nil {
		reg = patterns.Default()
	}
	tc := &TemplateSupportContext{
		reg:           reg,
		familyLayouts: map[types.CanonicalLayoutFamily][]string{},
		derivable:     map[string]template.DerivableLayout{},
	}
	if analysis == nil {
		return tc
	}

	tc.familyLayouts = template.CanonicalFamilyCoverage(analysis.Layouts)
	for _, d := range template.DerivableLayouts(analysis.Layouts) {
		tc.derivable[d.Name] = d
	}

	for i := range analysis.Layouts {
		l := &analysis.Layouts[i]
		switch template.EffectiveCanonicalType(l) {
		case types.CanonicalLayoutTwoContent:
			tc.nativeTwoContent = true
		case types.CanonicalLayoutBlank:
			tc.hasBlank = true
		case types.CanonicalLayoutBlankTitle:
			tc.hasBlankTitle = true
		}
		for j := range l.Placeholders {
			ph := l.Placeholders[j]
			switch ph.Type {
			case types.PlaceholderTitle, types.PlaceholderBody, types.PlaceholderContent:
				tc.gridBaseReady = true
			case types.PlaceholderImage:
				if ph.Bounds.Width > emuLargeImageWidth && ph.Bounds.Height > emuLargeImageHeight {
					tc.nativeLargeImage = true
				}
			}
			if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
				if ph.MaxChars > tc.maxBodyChars {
					tc.maxBodyChars = ph.MaxChars
				}
			}
		}
	}
	if tc.hasBlankTitle {
		tc.gridBaseReady = true
	}
	if !tc.gridBaseReady {
		tc.gridBaseMissing = []string{"no title, body, or blank-title canvas to overlay a shape_grid on"}
	}

	if analysis.Metadata != nil {
		tc.dataPalette = analysis.Metadata.DataPalette
		tc.accentUsageGuide = analysis.Metadata.AccentUsageGuide
	}
	return tc
}

// emuLargeImageWidth/Height mirror the "large image" thresholds used by the
// canonical classifier (internal/template/classifier.go). They are duplicated
// here because those constants are unexported; keep them in sync.
const (
	emuLargeImageWidth  int64 = 4500000
	emuLargeImageHeight int64 = 3400000
)

// Support evaluates a single candidate's fit for the template. category and name
// match patterns.VisualCandidate fields. hints may be nil.
func (tc *TemplateSupportContext) Support(category patterns.VisualCategory, name string, hints *patterns.VisualHints) *patterns.TemplateSupport {
	switch category {
	case patterns.VisualCategoryPlaceholder:
		return tc.placeholderSupport(name, hints)
	case patterns.VisualCategoryPattern:
		return tc.patternSupport(name)
	case patterns.VisualCategoryChart:
		return tc.overlaySupport("chart", true)
	case patterns.VisualCategoryDiagram:
		return tc.overlaySupport("diagram", true)
	case patterns.VisualCategoryCompose:
		return tc.overlaySupport("compose envelope", false)
	case patterns.VisualCategoryShapeGrid:
		return tc.overlaySupport("raw shape_grid", false)
	default:
		return tc.overlaySupport("visual", false)
	}
}

// placeholderSupport resolves a placeholder-layout slide type (title, section,
// content, two-column, image, blank) against the template's canonical coverage,
// derivable layouts, and body capacity.
func (tc *TemplateSupportContext) placeholderSupport(slideType string, hints *patterns.VisualHints) *patterns.TemplateSupport {
	switch slideType {
	case "title":
		return tc.familyRequirement(types.LayoutFamilyTitleSlide, "Title Slide", nil)
	case "section":
		// A divider is visually forgiving: when no dedicated divider exists, a
		// Title Slide or Blank+Title can be repurposed rather than failing.
		repurpose := tc.repurposableDividerLayouts()
		return tc.familyRequirement(types.LayoutFamilySectionDivider, "Section Divider", repurpose)
	case "content":
		return tc.contentSupport(hints)
	case "two-column":
		return tc.twoColumnSupport()
	case "image":
		return tc.imageSupport()
	case "blank":
		if tc.hasBlank || tc.hasBlankTitle {
			return supported("Blank", "template provides a blank canvas layout")
		}
		return unsupported("Blank", "template has no Blank or Blank + Title layout")
	default:
		// Unknown placeholder type — treat as a content slide.
		return tc.contentSupport(hints)
	}
}

// familyRequirement is the common path for placeholder types that map directly
// to a canonical family. When the family is absent it returns risky if any
// repurposable layout exists, otherwise unsupported.
func (tc *TemplateSupportContext) familyRequirement(fam types.CanonicalLayoutFamily, required string, repurpose []string) *patterns.TemplateSupport {
	if layouts := tc.familyLayouts[fam]; len(layouts) > 0 {
		return supported(required, fmt.Sprintf("native %s layout present: %s", required, joinNames(layouts)))
	}
	if len(repurpose) > 0 {
		return risky(required, fmt.Sprintf("no dedicated %s layout; can repurpose %s", required, joinNames(repurpose)))
	}
	return unsupported(required, fmt.Sprintf("template has no %s layout and no layout to repurpose as one", required))
}

// repurposableDividerLayouts lists layouts a section divider can borrow when the
// template lacks a dedicated one.
func (tc *TemplateSupportContext) repurposableDividerLayouts() []string {
	var out []string
	out = append(out, tc.familyLayouts[types.LayoutFamilyTitleSlide]...)
	if tc.hasBlankTitle {
		out = append(out, string(types.CanonicalLayoutBlankTitle))
	}
	return out
}

// contentSupport resolves a standard content slide: a native One Content family
// layout, else an overlay on any grid base. Layers a body-capacity caveat.
func (tc *TemplateSupportContext) contentSupport(hints *patterns.VisualHints) *patterns.TemplateSupport {
	var ts *patterns.TemplateSupport
	if layouts := tc.familyLayouts[types.LayoutFamilyOneContent]; len(layouts) > 0 {
		ts = supported("One Content", fmt.Sprintf("native content layout present: %s", joinNames(layouts)))
	} else if tc.gridBaseReady {
		ts = risky("One Content", "no native One Content layout; content overlaid on a title/body canvas")
	} else {
		return unsupported("One Content", tc.gridBaseMissing...)
	}
	tc.applyBodyCapacity(ts, hints)
	return ts
}

// twoColumnSupport resolves a two-column slide: native Two Content, else a One
// Content layout the synthesiser can split, else unsupported.
func (tc *TemplateSupportContext) twoColumnSupport() *patterns.TemplateSupport {
	if tc.nativeTwoContent {
		return supported("Two Content", "native Two Content layout present")
	}
	if d, ok := tc.derivable["two-content"]; ok && d.Ready {
		return risky("Two Content", "synthesised by splitting a One Content layout into two columns")
	}
	return unsupported("Two Content", tc.derivableMissing("two-content", "no Two Content layout and no One Content layout to split")...)
}

// imageSupport resolves an image-focused slide: a native large image
// placeholder, else a blank canvas for a full-bleed picture, else unsupported.
func (tc *TemplateSupportContext) imageSupport() *patterns.TemplateSupport {
	if tc.nativeLargeImage {
		return supported("full-image", "native large image placeholder present")
	}
	if d, ok := tc.derivable["full-image"]; ok && d.Ready {
		return risky("full-image", "no native large image placeholder; image placed full-bleed on a blank canvas")
	}
	return unsupported("full-image", tc.derivableMissing("full-image", "no large image placeholder and no blank canvas for a full-bleed image")...)
}

// patternSupport resolves a named pattern. Patterns expand into a shape_grid
// overlaid on the slide content area, so they require a grid base; the per-cell
// budgets they enforce are template-independent, hence no body-capacity caveat.
func (tc *TemplateSupportContext) patternSupport(name string) *patterns.TemplateSupport {
	if !tc.gridBaseReady {
		return unsupported("grid base", tc.gridBaseMissing...)
	}
	reason := "pattern expands into a shape_grid overlaid on the slide content area"
	if tc.reg != nil {
		if p, ok := tc.reg.Get(name); ok {
			if dc := p.Taxonomy().DensityClass; dc != "" {
				reason = fmt.Sprintf("%s (%s-density pattern; cell budgets are template-independent)", reason, dc)
			}
		}
	}
	return supported("grid base", reason)
}

// overlaySupport resolves any visual that renders into the slide content area as
// an overlay (chart, diagram, compose envelope, raw shape_grid). usesPalette
// adds a data-palette note for colour-bearing visuals.
func (tc *TemplateSupportContext) overlaySupport(kind string, usesPalette bool) *patterns.TemplateSupport {
	if !tc.gridBaseReady {
		return unsupported("grid base", tc.gridBaseMissing...)
	}
	ts := supported("grid base", fmt.Sprintf("%s hosted on the slide content area", kind))
	if usesPalette {
		if len(tc.dataPalette) > 0 {
			ts.Reasons = append(ts.Reasons, fmt.Sprintf("template data_palette (%d colours) will style series", len(tc.dataPalette)))
		} else if len(tc.accentUsageGuide) > 0 {
			ts.Reasons = append(ts.Reasons, "no data_palette; series fall back to accent rotation (see accent_usage_guide)")
		} else {
			ts.Reasons = append(ts.Reasons, "no data_palette; series fall back to the default accent rotation")
		}
	}
	return ts
}

// applyBodyCapacity downgrades a supported text candidate to risky when the
// template's largest body placeholder cannot hold substantial prose, or when the
// caller's item count is large relative to that capacity.
func (tc *TemplateSupportContext) applyBodyCapacity(ts *patterns.TemplateSupport, hints *patterns.VisualHints) {
	if ts == nil || ts.Status == patterns.TemplateSupportUnsupported {
		return
	}
	if tc.maxBodyChars == 0 {
		return // no body placeholder carries a capacity estimate; nothing to assert
	}
	if tc.maxBodyChars < minUsableBodyCharsForRecommend {
		downgradeToRisky(ts, fmt.Sprintf("largest body placeholder holds ~%d chars; text-heavy content may overflow", tc.maxBodyChars))
		return
	}
	if hints != nil && hints.ItemCount > 0 {
		// ~40 chars per item is a conservative budget; flag when the body can't
		// plausibly hold the requested item count.
		const charsPerItem = 40
		if tc.maxBodyChars < hints.ItemCount*charsPerItem {
			downgradeToRisky(ts, fmt.Sprintf("%d items vs ~%d-char body capacity; consider a denser layout or fewer items", hints.ItemCount, tc.maxBodyChars))
		}
	}
}

// derivableMissing returns the recorded Missing reasons for a derivable layout,
// or the supplied fallback when none are recorded.
func (tc *TemplateSupportContext) derivableMissing(name, fallback string) []string {
	if d, ok := tc.derivable[name]; ok && len(d.Missing) > 0 {
		return d.Missing
	}
	return []string{fallback}
}

// AnnotateTemplateSupport sets TemplateSupport on every candidate in result
// using the supplied template analysis. It is a no-op when result or analysis is
// nil. reg may be nil (defaults to the standard registry).
func AnnotateTemplateSupport(result *patterns.RecommendVisualResult, analysis *types.TemplateAnalysis, hints *patterns.VisualHints, reg *patterns.Registry) {
	if result == nil || analysis == nil {
		return
	}
	tc := NewTemplateSupportContext(analysis, reg)
	for i := range result.Candidates {
		c := &result.Candidates[i]
		c.TemplateSupport = tc.Support(c.Category, c.Name, hints)
	}
}

// ReorderByTemplateSupport stably re-sorts candidates so that, all else equal,
// template-supported candidates outrank risky ones and risky outrank
// unsupported. The displayed Score is left untouched (it remains the intent-match
// score); ordering uses an effective score that subtracts a support penalty so a
// strong intent match can still beat a weak supported candidate. It is a no-op
// when no candidate carries a TemplateSupport annotation.
func ReorderByTemplateSupport(result *patterns.RecommendVisualResult) {
	if result == nil || len(result.Candidates) < 2 {
		return
	}
	annotated := false
	for i := range result.Candidates {
		if result.Candidates[i].TemplateSupport != nil {
			annotated = true
			break
		}
	}
	if !annotated {
		return
	}
	sort.SliceStable(result.Candidates, func(i, j int) bool {
		ei := result.Candidates[i].Score - supportPenalty(result.Candidates[i].TemplateSupport)
		ej := result.Candidates[j].Score - supportPenalty(result.Candidates[j].TemplateSupport)
		if ei != ej {
			return ei > ej
		}
		return result.Candidates[i].Name < result.Candidates[j].Name
	})
}

// supportPenalty maps a support status to the score deduction used when ordering.
func supportPenalty(ts *patterns.TemplateSupport) float64 {
	if ts == nil {
		return 0
	}
	switch ts.Status {
	case patterns.TemplateSupportRisky:
		return 0.15
	case patterns.TemplateSupportUnsupported:
		return 0.5
	default:
		return 0
	}
}

// --- small constructors ---

func supported(required string, reasons ...string) *patterns.TemplateSupport {
	return &patterns.TemplateSupport{Status: patterns.TemplateSupportSupported, RequiredLayout: required, Reasons: reasons}
}

func risky(required string, reasons ...string) *patterns.TemplateSupport {
	return &patterns.TemplateSupport{Status: patterns.TemplateSupportRisky, RequiredLayout: required, Reasons: reasons}
}

func unsupported(required string, reasons ...string) *patterns.TemplateSupport {
	if len(reasons) == 0 {
		reasons = []string{fmt.Sprintf("template cannot provide the required %s", required)}
	}
	return &patterns.TemplateSupport{Status: patterns.TemplateSupportUnsupported, RequiredLayout: required, Reasons: reasons}
}

// downgradeToRisky lowers a supported status to risky and appends the caveat.
// A status already risky/unsupported keeps its status; the caveat is still added.
func downgradeToRisky(ts *patterns.TemplateSupport, reason string) {
	if ts.Status == patterns.TemplateSupportSupported {
		ts.Status = patterns.TemplateSupportRisky
	}
	ts.Reasons = append(ts.Reasons, reason)
}

// joinNames renders a slice of layout names for a reason string.
func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return "(none)"
	case 1:
		return names[0]
	}
	out := names[0]
	for _, n := range names[1:] {
		out += ", " + n
	}
	return out
}
