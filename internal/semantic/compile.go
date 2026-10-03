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
	"github.com/sebahrens/json2pptx/internal/slidepath"
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
		TypeScale:      "comfortable",
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
	// The deck-level default source is applied per data slide by the raw
	// pipeline's defaults pass, exactly as for a raw deck's top-level source.
	input.Source = ir.Source
	if ir.TypeScale != "" {
		input.TypeScale = ir.TypeScale
	}
	// A compiled deck is constrained; a spec that reaches for the
	// raw_json2pptx escape hatch can say "free" and mean it
	// (go-slide-creator-rs4h).
	if ir.DesignMode != "" {
		input.DesignMode = ir.DesignMode
	}
	input.Chrome = compileChrome(ir)
	forcedLayouts := make(map[int]string, len(ir.LayoutCoverage.Assigned))
	for _, assignment := range ir.LayoutCoverage.Assigned {
		forcedLayouts[assignment.SlideIndex] = assignment.Layout
	}

	for i := range ir.Slides {
		si := &ir.Slides[i]
		in := slides.Input{
			SourceIndex: si.SourceIndex,
			SourcePath:  si.SourcePath,
			OutputIndex: len(input.Slides),
			Title:       si.Title,
			Takeaway:    si.Takeaway,
			Pattern:     si.Visual.Pattern,
			Layout:      si.Visual.Layout,
			Body:        si.Body,
			Override:    si.Override,
		}
		compiled, links, err := compileSlide(si.Kind, in)
		if err != nil {
			return nil, result, fmt.Errorf("slide %d (%s): %w", si.SourceIndex, si.Kind, err)
		}
		if requiredLayout := forcedLayouts[i]; requiredLayout != "" {
			compiled.LayoutID = requiredLayout
			if requiredLayout == "blank-canvas" {
				compiled.Headline = si.Title
				compiled.Content = withoutTitleContent(compiled.Content)
			}
		}
		outputIndex := len(input.Slides)
		if si.SourcePath != "" {
			ir.SourceMap.SetSlidePath(outputIndex, si.SourcePath)
		}
		for _, l := range links {
			registerSourceLink(ir.SourceMap, compiled, l, outputIndex, si.SourceIndex)
			if si.Kind == KindRegions {
				// Grid findings (a nested pattern's budget, a cell's fit)
				// address a region's cell by JSON Pointer; register that
				// spelling so they resolve to the region, not the slide
				// (go-slide-creator-fn2ka).
				ir.SourceMap.Add(dottedToPointer(l.RawPath), l.SemanticPath, si.SourceIndex)
			}
		}
		// Universal per-slide fields every kind accepts: speaker notes and a
		// source/footnote line. They are plain strings with no layout impact,
		// and before go-slide-creator-zmjs only chart_insight could carry a
		// source — an option matrix or financial case could not cite anything.
		applyUniversalSlideFields(compiled, si, ir.SourceMap)
		// meta.source reaches every data slide through the raw pipeline's
		// deck-source default, which reads shape-grid cells too, so a table,
		// regions or option-matrix slide needs no per-kind copy here
		// (go-slide-creator-fn2ka, go-slide-creator-q2emv).
		// DATA_WITHOUT_SOURCE addresses the slide's source field, which a
		// data slide without one never set; map its pointer to the DeckSpec
		// field the author fills (go-slide-creator-cuszt).
		semSlide := si.SourcePath
		if semSlide == "" {
			semSlide = fmt.Sprintf("slides[%d]", si.SourceIndex)
		}
		ir.SourceMap.Add(slidepath.SlideField(outputIndex, "source"), semSlide+".source", si.SourceIndex)
		compiled.SectionTitle = si.SectionTitle
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

// NativeLayoutAlternative returns the layout override (e.g. "content") that
// takes a pattern slide off its generated grid onto template-native text
// without dropping source, or "" when the slide has no pattern or none of its
// kind's layout-only alternatives actually compiles without one. The check
// compiles the slide with the override, so a composition the kind merely
// lists is never offered as a repair (go-slide-creator-b7qqg.4).
func NativeLayoutAlternative(slide SlideIR) string {
	if slide.Visual.Pattern == "" {
		return ""
	}
	for _, alt := range slide.Visual.Alternatives {
		if alt.Pattern != "" || alt.Layout == "" || alt.Layout == slide.Visual.Layout {
			continue
		}
		compiled, _, err := compileSlide(slide.Kind, slides.Input{
			SourceIndex: slide.SourceIndex,
			SourcePath:  slide.SourcePath,
			Title:       slide.Title,
			Takeaway:    slide.Takeaway,
			Layout:      alt.Layout,
			Body:        slide.Body,
			Override:    slides.Composition{Layout: alt.Layout},
		})
		if err == nil && compiled != nil && compiled.Pattern == nil && compiled.ShapeGrid == nil && compiled.Compose == nil {
			return alt.Layout
		}
	}
	return ""
}

func withoutTitleContent(content []deckinput.ContentInput) []deckinput.ContentInput {
	out := content[:0]
	for _, item := range content {
		id := strings.ToLower(strings.TrimSpace(item.PlaceholderID))
		if id == "title" || strings.HasPrefix(id, "title_") {
			continue
		}
		out = append(out, item)
	}
	return out
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
	case KindBridge, KindPillars, KindOrg:
		return compileExtendedKind(kind, in)
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
	case KindNextSteps:
		return slides.CompileNextSteps(in)
	case KindClosing:
		return slides.CompileClosing(in)
	case KindRawJSON2pptx:
		return slides.CompileRaw(in)
	default:
		return compileOtherKind(kind, in)
	}
}

