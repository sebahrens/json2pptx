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
	"regexp"
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

// weakSubstringMarkers are distinctive enough (multi-word or sentinel-shaped)
// that any case-insensitive substring match reliably indicates placeholder text.
var weakSubstringMarkers = []string{"lorem ipsum", "__fill__", "placeholder"}

// weakTokenRE matches the short alpha filler markers (tbd, todo, fixme) only as
// whole words. Matching them as substrings false-positives on ordinary words
// (e.g. "Mastodon" contains "todo"). Word boundaries keep "TODO:" or "(tbd)"
// tripping while embedded occurrences do not.
var weakTokenRE = regexp.MustCompile(`\b(tbd|todo|fixme)\b`)

// kindNeedsTakeaway lists the content-bearing slide kinds expected to carry a
// one-line takeaway (or, for chart_insight, an insight). Structural slides
// (title, section, closing) and the raw escape hatch are exempt.
var kindNeedsTakeaway = map[SlideKind]bool{
	KindExecutiveSummary: true,
	KindKPISnapshot:      true,
	KindChartInsight:     true,
	KindComparison:       true,
	KindOptionMatrix:     true,
	KindTable:            true,
	KindArchitecture:     true,
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
	// shapeStringOrObject is for a field whose compiler reads either — an
	// image_case picture is a path string or a {path, alt} object.
	shapeStringOrObject
)

