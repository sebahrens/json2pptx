package semantic

// This file implements the MVP semantic-to-raw compiler: given a parsed
// DeckSpec it normalizes the deck to a DeckIR, dispatches each planned slide to
// its per-kind compiler in internal/semantic/slides, and assembles a raw
// internal/deckinput.PresentationInput consumable by the existing generator —
// proving that compact semantic specs compile to the established raw model
// without a new renderer. The SourceMap is populated as slides are emitted so
// raw findings trace back to the semantic fields the author wrote.

import (
	"fmt"

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
		for _, l := range links {
			ir.SourceMap.Add(l.RawPath, l.SemanticPath, si.SourceIndex)
		}
		input.Slides = append(input.Slides, *compiled)
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