// compileOtherKind compiles the kinds compileSlide's switch leaves out: the
// regions kind, and the generic content fallback for everything else.
func compileOtherKind(kind SlideKind, in slides.Input) (*deckinput.SlideInput, []slides.SourceLink, error) {
	if kind == KindRegions {
		return slides.CompileRegions(in)
	}
	return slides.CompileFallback(in)
}

func compileExtendedKind(kind SlideKind, in slides.Input) (*deckinput.SlideInput, []slides.SourceLink, error) {
	switch kind {
	case KindBridge:
		return slides.CompileBridge(in)
	case KindPillars:
		return slides.CompilePillars(in)
	default:
		return slides.CompileOrg(in)
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
//
// A spec with no meta.chrome gets consulting chrome by default
// (go-slide-creator-1iy0x): page numbers on every slide but the title and
// closing, meta.date in the footer, and the section tracker when the deck has
// sections. An unnumbered deck cannot be referenced in a meeting ("go to page
// 12") and reads as a draft. Opt out with meta.chrome.page_numbers.enabled:
// false; an explicit chrome block is copied as written.
func compileChrome(ir *DeckIR) *deckinput.ChromeInput {
	if ir == nil {
		return nil
	}
	if ir.Chrome == nil {
		return defaultChrome(ir)
	}
	c := ir.Chrome
	out := &deckinput.ChromeInput{
		Confidentiality: c.Confidentiality,
		ClientName:      c.ClientName,
		ProjectCode:     c.ProjectCode,
		FooterDate:      firstNonEmptyStr(c.FooterDate, ir.Date),
		SectionCrumb:    c.SectionCrumb,
		Tracker:         c.Tracker,
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

// defaultChrome is the chrome a spec without meta.chrome renders with: page
// numbers (the default skip list leaves the title and closing slides
// unnumbered), the footer date from meta.date, and the section tracker when the
// deck has section dividers — authored section kinds or structure.sections.
func defaultChrome(ir *DeckIR) *deckinput.ChromeInput {
	on := true
	out := &deckinput.ChromeInput{FooterDate: ir.Date, PageNumbers: &deckinput.PageNumbersInput{Enabled: &on}}
	for i := range ir.Slides {
		if ir.Slides[i].Kind == KindSection {
			out.Tracker = true
			break
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
	semSlide := si.SourcePath
	if semSlide == "" {
		semSlide = fmt.Sprintf("slides[%d]", si.SourceIndex)
	}
	if notes := bodyString(si.Body, "notes", "speaker_notes"); notes != "" {
		// A kind compiler may already have filed text here (a takeaway kept
		// off a slide that has its own conclusion band, go-slide-creator-zvu7c);
		// the author's notes come first.
		if compiled.SpeakerNotes != "" {
			notes += "\n\n" + compiled.SpeakerNotes
		}
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

// registerSourceLink records one compiler source link plus the JSON Pointer
// aliases render/validation findings address the same content by.
func registerSourceLink(sm *SourceMap, compiled *deckinput.SlideInput, l slides.SourceLink, outputIndex, sourceIndex int) {
	sm.Add(l.RawPath, l.SemanticPath, sourceIndex)
	// Validation findings now address authored content by array index.
	// Keep an item-level pointer alias so a finding can map back to the
	// semantic field that produced it.
	if alias := indexedContentAliasPath(compiled, l.RawPath, outputIndex); alias != "" {
		sm.Add(alias, l.SemanticPath, sourceIndex)
	}
	// Preserve the legacy placeholder-name alias for older findings and
	// repair_slide selectors. Compiler links use indexed dotted paths.
	if alias := placeholderAliasPath(compiled, l.RawPath, outputIndex); alias != "" {
		sm.Add(alias, l.SemanticPath, sourceIndex)
	}
	// Chrome fit findings use JSON Pointer paths, whereas compiler links
	// use dotted raw paths. Register the takeaway's pointer spelling too.
	if strings.HasSuffix(l.RawPath, ".takeaway") {
		sm.Add(slidepath.SlideField(outputIndex, "takeaway"), l.SemanticPath, sourceIndex)
	}
	// So do overlay findings (OVERLAY_TARGET_CROPPED at
	// /slides/N/overlays/M/to/anchor_image), which must land on the callout
	// that was authored (go-slide-creator-n3j96).
	if strings.Contains(l.RawPath, ".overlays[") {
		sm.Add(dottedToPointer(l.RawPath), l.SemanticPath, sourceIndex)
	}
	// A whole-slide link (the raw_json2pptx escape hatch) is spelled in
	// dotted form; register its JSON Pointer spelling too, so a render
	// finding anywhere inside the passed-through slide (e.g. a readability
	// refusal at /slides/N/content/M/diagram_value) resolves to the authored
	// slide payload (go-slide-creator-b7qqg.25).
	if normalizePath(l.RawPath) == fmt.Sprintf("slides[%d]", outputIndex) {
		sm.Add(slidepath.Slide(outputIndex), l.SemanticPath, sourceIndex)
	}
}

// placeholderAliasPath preserves the legacy placeholder-name selector for
// existing source-map clients. New findings use indexedContentAliasPath.
func placeholderAliasPath(compiled *deckinput.SlideInput, rawPath string, outputIndex int) string {
	idx := linkedContentIndex(compiled, rawPath)
	if idx < 0 {
		return ""
	}
	ph := compiled.Content[idx].PlaceholderID
	if ph == "" {
		return ""
	}
	return fmt.Sprintf("/slides/%d/content/%s", outputIndex, ph)
}

func indexedContentAliasPath(compiled *deckinput.SlideInput, rawPath string, outputIndex int) string {
	idx := linkedContentIndex(compiled, rawPath)
	if idx < 0 {
		return ""
	}
	return slidepath.ContentIndex(outputIndex, idx)
}

func linkedContentIndex(compiled *deckinput.SlideInput, rawPath string) int {
	if compiled == nil {
		return -1
	}
	const marker = ".content["
	i := strings.Index(rawPath, marker)
	if i < 0 {
		return -1
	}
	rest := rawPath[i+len(marker):]
	end := strings.IndexByte(rest, ']')
	if end <= 0 {
		return -1
	}
	idx, err := strconv.Atoi(rest[:end])
	if err != nil || idx < 0 || idx >= len(compiled.Content) {
		return -1
	}
	return idx
}
