// mcp_apply_patch.go implements the apply_deck_patch MCP tool — a pure deck
// JSON transform primitive. Agents send the full deck plus a bounded list of
// structural operations (insert / remove / replace / move / duplicate a slide,
// or replace a field at a JSON Pointer) and receive the patched deck plus
// validation / preflight findings.
//
// It is a PRIMITIVE, not a workflow facade: it never writes files, renders, or
// mutates server state. The patch is atomic — if any operation is invalid, the
// whole patch is rejected with structured diagnostics and no patched_deck is
// returned. The deck round-trips through a generic tree (map[string]any with
// json.Number) so fields this tool does not model survive the patch unchanged.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/template"
)

// --- Request / response types ---

// deckPatchOp is one bounded patch operation. The op discriminator selects
// which fields are read; unused fields are ignored.
type deckPatchOp struct {
	Op    string          `json:"op"`
	Index *int            `json:"index,omitempty"`
	From  *int            `json:"from,omitempty"`
	To    *int            `json:"to,omitempty"`
	Path  string          `json:"path,omitempty"`
	Slide json.RawMessage `json:"slide,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// appliedPatchOp reports the outcome of a single operation. Because the patch
// is atomic, on a success response every entry has Applied=true.
type appliedPatchOp struct {
	Op      string `json:"op"`
	Applied bool   `json:"applied"`
	Message string `json:"message,omitempty"`
}

// applyDeckPatchOutput is the success response for apply_deck_patch.
type applyDeckPatchOutput struct {
	PatchedDeck json.RawMessage             `json:"patched_deck"`
	AppliedOps  []appliedPatchOp            `json:"applied_ops"`
	Findings    diagnostics.FindingEnvelope `json:"findings"`
}

// deckPatchOpKinds returns the bounded vocabulary of supported operations,
// in catalog order. Surfaced in the unknown-op error so agents can recover
// without a get_capabilities round-trip.
func deckPatchOpKinds() []string {
	return []string{
		"insert_slide",
		"remove_slide",
		"replace_slide",
		"move_slide",
		"duplicate_slide",
		"replace_field",
	}
}

// --- Tool definition ---

func mcpApplyDeckPatchTool() mcp.Tool {
	return mcp.NewTool("apply_deck_patch",
		mcp.WithDescription(`Pure deck-JSON transform primitive: apply a bounded list of structural operations to a deck and return the patched JSON plus validation/preflight findings. It never writes files, renders, or mutates server state — feed patched_deck straight into validate_input / generate_presentation / repair_slide.

This is a PRIMITIVE, not a workflow facade: it only edits the deck structure you pass in. It does not score, repair, or render. Pair it with validate_input / score_deck to inspect the result. For persistent multi-deck CRUD use a workflow tool — this primitive returns the deck in-band and writes nothing.

The patch is ATOMIC. If any operation is invalid (index out of range, unknown op, missing field, JSON Pointer path that does not exist, or a change that would produce a deck that no longer parses), the whole patch is rejected with a structured error envelope and patched_deck is NOT returned.

Operations (ops[] array, applied in order). Each op has an "op" discriminator plus op-specific fields:
- insert_slide: insert a new slide. Fields: index (int, optional — defaults to append at end; valid 0..N where N=current slide count), slide (object, required — a SlideInput).
- remove_slide: remove a slide. Fields: index (int, required, 0..N-1).
- replace_slide: replace a whole slide. Fields: index (int, required, 0..N-1), slide (object, required).
- move_slide: move a slide to a new position. Fields: from (int, required, 0..N-1), to (int, required, 0..N-1).
- duplicate_slide: deep-copy a slide and insert the copy. Fields: index (int, required, 0..N-1), to (int, optional — defaults to index+1; valid 0..N).
- replace_field: replace the value at an EXISTING JSON Pointer (RFC 6901) path. Fields: path (string, required, e.g. "/slides/0/content/0/text_value" or "/template"), value (any, required). Replace semantics only — the path must already exist; it never creates new keys.

Returns {patched_deck, applied_ops[], findings} where findings is a FindingEnvelope of validation + fit/preflight findings for the patched deck (branch on findings.ok; findings.findings[] is empty when the patch left no issues).

Example ops: [{"op":"move_slide","from":3,"to":1}, {"op":"duplicate_slide","index":1}, {"op":"replace_field","path":"/slides/0/content/0/text_value","value":"New title"}]`),
		mcp.WithRawOutputSchema(outputSchemaApplyDeckPatch),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description(`Full presentation definition to patch. Same schema as generate_presentation.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithArray("ops",
			mcp.Required(),
			mcp.Description(`Ordered list of patch operations. Each is an object with an "op" discriminator (insert_slide, remove_slide, replace_slide, move_slide, duplicate_slide, replace_field) and op-specific fields (index/from/to/slide/value/path). Applied in order; the whole patch is atomic.`),
		),
	)
}

