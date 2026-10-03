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
	if bareCode(code) == string(diagnostics.CodeSemanticUnknownField) {
		return unknownFieldOps(data, path, pointer, params)
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

// unknownFieldOps is the patch for a key the kind does not read: a move to the
// key it was meant to be, or its removal. A rewrite of the key's value
// (go-slide-creator-vihnl) left the key where it was, and the same error came
// back.
func unknownFieldOps(data []byte, path, pointer string, params map[string]any) []any {
	if name, _ := params["did_you_mean"].(string); name != "" {
		at := strings.LastIndexByte(pointer, '/')
		target := pointer[:at+1] + escapePointerSegment(name)
		// Never overwrite a key the author also wrote.
		if _, _, taken := semanticNodeAt(data, parentDotted(path, false)+"."+name); !taken {
			return []any{map[string]any{"op": "move", "from": pointer, "path": target}}
		}
	}
	return []any{map[string]any{"op": "remove", "path": pointer}}
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
		if d.Code == diagnostics.CodeSemanticUnknownArchetype {
			// The registered archetypes, as a list an agent can choose from.
			names := make([]string, 0, 8)
			for _, a := range semantic.AllArchetypes() {
				names = append(names, string(a))
			}
			if d.Details == nil {
				d.Details = make(map[string]any)
			}
			d.Details["available"] = names
			continue
		}
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
		// The nearest kind, and for a chart or diagram type the kind that hosts
		// it (go-slide-creator-t0c1m). The next call then asks for that kind
		// alone rather than the whole catalogue.
		m := unknownKindRE.FindStringSubmatch(d.Message)
		if m == nil {
			continue
		}
		s := semantic.SuggestKind(m[1])
		if s.DidYouMean == "" {
			continue
		}
		params := map[string]any{"did_you_mean": string(s.DidYouMean)}
		// With a kind to use, the list of every kind is noise: the next call
		// returns the one that matters.
		delete(d.Details, "available")
		if s.HostedType != "" {
			params["hosted_type"], params["hosted_as"] = s.HostedType, s.HostedAs
		}
		d.Fix = &diagnostics.Fix{Kind: "choose_kind", Params: params}
		d.NextToolCall = &patterns.ToolCallSuggestion{Tool: "list_slide_kinds", ArgsTemplate: map[string]any{"kinds": []any{string(s.DidYouMean)}}}
	}
}

// unknownKindRE reads the rejected kind out of an unknown-kind message.
var unknownKindRE = regexp.MustCompile(`^unknown slide kind "([^"]*)"`)

// semanticizeFindings gives the findings of a DeckSpec envelope their remedy
// without trying any patch; see remedyEnvelope.
func semanticizeFindings(envelope *diagnostics.FindingEnvelope, data []byte, deckID string) {
	newRemedyContext("spec.json", data, deckID).remedyEnvelope(envelope, nil)
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

// trimEnvelopeForMCP drops what an MCP DeckSpec response repeats: the CLI
// describe command (the tool is describe_finding, offered as next_tool_call),
// and that next_tool_call on every finding of a code after the first
// (go-slide-creator-c2j5b).
func trimEnvelopeForMCP(envelope *diagnostics.FindingEnvelope) {
	described := map[string]bool{}
	for i := range envelope.Findings {
		f := &envelope.Findings[i]
		f.DescribeCommand = ""
		if f.NextToolCall == nil || f.NextToolCall.Tool != "describe_finding" {
			continue
		}
		if described[f.Code] {
			f.NextToolCall = nil
			continue
		}
		described[f.Code] = true
	}
}
