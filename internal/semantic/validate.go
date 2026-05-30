package semantic

// This file implements the semantic validation gates: the layer that, given a
// parsed DeckSpec, enforces the MVP authoring rules and emits agent-facing
// diagnostics. The diagnostics are produced directly as transport-neutral
// internal/diagnostics.Diagnostic values so a caller can wrap them with
// diagnostics.BuildEnvelope without an intermediate adapter. The parser's own
// path-scoped semantic.Diagnostics are bridged into the same shape via
// Diagnostics.ToDiagnostics, so a CLI/MCP surface can present parse and
// validation findings in one envelope.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// Strictness controls how strictly the advisory semantic rules are enforced.
// Hard structural rules (missing required fields, unknown kinds/archetypes) are
// always errors regardless of strictness.
type Strictness string

const (
	// StrictnessOff suppresses advisory findings entirely; only hard structural
	// errors are emitted.
	StrictnessOff Strictness = "off"
	// StrictnessWarn (the default) emits advisory findings as warnings.
	StrictnessWarn Strictness = "warn"
	// StrictnessStrict promotes advisory findings to errors.
	StrictnessStrict Strictness = "strict"
)

// weakMarkers are case-insensitive substrings that flag placeholder/filler text.
var weakMarkers = []string{"tbd", "lorem ipsum", "__fill__", "todo", "fixme", "placeholder"}

// kindNeedsTakeaway lists the content-bearing slide kinds expected to carry a
// one-line takeaway (or, for chart_insight, an insight). Structural slides
// (title, section, closing) and the raw escape hatch are exempt.
var kindNeedsTakeaway = map[SlideKind]bool{
	KindExecutiveSummary: true,
	KindKPISnapshot:      true,
	KindChartInsight:     true,
	KindComparison:       true,
	KindProcess:          true,
	KindRoadmap:          true,
	KindDecision:         true,
}

// semDiags accumulates validation diagnostics, applying the strictness policy to
// advisory findings as they are added.
type semDiags struct {
	strict Strictness
	out    []diagnostics.Diagnostic
}

// hard appends an always-error structural diagnostic.
func (s *semDiags) hard(path, code, msg string) {
	s.out = append(s.out, diagnostics.Diagnostic{
		Code:     code,
		Message:  msg,
		Path:     path,
		Severity: diagnostics.SeverityError,
	})
}

// advisory appends a finding whose severity follows the strictness policy:
// suppressed under off, a warning under warn, an error under strict.
func (s *semDiags) advisory(path, code, msg string) {
	if s.strict == StrictnessOff {
		return
	}
	sev := diagnostics.SeverityWarning
	if s.strict == StrictnessStrict {
		sev = diagnostics.SeverityError
	}
	s.out = append(s.out, diagnostics.Diagnostic{
		Code:     code,
		Message:  msg,
		Path:     path,
		Severity: sev,
	})
}

// Validate enforces the MVP semantic authoring rules over a parsed DeckSpec and
// returns transport-neutral diagnostics. The result is always non-nil-safe to
// pass to diagnostics.BuildEnvelope; an empty slice means the spec is clean at
// the given strictness. An unrecognized strict value is treated as warn.
func Validate(spec *DeckSpec, strict Strictness) []diagnostics.Diagnostic {
	switch strict {
	case StrictnessOff, StrictnessWarn, StrictnessStrict:
	default:
		strict = StrictnessWarn
	}
	s := &semDiags{strict: strict}
	if spec == nil {
		s.hard("", diagnostics.CodeSemanticRequired, "semantic deck spec is empty")
		return s.out
	}
	validateMeta(spec, s)
	for i := range spec.Slides {
		validateSlide(i, spec.Slides[i], s)
	}
	return s.out
}

// Check parses a semantic document and validates the resulting spec, returning
// the parse and validation diagnostics together as one transport-neutral slice
// ready for diagnostics.BuildEnvelope.
func Check(filename string, data []byte, strict Strictness) []diagnostics.Diagnostic {
	spec, parseDiags := Parse(filename, data)
	out := parseDiags.ToDiagnostics()
	if spec != nil {
		out = append(out, Validate(spec, strict)...)
		// Deck-rhythm advisories are computed from the normalized IR so `semantic
		// validate` surfaces the same monotony/structure findings the compile and
		// render paths emit. Compile gathers these itself (it does not call Check),
		// so there is no double-emit.
		out = append(out, rhythmDiagnostics(Normalize(spec), strict)...)
	}
	return out
}