// --- Handler ---

func (mc *mcpConfig) handleApplyDeckPatch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("apply_deck_patch", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	// Parse the deck into a generic tree so every field — including ones this
	// tool does not model — round-trips faithfully. UseNumber keeps EMU and
	// alpha integers exact instead of reformatting them through float64.
	deck, err := decodeDeckTree([]byte(jsonStr))
	if err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nil), nil
	}

	// Extract the ops array.
	ops, opsErr := extractDeckPatchOps(request)
	if opsErr != nil {
		return argInvalidValue("apply_deck_patch", "INVALID_PARAMETER", "ops", opsErr.Error(), "array",
			[]any{map[string]any{"op": "move_slide", "from": 1, "to": 0}}, nil), nil
	}
	if len(ops) == 0 {
		return argMissing("apply_deck_patch", "ops", "array",
			[]any{map[string]any{"op": "remove_slide", "index": 0}}, nil), nil
	}

	// Apply each op in order against the working tree. Atomic: the first
	// failure aborts the whole patch with a structured diagnostic and no
	// patched_deck is returned.
	applied := make([]appliedPatchOp, 0, len(ops))
	for i, op := range ops {
		res, d := applyDeckPatchOp(deck, op, i)
		if d != nil {
			return api.MCPDiagnosticsError([]diagnostics.Diagnostic{*d}), nil
		}
		applied = append(applied, res)
	}

	// Re-marshal the patched deck.
	patchedJSON, err := json.Marshal(deck)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal patched deck: %v", err)), nil
	}

	// Schema check: the patched deck must still parse as a PresentationInput.
	// A patch that breaks the type contract (e.g. sets slides to a string, or
	// a text_value to a number) is rejected here so agents never receive a
	// deck that downstream tools cannot load.
	var pi PresentationInput
	if err := strictUnmarshalJSON(patchedJSON, &pi); err != nil {
		return api.MCPDiagnosticsError([]diagnostics.Diagnostic{{
			Code:         diagnostics.CodeInvalidSlide,
			Path:         "patched_deck",
			Message:      fmt.Sprintf("patch produced a deck that no longer parses as a presentation: %v", err),
			Severity:     diagnostics.SeverityError,
			NextToolCall: nextCallGetInputSchema(),
		}}), nil
	}

	// Collect validation + fit/preflight findings for the patched deck.
	diags := mc.deckPatchFindings(&pi)

	output := applyDeckPatchOutput{
		PatchedDeck: patchedJSON,
		AppliedOps:  applied,
		Findings: diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand:  "apply_deck_patch",
			Template:    pi.Template,
			InputSHA256: diagnostics.ComputeInputSHA256(patchedJSON),
		}, diags),
	}

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// deckPatchFindings runs the same boundary + structure-expansion checks as
// validate_input and best-effort fit/preflight findings against the patched
// deck. Template resolution failures (e.g. minimal environments) degrade
// gracefully to boundary-only findings — the patch result is still valid.
func (mc *mcpConfig) deckPatchFindings(pi *PresentationInput) []diagnostics.Diagnostic {
	applyDefaults(pi)
	mc.resolveInputNamedSettings(pi)

	// Expand a structure block into flat slides (mirrors validate_input) so
	// the slide-count boundary check is correct for structure-authored decks.
	if structDiags := applyStructureExpansion(pi); len(structDiags) > 0 {
		return structDiags
	}

	var diags []diagnostics.Diagnostic
	if pi.Template == "" {
		diags = append(diags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "template", Message: "template is required",
			Severity:     diagnostics.SeverityError,
			Fix:          &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "template"}},
			NextToolCall: nextCallListTemplates(),
		})
	}
	if len(pi.Slides) == 0 {
		diags = append(diags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "slides", Message: "at least one slide is required after patching",
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "slides"}},
		})
	}
	if diagnostics.HasErrors(diags) {
		return diags
	}

	// Best-effort fit/preflight findings against the target template.
	templatePath, cleanup, err := resolveTemplatePath(pi.Template, mc.templatesDir)
	if err != nil {
		return diags
	}
	defer cleanup()
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return diags
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return diags
	}
	slideWidth, slideHeight := template.ParseSlideDimensions(reader)
	theme := template.ParseTheme(reader)
	resolveCanonicalLayoutIDs(pi.Slides, layouts)
	fitFindings := collectFitFindings(pi, layouts, slideWidth, slideHeight, &theme)
	fitFindings = BudgetFitFindings(fitFindings, DefaultFindingBudget, false)
	diags = append(diags, diagnostics.FromFitFindings(fitFindings)...)
	return diags
}

