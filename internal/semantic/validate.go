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
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
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

// shapeKind names the JSON type a kind's compiler expects for a payload field.
// A field present with a different type is silently dropped by the per-kind
// compilers (strField/stringList/mapList all skip wrong-typed values), so
// validation flags the mismatch instead of letting the content vanish.
type shapeKind int

const (
	shapeString shapeKind = iota
	shapeArray
	shapeObject
)

// label returns the human-readable expected-type phrase for a finding message.
func (k shapeKind) label() string {
	switch k {
	case shapeArray:
		return "an array"
	case shapeObject:
		return "an object"
	default:
		return "a string"
	}
}

// kindFieldShapes is the per-kind payload-shape contract: for each slide kind it
// lists the payload fields whose compiler reads a fixed JSON type, mapping each
// to that expected type. A field present with a mismatching type compiles to
// nothing (the extractors drop it), so validation emits a SEMANTIC_FIELD_TYPE
// finding rather than shipping an empty/incomplete slide. Fields a kind does not
// read are absent here and left untouched (a later phase may interpret them).
// raw_json2pptx's "slide" is validated structurally by validateRawEscapeHatch.
var kindFieldShapes = map[SlideKind]map[string]shapeKind{
	KindTitle:            {"title": shapeString, "subtitle": shapeString, "eyebrow": shapeString},
	KindSection:          {"title": shapeString, "subtitle": shapeString},
	KindExecutiveSummary: {"title": shapeString, "points": shapeArray, "takeaways": shapeArray, "takeaway": shapeString},
	KindKPISnapshot:      {"title": shapeString, "kpis": shapeArray, "metrics": shapeArray, "takeaway": shapeString},
	KindChartInsight:     {"title": shapeString, "chart": shapeObject, "insights": shapeArray, "insight": shapeString, "source": shapeString, "takeaway": shapeString},
	KindComparison:       {"title": shapeString, "columns": shapeArray, "takeaway": shapeString},
	KindProcess:          {"title": shapeString, "steps": shapeArray, "takeaway": shapeString},
	KindRoadmap:          {"title": shapeString, "phases": shapeArray, "takeaway": shapeString},
	KindDecision:         {"title": shapeString, "options": shapeArray, "recommendation": shapeString, "takeaway": shapeString},
	KindClosing:          {"title": shapeString, "subtitle": shapeString},
}

// shapeMatches reports whether v has the JSON type the shape expects.
func shapeMatches(v any, k shapeKind) bool {
	switch k {
	case shapeString:
		_, ok := v.(string)
		return ok
	case shapeArray:
		_, ok := v.([]any)
		return ok
	case shapeObject:
		_, ok := v.(map[string]any)
		return ok
	}
	return false
}