// validateMeta enforces deck-level rules: a title is required and a present
// archetype must be registered. Meta text is scanned for placeholder content.
func validateMeta(spec *DeckSpec, s *semDiags) {
	if strings.TrimSpace(spec.Meta.Title) == "" {
		s.hard("meta.title", diagnostics.CodeSemanticRequired, "deck title (meta.title) is required")
	} else {
		scanWeak("meta.title", spec.Meta.Title, s)
	}
	scanWeak("meta.subtitle", spec.Meta.Subtitle, s)

	if spec.Meta.Archetype != "" && !spec.Meta.Archetype.Valid() {
		s.hard("meta.archetype", diagnostics.CodeSemanticUnknownArchetype,
			fmt.Sprintf("unknown archetype %q; expected one of %s", spec.Meta.Archetype, joinArchetypes()))
	}
}

// validateSlide enforces per-slide rules: a known kind, the kind's required
// payload fields, kind-specific density/richness rules, a takeaway for content
// slides, and placeholder-content detection across the payload.
func validateSlide(i int, slide SlideSpec, s *semDiags) {
	path := fmt.Sprintf("slides[%d]", i)

	// Kind discriminator must be present and registered before the payload can
	// be interpreted.
	if slide.Kind == "" {
		s.hard(path+".kind", diagnostics.CodeSemanticUnknownKind,
			fmt.Sprintf("slide is missing the required \"kind\" field; expected one of %s", joinKinds()))
		scanWeakBody(path, slide.Body, s)
		return
	}
	info, ok := LookupKind(slide.Kind)
	if !ok {
		s.hard(path+".kind", diagnostics.CodeSemanticUnknownKind,
			fmt.Sprintf("unknown slide kind %q; expected one of %s", slide.Kind, joinKinds()))
		scanWeakBody(path, slide.Body, s)
		return
	}

	// Every kind-specific required payload field must be present and non-empty.
	for _, field := range info.RequiredFields {
		if !hasNonEmpty(slide.Body, field) {
			s.hard(path+"."+field, diagnostics.CodeSemanticRequired,
				fmt.Sprintf("%s slide requires a %q field", slide.Kind, field))
		}
	}

	validateKindRules(path, slide, s)

	// Content-bearing slides should carry a one-line takeaway (insight counts
	// for chart_insight).
	if kindNeedsTakeaway[slide.Kind] {
		if slide.String("takeaway") == "" && slide.String("insight") == "" {
			s.advisory(path+".takeaway", diagnostics.CodeSemanticTakeawayRequired,
				fmt.Sprintf("%s slide should carry a one-line takeaway", slide.Kind))
		}
	}

	scanWeakBody(path, slide.Body, s)
}

// validateKindRules applies the density and richness rules that are specific to
// individual slide kinds.
func validateKindRules(path string, slide SlideSpec, s *semDiags) {
	switch slide.Kind {
	case KindExecutiveSummary:
		if n, ok := listLen(slide.Body, "points"); ok && (n < 3 || n > 5) {
			s.advisory(path+".points", diagnostics.CodeSemanticDensity,
				fmt.Sprintf("executive summary has %d points; 3–5 is recommended", n))
		}
	case KindKPISnapshot:
		if n, ok := listLen(slide.Body, "kpis"); ok && (n < 2 || n > 6) {
			s.advisory(path+".kpis", diagnostics.CodeSemanticDensity,
				fmt.Sprintf("kpi snapshot has %d KPIs; 2–6 is recommended", n))
		}
	case KindChartInsight:
		// The semantic chart payload is minimal at this layer: series data may be
		// attached by a later compiler phase, so a missing/empty series is an
		// advisory richness finding rather than a hard error.
		if chart, ok := slide.Body["chart"].(map[string]any); ok {
			if !chartHasSeries(chart) {
				s.advisory(path+".chart.series", diagnostics.CodeSemanticDensity,
					"chart_insight chart declares no data series")
			}
		}
	case KindComparison:
		validateComparison(path, slide, s)
	}
}

// chartHasSeries reports whether a chart_insight payload declares at least one
// data series. It accepts the documented chart.data.series shape (used by
// examples/semantic/qbr.yaml and docs/SEMANTIC_COMPILER.md) as well as a flat
// chart.series fallback for older specs.
func chartHasSeries(chart map[string]any) bool {
	if n, ok := listLen(chart, "series"); ok && n > 0 {
		return true
	}
	if data, ok := chart["data"].(map[string]any); ok {
		if n, ok := listLen(data, "series"); ok && n > 0 {
			return true
		}
	}
	return false
}