// --- Operation dispatch ---

// applyDeckPatchOp applies a single operation to the deck tree in place. It
// returns a non-nil diagnostic (with the offending ops[opIdx] path) when the
// operation is invalid; the caller aborts the whole patch on first failure.
func applyDeckPatchOp(deck map[string]any, op deckPatchOp, opIdx int) (appliedPatchOp, *diagnostics.Diagnostic) {
	switch op.Op {
	case "insert_slide":
		return applyInsertSlide(deck, op, opIdx)
	case "remove_slide":
		return applyRemoveSlide(deck, op, opIdx)
	case "replace_slide":
		return applyReplaceSlide(deck, op, opIdx)
	case "move_slide":
		return applyMoveSlide(deck, op, opIdx)
	case "duplicate_slide":
		return applyDuplicateSlide(deck, op, opIdx)
	case "replace_field":
		return applyReplaceField(deck, op, opIdx)
	default:
		return appliedPatchOp{Op: op.Op, Applied: false}, &diagnostics.Diagnostic{
			Code:     diagnostics.CodeUnsupported,
			Path:     fmt.Sprintf("ops[%d].op", opIdx),
			Message:  fmt.Sprintf("unknown op %q", op.Op),
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "use_one_of", Params: map[string]any{"allowed": deckPatchOpKinds()}},
		}
	}
}

func applyInsertSlide(deck map[string]any, op deckPatchOp, opIdx int) (appliedPatchOp, *diagnostics.Diagnostic) {
	slides, ok := deckSlidesForWrite(deck)
	if !ok {
		return failedOp(op), diagAt(opIdx, "", diagnostics.CodeInvalidSlide, `deck "slides" is present but is not an array`)
	}
	slideVal, d := slideObjectParam(op, opIdx)
	if d != nil {
		return failedOp(op), d
	}
	idx := len(slides)
	if op.Index != nil {
		idx = *op.Index
	}
	if idx < 0 || idx > len(slides) {
		return failedOp(op), diagAt(opIdx, "index", diagnostics.CodeInvalidSlideIndex,
			fmt.Sprintf("insert index %d out of range (valid 0..%d)", idx, len(slides)))
	}
	deck["slides"] = insertAt(slides, idx, slideVal)
	return appliedPatchOp{Op: op.Op, Applied: true, Message: fmt.Sprintf("inserted slide at index %d", idx)}, nil
}

