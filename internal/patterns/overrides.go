package patterns

import (
	"fmt"
	"hash/fnv"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// TextOverrides contains pattern-level overrides common to patterns with
// header/body text: accent color, header font size, and body font size.
// Patterns with identical override shapes (card-grid, comparison-2col)
// alias this type directly. Patterns with extra fields (matrix-2x2) embed it.
type TextOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	HeaderSize     float64 `json:"header_size,omitempty"`
	BodySize       float64 `json:"body_size,omitempty"`
	CellAccentMode string  `json:"cell_accent_mode,omitempty"` // uniform | alternate | progressive
}

// ValidSemanticAccents is the set of recognised semantic accent roles.
var ValidSemanticAccents = map[string]bool{
	"positive": true,
	"negative": true,
	"neutral":  true,
}

// AccentStrategy controls how the default accent color is chosen for patterns
// that don't set an explicit accent.
type AccentStrategy string

const (
	// AccentStrategyPrimary always uses accent1 (the legacy default).
	AccentStrategyPrimary AccentStrategy = "primary"
	// AccentStrategyRotate starts at a content-stable slot and skips unreadable
	// or negative-semantic accents when template colours are available.
	AccentStrategyRotate AccentStrategy = "rotate"
	// AccentStrategySectionKeyed assigns one accent per section (wraps at 6).
	AccentStrategySectionKeyed AccentStrategy = "section-keyed"
)

// NumAccentSlots is the number of OOXML accent color slots (accent1–accent6).
const NumAccentSlots = 6

// ValidAccentStrategies is the set of allowed AccentStrategy values.
var ValidAccentStrategies = []string{
	string(AccentStrategyPrimary),
	string(AccentStrategyRotate),
	string(AccentStrategySectionKeyed),
}

// IsValidAccentStrategy returns true if s is a recognised accent strategy.
func IsValidAccentStrategy(s string) bool {
	for _, v := range ValidAccentStrategies {
		if s == v {
			return true
		}
	}
	return false
}

// AccentForStrategy returns the default accent name (e.g. "accent2") for the
// given strategy, slide index, and section index. The caller should only use
// this when no explicit accent or semantic_accent was provided.
func AccentForStrategy(strategy AccentStrategy, slideIndex, sectionIndex int) string {
	switch strategy {
	case AccentStrategyRotate:
		return fmt.Sprintf("accent%d", (slideIndex%NumAccentSlots)+1)
	case AccentStrategySectionKeyed:
		return fmt.Sprintf("accent%d", (sectionIndex%NumAccentSlots)+1)
	default: // primary or empty
		return "accent1"
	}
}

// ResolveAccent returns the accent color for a pattern invocation.
// Priority: explicit accent > semantic_accent resolved via metadata > strategy-derived default > "accent1".
func ResolveAccent(accent, semanticAccent string, metadata *types.TemplateMetadata) string {
	if accent != "" {
		return accent
	}
	if semanticAccent != "" && metadata != nil && len(metadata.SemanticAccents) > 0 {
		if resolved, ok := metadata.SemanticAccents[semanticAccent]; ok {
			return resolved
		}
	}
	return "accent1"
}

// ResolveAccentWithStrategy is like ResolveAccent but uses the deck-level
// accent strategy to choose the default when neither explicit accent nor
// semantic_accent is specified.
func ResolveAccentWithStrategy(accent, semanticAccent string, metadata *types.TemplateMetadata, strategy AccentStrategy, slideIndex, sectionIndex int) string {
	if accent != "" {
		return accent
	}
	if semanticAccent != "" && metadata != nil && len(metadata.SemanticAccents) > 0 {
		if resolved, ok := metadata.SemanticAccents[semanticAccent]; ok {
			return resolved
		}
	}
	return AccentForStrategy(strategy, slideIndex, sectionIndex)
}