// validateComparison checks that a comparison has at least two columns and that
// the columns are balanced (equal item counts), when the column shape exposes an
// "items" list.
func validateComparison(path string, slide SlideSpec, s *semDiags) {
	cols, ok := slide.Body["columns"].([]any)
	if !ok {
		return
	}
	if len(cols) < 2 {
		s.advisory(path+".columns", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("a comparison needs at least two columns; found %d", len(cols)))
		return
	}
	counts := make([]int, 0, len(cols))
	for _, c := range cols {
		m, ok := c.(map[string]any)
		if !ok {
			return
		}
		n, ok := listLen(m, "items")
		if !ok {
			return
		}
		counts = append(counts, n)
	}
	for _, n := range counts {
		if n != counts[0] {
			s.advisory(path+".columns", diagnostics.CodeSemanticDensity,
				"comparison columns are unbalanced; give each column the same number of items")
			return
		}
	}
}

// scanWeakBody scans a slide payload for placeholder/filler content.
func scanWeakBody(path string, body map[string]any, s *semDiags) {
	for _, k := range sortedKeys(body) {
		scanWeak(path+"."+k, body[k], s)
	}
}

// scanWeak walks an arbitrary payload value, emitting a SEMANTIC_WEAK_CONTENT
// advisory for any string that looks like placeholder text. Maps are visited in
// sorted key order so diagnostics are deterministic.
func scanWeak(path string, v any, s *semDiags) {
	switch t := v.(type) {
	case string:
		if marker := weakMarker(t); marker != "" {
			s.advisory(path, diagnostics.CodeSemanticWeakContent,
				fmt.Sprintf("content looks like a placeholder (%q); replace it with real text", marker))
		}
	case map[string]any:
		for _, k := range sortedKeys(t) {
			scanWeak(path+"."+k, t[k], s)
		}
	case []any:
		for i, e := range t {
			scanWeak(fmt.Sprintf("%s[%d]", path, i), e, s)
		}
	}
}

// weakMarker returns the first placeholder marker found in v, or "" if none.
func weakMarker(v string) string {
	l := strings.ToLower(v)
	for _, m := range weakMarkers {
		if strings.Contains(l, m) {
			return m
		}
	}
	return ""
}

// hasNonEmpty reports whether body has a present, non-empty value for field. A
// string is empty when blank after trimming; a list/map is empty when it has no
// entries; nil is always empty.
func hasNonEmpty(body map[string]any, field string) bool {
	v, ok := body[field]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) != ""
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	default:
		return true
	}
}

// listLen returns the length of a list-valued field and whether it is a list.
func listLen(body map[string]any, field string) (int, bool) {
	if l, ok := body[field].([]any); ok {
		return len(l), true
	}
	return 0, false
}

// sortedKeys returns the keys of a map in sorted order for deterministic walks.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ToDiagnostics adapts the parser's path-scoped semantic.Diagnostics into the
// transport-neutral diagnostics.Diagnostic shape so parse findings can be
// wrapped by diagnostics.BuildEnvelope alongside validation findings. Parser
// codes map onto the SEMANTIC_* taxonomy; decode failures map onto INVALID_JSON.
func (ds Diagnostics) ToDiagnostics() []diagnostics.Diagnostic {
	if len(ds) == 0 {
		return nil
	}
	out := make([]diagnostics.Diagnostic, len(ds))
	for i, d := range ds {
		out[i] = diagnostics.Diagnostic{
			Code:     mapParseCode(d.Code),
			Message:  d.Message,
			Path:     d.Path,
			Severity: diagnostics.Severity(string(d.Severity)),
		}
	}
	return out
}

// mapParseCode maps a parser diagnostic code onto the shared diagnostics
// taxonomy.
func mapParseCode(code string) string {
	switch code {
	case CodeUnknownKind, CodeInvalidKindType:
		return diagnostics.CodeSemanticUnknownKind
	case CodeMissingKind:
		return diagnostics.CodeSemanticRequired
	case CodeUnknownArchetype:
		return diagnostics.CodeSemanticUnknownArchetype
	case CodeParseError, CodeInvalidRoot, CodeInvalidSlides, CodeInvalidSlide:
		return diagnostics.CodeInvalidJSON
	default:
		return diagnostics.CodeInvalidParameter
	}
}
