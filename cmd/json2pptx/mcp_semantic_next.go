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
	ops := semanticPatchOps(data, path, code, nil)
	if deckID == "" || len(ops) == 0 {
		return nil
	}
	return semanticPatchSuggestion(deckID, ops)
}

func semanticPatchSuggestion(deckID string, ops []any) *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{
		Tool:         "validate_deck_spec",
		ArgsTemplate: map[string]any{"deck_id": deckID, "patch": ops},
	}
}

// semanticNodeAt resolves a semantic source path against the stored spec,
// returning its JSON pointer and current value.
func semanticNodeAt(data []byte, path string) (string, any, bool) {
	pointer, ok := semanticPointer(path)
	if !ok {
		return "", nil, false
	}
	canonical, _ := canonicalSpec("spec.json", data)
	var node any
	if json.Unmarshal(canonical, &node) != nil {
		return "", nil, false
	}
	parts, err := parsePointer(pointer)
	if err != nil {
		return "", nil, false
	}
	for _, part := range parts {
		switch current := node.(type) {
		case map[string]any:
			var exists bool
			node, exists = current[part]
			if !exists {
				return "", nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(current) {
				return "", nil, false
			}
			node = current[index]
		default:
			return "", nil, false
		}
	}
	return pointer, node, true
}

// semanticPatchOps builds the DeckSpec patch that resolves a finding at path:
// a rewrite of a string field (naming the raw budget when one is known), or —
// for a list with a max_items budget — the removals that bring it within it
// (go-slide-creator-pi6ea). params are the finding's raw fix params.
func semanticPatchOps(data []byte, path, code string, params map[string]any) []any {
	pointer, node, ok := semanticNodeAt(data, path)
	if !ok {
		return nil
	}
	switch v := node.(type) {
	case string:
		return []any{map[string]any{"op": "replace", "path": pointer, "value": rewriteHint(code, params)}}
	case []any:
		limit, ok := intFixParam(params, "max_items")
		if !ok || limit < 1 || limit >= len(v) {
			return nil
		}
		// Remove from the end so each pointer is still valid when applied.
		ops := make([]any, 0, len(v)-limit)
		for i := len(v) - 1; i >= limit; i-- {
			ops = append(ops, map[string]any{"op": "remove", "path": pointer + "/" + strconv.Itoa(i)})
		}
		return ops
	}
	return nil
}

func rewriteHint(code string, params map[string]any) string {
	if params["repair"] == "composition" {
		// No single field's cut clears a shared cell (go-slide-creator-ifcng).
		return "<composition-level repair for " + code + ": this field shares its box with the rest of the slide's text; shorten it and drop a bullet or row, or give the slide more room>"
	}
	hint := "<rewrite this field to resolve " + code + " while preserving meaning"
	if n, ok := intFixParam(params, "max_chars"); ok && n > 0 {
		hint += "; at most " + strconv.Itoa(n) + " characters"
	} else if n, ok := intFixParam(params, "max_length"); ok && n > 0 {
		hint += "; at most " + strconv.Itoa(n) + " characters"
	} else if n, ok := intFixParam(params, "max_words"); ok && n > 0 {
		hint += "; at most " + strconv.Itoa(n) + " words"
	}
	return hint + ">"
}

func intFixParam(params map[string]any, key string) (int, bool) {
	switch v := params[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	}
	return 0, false
}

// semanticFixParams keeps a raw fix's budgets and measurements (max_chars,
// max_items, threshold_pct, …) and drops its raw-model locators, which address
// the compiled PresentationInput rather than the DeckSpec the author edits.
// fix_kind records the raw kind the params belong to.
func semanticFixParams(kind string, raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		switch k {
		case "path", "target_path", "raw_path", "slide_index", "slide", "pointer", "kind":
			continue
		}
		out[k] = v
	}
	// A shared grid cell's max_chars budgets the whole cell; the DeckSpec
	// patch edits one field, so it carries that field's own budget, or none
	// when the repair is composition-level (go-slide-creator-ifcng).
	if n, ok := out["field_max_chars"]; ok || out["repair"] == "composition" {
		if cell, had := out["max_chars"]; had {
			out["cell_max_chars"] = cell
		}
		delete(out, "max_chars")
		delete(out, "field_max_chars")
		if ok {
			out["max_chars"] = n
		}
	}
	delete(out, "edit_text")
	if len(out) == 0 {
		return nil
	}
	if kind != "" {
		out["fix_kind"] = kind
	}
	return out
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
		if strings.HasSuffix(f.Code, diagnostics.CodeSemanticUnknownKind) {
			f.NextToolCall = &patterns.ToolCallSuggestion{Tool: "list_slide_kinds", ArgsTemplate: map[string]any{}}
			continue
		}
		semanticizeFinding(f, data, deckID)
	}
}