// rotatedAccent picks a content-stable slot from the template-safe rotation
// set: the theme accents that can carry normal-size lt1 text and are not the
// template's negative semantic hue. Explicit accent and semantic_accent
// overrides are resolved before this path.
//
// It used to start at slot index%6 and walk forward to the next safe accent,
// which piled every unsafe slot onto the same neighbour: on midnight-blue
// eight rotating slides landed on two accents, each with its own
// substitution finding (go-slide-creator-kspkr). Indexing into the safe set
// spreads slides evenly over it; with all six accents safe it is the old
// slot, so templates without unsafe accents are unchanged.
func (c ExpandContext) rotatedAccent() (chosen, requested, reason string) {
	var key uint64
	if c.RotationKey != "" {
		h := fnv.New64a()
		_, _ = h.Write([]byte(c.RotationKey))
		key = h.Sum64()
	} else {
		index := c.SlideIndex
		if index < 0 {
			index = -index
		}
		key = uint64(index)
	}
	requested = fmt.Sprintf("accent%d", key%NumAccentSlots+1)
	if len(c.Theme.Colors) == 0 {
		return requested, requested, ""
	}
	ink, ok := resolveThemeColor(c, "lt1")
	if !ok {
		ink = svggen.MustParseColor("#FFFFFF")
	}
	var safe, unsafe []string
	for slot := 1; slot <= NumAccentSlots; slot++ {
		candidate := fmt.Sprintf("accent%d", slot)
		if c.safeRotatingAccent(candidate, ink) {
			safe = append(safe, candidate)
		} else {
			unsafe = append(unsafe, candidate)
		}
	}
	if len(safe) > 0 {
		chosen = safe[key%uint64(len(safe))]
		if len(unsafe) > 0 {
			reason = fmt.Sprintf("rotation cycles through this template's safe accents %s; excluded: %s (unreadable under lt1 body text, or reserved as the negative semantic accent)",
				strings.Join(safe, ", "), strings.Join(unsafe, ", "))
		}
		return chosen, requested, reason
	}
	// If no theme accent qualifies, prefer the template's own dark brand colour
	// (dk2, a.k.a. tx2) over a literal: pastel-accent templates such as
	// blue-corporate otherwise paint every accent fill jet black
	// (go-slide-creator-csclk.93).
	if dk2, ok := resolveThemeColor(c, "dk2"); ok && dk2.ContrastWith(ink) >= svggen.WCAGAANormal {
		return "dk2", requested, "no theme accent is safe for lt1 body text; selected dk2"
	}
	// Otherwise a literal contrast-safe fill preserves readability.
	// ResolveCellAccent keeps non-accent fallback colours uniform.
	black := svggen.MustParseColor("#000000")
	white := svggen.MustParseColor("#FFFFFF")
	fallback := "#000000"
	if white.ContrastWith(ink) > black.ContrastWith(ink) {
		fallback = "#FFFFFF"
	}
	return fallback, requested, fmt.Sprintf("no theme accent is safe for lt1 body text; selected %s", fallback)
}

// minVisibleAccentContrast is the minimum fill-vs-lt1 contrast for a light
// accent to carry dark text instead of falling back to a dark fill. Below 2:1
// the slot is a pastel mid-tint (p-style's #FFAA72 at 1.87, its grey accent5
// at 1.92), not an accent: a roof or card in it reads as a muddy wash with
// black text (go-slide-creator-8xsj3), so rotation skips it.
const minVisibleAccentContrast = 2.0

// minAccentSaturation is the HSL saturation below which a theme accent slot
// is a grey and rotation skips it.
const minAccentSaturation = 0.15

