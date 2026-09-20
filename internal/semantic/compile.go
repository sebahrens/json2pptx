package semantic

// This file implements the MVP semantic-to-raw compiler: given a parsed
// DeckSpec it normalizes the deck to a DeckIR, dispatches each planned slide to
// its per-kind compiler in internal/semantic/slides, and assembles a raw
// internal/deckinput.PresentationInput consumable by the existing generator —
// proving that compact semantic specs compile to the established raw model
// without a new renderer. The SourceMap is populated as slides are emitted so
// raw findings trace back to the semantic fields the author wrote.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

// CompileOptions tunes the compile pass.
type CompileOptions struct {
	// Strict is the validation strictness applied before compiling. An empty
	// value defaults to StrictnessWarn (advisory findings become warnings).
	Strict Strictness
	// DefaultTemplate is the json2pptx template used when the deck meta pins no
	// template of its own.
	DefaultTemplate string
	// OutputFilename optionally sets the emitted deck's output_filename.
	OutputFilename string
	// AccentStrategy optionally overrides the emitted accent strategy. Empty
	// leaves the raw default ("primary").
	AccentStrategy string
}

// CompileResult carries the compiler's planning artifacts alongside the emitted
// deck: the normalized IR, the populated raw<->semantic SourceMap (also reachable
// via IR.SourceMap), and the validation diagnostics gathered before compiling.
type CompileResult struct {
	IR          *DeckIR
	SourceMap   *SourceMap
	Diagnostics []diagnostics.Diagnostic
}

// Compile validates and compiles a semantic DeckSpec into a raw
// PresentationInput. It returns the emitted deck, a CompileResult with the IR,
// source map, and diagnostics, and an error. When validation surfaces blocking
// (error-severity) findings the deck is not emitted: the returned input is nil
// and the error reports the blocking count, while the diagnostics remain
// available on the result for presentation.
func Compile(spec *DeckSpec, opts CompileOptions) (*deckinput.PresentationInput, *CompileResult, error) {
	strict := opts.Strict
	switch strict {
	case StrictnessOff, StrictnessWarn, StrictnessStrict:
	default:
		strict = StrictnessWarn
	}

	diags := Validate(spec, strict)
	ir := Normalize(spec)
	// Deck-rhythm advisories are deck-level (read from the normalized IR), so they
	// are gathered here rather than in the per-slide Validate pass. Under strict
	// they become errors and block the compile alongside structural errors.
	diags = append(diags, rhythmDiagnostics(ir, strict)...)
	result := &CompileResult{IR: ir, SourceMap: ir.SourceMap, Diagnostics: diags}

	if diagnostics.HasErrors(diags) {
		return nil, result, fmt.Errorf("semantic deck cannot compile: %d blocking error(s)", countErrors(diags))
	}

	input := &deckinput.PresentationInput{
		// Template precedence: the spec's own pin wins, then the caller default,
		// then the archetype's preferred template.
		Template:       firstNonEmptyStr(ir.Template, opts.DefaultTemplate, ir.ArchetypeTemplate),
		OutputFilename: opts.OutputFilename,
		DesignMode:     "constrained",
	}
	if opts.AccentStrategy != "" {
		input.AccentStrategy = opts.AccentStrategy
	}
	// Deck-level passthroughs: the spec's own choices win over caller defaults.
	if ir.AccentStrategy != "" {
		input.AccentStrategy = ir.AccentStrategy
	}
	if ir.ViewingMode != "" {
		input.ViewingMode = ir.ViewingMode
	}
	// A compiled deck is constrained; a spec that reaches for the
	// raw_json2pptx escape hatch can say "free" and mean it
	// (go-slide-creator-rs4h).
	if ir.DesignMode != "" {
		input.DesignMode = ir.DesignMode
	}
	input.Chrome = compileChrome(ir)

	for i := range ir.Slides {
		si := &ir.Slides[i]
		in := slides.Input{
			SourceIndex: si.SourceIndex,
			OutputIndex: len(input.Slides),
			Title:       si.Title,
			Takeaway:    si.Takeaway,
			Pattern:     si.Visual.Pattern,
			Layout:      si.Visual.Layout,
			Body:        si.Body,
		}
		compiled, links, err := compileSlide(si.Kind, in)
		if err != nil {
			return nil, result, fmt.Errorf("slide %d (%s): %w", si.SourceIndex, si.Kind, err)
		}
		outputIndex := len(input.Slides)
		for _, l := range links {
			ir.SourceMap.Add(l.RawPath, l.SemanticPath, si.SourceIndex)
			// Fit findings address content by PLACEHOLDER ("/slides/0/content/body"),
			// while the compiler's links address it by index
			// ("slides[0].content[1].bullets_value"). Register the placeholder
			// spelling too, or every finding the shared collectors produce comes
			// back with no semantic_path (go-slide-creator-05wn).
			if alias := placeholderAliasPath(compiled, l.RawPath, outputIndex); alias != "" {
				ir.SourceMap.Add(alias, l.SemanticPath, si.SourceIndex)
			}
		}
		// Universal per-slide fields every kind accepts: speaker notes and a
		// source/footnote line. They are plain strings with no layout impact,
		// and before go-slide-creator-zmjs only chart_insight could carry a
		// source — an option matrix or financial case could not cite anything.
		applyUniversalSlideFields(compiled, si, ir.SourceMap)
		input.Slides = append(input.Slides, *compiled)
	}

	// Post-compile raw preflight: validate the emitted patterns against the raw
	// pattern registry — the same gate the renderer applies — so known-invalid
	// raw JSON is caught here and traced back to the semantic source, rather than
	// failing later at render. Failures are blocking; a clean preflight is a
	// proxy for "the compiled deck renders without a pattern-validation refusal".
	if pf := preflightRawPatterns(input, ir.SourceMap); len(pf) > 0 {
		diags = append(diags, pf...)
		result.Diagnostics = diags
		if diagnostics.HasErrors(pf) {
			return nil, result, fmt.Errorf("semantic deck cannot compile: %d raw preflight error(s)", countErrors(pf))
		}
	}

	return input, result, nil
}

