package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// presentationForTool accepts an inline raw presentation, a stored raw deck,
// or a stored semantic deck. Semantic compilation is read-only; raw repair
// never mutates the DeckSpec behind a semantic deck_id.
func (mc *mcpConfig) presentationForTool(tool string, request mcp.CallToolRequest) (string, string, *mcp.CallToolResult) {
	args := request.GetArguments()
	_, hasPresentation := args["presentation"]
	rawID, hasID := args["deck_id"]
	if _, hasPatch := args["patch"]; hasPatch {
		return "", "", argInvalidValue(tool, "INVALID_PARAMETER", "patch", "this tool does not edit a DeckSpec; apply patch with validate_deck_spec or render_deck_spec first", "array", specPatchExample(), nil)
	}
	if hasPresentation && hasID {
		return "", "", argError(argErrorEnvelope{
			Code: diagnostics.CodeAmbiguousInput, Path: "deck_id",
			Message:      "set presentation OR deck_id, not both",
			ExpectedType: "string", NextToolCall: nextCallRetry(tool, "deck_id"),
		})
	}
	if hasID {
		id, ok := rawID.(string)
		if !ok || strings.TrimSpace(id) == "" {
			return "", "", argInvalidValue(tool, "INVALID_PARAMETER", "deck_id", "deck_id must be a non-empty string", "string", "<deck-id>", nil)
		}
		id = strings.TrimSpace(id)
		handle, ok := mc.deckHandles.Load(id)
		if !ok {
			return "", "", argInvalidValue(tool, "INVALID_PARAMETER", "deck_id", fmt.Sprintf("deck_id %q is unknown or expired; send the presentation or DeckSpec again", id), "string", "<deck-id>", nil)
		}
		if handle.RawPresentation != nil {
			return string(handle.RawPresentation), id, nil
		}
		spec, parseDiags := semantic.Parse(handle.Filename, handle.Spec)
		if spec == nil || parseDiags.HasErrors() {
			return "", "", apiSemanticCompileError(tool, "stored DeckSpec could not be parsed")
		}
		input, _, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn, DefaultTemplate: handle.Template})
		if err != nil || input == nil {
			return "", "", apiSemanticCompileError(tool, fmt.Sprintf("stored DeckSpec could not be compiled: %v", err))
		}
		applyHandleTemplateFile(request, handle, spec.Meta.Template, input)
		data, err := json.Marshal(input)
		if err != nil {
			return "", "", apiSemanticCompileError(tool, fmt.Sprintf("compiled presentation could not be encoded: %v", err))
		}
		return string(data), id, nil
	}
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return "", "", paramErr
	}
	if jsonStr == "" {
		return "", "", argRequired(request, tool, "presentation", "object", map[string]any{
			"template": "<template-name>", "slides": []any{},
		}, nextCallGetInputSchema())
	}
	return jsonStr, "", nil
}

// applyHandleTemplateFile carries a semantic handle's bring-your-own template
// into the compiled raw deck, so tools that act on a deck_id (score_deck,
// generate_presentation, …) use the .pptx render_deck_spec rendered with
// instead of failing on template "" (go-slide-creator-b7qqg.8). The template
// file was vetted against the handle's base_dir, so that root also becomes the
// call's base_dir unless the caller passes one: the template_path containment
// guard and relative asset paths then resolve exactly as they did at render.
// A spec that pins meta.template keeps it.
func applyHandleTemplateFile(request mcp.CallToolRequest, handle *deckHandle, metaTemplate string, input *PresentationInput) {
	if handle.TemplatePath == "" || metaTemplate != "" {
		return
	}
	input.Template = ""
	input.TemplatePath = handle.TemplatePath
	args := request.GetArguments()
	if raw, _ := args["base_dir"].(string); strings.TrimSpace(raw) == "" && handle.BaseDir != "" && args != nil {
		args["base_dir"] = handle.BaseDir
	}
}

func apiSemanticCompileError(tool, message string) *mcp.CallToolResult {
	return argInvalidValue(tool, "INVALID_DECK_SPEC", "deck_id", message, "string", "<valid-deck-id>", nil)
}

// withPresentationOrDeckIDChoice publishes the input XOR for raw-deck tools.
// The handler enforces it too, but an explicit schema lets MCP clients send a
// handle-only call without treating presentation as required.
func withPresentationOrDeckIDChoice(tool mcp.Tool) mcp.Tool {
	schema := map[string]any{
		"type":       "object",
		"properties": tool.InputSchema.Properties,
		"oneOf": []any{
			map[string]any{"required": []string{"presentation"}},
			map[string]any{"required": []string{"deck_id"}},
		},
	}
	if len(tool.InputSchema.Required) > 0 {
		schema["required"] = tool.InputSchema.Required
	}
	if len(tool.InputSchema.Defs) > 0 {
		schema["$defs"] = tool.InputSchema.Defs
	}
	if tool.InputSchema.AdditionalProperties != nil {
		schema["additionalProperties"] = tool.InputSchema.AdditionalProperties
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("%s input schema: %v", tool.Name, err))
	}
	tool.RawInputSchema = raw
	tool.InputSchema.Type = ""
	return tool
}
