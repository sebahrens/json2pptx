package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// Design-mode enforcement on the DeckSpec path (go-slide-creator-rs4h).
//
// A compiled deck is stamped design_mode "constrained" — the template owns
// sizes and colours — but the check that enforces it ran only on the raw
// generate path. The raw_json2pptx escape hatch carries an author's slide
// payload straight through, so the SAME shape_grid was refused by
// generate_presentation with blocking design_mode_violation findings and
// accepted in silence by render_deck_spec. An agent learned opposite rules
// depending on which recommended path it took, and the constrained-mode
// guarantee was not a guarantee on the fast one.
//
// The escape hatch needs a way to say the raw values are deliberate, which is
// what meta.design_mode: "free" is for; without it there was no opt-out on this
// path at all.

// compiledDesignModeDiagnostics returns the design-mode violations of a
// compiled DeckSpec, in the transport-neutral shape the spec tools report.
// Returns nil when the deck is in free mode or carries no violations.
func compiledDesignModeDiagnostics(input *PresentationInput) []diagnostics.Diagnostic {
	if input == nil {
		return nil
	}
	return designModeDiagnostics(validateDesignMode(input))
}

// appendCompiledDesignModeDiags folds the violations into a compile result's
// diagnostics so every DeckSpec surface reports them the same way.
func appendCompiledDesignModeDiags(result *semantic.CompileResult, input *PresentationInput) {
	if result == nil {
		return
	}
	result.Diagnostics = append(result.Diagnostics, compiledDesignModeDiagnostics(input)...)
}

// specDesignModeDiagnostics compiles a spec far enough to judge its design
// mode. A spec that cannot be parsed or compiled reports nothing here: those
// failures are the validation pass's own to describe.
func specDesignModeDiagnostics(filename string, data []byte, strict semantic.Strictness) []diagnostics.Diagnostic {
	spec, parseDiags := semantic.Parse(filename, data)
	if spec == nil || parseDiags.HasErrors() {
		return nil
	}
	input, _, err := semantic.Compile(spec, semantic.CompileOptions{Strict: strict})
	if err != nil || input == nil {
		return nil
	}
	return compiledDesignModeDiagnostics(input)
}

// blockingDesignModeError summarises the violations for a caller that refuses
// the deck, mirroring the generate path's message.
func blockingDesignModeError(diags []diagnostics.Diagnostic) error {
	if len(diags) == 0 {
		return nil
	}
	return fmt.Errorf("design mode violations: %d slide element(s) set values the template owns in constrained mode; "+
		"use the template's semantic colours and sizes, or set meta.design_mode: \"free\" when the raw values are deliberate",
		len(diags))
}