func (c ExpandContext) safeRotatingAccent(candidate string, ink svggen.Color) bool {
	if c.Metadata != nil && candidate == c.Metadata.SemanticAccents["negative"] {
		return false
	}
	if candidate == c.Theme.SemanticAccents["negative"] {
		return false
	}
	fill, found := resolveThemeColor(c, candidate)
	if !found {
		return false
	}
	// A grey theme slot (p-style's accent4 #A1A8B3, abstract's #A5A5A5) is a
	// neutral, not an accent: rotating onto it paints the slide's one
	// emphasised element grey (go-slide-creator-8xsj3).
	if _, sat, _ := toHSL(fill); sat < minAccentSaturation {
		return false
	}
	if fill.ContrastWith(ink) >= svggen.WCAGAANormal {
		return true
	}
	// A light accent stays on-brand when a theme dark ink reads on it:
	// ApplyReadableInk swaps the pattern's lt1 text for that ink after
	// expansion, so the fill need not fall back to dk2 / black.
	// The fill must still read as a shape on the (lt1) slide: a near-white
	// accent with dark text would be an invisible card.
	if fill.ContrastWith(ink) < minVisibleAccentContrast {
		return false
	}
	for _, dark := range []string{"dk2", "dk1"} {
		if d, ok := resolveThemeColor(c, dark); ok && fill.ContrastWith(d) >= svggen.WCAGAANormal {
			return true
		}
	}
	return false
}

// ResolveCellAccent keeps automatic alternate/progressive fills within the
// same certified set as the base. Explicitly authored accents retain the
// historical six-slot variation contract.
func (c ExpandContext) ResolveCellAccent(base string, idx int, mode string) string {
	if !c.AutoAccent || c.AccentStrategy != AccentStrategyRotate || len(c.Theme.Colors) == 0 || mode == "" || mode == CellAccentUniform {
		return ResolveCellAccent(base, idx, mode)
	}
	ink, ok := resolveThemeColor(c, "lt1")
	if !ok {
		ink = svggen.MustParseColor("#FFFFFF")
	}
	var safe []string
	baseAt := -1
	for slot := 1; slot <= NumAccentSlots; slot++ {
		candidate := fmt.Sprintf("accent%d", slot)
		if !c.safeRotatingAccent(candidate, ink) {
			continue
		}
		if candidate == base {
			baseAt = len(safe)
		}
		safe = append(safe, candidate)
	}
	if len(safe) == 0 || baseAt < 0 {
		return base
	}
	offset := idx
	if mode == CellAccentAlternate {
		offset = idx % 2
	} else if mode != CellAccentProgressive {
		return base
	}
	return safe[(baseAt+offset)%len(safe)]
}

// RotationWarning reports a palette-driven change to the automatic accent.
// Pattern expansion surfaces it as a structured fit finding.
func (c ExpandContext) RotationWarning() string {
	if c.AccentStrategy != AccentStrategyRotate {
		return ""
	}
	_, _, reason := c.rotatedAccent()
	if reason == "" {
		return ""
	}
	return ErrCodeRotatedAccentUnreadable + ": " + reason
}

// ValidSurfaceTintRoles is the set of recognised surface tint roles.
var ValidSurfaceTintRoles = map[string]bool{
	"subtle":   true,
	"paper":    true,
	"elevated": true,
	"inverse":  true,
}

// CellAccentMode constants control per-cell accent color variation within a
// grid-shaped pattern.
const (
	CellAccentUniform     = "uniform"     // every cell uses the resolved base accent
	CellAccentAlternate   = "alternate"   // cells alternate base, base+1
	CellAccentProgressive = "progressive" // cells walk base, base+1, base+2, ...
)

// ValidCellAccentModes is the set of allowed cell_accent_mode values.
var ValidCellAccentModes = map[string]bool{
	"":                    true, // default = uniform
	CellAccentUniform:     true,
	CellAccentAlternate:   true,
	CellAccentProgressive: true,
}

// ResolveCellAccent returns the accent color for a cell at position idx given the
// base accent and the cell accent mode. base must be "accent1"–"accent6".
func ResolveCellAccent(base string, idx int, mode string) string {
	if mode == "" || mode == CellAccentUniform {
		return base
	}
	if len(base) != 7 || base[:6] != "accent" {
		return base
	}
	baseNum := accentNumber(base)
	switch mode {
	case CellAccentAlternate:
		offset := idx % 2
		return fmt.Sprintf("accent%d", ((baseNum-1+offset)%NumAccentSlots)+1)
	case CellAccentProgressive:
		return fmt.Sprintf("accent%d", ((baseNum-1+idx)%NumAccentSlots)+1)
	default:
		return base
	}
}