func applyRemoveSlide(deck map[string]any, op deckPatchOp, opIdx int) (appliedPatchOp, *diagnostics.Diagnostic) {
	slides, ok := deckSlidesForWrite(deck)
	if !ok {
		return failedOp(op), diagAt(opIdx, "", diagnostics.CodeInvalidSlide, `deck "slides" is present but is not an array`)
	}
	if op.Index == nil {
		return failedOp(op), diagAt(opIdx, "index", diagnostics.CodeInvalidParameter, "index is required for remove_slide")
	}
	idx := *op.Index
	if idx < 0 || idx >= len(slides) {
		return failedOp(op), diagAt(opIdx, "index", diagnostics.CodeInvalidSlideIndex,
			fmt.Sprintf("remove index %d out of range (deck has %d slides)", idx, len(slides)))
	}
	out := make([]any, 0, len(slides)-1)
	out = append(out, slides[:idx]...)
	out = append(out, slides[idx+1:]...)
	deck["slides"] = out
	return appliedPatchOp{Op: op.Op, Applied: true, Message: fmt.Sprintf("removed slide at index %d", idx)}, nil
}

func applyReplaceSlide(deck map[string]any, op deckPatchOp, opIdx int) (appliedPatchOp, *diagnostics.Diagnostic) {
	slides, ok := deckSlidesForWrite(deck)
	if !ok {
		return failedOp(op), diagAt(opIdx, "", diagnostics.CodeInvalidSlide, `deck "slides" is present but is not an array`)
	}
	if op.Index == nil {
		return failedOp(op), diagAt(opIdx, "index", diagnostics.CodeInvalidParameter, "index is required for replace_slide")
	}
	idx := *op.Index
	if idx < 0 || idx >= len(slides) {
		return failedOp(op), diagAt(opIdx, "index", diagnostics.CodeInvalidSlideIndex,
			fmt.Sprintf("replace index %d out of range (deck has %d slides)", idx, len(slides)))
	}
	slideVal, d := slideObjectParam(op, opIdx)
	if d != nil {
		return failedOp(op), d
	}
	slides[idx] = slideVal
	deck["slides"] = slides
	return appliedPatchOp{Op: op.Op, Applied: true, Message: fmt.Sprintf("replaced slide at index %d", idx)}, nil
}

func applyMoveSlide(deck map[string]any, op deckPatchOp, opIdx int) (appliedPatchOp, *diagnostics.Diagnostic) {
	slides, ok := deckSlidesForWrite(deck)
	if !ok {
		return failedOp(op), diagAt(opIdx, "", diagnostics.CodeInvalidSlide, `deck "slides" is present but is not an array`)
	}
	if op.From == nil {
		return failedOp(op), diagAt(opIdx, "from", diagnostics.CodeInvalidParameter, "from is required for move_slide")
	}
	if op.To == nil {
		return failedOp(op), diagAt(opIdx, "to", diagnostics.CodeInvalidParameter, "to is required for move_slide")
	}
	from, to := *op.From, *op.To
	if from < 0 || from >= len(slides) {
		return failedOp(op), diagAt(opIdx, "from", diagnostics.CodeInvalidSlideIndex,
			fmt.Sprintf("from index %d out of range (deck has %d slides)", from, len(slides)))
	}
	if to < 0 || to >= len(slides) {
		return failedOp(op), diagAt(opIdx, "to", diagnostics.CodeInvalidSlideIndex,
			fmt.Sprintf("to index %d out of range (valid 0..%d)", to, len(slides)-1))
	}
	moved := slides[from]
	rest := make([]any, 0, len(slides)-1)
	rest = append(rest, slides[:from]...)
	rest = append(rest, slides[from+1:]...)
	deck["slides"] = insertAt(rest, to, moved)
	return appliedPatchOp{Op: op.Op, Applied: true, Message: fmt.Sprintf("moved slide from %d to %d", from, to)}, nil
}

