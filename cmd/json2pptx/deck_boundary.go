package main

import (
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/policy/emoji"
)

// deckRequiredFieldDiagnostics is shared by raw validation and generation so
// the same missing field has one message, schema hint and repair call.
func deckRequiredFieldDiagnostics(input *PresentationInput) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	if input.Template == "" && input.TemplatePath == "" {
		out = append(out, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "/template", Message: "template is required: a registered name, or template_path for a local .pptx inside base_dir",
			Severity: diagnostics.SeverityError, ExpectedType: "string", ExampleValue: "midnight-blue",
			Fix: &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "template"}}, NextToolCall: nextCallListTemplates(),
		})
	}
	if len(input.Slides) == 0 {
		d := deckSlidesRequiredDiagnostic()
		d.ExpectedType = "array"
		d.ExampleValue = []any{map[string]any{"layout_id": "title", "content": []any{}}}
		d.NextToolCall = nextCallGetInputSchema()
		out = append(out, d)
	}
	return out
}

// deckBoundaryDiagnostics applies the authored-deck policies shared by CLI and
// MCP generation. Parsing and partial-mode slide selection happen before this.
func deckBoundaryDiagnostics(input *PresentationInput, placeholderPolicy string, skipDesignMode bool) ([]diagnostics.Diagnostic, []string) {
	out := deckRequiredFieldDiagnostics(input)
	out = append(out, diagnostics.FromValidationErrors(checkInputEnumValues(input))...)
	if !skipDesignMode {
		out = append(out, designModeDiagnostics(validateDesignMode(input))...)
	}
	out = append(out, droppedDiagramColorDiagnostics(collectDroppedDiagramColorWarnings(input))...)
	out = append(out, noEmojiDiagnostics(emoji.ValidateNoEmojiInText(input))...)
	ph, _ := scanPlaceholderDiagnostics(input, placeholderPolicy)
	out = append(out, ph...)
	return out, validateStructure(input)
}