// label returns the human-readable expected-type phrase for a finding message.
func (k shapeKind) label() string {
	switch k {
	case shapeArray:
		return "an array"
	case shapeObject:
		return "an object"
	case shapeStringOrObject:
		return "a string or an object"
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
	KindExecutiveSummary: {"title": shapeString, "points": shapeArray, "takeaways": shapeArray, "takeaway": shapeString, "bottom_line": shapeString},
	KindKPISnapshot:      {"title": shapeString, "kpis": shapeArray, "metrics": shapeArray, "takeaway": shapeString},
	KindChartInsight:     {"title": shapeString, "chart": shapeObject, "insights": shapeArray, "insight": shapeString, "source": shapeString, "takeaway": shapeString},
	KindComparison:       {"title": shapeString, "columns": shapeArray, "takeaway": shapeString},
	KindOptionMatrix: {
		"title": shapeString, "criteria": shapeArray, "columns": shapeArray,
		"options": shapeArray, "rows": shapeArray, "scale": shapeString,
		"decisive_criterion": shapeString, "highlight_label": shapeString,
		"corner_label": shapeString, "takeaway": shapeString,
	},
	KindTable: {
		"title": shapeString, "headers": shapeArray, "columns": shapeArray, "rows": shapeArray,
		"column_alignments": shapeArray, "column_types": shapeArray, "takeaway": shapeString,
	},
	KindArchitecture: {
		"title": shapeString, "tiers": shapeArray, "layers": shapeArray,
		"rails": shapeArray, "side_rails": shapeArray, "takeaway": shapeString,
	},
	KindAgenda: {
		"title": shapeString, "sections": shapeArray, "items": shapeArray,
		"agenda": shapeArray, "takeaway": shapeString,
	},
	KindTeam: {
		"title": shapeString, "members": shapeArray, "people": shapeArray,
		"team": shapeArray, "takeaway": shapeString,
	},
	KindStat: {
		"title": shapeString, "value": shapeString, "stat": shapeString, "number": shapeString,
		"metric": shapeString, "label": shapeString, "caption": shapeString, "subtitle": shapeString,
		"unit": shapeString, "suffix": shapeString, "context": shapeString, "detail": shapeString,
		"description": shapeString, "source": shapeString, "takeaway": shapeString,
	},
	KindTimeline: {
		"title": shapeString, "milestones": shapeArray, "stops": shapeArray,
		"events": shapeArray, "timeline": shapeArray, "takeaway": shapeString,
	},
	KindMatrix2x2: {
		"title": shapeString, "quadrants": shapeArray, "cells": shapeArray, "boxes": shapeArray,
		"top_left": shapeObject, "top_right": shapeObject, "bottom_left": shapeObject, "bottom_right": shapeObject,
		"x_axis": shapeString, "x_axis_label": shapeString,
		"y_axis": shapeString, "y_axis_label": shapeString,
		"x_low": shapeString, "x_high": shapeString, "y_low": shapeString, "y_high": shapeString,
		"takeaway": shapeString,
	},
	KindFramework: {
		"title": shapeString, "framework": shapeString, "type": shapeString,
		"model": shapeString, "sections": shapeObject, "takeaway": shapeString,
	},
	KindImageCase: {
		"title": shapeString, "image": shapeStringOrObject, "eyebrow": shapeString, "heading": shapeString,
		"body": shapeString, "text": shapeString, "story": shapeString, "description": shapeString,
		"bullets": shapeArray, "metrics": shapeArray, "caption": shapeString,
		"image_side": shapeString, "image_label": shapeString, "takeaway": shapeString,
	},
	KindProcess: {"title": shapeString, "steps": shapeArray, "takeaway": shapeString},
	KindRoadmap: {"title": shapeString, "phases": shapeArray, "takeaway": shapeString},
	KindDecision: {
		"title": shapeString, "options": shapeArray, "choices": shapeArray, "alternatives": shapeArray,
		"recommendation": shapeString, "takeaway": shapeString,
	},
	KindClosing: {"title": shapeString, "subtitle": shapeString},
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
	case shapeStringOrObject:
		switch v.(type) {
		case string, map[string]any:
			return true
		}
		return false
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

// advisoryFix is advisory with an attached fix suggestion.
func (s *semDiags) advisoryFix(path, code, msg string, fix *diagnostics.Fix) {
	n := len(s.out)
	s.advisory(path, code, msg)
	if len(s.out) > n {
		s.out[len(s.out)-1].Fix = fix
	}
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
func (s *semDiags) requireUsableContent(path, field string, body map[string]any, usable int, aliases ...string) bool {
	if usable > 0 {
		return true
	}
	// Report against whichever key the author actually supplied (the canonical
	// field or an accepted alias), so the path points at real payload content.
	present := field
	if !hasNonEmpty(body, field) {
		for _, a := range aliases {
			if hasNonEmpty(body, a) {
				present = a
				break
			}
		}
	}
	if hasNonEmpty(body, present) {
		s.hard(path+"."+present, diagnostics.CodeSemanticRequired,
			fmt.Sprintf("%q is present but every entry is blank; provide at least one entry with usable content", present))
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
	// The parse and validate passes both report an unregistered, non-empty kind
	// at the same code+path+message (parse: CodeUnknownKind→SEMANTIC_UNKNOWN_KIND;
	// validateSlide: SEMANTIC_UNKNOWN_KIND), so an unknown kind surfaces twice in
	// the merged slice. Each pass must keep emitting standalone — the parser feeds
	// callers that never validate, and Validate runs without a parse pass on the
	// compile path — so collapse exact duplicates here, where both passes meet.
	return dedupExact(out)
}

// dedupExact drops findings that are byte-identical to an earlier one
// (code+path+message+severity), preserving first-occurrence order. Two findings
// indistinguishable on all four fields carry no extra signal for the consuming
// agent, so collapsing them removes double-emit noise without losing diagnostics
// that differ in any field (e.g. the same path with a different message).
func dedupExact(in []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	if len(in) < 2 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := in[:0]
	for _, d := range in {
		key := string(d.Severity) + "\x00" + d.Code + "\x00" + d.Path + "\x00" + d.Message
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, d)
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
		msg := fmt.Sprintf("unknown slide kind %q; expected one of %s", slide.Kind, joinKinds())
		if canonical, hinted := SpellingFor(slide.Kind); hinted {
			msg = fmt.Sprintf("unknown slide kind %q; use %q — expected one of %s", slide.Kind, canonical, joinKinds())
		}
		s.hard(path+".kind", diagnostics.CodeSemanticUnknownKind, msg)
		scanWeakBody(path, slide.Body, s)
		return
	}

	// A pattern / layout override that names something this kind cannot compile
	// to is a no-op: normalizeSlide keeps its own choice. Saying nothing meant
	// an agent following analyze_deck_rhythm's "break this run" advice watched
	// its variation vanish into a clean validate (go-slide-creator-u5az).
	validateCompositionOverride(path, slide, s)

	// Every kind-specific required payload field must be present and non-empty.
	// A field with registered aliases is satisfied when the canonical name OR any
	// alias carries content (required-one-of): the compiler reads the aliases
	// interchangeably, so requiring only the canonical name would block an
	// otherwise-compileable spec (e.g. kpi_snapshot's "metrics" alias for "kpis").
	for _, field := range info.RequiredFields {
		if fieldOrAliasPresent(slide.Body, field, info.RequiredAliases[field]) {
			continue
		}
		s.hard(path+"."+field, diagnostics.CodeSemanticRequired,
			fmt.Sprintf("%s slide requires a %s field", slide.Kind, requiredFieldPhrase(field, info.RequiredAliases[field])))
	}

	// Flag payload fields present with the wrong JSON type for the kind. The
	// compilers silently drop wrong-typed values, so without this the content
	// disappears behind a green validate gate (a numeric title, points given as a
	// string). This is advisory (warn/error by strictness); a required list that
	// extracts to zero usable content is separately blocked by requireUsableContent.
	validateFieldShapes(path, slide, s)

	// Flag payload keys the compiler never reads (typos, invented fields, chart
	// data in the wrong place) instead of silently dropping their content.
	validateUnknownFields(path, slide, s)

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
		validateExecutiveSummary(path, slide, s)
	case KindKPISnapshot:
		validateKPISnapshot(path, slide, s)
	case KindChartInsight:
		validateChartInsight(path, slide, s)
	case KindComparison:
		validateComparison(path, slide, s)
	case KindOptionMatrix:
		validateOptionMatrix(path, slide, s)
	case KindTable:
		validateTable(path, slide, s)
	case KindAgenda:
		validateAgenda(path, slide, s)
	case KindTeam:
		validateTeam(path, slide, s)
	case KindStat:
		validateStat(path, slide, s)
	case KindArchitecture:
		validateArchitecture(path, slide, s)
	case KindTimeline:
		validateTimeline(path, slide, s)
	case KindMatrix2x2:
		validateMatrix(path, slide, s)
	case KindDecision:
		validateDecision(path, slide, s)
	case KindFramework:
		validateFramework(path, slide, s)
	case KindImageCase:
		validateImageCase(path, slide, s)
	case KindProcess:
		validateProcess(path, slide, s)
	case KindRoadmap:
		validateRoadmap(path, slide, s)
	case KindRawJSON2pptx:
		// The escape hatch carries a verbatim raw slide; validate it structurally
		// so an invalid payload fails fast here instead of compiling to an empty
		// slide. See validateRawEscapeHatch.
		validateRawEscapeHatch(path, slide.Body, s)
	}
}

// validateAgenda reports which agenda visual a section list is losing. An
// agenda with one section is a heading, not a contents page; with eleven it is
// a wall. Both still render — as a numbered bullet list — so the rule says what
// is lost rather than blocking (go-slide-creator-3rvk).
func validateAgenda(path string, slide SlideSpec, s *semDiags) {
	n := slides.UsableAgendaSectionCount(slide.Body)
	if !s.requireUsableContent(path, "sections", slide.Body, n) {
		return
	}
	if slides.AgendaPattern(slide.Body) == "" {
		s.advisory(path+".sections", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("agenda has %d usable sections; 2–10 render as the numbered agenda visual and 3–6 with subtitles as agenda rows (otherwise it degrades to a bullet list)", n))
	}
}

// validateTeam reports a roster outside the team-bios budgets. They are the
// pattern's own: outside them the roster still renders, as a bullet list, so
// the advisory names what broke rather than blocking (go-slide-creator-13lj).
func validateTeam(path string, slide SlideSpec, s *semDiags) {
	n := slides.UsableTeamMemberCount(slide.Body)
	if !s.requireUsableContent(path, "members", slide.Body, n, "people") {
		return
	}
	if over := slides.TeamOverBudget(slide.Body); over != "" {
		s.advisory(path+".members", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("team %s (otherwise it degrades to a bullet list)", over))
	}
}

// validateStat reports a number or its words outside the stat-hero budgets.
// Past them the slide still renders, as a content slide, so the advisory names
// the budget that broke rather than blocking. A missing value is the
// required-field gate's business (value, with its stat / number / metric
// aliases), so this rule speaks only to budgets (go-slide-creator-2hkc).
func validateStat(path string, slide SlideSpec, s *semDiags) {
	if over := slides.StatOverBudget(slide.Body); over != "" {
		s.advisory(path+".value", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("stat %s (otherwise it degrades to a content slide)", over))
	}
}

// validateArchitecture reports a stack outside the arch-stack budgets. They are
// the pattern's own: outside them the slide still renders, as a bullet list, so
// the advisory says which budget it broke rather than blocking
// (go-slide-creator-162os).
func validateArchitecture(path string, slide SlideSpec, s *semDiags) {
	n := slides.UsableTierCount(slide.Body)
	if !s.requireUsableContent(path, "tiers", slide.Body, n) {
		return
	}
	if over := slides.ArchitectureOverBudget(slide.Body); over != "" {
		s.advisory(path+".tiers", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("architecture %s (otherwise it degrades to a bullet list)", over))
	}
}

// validateTimeline reports milestones outside the timeline-horizontal bounds.
// They are the pattern's own: outside them the dates still render, as a dated
// bullet list, so the advisory names what broke rather than blocking
// (go-slide-creator-wrsb).
func validateTimeline(path string, slide SlideSpec, s *semDiags) {
	n := slides.UsableTimelineStopCount(slide.Body)
	if !s.requireUsableContent(path, "milestones", slide.Body, n, "stops", "events", "timeline") {
		return
	}
	if over := slides.TimelineOverBudget(slide.Body); over != "" {
		s.advisory(path+".milestones", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("timeline %s (otherwise it degrades to a dated bullet list)", over))
	}
}

// validateMatrix reports a 2x2 that cannot take the quadrant visual. Short of
// four headed quadrants and two named axes it still renders, as a bullet list
// naming each quadrant's position, so the advisory says what broke rather than
// blocking (go-slide-creator-ykjh).
func validateMatrix(path string, slide SlideSpec, s *semDiags) {
	n := slides.UsableMatrixQuadrantCount(slide.Body)
	if !s.requireUsableContent(path, "quadrants", slide.Body, n, "cells", "boxes") {
		return
	}
	if over := slides.MatrixOverBudget(slide.Body); over != "" {
		s.advisory(path+".quadrants", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("matrix %s (otherwise it degrades to a bullet list)", over))
	}
}

// validateFramework reports a framework the visual cannot draw. A framework is
// a named thing with fixed parts, so a missing part is named rather than
// silently rendered as an empty quadrant (go-slide-creator-anzx).
func validateFramework(path string, slide SlideSpec, s *semDiags) {
	n := slides.UsableFrameworkSectionCount(slide.Body)
	if !s.requireUsableContent(path, "sections", slide.Body, n) {
		return
	}
	if over := slides.FrameworkOverBudget(slide.Body); over != "" {
		s.advisory(path+".sections", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("framework %s (otherwise it degrades to grouped bullets)", over))
	}
}

// validateImageCase reports a case study the split cannot take. An image with
// nothing said about it is a plain image slide, which the pattern refuses too
// (go-slide-creator-q31s).
func validateImageCase(path string, slide SlideSpec, s *semDiags) {
	if over := slides.ImageCaseOverBudget(slide.Body); over != "" {
		s.advisory(path+".body", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("image case %s (otherwise it degrades to a content slide)", over))
	}
}

// validateDecision reports options that cannot take a numbered visual. They
// still render, as the content slide this kind has always produced, so the
// advisory says which visual is lost rather than blocking
// (go-slide-creator-4ndv).
func validateDecision(path string, slide SlideSpec, s *semDiags) {
	if over := slides.DecisionOverBudget(slide.Body); over != "" {
		s.advisory(path+".options", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("decision %s (otherwise it degrades to a content slide)", over))
	}
}

// validateProcess counts steps the compiler can render (blank entries are
// dropped), so a process of all-blank steps fails fast instead of compiling to
// a title-only slide, and the 3–8 process-flow range reflects real content.
func validateProcess(path string, slide SlideSpec, s *semDiags) {
	n := slides.UsableStepCount(slide.Body)
	if !s.requireUsableContent(path, "steps", slide.Body, n) {
		return
	}
	if over := slides.ProcessOverBudget(slide.Body); over != "" {
		s.advisory(path+".steps", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("process %s (otherwise it degrades to a bullet list)", over))
	}
}

// validateRoadmap applies the same usable-count rule to a roadmap's phases.
func validateRoadmap(path string, slide SlideSpec, s *semDiags) {
	if n := slides.UsablePhaseCount(slide.Body); s.requireUsableContent(path, "phases", slide.Body, n) && (n < 3 || n > 6) {
		s.advisory(path+".phases", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("roadmap has %d usable phases; 3–6 render as a phase-roadmap visual (otherwise it degrades to a bullet list)", n))
	}
}

// valuesChartTypes are chart types whose data is a flat {categories, values}
// list rather than {categories, series[]}.
var valuesChartTypes = map[string]bool{
	"pie_chart": true, "donut_chart": true, "pie": true, "donut": true, "funnel_chart": true, "funnel": true,
}

// chartDataShape returns the expected chart.data shape (as a readable string)
// and a minimal example for a chart type.
func chartDataShape(chartType string) (string, map[string]any) {
	if valuesChartTypes[chartType] {
		return "{categories:[…], values:[…]}", map[string]any{
			"categories": []any{"Enterprise", "Mid-market", "SMB"},
			"values":     []any{55, 30, 15},
		}
	}
	return "{categories:[…], series:[{name, values:[…]}]}", map[string]any{
		"categories": []any{"Q1", "Q2", "Q3", "Q4"},
		"series":     []any{map[string]any{"name": "Revenue", "values": []any{34, 40, 44, 48}}},
	}
}

// validateChartData checks a chart_insight chart carries data in the shape the
// renderer reads: chart.data {categories, series[]} (or {categories, values}
// for pie/donut). Findings point at chart.data — the path the author edits —
// and carry the expected shape and an example in fix.params. The flat
// chart.series form is NOT read by the compiler (it only reads chart.data), so
// it no longer counts as data.
func validateChartData(chartPath string, chart map[string]any, s *semDiags) {
	chartType, _ := chart["type"].(string)
	shape, example := chartDataShape(chartType)
	dataPath := chartPath + ".data"
	fix := &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{
		"path":           dataPath,
		"expected_shape": shape,
		"example":        example,
	}}

	data, ok := chart["data"].(map[string]any)
	if !ok || len(data) == 0 {
		s.advisoryFix(dataPath, diagnostics.CodeSemanticDensity,
			fmt.Sprintf("chart_insight chart has no data; the chart panel is dropped. Provide chart.data as %s", shape), fix)
		return
	}
	if chartDataHasValues(data, chartType) {
		return
	}
	s.advisoryFix(dataPath, diagnostics.CodeSemanticDensity,
		fmt.Sprintf("chart_insight chart.data declares no data series (found keys %s); expected chart.data as %s", joinQuoted(sortedKeys(data)), shape), fix)
}

// chartDataHasValues reports whether chart.data carries a non-empty series
// list (or, for pie/donut-style charts, a non-empty values list).
func chartDataHasValues(data map[string]any, chartType string) bool {
	if n, ok := listLen(data, "series"); ok && n > 0 {
		return true
	}
	if valuesChartTypes[chartType] {
		if n, ok := listLen(data, "values"); ok && n > 0 {
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
		// 3–5 columns are a visual now, not a degradation (go-slide-creator-3bgf):
		// stylish-panels gives each column its own titled panel and bullet list,
		// card-grid a titled card. Only a count or a shape neither can hold still
		// falls back to bullets, and only that case is worth a warning.
		if pattern := slides.ComparisonPattern(slide.Body); pattern != "" {
			return
		}
		s.advisory(path+".columns", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("a comparison renders as a visual with 2 columns (comparison-2col) or 3–5 (stylish-panels / card-grid); found %d, so this slide degrades to a bullet list — split it, or use kind raw_json2pptx with a table-highlight pattern for a wider matrix", len(cols)))
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
			// An unbalanced pair still gets a visual (card-grid) rather than
			// bullets, so say what it costs — the side-by-side row alignment —
			// instead of implying the content is lost.
			msg := "comparison columns are unbalanced; give each column the same number of items to get the side-by-side comparison-2col visual"
			if slides.ComparisonPattern(slide.Body) == "" {
				msg = "comparison columns are unbalanced; give each column the same number of items (otherwise this slide degrades to a bullet list)"
			}
			s.advisory(path+".columns", diagnostics.CodeSemanticDensity, msg)
			return
		}
	}
	// Balanced two-column comparisons still degrade to a bullet list when a column
	// exceeds the comparison-2col row cap. Validation alone passes the raw shape,
	// so flag the over-cap count here (blocking under strict) to keep validate in
	// step with what compile emits.
	if counts[0] > slides.ComparisonMaxRows {
		tail := "otherwise it degrades to a bullet list"
		if pattern := slides.ComparisonPattern(slide.Body); pattern != "" {
			tail = "otherwise it renders as " + pattern
		}
		s.advisory(path+".columns", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("comparison-2col renders 1–%d rows per column; found %d — split or shorten the comparison (%s)", slides.ComparisonMaxRows, counts[0], tail))
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
	for _, m := range weakSubstringMarkers {
		if strings.Contains(l, m) {
			return m
		}
	}
	if m := weakTokenRE.FindString(l); m != "" {
		return m
	}
	return ""
}

// fieldOrAliasPresent reports whether the canonical field or any of its aliases
// carries non-empty content. It backs required-one-of enforcement: the compiler
// reads aliases interchangeably, so any one of them satisfies the requirement.
func fieldOrAliasPresent(body map[string]any, field string, aliases []string) bool {
	if hasNonEmpty(body, field) {
		return true
	}
	for _, a := range aliases {
		if hasNonEmpty(body, a) {
			return true
		}
	}
	return false
}

// requiredFieldPhrase renders the field name for a missing-required message,
// listing accepted aliases so the agent learns both keys (e.g. `kpis` (or
// `metrics`)).
func requiredFieldPhrase(field string, aliases []string) string {
	if len(aliases) == 0 {
		return fmt.Sprintf("%q", field)
	}
	quoted := make([]string, len(aliases))
	for i, a := range aliases {
		quoted[i] = fmt.Sprintf("%q", a)
	}
	return fmt.Sprintf("%q (or %s)", field, strings.Join(quoted, ", "))
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
	case CodeParseError, CodeInvalidRoot, CodeInvalidMeta, CodeInvalidSlides, CodeInvalidSlide:
		return diagnostics.CodeInvalidJSON
	default:
		return diagnostics.CodeInvalidParameter
	}
}

// execSummaryPointCount counts the executive-summary points in whichever field
// carries them, so the density rule reads the same list the compiler does.
func execSummaryPointCount(body map[string]any) (int, bool) {
	if n, ok := listLen(body, "points"); ok {
		return n, true
	}
	return listLen(body, "takeaways")
}

// execSummaryPointsPath names the field the points came from, so the finding
// addresses what the author actually wrote.
func execSummaryPointsPath(body map[string]any) string {
	if _, ok := listLen(body, "points"); ok {
		return "points"
	}
	return "takeaways"
}

// validateOptionMatrix checks an option matrix against table-highlight's bounds
// (go-slide-creator-6o1r). A matrix outside them still renders — as a scored
// bullet list — so the rules are advisory, but they name the visual the author
// loses rather than only the number that is wrong.
func validateOptionMatrix(path string, slide SlideSpec, s *semDiags) {
	criteria, options := slides.UsableOptionMatrixCounts(slide.Body)
	criteriaField, optionsField := optionMatrixFieldNames(slide.Body)

	if !s.requireUsableContent(path, "criteria", slide.Body, criteria, "columns") {
		return
	}
	if !s.requireUsableContent(path, "options", slide.Body, options, "rows") {
		return
	}
	if criteria < 2 || criteria > 6 {
		s.advisory(path+"."+criteriaField, diagnostics.CodeSemanticDensity,
			fmt.Sprintf("table-highlight scores 2–6 criteria; found %d — group them or split the matrix across two slides (otherwise this slide degrades to a scored bullet list)", criteria))
	}
	if options < 2 || options > 6 {
		s.advisory(path+"."+optionsField, diagnostics.CodeSemanticDensity,
			fmt.Sprintf("table-highlight scores 2–6 options; found %d — shortlist them or split the matrix across two slides (otherwise this slide degrades to a scored bullet list)", options))
	}

	// One score per criterion is the matrix's own contract: a row with fewer
	// scores has columns the author never filled, and the pattern refuses it.
	rows, _ := slide.Body[optionsField].([]any)
	for i, raw := range rows {
		o, isMap := raw.(map[string]any)
		if !isMap {
			continue
		}
		scores, ok := optionScoreCount(o)
		if !ok {
			continue
		}
		if scores != criteria {
			s.advisory(fmt.Sprintf("%s.%s[%d].scores", path, optionsField, i), diagnostics.CodeSemanticDensity,
				fmt.Sprintf("option %d has %d scores for %d criteria; give every option exactly one score per criterion (otherwise this slide degrades to a scored bullet list)", i+1, scores, criteria))
		}
	}

	for _, field := range []string{"highlight_label", "corner_label"} {
		if label := strPayloadField(slide.Body, field); label != "" && !slides.OptionMatrixLabelFits(label) {
			s.advisory(path+"."+field, diagnostics.CodeSemanticDensity,
				fmt.Sprintf("%s is %d characters; table-highlight renders at most 24, so this badge is dropped (the row it marks is still highlighted)", field, len(label)))
		}
	}

	// A matrix inside every bound can still fail on a score the scale does not
	// read (a harvey column given "yes", a RAG column given 7). The compiler asks
	// the pattern; so does this, so validate and compile agree.
	if criteria >= 2 && criteria <= 6 && options >= 2 && options <= 6 && !slides.OptionMatrixPatternFeasible(slide.Body) {
		s.advisory(path+"."+optionsField, diagnostics.CodeSemanticDensity,
			"a score is not readable on this matrix's scale (harvey: 0–4 or none/quarter/half/three-quarter/full; rag: red/amber/green; text: ≤24 chars; \"-\" for n/a) — this slide degrades to a scored bullet list")
	}
}

// optionMatrixFieldNames reports which payload keys the matrix was authored
// with, so findings address what the author wrote rather than the canonical name.
func optionMatrixFieldNames(body map[string]any) (criteria, options string) {
	criteria, options = "criteria", "options"
	if !hasNonEmpty(body, "criteria") && hasNonEmpty(body, "columns") {
		criteria = "columns"
	}
	if !hasNonEmpty(body, "options") && hasNonEmpty(body, "rows") {
		options = "rows"
	}
	return criteria, options
}

// optionScoreCount returns the number of scores on an option row.
func optionScoreCount(option map[string]any) (int, bool) {
	for _, field := range []string{"scores", "values"} {
		if raw, ok := option[field].([]any); ok {
			return len(raw), true
		}
	}
	return 0, false
}

// strPayloadField reads a trimmed string payload field.
func strPayloadField(body map[string]any, key string) string {
	v, _ := body[key].(string)
	return strings.TrimSpace(v)
}

// validateTable checks a native table against the renderer's density rules
// (go-slide-creator-e4h1). A table past them still renders — the fit report
// reports density_exceeded on the compiled deck — but saying it at authoring
// time is cheaper than a render round trip, and a table with no header row does
// not render as a table at all.
func validateTable(path string, slide SlideSpec, s *semDiags) {
	columns, rows := slides.UsableTableCounts(slide.Body)
	headersField := "headers"
	if !hasNonEmpty(slide.Body, "headers") && hasNonEmpty(slide.Body, "columns") {
		headersField = "columns"
	}

	if !s.requireUsableContent(path, "headers", slide.Body, columns, "columns") {
		return
	}
	if !s.requireUsableContent(path, "rows", slide.Body, rows, "") {
		return
	}
	if columns > slides.TableMaxColumns {
		s.advisory(path+"."+headersField, diagnostics.CodeSemanticDensity,
			fmt.Sprintf("table has %d columns; the renderer lays out at most %d before the text is unreadable — drop a column or split the table across two slides", columns, slides.TableMaxColumns))
	}
	// The density rule counts the header row too, so a table of N data rows
	// occupies N+1 of the budget.
	if logical := rows + 1; logical > slides.TableMaxRows {
		s.advisory(path+".rows", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("table has %d rows including the header; the renderer lays out at most %d — split it across two slides", logical, slides.TableMaxRows))
	}

	// A row shorter than the header row leaves blank cells; a longer one has
	// values with no column to land in. Both are worth saying before the render.
	rawRows, _ := slide.Body["rows"].([]any)
	for i, raw := range rawRows {
		list, isList := raw.([]any)
		if !isList {
			continue
		}
		if len(list) != columns {
			s.advisory(fmt.Sprintf("%s.rows[%d]", path, i), diagnostics.CodeSemanticDensity,
				fmt.Sprintf("row %d has %d cells for %d columns; give every row one cell per header (short rows render blank cells)", i+1, len(list), columns))
		}
	}
}

// validateExecutiveSummary applies the exec-summary count and budget rules. The
// count rule and the visual are the same rule: 3–5 points render as the
// exec-summary pattern (numbered conclusions with supporting lines), anything
// else degrades to a bullet list. Say which one the author is getting rather
// than only that the count is off (go-slide-creator-ku6t).
func validateExecutiveSummary(path string, slide SlideSpec, s *semDiags) {
	n, ok := execSummaryPointCount(slide.Body)
	if !ok {
		return
	}
	pointsPath := path + "." + execSummaryPointsPath(slide.Body)
	if n < 3 || n > 5 {
		s.advisory(pointsPath, diagnostics.CodeSemanticDensity,
			fmt.Sprintf("executive summary has %d points; exec-summary renders 3–5 as numbered conclusions (otherwise it degrades to a bullet list)", n))
		return
	}
	if !slides.ExecSummaryPatternFeasible(slide.Body) {
		s.advisory(pointsPath, diagnostics.CodeSemanticDensity,
			"executive summary points exceed the exec-summary text budgets (lead ≤90 chars, support ≤200); shorten them or the slide degrades to a bullet list")
	}
}

// validateKPISnapshot applies the kpi-Nup count range and the compact cards'
// text budgets. Both decide the same thing — whether the slide renders as KPI
// cards or degrades to a bullet list — so both are reported here, and the
// budget rule asks the compiler rather than re-deriving the limits
// (go-slide-creator-5ok4).
func validateKPISnapshot(path string, slide SlideSpec, s *semDiags) {
	// Count KPIs the compiler can actually render, not raw list entries: a list
	// of blank/labelless cells passes the required-field gate but compiles to a
	// title-only slide. Below 1 usable cell is a blocking error; otherwise the
	// 2–6 density range is advisory.
	n := slides.UsableKPICount(slide.Body)
	if !s.requireUsableContent(path, "kpis", slide.Body, n, "metrics") {
		return
	}
	if n < 2 || n > 6 {
		s.advisory(path+".kpis", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("kpi snapshot has %d usable KPIs; 2–6 render as KPI cards (otherwise it degrades to a bullet list)", n))
		return
	}
	// A metric can be valid semantically and still too long for the compact
	// cards, and the compiler silently swaps the whole slide for bullets. Say
	// which value broke which budget here, where the author can shorten it.
	if reason := slides.KPIDegradeReason(slide.Body); reason != "" {
		s.advisory(path+".kpis", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("kpi snapshot degrades to a bullet list — %s; shorten the value or drop a metric to keep the KPI cards", reason))
	}
}

// validateCompositionOverride reports a pattern / layout override the kind has
// no candidate for, naming the ones it does have.
func validateCompositionOverride(path string, slide SlideSpec, s *semDiags) {
	requestedPattern := slide.String("pattern")
	requestedLayout := slide.String("layout")
	if requestedPattern == "" && requestedLayout == "" {
		return
	}

	alternatives := SlideAlternatives(slide.Kind, slide.Body)
	var patterns, layouts []string
	for _, c := range alternatives {
		if c.Pattern != "" {
			patterns = append(patterns, c.Pattern)
		}
		if c.Layout != "" {
			layouts = append(layouts, c.Layout)
		}
	}

	if requestedPattern != "" && !containsString(patterns, requestedPattern) {
		s.advisoryFix(path+".pattern", diagnostics.CodeSemanticPatternNotAvailable,
			fmt.Sprintf("%q is not one of the patterns a %q slide compiles to, so the override is ignored and the compiler keeps its own choice; available: %s",
				requestedPattern, slide.Kind, joinOrNone(patterns)),
			&diagnostics.Fix{
				Kind:   "use_one_of",
				Params: map[string]any{"path": path + ".pattern", "allowed": patterns},
			})
	}
	if requestedLayout != "" && !containsString(layouts, requestedLayout) {
		s.advisoryFix(path+".layout", diagnostics.CodeSemanticPatternNotAvailable,
			fmt.Sprintf("%q is not one of the layouts a %q slide compiles to, so the override is ignored and the compiler keeps its own choice; available: %s",
				requestedLayout, slide.Kind, joinOrNone(layouts)),
			&diagnostics.Fix{
				Kind:   "use_one_of",
				Params: map[string]any{"path": path + ".layout", "allowed": layouts},
			})
	}
}

// containsString reports whether list holds v.
func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// joinOrNone renders a candidate list for a message, naming the empty case
// rather than trailing off after "available:".
func joinOrNone(list []string) string {
	if len(list) == 0 {
		return "none — this kind has no alternative composition"
	}
	return strings.Join(list, ", ")
}

// validateChartInsight applies the chart-data and insight-count rules.
func validateChartInsight(path string, slide SlideSpec, s *semDiags) {
	// A chart without usable data compiles to an insights-only slide (the chart
	// panel is dropped), so flag it at the path the author must edit —
	// chart.data — with the expected shape for the chart type.
	if chart, ok := slide.Body["chart"].(map[string]any); ok {
		validateChartData(path+".chart", chart, s)
	}
	// chart-insights-split renders 1–6 insights alongside the chart; beyond the
	// cap the compiler degrades to a native two-column slide (chart beside the
	// full insight list). Flag the over-cap count here (blocking under strict)
	// to keep validate in step with compile.
	if n := slides.ChartInsightInsightCount(slide.Body); n > slides.ChartInsightMaxInsights {
		s.advisory(path+".insights", diagnostics.CodeSemanticDensity,
			fmt.Sprintf("chart-insights-split renders 1–%d insights alongside the chart; found %d — split or shorten the insights (otherwise it degrades to a native two-column slide: chart beside the full insight list)", slides.ChartInsightMaxInsights, n))
	}
}
