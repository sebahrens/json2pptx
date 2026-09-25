package main

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

var semanticPathPart = regexp.MustCompile(`^([A-Za-z_][A-Za-z_0-9]*|\[[0-9]+\])`)

// semanticPointer only accepts unambiguous field/index paths into the actual
// stored spec. A source-map ancestor such as slides[2].kpis can point at an
// entire array, so callers must also check the target type before suggesting
// a replace operation.
func semanticPointer(path string) (string, bool) {
	var segments []string
	for path != "" {
		path = strings.TrimPrefix(path, ".")
		part := semanticPathPart.FindString(path)
		if part == "" {
			return "", false
		}
		path = path[len(part):]
		if strings.HasPrefix(part, "[") {
			part = part[1 : len(part)-1]
		}
		segments = append(segments, strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1"))
		if path != "" && !strings.HasPrefix(path, ".") && !strings.HasPrefix(path, "[") {
			return "", false
		}
	}
	if len(segments) == 0 {
		return "", false
	}
	return "/" + strings.Join(segments, "/"), true
}

func semanticPatchCall(data []byte, deckID, path, code string) *patterns.ToolCallSuggestion {
	if deckID == "" {
		return nil
	}
	pointer, ok := semanticPointer(path)
	if !ok {
		return nil
	}
	canonical, _ := canonicalSpec("spec.json", data)
	var node any
	if json.Unmarshal(canonical, &node) != nil {
		return nil
	}
	parts, err := parsePointer(pointer)
	if err != nil {
		return nil
	}
	for _, part := range parts {
		switch current := node.(type) {
		case map[string]any:
			var exists bool
			node, exists = current[part]
			if !exists {
				return nil
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(current) {
				return nil
			}
			node = current[index]
		default:
			return nil
		}
	}
	if _, ok := node.(string); !ok {
		return nil
	}
	return &patterns.ToolCallSuggestion{
		Tool: "validate_deck_spec",
		ArgsTemplate: map[string]any{
			"deck_id": deckID,
			"patch": []any{map[string]any{
				"op": "replace", "path": pointer,
				"value": "<rewrite this field to resolve " + code + " while preserving meaning>",
			}},
		},
	}
}

// Make a parser's unknown-kind error actionable without asking the agent to
// scrape the prose list of kinds from its message. The same enrichment is used
// by validation and render, whose parse failures have different outer shapes.
func enrichSemanticKindDiagnostics(ds []diagnostics.Diagnostic) {
	var available []string
	for i := range ds {
		d := &ds[i]
		if d.Code != diagnostics.CodeSemanticUnknownKind {
			continue
		}
		if available == nil {
			for _, kind := range semantic.AllSlideKinds() {
				available = append(available, string(kind))
			}
		}
		if d.Details == nil {
			d.Details = make(map[string]any)
		}
		d.Details["available"] = available
		d.Fix = &diagnostics.Fix{Kind: "choose_kind", Params: map[string]any{"available": available}}
		d.NextToolCall = &patterns.ToolCallSuggestion{Tool: "list_slide_kinds", ArgsTemplate: map[string]any{}}
	}
}

func semanticizeFindings(envelope *diagnostics.FindingEnvelope, data []byte, deckID string) {
	for i := range envelope.Findings {
		f := &envelope.Findings[i]
		templateNotFound := strings.HasSuffix(f.Code, diagnostics.CodeTemplateNotFound)
		templateRemediation := f.Remediation
		if strings.HasSuffix(f.Code, diagnostics.CodeSemanticUnknownKind) {
			f.NextToolCall = &patterns.ToolCallSuggestion{Tool: "list_slide_kinds", ArgsTemplate: map[string]any{}}
			continue
		}
		f.NextToolCall = nil
		f.Remediation = nil // raw PresentationInput fix parameters are not DeckSpec edits
		if path, ok := f.Evidence["path"].(string); ok {
			f.NextToolCall = semanticPatchCall(data, deckID, path, f.Code)
			if f.NextToolCall != nil {
				pointer, _ := semanticPointer(path)
				params := map[string]any{
					"path": pointer, "op": "replace",
					"value": "<rewrite this field to resolve " + f.Code + " while preserving meaning>",
				}
				if templateNotFound && templateRemediation != nil && templateRemediation.Primary != nil {
					if match, ok := templateRemediation.Primary.Params["did_you_mean"]; ok {
						params["did_you_mean"] = match
					}
				}
				f.Remediation = &diagnostics.Remediation{Primary: &diagnostics.RemediationAction{
					Action: diagnostics.ActionApplyPatch,
					Params: params,
				}}
			}
		}
		if f.NextToolCall == nil {
			if templateNotFound {
				f.NextToolCall = nextCallListTemplates()
				f.Remediation = templateRemediation
			} else {
				f.NextToolCall = &patterns.ToolCallSuggestion{Tool: "describe_finding", ArgsTemplate: map[string]any{"code": f.Code}}
			}
		}
	}
}

func semanticizeRenderDiagnostics(ds []semanticDiagnostic, data []byte, deckID string) {
	for i := range ds {
		ds[i].NextToolCall = semanticPatchCall(data, deckID, ds[i].SemanticPath, ds[i].Code)
		if ds[i].NextToolCall == nil {
			ds[i].NextToolCall = &patterns.ToolCallSuggestion{Tool: "describe_finding", ArgsTemplate: map[string]any{"code": ds[i].Code}}
		}
	}
}