func applyDuplicateSlide(deck map[string]any, op deckPatchOp, opIdx int) (appliedPatchOp, *diagnostics.Diagnostic) {
	slides, ok := deckSlidesForWrite(deck)
	if !ok {
		return failedOp(op), diagAt(opIdx, "", diagnostics.CodeInvalidSlide, `deck "slides" is present but is not an array`)
	}
	if op.Index == nil {
		return failedOp(op), diagAt(opIdx, "index", diagnostics.CodeInvalidParameter, "index is required for duplicate_slide")
	}
	idx := *op.Index
	if idx < 0 || idx >= len(slides) {
		return failedOp(op), diagAt(opIdx, "index", diagnostics.CodeInvalidSlideIndex,
			fmt.Sprintf("duplicate index %d out of range (deck has %d slides)", idx, len(slides)))
	}
	to := idx + 1
	if op.To != nil {
		to = *op.To
	}
	if to < 0 || to > len(slides) {
		return failedOp(op), diagAt(opIdx, "to", diagnostics.CodeInvalidSlideIndex,
			fmt.Sprintf("to index %d out of range (valid 0..%d)", to, len(slides)))
	}
	clone, err := deepCopyValue(slides[idx])
	if err != nil {
		return failedOp(op), diagAt(opIdx, "index", diagnostics.CodeInvalidSlide,
			fmt.Sprintf("failed to copy slide %d: %v", idx, err))
	}
	deck["slides"] = insertAt(slides, to, clone)
	return appliedPatchOp{Op: op.Op, Applied: true, Message: fmt.Sprintf("duplicated slide %d to index %d", idx, to)}, nil
}

func applyReplaceField(deck map[string]any, op deckPatchOp, opIdx int) (appliedPatchOp, *diagnostics.Diagnostic) {
	if op.Path == "" {
		return failedOp(op), diagAt(opIdx, "path", diagnostics.CodeInvalidParameter, "path (JSON Pointer) is required for replace_field")
	}
	if len(op.Value) == 0 {
		return failedOp(op), diagAt(opIdx, "value", diagnostics.CodeInvalidParameter, "value is required for replace_field")
	}
	val, err := decodeJSONValue(op.Value)
	if err != nil {
		return failedOp(op), diagAt(opIdx, "value", diagnostics.CodeInvalidParameter, fmt.Sprintf("value is not valid JSON: %v", err))
	}
	if err := setJSONPointer(deck, op.Path, val); err != nil {
		return failedOp(op), diagAt(opIdx, "path", diagnostics.CodeInvalidPath, err.Error())
	}
	return appliedPatchOp{Op: op.Op, Applied: true, Message: fmt.Sprintf("replaced value at %s", op.Path)}, nil
}

// --- Helpers ---

// failedOp builds the appliedPatchOp half of a failed-operation return.
func failedOp(op deckPatchOp) appliedPatchOp {
	return appliedPatchOp{Op: op.Op, Applied: false}
}

// diagAt builds an error diagnostic anchored at ops[opIdx].field.
func diagAt(opIdx int, field, code, msg string) *diagnostics.Diagnostic {
	path := fmt.Sprintf("ops[%d]", opIdx)
	if field != "" {
		path += "." + field
	}
	return &diagnostics.Diagnostic{Code: code, Path: path, Message: msg, Severity: diagnostics.SeverityError}
}

// deckSlidesForWrite returns the deck's slides array, treating an absent or
// null "slides" key as an empty array (so insert_slide can build one). It
// returns ok=false only when "slides" is present but is not an array, which
// this tool refuses to silently clobber.
func deckSlidesForWrite(deck map[string]any) (slides []any, ok bool) {
	raw, present := deck["slides"]
	if !present || raw == nil {
		return []any{}, true
	}
	s, isArr := raw.([]any)
	if !isArr {
		return nil, false
	}
	return s, true
}

// slideObjectParam decodes and validates the op's slide field as a JSON object.
func slideObjectParam(op deckPatchOp, opIdx int) (map[string]any, *diagnostics.Diagnostic) {
	if len(op.Slide) == 0 {
		return nil, diagAt(opIdx, "slide", diagnostics.CodeInvalidSlide, "slide (object) is required for this op")
	}
	v, err := decodeJSONValue(op.Slide)
	if err != nil {
		return nil, diagAt(opIdx, "slide", diagnostics.CodeInvalidSlide, fmt.Sprintf("slide is not valid JSON: %v", err))
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, diagAt(opIdx, "slide", diagnostics.CodeInvalidSlide, "slide must be a JSON object")
	}
	return m, nil
}

// insertAt returns a new slice with val inserted at idx. Callers guarantee
// 0 <= idx <= len(slides).
func insertAt(slides []any, idx int, val any) []any {
	out := make([]any, 0, len(slides)+1)
	out = append(out, slides[:idx]...)
	out = append(out, val)
	out = append(out, slides[idx:]...)
	return out
}

