package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// Architecture slides (go-slide-creator-162os).
//
// A 10-slide product-launch deck needed raw_json2pptx for its platform
// architecture: the DeckSpec had no kind for a tiered stack, so the agent
// dropped to the raw pattern dialect — values-object vs values-array, label vs
// name — and lost every semantic_path diagnostic on the way. The payload here
// is the one an author would write ({tiers: [{label, items}], rails}), and the
// compiler maps it onto the arch-stack pattern.
//
// The pattern's budgets are real: 3–6 tiers, a 60-character label, a
// 120-character description, at most 3 side rails. A payload outside them
// degrades to a bullet list rather than being truncated into the pattern —
// losing a tier's second half to a maxLength is worse than rendering the whole
// thing as text.

const (
	// archStackMinTiers / archStackMaxTiers mirror the pattern's own bounds.
	archStackMinTiers = 3
	archStackMaxTiers = 6
	// archStackLabelMax and archStackDescMax mirror the pattern's string budgets.
	archStackLabelMax = 60
	archStackDescMax  = 120
	// archStackMaxRails is how many cross-cutting rails the pattern draws.
	archStackMaxRails = 3
)

// archTier is one resolved tier of an architecture payload.
type archTier struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// archStackValues is the arch-stack pattern's values object.
type archStackValues struct {
	Tiers     []archTier `json:"tiers"`
	SideRails []string   `json:"side_rails,omitempty"`
}

// CompileArchitecture compiles an architecture payload onto the arch-stack
// pattern, falling back to a bullet list when the payload does not fit the
// pattern's budgets.
func CompileArchitecture(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	tiers := ArchitectureTiers(in.Body)
	if !archStackFits(tiers, architectureRails(in.Body)) {
		return compileArchitectureFallback(in, tiers)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	var links []SourceLink
	links = append(links, titleLink(slide, in)...)

	values := archStackValues{Tiers: tiers, SideRails: architectureRails(in.Body)}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal arch-stack values: %w", err)
	}
	slide.Pattern = &deckinput.PatternInput{Name: "arch-stack", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.tiers",
		SemanticPath: in.semSlide() + ".tiers",
	})
	if len(values.SideRails) > 0 {
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".pattern.values.side_rails",
			SemanticPath: in.semSlide() + "." + architectureRailsField(in.Body),
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileArchitectureFallback renders the stack as "Tier — detail" bullets, top
// tier first, so nothing is lost when the pattern cannot take the payload.
func compileArchitectureFallback(in Input, tiers []archTier) (*deckinput.SlideInput, []SourceLink, error) {
	bullets := make([]string, 0, len(tiers))
	for _, t := range tiers {
		if t.Description == "" {
			bullets = append(bullets, t.Label)
			continue
		}
		bullets = append(bullets, t.Label+" — "+t.Description)
	}
	for _, rail := range architectureRails(in.Body) {
		bullets = append(bullets, "Across all tiers: "+rail)
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "content"}
	var links []SourceLink
	links = append(links, titleLink(slide, in)...)
	idx := appendContent(slide, bulletsContent("body", bullets))
	links = append(links, SourceLink{
		RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
		SemanticPath: in.semSlide() + ".tiers",
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// ArchitectureTiers resolves an architecture payload's tiers. A tier is a
// string, or an object carrying a label and either a description or a list of
// items (joined with ", ", which is how a stack diagram reads a layer's
// contents). Tiers with no usable label are dropped.
func ArchitectureTiers(body map[string]any) []archTier {
	raw, ok := firstList(body, "tiers", "layers")
	if !ok {
		return nil
	}
	var out []archTier
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				out = append(out, archTier{Label: s})
			}
		case map[string]any:
			label := firstNonEmpty(strField(t, "label"), strField(t, "name"), strField(t, "title"), strField(t, "tier"), strField(t, "layer"))
			if label == "" {
				continue
			}
			out = append(out, archTier{Label: label, Description: archTierDescription(t)})
		}
	}
	return out
}

// archTierDescription resolves a tier's detail line: an explicit description, or
// the tier's items joined into one.
func archTierDescription(tier map[string]any) string {
	if d := firstNonEmpty(strField(tier, "description"), strField(tier, "detail"), strField(tier, "summary"), strField(tier, "text")); d != "" {
		return d
	}
	items, ok := firstList(tier, "items", "components", "services", "elements")
	if !ok {
		return ""
	}
	var parts []string
	for _, item := range items {
		switch v := item.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				parts = append(parts, s)
			}
		case map[string]any:
			if s := firstNonEmpty(strField(v, "label"), strField(v, "name"), strField(v, "title"), strField(v, "text")); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, ", ")
}