// compileSlide dispatches a planned slide to its per-kind compiler, falling back
// to the generic content compiler for kinds the MVP does not yet model with a
// bespoke layout.
func compileSlide(kind SlideKind, in slides.Input) (*deckinput.SlideInput, []slides.SourceLink, error) {
	switch kind {
	case KindTitle:
		return slides.CompileTitle(in)
	case KindSection:
		return slides.CompileSection(in)
	case KindExecutiveSummary:
		return slides.CompileExecutiveSummary(in)
	case KindKPISnapshot:
		return slides.CompileKPISnapshot(in)
	case KindChartInsight:
		return slides.CompileChartInsight(in)
	case KindComparison:
		return slides.CompileComparison(in)
	case KindOptionMatrix:
		return slides.CompileOptionMatrix(in)
	case KindTable:
		return slides.CompileTable(in)
	case KindArchitecture:
		return slides.CompileArchitecture(in)
	case KindAgenda:
		return slides.CompileAgenda(in)
	case KindQuote:
		return slides.CompileQuote(in)
	case KindTeam:
		return slides.CompileTeam(in)
	case KindStat:
		return slides.CompileStat(in)
	case KindTimeline:
		return slides.CompileTimeline(in)
	case KindMatrix2x2:
		return slides.CompileMatrix(in)
	case KindFramework:
		return slides.CompileFramework(in)
	case KindImageCase:
		return slides.CompileImageCase(in)
	case KindProcess:
		return slides.CompileProcess(in)
	case KindRoadmap:
		return slides.CompileRoadmap(in)
	case KindDecision:
		return slides.CompileDecision(in)
	case KindClosing:
		return slides.CompileClosing(in)
	case KindRawJSON2pptx:
		return slides.CompileRaw(in)
	default:
		return slides.CompileFallback(in)
	}
}

// countErrors counts the error-severity diagnostics in ds.
func countErrors(ds []diagnostics.Diagnostic) int {
	n := 0
	for i := range ds {
		if ds[i].Severity == diagnostics.SeverityError {
			n++
		}
	}
	return n
}