// jsonTypeName returns the article+name of a decoded JSON value's type, for
// finding messages. JSON numbers decode to float64 under the lenient decoder.
func jsonTypeName(v any) string {
	switch v.(type) {
	case string:
		return "a string"
	case []any:
		return "an array"
	case map[string]any:
		return "an object"
	case bool:
		return "a boolean"
	case float64, int, int64:
		return "a number"
	default:
		return "an unexpected type"
	}
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

// requireUsableContent reports whether a required content-bearing list field
// yields at least one entry the compiler can render. When the field cleared the
// raw presence gate (so the required-field loop stayed silent) but every entry
// is blank or labelless — extracting to zero usable content — it emits a
// blocking error: the silent content-drop that otherwise compiles to a
// title-only slide. The error is always-hard (independent of strictness) because
// it is a missing-content condition, matching the required-field gate. It
// returns true only when usable > 0, so callers can gate density/range
// advisories on real content.
func (s *semDiags) requireUsableContent(path, field string, body map[string]any, usable int) bool {
	if usable > 0 {
		return true
	}
	if hasNonEmpty(body, field) {
		s.hard(path+"."+field, diagnostics.CodeSemanticRequired,
			fmt.Sprintf("%q is present but every entry is blank; provide at least one entry with usable content", field))
	}
	return false
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
	// The slides array is required and must hold at least one slide. The lenient
	// decoder turns an absent or empty slides field into a zero-length slice, so
	// both the missing-array case and an explicit empty array land here. Blocking
	// early keeps a zero-slide deck from compiling to a null/empty deck behind a
	// green validate gate.
	if len(spec.Slides) == 0 {
		s.hard("slides", diagnostics.CodeSemanticRequired,
			"deck must contain at least one slide; the required \"slides\" array is missing or empty")
	}
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

	// Flag payload fields present with the wrong JSON type for the kind. The
	// compilers silently drop wrong-typed values, so without this the content
	// disappears behind a green validate gate (a numeric title, points given as a
	// string). This is advisory (warn/error by strictness); a required list that
	// extracts to zero usable content is separately blocked by requireUsableContent.
	validateFieldShapes(path, slide, s)

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

// validateFieldShapes emits a SEMANTIC_FIELD_TYPE advisory for each payload
// field present with a JSON type the kind's compiler cannot read. It consults
// kindFieldShapes, the per-kind payload-shape contract, and walks the body in
// sorted key order for deterministic diagnostics. A null value is treated as
// absent (presence is the required-field loop's job); only a present, wrong-
// typed value is flagged. The finding is advisory so it warns under warn and
// errors under strict, matching the other authoring advisories; total content
// loss on a required list is separately blocked by requireUsableContent.
func validateFieldShapes(path string, slide SlideSpec, s *semDiags) {
	shapes, ok := kindFieldShapes[slide.Kind]
	if !ok {
		return
	}
	for _, field := range sortedKeys(slide.Body) {
		want, governed := shapes[field]
		if !governed {
			continue
		}
		v := slide.Body[field]
		if v == nil || shapeMatches(v, want) {
			continue
		}
		s.advisory(path+"."+field, diagnostics.CodeSemanticFieldType,
			fmt.Sprintf("%q must be %s but is %s; a %q slide drops the wrong-typed value, losing this content",
				field, want.label(), jsonTypeName(v), slide.Kind))
	}
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
		// Count KPIs the compiler can actually render, not raw list entries: a
		// list of blank/labelless cells passes the required-field gate but
		// compiles to a title-only slide. Below 1 usable cell is a blocking error;
		// otherwise the 2–6 density range is advisory.
		if n := slides.UsableKPICount(slide.Body); s.requireUsableContent(path, "kpis", slide.Body, n) && (n < 2 || n > 6) {
			s.advisory(path+".kpis", diagnostics.CodeSemanticDensity,
				fmt.Sprintf("kpi snapshot has %d usable KPIs; 2–6 is recommended", n))
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
	case KindProcess:
		// Count steps the compiler can render (blank entries are dropped), so a
		// process of all-blank steps fails fast instead of compiling to a
		// title-only slide, and the 3–8 process-flow range reflects real content.
		if n := slides.UsableStepCount(slide.Body); s.requireUsableContent(path, "steps", slide.Body, n) && (n < 3 || n > 8) {
			s.advisory(path+".steps", diagnostics.CodeSemanticDensity,
				fmt.Sprintf("process has %d usable steps; 3–8 render as a process-flow visual (otherwise it degrades to a bullet list)", n))
		}
	case KindRoadmap:
		if n := slides.UsablePhaseCount(slide.Body); s.requireUsableContent(path, "phases", slide.Body, n) && (n < 3 || n > 6) {
			s.advisory(path+".phases", diagnostics.CodeSemanticDensity,
				fmt.Sprintf("roadmap has %d usable phases; 3–6 render as a phase-roadmap visual (otherwise it degrades to a bullet list)", n))
		}
	case KindRawJSON2pptx:
		// The escape hatch carries a verbatim raw slide; validate it structurally
		// so an invalid payload fails fast here instead of compiling to an empty
		// slide. See validateRawEscapeHatch.
		validateRawEscapeHatch(path, slide.Body, s)
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
	// Columns present but every column blank (no header, no items) compiles to a
	// title-only slide; fail fast on the dropped comparison instead of passing it.
	if slides.UsableComparisonColumnCount(slide.Body) == 0 {
		if hasNonEmpty(slide.Body, "columns") {
			s.hard(path+".columns", diagnostics.CodeSemanticRequired,
				"comparison \"columns\" carry no usable header or items; provide content for at least one column")
		}
		return
	}
	if len(cols) != 2 {
		s.advisory(path+".columns", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("a comparison renders exactly two columns as a comparison-2col visual; found %d (otherwise it degrades to a bullet list)", len(cols)))
		return
	}
	counts := make([]int, 0, len(cols))
	for _, c := range cols {
		m, ok := c.(map[string]any)
		if !ok {
			return
		}
		n, ok := comparisonColumnItemCount(m)
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

// comparisonColumnItemCount reports the rendered row count for one comparison
// column, mirroring the slides compiler: an explicit "items" list is counted
// verbatim, otherwise "pros"/"cons" each collapse to a single rendered line.
// ok is false only when the column exposes none of these shapes.
func comparisonColumnItemCount(m map[string]any) (int, bool) {
	if n, ok := listLen(m, "items"); ok {
		return n, true
	}
	count, found := 0, false
	if n, ok := listLen(m, "pros"); ok {
		found = true
		if n > 0 {
			count++
		}
	}
	if n, ok := listLen(m, "cons"); ok {
		found = true
		if n > 0 {
			count++
		}
	}
	return count, found
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
	case CodeUnknownField:
		return diagnostics.CodeSemanticUnknownField
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