// architectureRails resolves the cross-cutting concerns drawn as side rails.
func architectureRails(body map[string]any) []string {
	raw, ok := firstList(body, "rails", "side_rails", "cross_cutting")
	if !ok {
		return nil
	}
	var out []string
	for _, e := range raw {
		switch v := e.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				out = append(out, s)
			}
		case map[string]any:
			if s := firstNonEmpty(strField(v, "label"), strField(v, "name"), strField(v, "title")); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// architectureRailsField names the payload key the rails came from, for the
// source map: a diagnostic has to point at the field the author wrote.
func architectureRailsField(body map[string]any) string {
	for _, key := range []string{"rails", "side_rails", "cross_cutting"} {
		if _, ok := body[key]; ok {
			return key
		}
	}
	return "rails"
}

// ArchitecturePatternFeasible reports whether an architecture payload will
// actually compile to arch-stack. The plan projection calls it so explain and
// compile agree on the visual.
func ArchitecturePatternFeasible(body map[string]any) bool {
	return archStackFits(ArchitectureTiers(body), architectureRails(body))
}

// archStackFits reports whether tiers and rails sit inside the pattern's
// budgets.
func archStackFits(tiers []archTier, rails []string) bool {
	if len(tiers) < archStackMinTiers || len(tiers) > archStackMaxTiers {
		return false
	}
	if len(rails) > archStackMaxRails {
		return false
	}
	for _, t := range tiers {
		if len([]rune(t.Label)) > archStackLabelMax || len([]rune(t.Description)) > archStackDescMax {
			return false
		}
	}
	for _, r := range rails {
		if len([]rune(r)) > archStackRailMax {
			return false
		}
	}
	return true
}

// archStackRailMax mirrors the pattern's side-rail string budget.
const archStackRailMax = 30

// ArchitectureOverBudget describes the first budget an architecture payload
// breaks, for the validator's advisory. It returns "" when the payload fits.
func ArchitectureOverBudget(body map[string]any) string {
	tiers := ArchitectureTiers(body)
	rails := architectureRails(body)
	switch {
	case len(tiers) < archStackMinTiers || len(tiers) > archStackMaxTiers:
		return fmt.Sprintf("has %d usable tiers; %d–%d render as an arch-stack visual", len(tiers), archStackMinTiers, archStackMaxTiers)
	case len(rails) > archStackMaxRails:
		return fmt.Sprintf("has %d rails; arch-stack draws at most %d", len(rails), archStackMaxRails)
	}
	for i, t := range tiers {
		if n := len([]rune(t.Label)); n > archStackLabelMax {
			return fmt.Sprintf("tier %d's label is %d characters; arch-stack allows %d", i+1, n, archStackLabelMax)
		}
		if n := len([]rune(t.Description)); n > archStackDescMax {
			return fmt.Sprintf("tier %d's detail is %d characters; arch-stack allows %d", i+1, n, archStackDescMax)
		}
	}
	for i, r := range rails {
		if n := len([]rune(r)); n > archStackRailMax {
			return fmt.Sprintf("rail %d is %d characters; arch-stack allows %d", i+1, n, archStackRailMax)
		}
	}
	return ""
}

// UsableTierCount is the count the validator reports on.
func UsableTierCount(body map[string]any) int {
	return len(ArchitectureTiers(body))
}

// firstList returns the first of the named keys holding a JSON array. An author
// writing "layers" instead of "tiers" or "side_rails" instead of "rails" gets
// the same slide rather than an empty one.
func firstList(body map[string]any, keys ...string) ([]any, bool) {
	if body == nil {
		return nil, false
	}
	for _, key := range keys {
		if list, ok := body[key].([]any); ok {
			return list, true
		}
	}
	return nil, false
}