// semanticizeFinding rewrites one finding's remediation for the DeckSpec.
// Raw PresentationInput locators are not DeckSpec edits, but the fix's budgets
// are: keep them rather than dropping max_chars / max_items / threshold_pct
// with the locators (go-slide-creator-pi6ea).
func semanticizeFinding(f *diagnostics.Finding, data []byte, deckID string) {
	templateNotFound := strings.HasSuffix(f.Code, diagnostics.CodeTemplateNotFound)
	templateRemediation := f.Remediation
	rawAction, fixParams := findingFixParams(f.Remediation)
	f.NextToolCall = nil
	f.Remediation = nil
	fallback, _ := f.Evidence[compositionPatchDetail].([]any)
	delete(f.Evidence, compositionPatchDetail)
	editPath, _ := f.Evidence[editPathDetail].(string)
	delete(f.Evidence, editPathDetail)
	if path, ok := f.Evidence["path"].(string); ok && deckID != "" {
		if editPath != "" {
			path = editPath
		}
		var ops []any
		if fixParams["repair"] != "composition" || len(fallback) == 0 {
			ops = semanticPatchOps(data, path, f.Code, fixParams)
		}
		usedFallback := false
		if len(ops) == 0 && len(fallback) > 0 {
			// A refused list has no single field to rewrite; switching the
			// slide to its native-layout composition keeps every item
			// (go-slide-creator-b7qqg.4).
			ops, usedFallback = fallback, true
		}
		if len(ops) > 0 {
			f.NextToolCall = semanticPatchSuggestion(deckID, ops)
			params := patchRemediationParams(path, ops, fixParams)
			if first, ok := ops[0].(map[string]any); ok && usedFallback {
				params["path"] = first["path"]
			}
			if match := templateDidYouMean(templateNotFound, templateRemediation); match != nil {
				params["did_you_mean"] = match
			}
			f.Remediation = &diagnostics.Remediation{Primary: &diagnostics.RemediationAction{
				Action: diagnostics.ActionApplyPatch,
				Params: params,
			}}
			return
		}
	}
	switch {
	case templateNotFound:
		f.NextToolCall = nextCallListTemplates()
		f.Remediation = templateRemediation
	default:
		f.NextToolCall = &patterns.ToolCallSuggestion{Tool: "describe_finding", ArgsTemplate: map[string]any{"code": f.Code}}
		if fixParams != nil {
			f.Remediation = &diagnostics.Remediation{Primary: &diagnostics.RemediationAction{Action: rawAction, Params: fixParams}}
		}
	}
}

// findingFixParams returns a finding's raw action and its DeckSpec-safe params.
func findingFixParams(r *diagnostics.Remediation) (diagnostics.Action, map[string]any) {
	if r == nil || r.Primary == nil {
		return "", nil
	}
	kind, _ := r.Primary.Params["kind"].(string)
	if kind == "" {
		kind = string(r.Primary.Action)
	}
	return r.Primary.Action, semanticFixParams(kind, r.Primary.Params)
}

// patchRemediationParams describes a semantic patch as remediation params:
// the target pointer, the first op (and every op when there are several), and
// the raw budgets it satisfies.
func patchRemediationParams(path string, ops []any, fixParams map[string]any) map[string]any {
	pointer, _ := semanticPointer(path)
	params := map[string]any{"path": pointer}
	for k, v := range fixParams {
		params[k] = v
	}
	if first, ok := ops[0].(map[string]any); ok {
		params["op"] = first["op"]
		if value, ok := first["value"]; ok {
			params["value"] = value
		}
	}
	if len(ops) > 1 {
		params["ops"] = ops
	}
	return params
}

func templateDidYouMean(templateNotFound bool, r *diagnostics.Remediation) any {
	if !templateNotFound || r == nil || r.Primary == nil {
		return nil
	}
	return r.Primary.Params["did_you_mean"]
}

func semanticizeRenderDiagnostics(ds []semanticDiagnostic, data []byte, deckID string) {
	for i := range ds {
		ds[i].NextToolCall = nil
		if deckID != "" {
			var params map[string]any
			if ds[i].RecommendedEdit != nil {
				params = ds[i].RecommendedEdit.Params
			}
			if ops := semanticPatchOps(data, ds[i].SemanticPath, ds[i].Code, params); len(ops) > 0 {
				ds[i].NextToolCall = semanticPatchSuggestion(deckID, ops)
			} else if len(ds[i].fallbackPatch) > 0 {
				ds[i].NextToolCall = semanticPatchSuggestion(deckID, ds[i].fallbackPatch)
			}
		}
		if ds[i].NextToolCall == nil {
			ds[i].NextToolCall = &patterns.ToolCallSuggestion{Tool: "describe_finding", ArgsTemplate: map[string]any{"code": ds[i].Code}}
		}
	}
}