// accentNumber extracts the 1-based number from an accent name like "accent3".
// Returns 1 if the name doesn't match the expected format.
func accentNumber(name string) int {
	if len(name) == 7 && name[:6] == "accent" {
		n := int(name[6] - '0')
		if n >= 1 && n <= NumAccentSlots {
			return n
		}
	}
	return 1
}

// ValidateCellAccentMode returns a ValidationError if mode is not a valid
// cell_accent_mode value, or nil if valid.
func ValidateCellAccentMode(patternName, mode string) error {
	if ValidCellAccentModes[mode] {
		return nil
	}
	return &ValidationError{
		Pattern: patternName,
		Path:    "overrides.cell_accent_mode",
		Code:    "invalid_enum",
		Message: fmt.Sprintf("%s: overrides.cell_accent_mode must be one of uniform, alternate, progressive; got %q", patternName, mode),
	}
}

// ResolveSurface returns the scheme color name for a surface tint role.
// Falls back to defaultColor if the role is not defined in the template metadata
// or its value is neither a scheme color name nor 6-digit hex; an invalid value
// would otherwise be written verbatim as <a:srgbClr val="..."/>
// (go-slide-creator-csclk.36).
func ResolveSurface(role string, metadata *types.TemplateMetadata, defaultColor string) string {
	if metadata != nil && len(metadata.SurfaceTints) > 0 {
		if resolved, ok := metadata.SurfaceTints[role]; ok && (pptx.IsSchemeColor(resolved) || isHexColor(resolved)) {
			return resolved
		}
	}
	return defaultColor
}

// ResolveSize returns size if positive, otherwise defaultSize.
func ResolveSize(size, defaultSize float64) float64 {
	if size > 0 {
		return size
	}
	return defaultSize
}

// textOverridesSchema returns the JSON Schema for the standard
// {accent, semantic_accent, header_size, body_size} overrides object.
func textOverridesSchema() *Schema {
	return textOverridesSchemaWithout()
}

// textOverridesSchemaWithout returns the standard text overrides schema minus
// the named keys, for patterns that embed TextOverrides but have no use for
// some of its fields (e.g. no header text for header_size to size). Pair it
// with rejectUnusedTextOverrides in Validate so the omitted keys are refused
// rather than silently ignored (go-slide-creator-s1uvj.41).
func textOverridesSchemaWithout(omit ...string) *Schema {
	props := map[string]*Schema{
		"accent":           StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
		"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
		"header_size":      NumberSchema(6, 120).WithDescription("Font size for headers in points"),
		"body_size":        NumberSchema(6, 120).WithDescription("Font size for body text in points"),
		"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent variation: uniform (default, all cells same accent), alternate (base/base+1), progressive (walks accent1-6)").WithDefault("uniform"),
	}
	for _, k := range omit {
		delete(props, k)
	}
	return ObjectSchema(props, nil).WithAdditionalProperties(false)
}

// rejectUnusedTextOverrides returns an UNKNOWN_KEY error for each standard
// text override the pattern does not read (and so omits from its schema via
// textOverridesSchemaWithout) but the caller set anyway. Only header_size is
// supported here, the one standard key some patterns have no text for.
func rejectUnusedTextOverrides(pattern string, ovr *TextOverrides, unused ...string) []error {
	if ovr == nil {
		return nil
	}
	allowed := []string{"accent", "semantic_accent", "header_size", "body_size", "cell_accent_mode"}
	var errs []error
	for _, key := range unused {
		set := false
		switch key {
		case "header_size":
			set = ovr.HeaderSize != 0
		}
		if !set {
			continue
		}
		var keep []string
		for _, a := range allowed {
			if !slices.Contains(unused, a) {
				keep = append(keep, a)
			}
		}
		errs = append(errs, &ValidationError{
			Pattern: pattern,
			Path:    "overrides",
			Code:    ErrCodeUnknownKey,
			Message: fmt.Sprintf("%s: overrides.%s is not used by this pattern (it has no header text); allowed overrides: %s — size the text with body_size", pattern, key, strings.Join(keep, ", ")),
			Fix:     RemoveKeyFix(key, "overrides", keep),
		})
	}
	return errs
}