// decodeDeckTree parses deck JSON into a generic tree, preserving numbers
// exactly (UseNumber) so EMU and alpha values are not reformatted on the way
// back out.
func decodeDeckTree(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("deck must be a JSON object")
	}
	return m, nil
}

// decodeJSONValue parses a json.RawMessage into a generic value, preserving
// numbers (UseNumber) so values inserted into the deck tree marshal back
// identically.
func decodeJSONValue(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// deepCopyValue returns an independent copy of a generic JSON value via a
// marshal/decode round-trip (UseNumber preserves number fidelity).
func deepCopyValue(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return decodeJSONValue(data)
}

// extractDeckPatchOps reads the ops array from the request and decodes it into
// []deckPatchOp, preserving slide/value payloads as raw JSON.
func extractDeckPatchOps(request mcp.CallToolRequest) ([]deckPatchOp, error) {
	args := request.GetArguments()
	raw, ok := args["ops"]
	if !ok || raw == nil {
		return nil, fmt.Errorf("ops is required")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("ops: %w", err)
	}
	var ops []deckPatchOp
	if err := json.Unmarshal(data, &ops); err != nil {
		return nil, fmt.Errorf("ops must be an array of operation objects: %w", err)
	}
	return ops, nil
}

// setJSONPointer replaces the value at an existing RFC 6901 JSON Pointer within
// the deck tree. The target must already exist (replace semantics) — a missing
// key or out-of-range index is an error, never a create.
func setJSONPointer(root map[string]any, pointer string, val any) error {
	if pointer == "/" {
		return fmt.Errorf("path %q does not address a field", pointer)
	}
	if !strings.HasPrefix(pointer, "/") {
		return fmt.Errorf("path %q must be a JSON Pointer beginning with '/'", pointer)
	}
	tokens := strings.Split(pointer[1:], "/")
	for i := range tokens {
		tokens[i] = unescapeJSONPointerToken(tokens[i])
	}

	// Navigate to the parent container of the final token.
	var current any = root
	for _, tok := range tokens[:len(tokens)-1] {
		next, err := pointerDescend(current, tok)
		if err != nil {
			return fmt.Errorf("path %q: %w", pointer, err)
		}
		current = next
	}

	last := tokens[len(tokens)-1]
	switch parent := current.(type) {
	case map[string]any:
		if _, ok := parent[last]; !ok {
			return fmt.Errorf("path %q: key %q does not exist (replace_field cannot create new keys)", pointer, last)
		}
		parent[last] = val
		return nil
	case []any:
		idx, err := strconv.Atoi(last)
		if err != nil {
			return fmt.Errorf("path %q: %q is not a valid array index", pointer, last)
		}
		if idx < 0 || idx >= len(parent) {
			return fmt.Errorf("path %q: array index %d out of range (len %d)", pointer, idx, len(parent))
		}
		parent[idx] = val
		return nil
	default:
		return fmt.Errorf("path %q: cannot set a field on a %T value", pointer, current)
	}
}

// pointerDescend resolves a single JSON Pointer token against the current node.
func pointerDescend(current any, token string) (any, error) {
	switch node := current.(type) {
	case map[string]any:
		v, ok := node[token]
		if !ok {
			return nil, fmt.Errorf("key %q does not exist", token)
		}
		return v, nil
	case []any:
		idx, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("%q is not a valid array index", token)
		}
		if idx < 0 || idx >= len(node) {
			return nil, fmt.Errorf("array index %d out of range (len %d)", idx, len(node))
		}
		return node[idx], nil
	default:
		return nil, fmt.Errorf("cannot descend into a %T value at token %q", current, token)
	}
}

// unescapeJSONPointerToken applies RFC 6901 token unescaping (~1 → /, ~0 → ~).
func unescapeJSONPointerToken(tok string) string {
	tok = strings.ReplaceAll(tok, "~1", "/")
	tok = strings.ReplaceAll(tok, "~0", "~")
	return tok
}