// firstNonEmptyStr returns the first non-empty argument.
func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// compileChrome copies the spec's chrome block into the raw model, filling the
// footer date from meta.date when the block does not set its own. Deck furniture
// — a confidentiality stamp, the client name, page numbers — is on every board
// deck, and the semantic path could not express any of it
// (go-slide-creator-zmjs).
func compileChrome(ir *DeckIR) *deckinput.ChromeInput {
	if ir == nil || ir.Chrome == nil {
		return nil
	}
	c := ir.Chrome
	out := &deckinput.ChromeInput{
		Confidentiality: c.Confidentiality,
		ClientName:      c.ClientName,
		ProjectCode:     c.ProjectCode,
		FooterDate:      firstNonEmptyStr(c.FooterDate, ir.Date),
		SectionCrumb:    c.SectionCrumb,
	}
	if c.PageNumbers != nil {
		out.PageNumbers = &deckinput.PageNumbersInput{
			Enabled: c.PageNumbers.Enabled,
			Format:  c.PageNumbers.Format,
			Skip:    append([]string(nil), c.PageNumbers.Skip...),
		}
	}
	return out
}

// applyUniversalSlideFields copies the kind-independent payload fields (speaker
// notes, source line) onto a compiled slide and records their source links.
func applyUniversalSlideFields(compiled *deckinput.SlideInput, si *SlideIR, sm *SourceMap) {
	if compiled == nil || si == nil {
		return
	}
	rawSlide := fmt.Sprintf("/slides/%d", si.SourceIndex)
	semSlide := fmt.Sprintf("slides[%d]", si.SourceIndex)
	if notes := bodyString(si.Body, "notes", "speaker_notes"); notes != "" {
		compiled.SpeakerNotes = notes
		if sm != nil {
			sm.Add(rawSlide+"/speaker_notes", semSlide+".notes", si.SourceIndex)
		}
	}
	// Some kinds render the source themselves — chart_insight puts it in the
	// chart-insights-split pattern's own values, under the chart. Setting the
	// slide-level source as well printed it twice: once under the chart and once
	// in the chrome source band (go-slide-creator-xg48).
	if compiled.Source == "" && !patternRendersSource(compiled) {
		if src := bodyString(si.Body, "source"); src != "" {
			compiled.Source = src
			if sm != nil {
				sm.Add(rawSlide+"/source", semSlide+".source", si.SourceIndex)
			}
		}
	}
}

// patternRendersSource reports whether a compiled slide's pattern already
// carries a non-empty "source" value, and so will draw the attribution itself.
func patternRendersSource(compiled *deckinput.SlideInput) bool {
	if compiled == nil || compiled.Pattern == nil || len(compiled.Pattern.Values) == 0 {
		return false
	}
	var vals map[string]json.RawMessage
	if err := json.Unmarshal(compiled.Pattern.Values, &vals); err != nil {
		return false
	}
	raw, ok := vals["source"]
	if !ok {
		return false
	}
	var src string
	return json.Unmarshal(raw, &src) == nil && strings.TrimSpace(src) != ""
}

// bodyString returns the first non-empty string value among keys.
func bodyString(body map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := body[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// placeholderAliasPath converts a compiler link path that addresses a content
// item by index into the placeholder-addressed pointer form the fit collectors
// emit. Returns "" when the link is not a content-item path or the index has no
// placeholder ID.
func placeholderAliasPath(compiled *deckinput.SlideInput, rawPath string, outputIndex int) string {
	if compiled == nil {
		return ""
	}
	const marker = ".content["
	i := strings.Index(rawPath, marker)
	if i < 0 {
		return ""
	}
	rest := rawPath[i+len(marker):]
	end := strings.IndexByte(rest, ']')
	if end <= 0 {
		return ""
	}
	idx, err := strconv.Atoi(rest[:end])
	if err != nil || idx < 0 || idx >= len(compiled.Content) {
		return ""
	}
	ph := compiled.Content[idx].PlaceholderID
	if ph == "" {
		return ""
	}
	return fmt.Sprintf("/slides/%d/content/%s", outputIndex, ph)
}
