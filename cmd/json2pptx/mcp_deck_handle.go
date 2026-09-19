package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/safeyaml"
)

// Server-side deck handles (go-slide-creator-voxp).
//
// Measured on a 15-slide deck: the DeckSpec loop (validate → render → thumbs →
// fix → validate → render → thumbs) resent the whole 3,695-byte spec on 5 of 8
// calls. A one-line title fix cost a full spec re-upload. Every tool was
// stateless, so the agent was the only place the deck lived, and it paid for
// that on every call.
//
// A deck_id is the spec, held server-side under a TTL'd handle. Pass it instead
// of the spec on the next call; pass a patch alongside it to change one field
// without re-uploading the rest. The stateless form is untouched — deck_id is
// optional everywhere — so nothing that works today stops working.

// deckHandleTTL bounds how long a handle stays loadable. It matches the loop
// session TTL: both are interactive-session scratch state, not persistence.
const deckHandleTTL = time.Hour

// deckHandle is the server's copy of a spec, plus what the last render of it
// produced, so a follow-up call can answer "what changed" without the agent
// re-sending anything.
type deckHandle struct {
	// Spec is the DeckSpec as bytes, in whatever form it was authored (JSON or
	// YAML). Patches are applied to its decoded form and it is re-encoded as
	// JSON, so a YAML spec becomes JSON on the first patch — the content is
	// what matters, not the syntax.
	Spec []byte
	// Filename is the name the spec was parsed under, for diagnostics paths.
	Filename string
	// SlideDigests is the per-slide content digest of the last stored spec, so
	// changed_slides reflects which slides a patch actually changed.
	SlideDigests []string
	// Template is the template the last render used, echoed so a re-render
	// without an explicit template keeps the deck looking the same.
	Template string
}

// deckHandleStore is a per-process, TTL'd map of handles. Same scope as the
// idempotency cache and the loop sessions: it drops on restart, and is a
// convenience for one interactive session rather than a deck database.
type deckHandleStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]deckHandleEntry
	now     func() time.Time
}

type deckHandleEntry struct {
	handle    *deckHandle
	expiresAt time.Time
}

func newDeckHandleStore(ttl time.Duration) *deckHandleStore {
	return &deckHandleStore{
		ttl:     ttl,
		entries: make(map[string]deckHandleEntry),
		now:     time.Now,
	}
}

// Save stores a handle under a fresh id. An empty id means the caller simply
// does not offer one; it is never an error.
func (s *deckHandleStore) Save(h *deckHandle) string {
	if s == nil || h == nil {
		return ""
	}
	id := newDeckID()
	if id == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[id] = deckHandleEntry{handle: h, expiresAt: s.now().Add(s.ttl)}
	return id
}

// Update replaces the handle behind an existing id, keeping the id stable
// across a patch so an agent holds one identifier for the whole session. It
// refreshes the TTL: an actively edited deck should not expire mid-session.
func (s *deckHandleStore) Update(id string, h *deckHandle) {
	if s == nil || id == "" || h == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[id]; !ok {
		return
	}
	s.entries[id] = deckHandleEntry{handle: h, expiresAt: s.now().Add(s.ttl)}
}

// Load returns the live handle for an id. An expired or unknown id (and a nil
// receiver) reports ok=false.
func (s *deckHandleStore) Load(id string) (*deckHandle, bool) {
	if s == nil || id == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	if !ok {
		return nil, false
	}
	if s.now().After(entry.expiresAt) {
		delete(s.entries, id)
		return nil, false
	}
	return entry.handle, true
}

// newDeckID mints a 128-bit random hex id.
func newDeckID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return "deck_" + hex.EncodeToString(b[:])
}

// --- resolving a call's spec ---

// specSource is where a spec tool's deck came from: the bytes to act on, the
// handle to echo, and what a patch on this call changed.
type specSource struct {
	// Data is the spec to act on, already patched.
	Data []byte
	// Filename is the name the spec was (or was originally) parsed under.
	Filename string
	// DeckID is the handle the call named, empty when the caller sent a spec.
	DeckID string
	// ChangedSlides lists the 0-based slides a patch on this call touched; nil
	// for a plain spec or an unpatched handle.
	ChangedSlides []int
	// Template is the template the handle's last render resolved to. A
	// handle-driven re-render that names no template falls back to it, so
	// patching a deck cannot silently restyle it.
	Template string
}

// resolveSpecSource returns the spec a call should act on, from whichever of
// spec / deck_id (+ optional patch) the caller supplied. It is the one place the
// three forms are reconciled, so every spec tool accepts the same three.
func (mc *mcpConfig) resolveSpecSource(tool string, request mcp.CallToolRequest) (specSource, *mcp.CallToolResult) {
	args := request.GetArguments()
	rawID, _ := args["deck_id"].(string)
	rawID = strings.TrimSpace(rawID)
	_, hasSpec := args["spec"]

	if rawID == "" {
		return mc.specSourceFromSpec(tool, request, hasSpec)
	}
	if hasSpec {
		return specSource{}, argError(argErrorEnvelope{
			Code:         diagnostics.CodeAmbiguousInput,
			Path:         "deck_id",
			Message:      "set deck_id OR spec, not both: deck_id names the spec the server already holds, and sending a spec alongside it makes it ambiguous which one the call means",
			ExpectedType: "string",
			NextToolCall: nextCallRetry(tool, "deck_id"),
		})
	}

	handle, ok := mc.deckHandles.Load(rawID)
	if !ok {
		return specSource{}, argError(argErrorEnvelope{
			Code:         diagnostics.CodeInvalidParameter,
			Path:         "deck_id",
			Message:      fmt.Sprintf("deck_id %q is unknown or expired — handles are per-process and live for %s; send the spec itself to start a new one", rawID, deckHandleTTL),
			ExpectedType: "string",
			NextToolCall: nextCallRetry(tool, "spec"),
		})
	}

	patched, changed, errRes := applySpecPatchArg(tool, request, handle)
	if errRes != nil {
		return specSource{}, errRes
	}
	return specSource{
		Data:          patched,
		Filename:      handle.Filename,
		DeckID:        rawID,
		ChangedSlides: changed,
		Template:      handle.Template,
	}, nil
}

// specSourceFromSpec handles the stateless form: a spec in the call itself.
func (mc *mcpConfig) specSourceFromSpec(tool string, request mcp.CallToolRequest, hasSpec bool) (specSource, *mcp.CallToolResult) {
	args := request.GetArguments()
	if !hasSpec {
		return specSource{}, argMissing(tool, "spec", "object|string", map[string]any{
			"meta":   map[string]any{"title": "My Deck"},
			"slides": []any{map[string]any{"kind": "title", "title": "My Deck"}},
		}, nil)
	}
	if _, patching := args["patch"]; patching {
		return specSource{}, argError(argErrorEnvelope{
			Code:         diagnostics.CodeInvalidParameter,
			Path:         "patch",
			Message:      "patch edits a deck this server holds, so it needs deck_id; send the edited spec directly instead, or render once and patch the deck_id that call returns",
			ExpectedType: "array",
			NextToolCall: nextCallRetry(tool, "deck_id"),
		})
	}
	data, name, errRes := semanticSpecBytes(tool, request)
	if errRes != nil {
		return specSource{}, errRes
	}
	return specSource{Data: data, Filename: name}, nil
}

// applySpecPatchArg applies the call's patch (when present) to a handle's spec
// and returns the resulting bytes plus the slide indices that changed. With no
// patch it returns the stored spec unchanged.
func applySpecPatchArg(tool string, request mcp.CallToolRequest, handle *deckHandle) ([]byte, []int, *mcp.CallToolResult) {
	rawPatch, ok := request.GetArguments()["patch"]
	if !ok || rawPatch == nil {
		return handle.Spec, nil, nil
	}
	ops, errRes := parseSpecPatchOps(tool, rawPatch)
	if errRes != nil {
		return nil, nil, errRes
	}
	if len(ops) == 0 {
		return handle.Spec, nil, nil
	}

	var doc any
	if err := json.Unmarshal(handle.Spec, &doc); err != nil {
		return nil, nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "deck_id",
			fmt.Sprintf("the stored spec could not be decoded for patching: %v", err), "string", nil, nil)
	}
	for i, op := range ops {
		var err error
		doc, err = op.apply(doc)
		if err != nil {
			return nil, nil, argError(argErrorEnvelope{
				Code:         diagnostics.CodeInvalidParameter,
				Path:         fmt.Sprintf("patch[%d].path", i),
				Message:      fmt.Sprintf("patch[%d] %s %s: %v", i, op.Op, op.Path, err),
				ExpectedType: "string",
				ExampleValue: "/slides/3/title",
			})
		}
	}
	patched, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "patch",
			fmt.Sprintf("the patched spec could not be re-encoded: %v", err), "array", nil, nil)
	}
	return patched, changedSlideIndices(handle, patched), nil
}

// --- patch ops ---

// specPatchOp is one JSON-Pointer-addressed edit to a stored spec. The
// vocabulary is deliberately the three operations a deck revision needs —
// replace a field, add a slide, remove a slide — rather than all of RFC 6902:
// an agent revising a deck does not need move/copy/test, and a smaller
// vocabulary is one an agent can use correctly from the tool description alone.
type specPatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// deckPatchOps are the accepted operations.
const (
	specPatchReplace = "replace"
	specPatchAdd     = "add"
	specPatchRemove  = "remove"
)

// parseSpecPatchOps decodes and validates the patch argument.
func parseSpecPatchOps(tool string, raw any) ([]specPatchOp, *mcp.CallToolResult) {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "patch",
			fmt.Sprintf("patch could not be decoded: %v", err), "array", specPatchExample(), nil)
	}
	var ops []specPatchOp
	if err := json.Unmarshal(encoded, &ops); err != nil {
		return nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "patch",
			fmt.Sprintf("patch must be an array of {op, path, value}: %v", err), "array", specPatchExample(), nil)
	}
	for i, op := range ops {
		switch op.Op {
		case specPatchReplace, specPatchAdd, specPatchRemove:
		default:
			return nil, argInvalidValue(tool, diagnostics.CodeUnknownEnum, fmt.Sprintf("patch[%d].op", i),
				fmt.Sprintf("unknown patch op %q; use replace, add or remove", op.Op), "string", specPatchExample(), nil)
		}
		if !strings.HasPrefix(op.Path, "/") {
			return nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, fmt.Sprintf("patch[%d].path", i),
				fmt.Sprintf("path must be a JSON Pointer into the spec, e.g. /slides/3/title; got %q", op.Path), "string", specPatchExample(), nil)
		}
		if op.Op != specPatchRemove && op.Value == nil {
			return nil, argInvalidValue(tool, diagnostics.CodeMissingParameter, fmt.Sprintf("patch[%d].value", i),
				fmt.Sprintf("%s needs a value", op.Op), "any", specPatchExample(), nil)
		}
	}
	return ops, nil
}

// specPatchExample is the copy-ready patch shown in every patch diagnostic.
func specPatchExample() any {
	return []any{map[string]any{"op": "replace", "path": "/slides/3/title", "value": "Enterprise carried the year"}}
}

// apply performs one op against a decoded spec, returning the new document.
func (op specPatchOp) apply(doc any) (any, error) {
	segments, err := parsePointer(op.Path)
	if err != nil {
		return nil, err
	}
	if len(segments) == 0 {
		return nil, fmt.Errorf("the whole document cannot be replaced; patch a field under it")
	}
	return applyPointer(doc, segments, op)
}

// parsePointer splits a JSON Pointer into its unescaped segments.
func parsePointer(pointer string) ([]string, error) {
	if pointer == "/" {
		return nil, fmt.Errorf("path must name a field, not the document root")
	}
	raw := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	out := make([]string, 0, len(raw))
	for _, seg := range raw {
		if seg == "" {
			return nil, fmt.Errorf("path has an empty segment")
		}
		seg = strings.ReplaceAll(seg, "~1", "/")
		seg = strings.ReplaceAll(seg, "~0", "~")
		out = append(out, seg)
	}
	return out, nil
}

// applyPointer walks to the parent of the addressed location and performs op.
func applyPointer(doc any, segments []string, op specPatchOp) (any, error) {
	if len(segments) == 1 {
		return applyHere(doc, segments[0], op)
	}
	head, rest := segments[0], segments[1:]
	switch node := doc.(type) {
	case map[string]any:
		child, ok := node[head]
		if !ok {
			return nil, fmt.Errorf("%q does not exist", head)
		}
		updated, err := applyPointer(child, rest, op)
		if err != nil {
			return nil, err
		}
		node[head] = updated
		return node, nil
	case []any:
		idx, err := listIndex(head, len(node), false)
		if err != nil {
			return nil, err
		}
		updated, err := applyPointer(node[idx], rest, op)
		if err != nil {
			return nil, err
		}
		node[idx] = updated
		return node, nil
	default:
		return nil, fmt.Errorf("%q is not a container", head)
	}
}

// applyHere performs op on the named member of doc.
func applyHere(doc any, segment string, op specPatchOp) (any, error) {
	switch node := doc.(type) {
	case map[string]any:
		switch op.Op {
		case specPatchRemove:
			if _, ok := node[segment]; !ok {
				return nil, fmt.Errorf("%q does not exist", segment)
			}
			delete(node, segment)
		case specPatchReplace:
			if _, ok := node[segment]; !ok {
				return nil, fmt.Errorf("%q does not exist; use add to create it", segment)
			}
			node[segment] = op.Value
		case specPatchAdd:
			node[segment] = op.Value
		}
		return node, nil
	case []any:
		// "-" appends, matching RFC 6902.
		if segment == "-" {
			if op.Op != specPatchAdd {
				return nil, fmt.Errorf(`"-" appends, so it only works with add`)
			}
			return append(node, op.Value), nil
		}
		idx, err := listIndex(segment, len(node), op.Op == specPatchAdd)
		if err != nil {
			return nil, err
		}
		switch op.Op {
		case specPatchRemove:
			return append(node[:idx], node[idx+1:]...), nil
		case specPatchReplace:
			node[idx] = op.Value
			return node, nil
		case specPatchAdd:
			node = append(node, nil)
			copy(node[idx+1:], node[idx:])
			node[idx] = op.Value
			return node, nil
		}
		return node, nil
	default:
		return nil, fmt.Errorf("%q is not a container", segment)
	}
}

// listIndex parses an array index, allowing one past the end for an insert.
func listIndex(segment string, length int, allowAppend bool) (int, error) {
	idx, err := strconv.Atoi(segment)
	if err != nil {
		return 0, fmt.Errorf("%q is not an array index", segment)
	}
	limit := length
	if !allowAppend {
		limit = length - 1
	}
	if idx < 0 || idx > limit {
		return 0, fmt.Errorf("index %d is outside the array (length %d)", idx, length)
	}
	return idx, nil
}

// --- change detection ---

// changedSlideIndices reports which slides differ between a handle's stored
// spec and a patched one. It compares per-slide digests, so any edit counts,
// and reports every index from the first structural change onward when the
// slide COUNT changed — inserting a slide shifts everything after it.
func changedSlideIndices(handle *deckHandle, patched []byte) []int {
	after := slideDigests(patched)
	before := handle.SlideDigests
	changed := map[int]bool{}
	for i := 0; i < len(after); i++ {
		if i >= len(before) || before[i] != after[i] {
			changed[i] = true
		}
	}
	// A removal leaves trailing indices that no longer exist; report the last
	// surviving slide so the caller re-pulls the tail it can still see.
	if len(after) < len(before) && len(after) > 0 {
		changed[len(after)-1] = true
	}
	out := make([]int, 0, len(changed))
	for i := range changed {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// slideDigests returns a content digest per slide of an encoded spec. Each
// slide is re-encoded before hashing, so the digest answers "did this slide's
// content change" rather than "were these bytes formatted the same way" — the
// stored spec is machine-encoded and the caller's was hand-written.
func slideDigests(spec []byte) []string {
	var doc struct {
		Slides []json.RawMessage `json:"slides"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil
	}
	out := make([]string, 0, len(doc.Slides))
	for _, s := range doc.Slides {
		out = append(out, diagnostics.ComputeInputSHA256(canonicalJSON(s)))
	}
	return out
}

// canonicalJSON re-encodes a JSON value with sorted keys and no insignificant
// whitespace. Undecodable input is returned unchanged: a digest over the raw
// bytes is still a digest, it is only less forgiving of reformatting.
func canonicalJSON(raw []byte) []byte {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}

// canonicalSpec converts a spec to the one form a handle stores: JSON. A YAML
// spec is decoded and re-encoded, and its filename follows, because the next
// call parses the stored bytes and Parse dispatches on the extension. A spec
// that decodes as neither is stored verbatim — it will not patch, but it is
// also about to be reported as a parse error by the tool that stored it.
func canonicalSpec(filename string, spec []byte) ([]byte, string) {
	var doc any
	if err := json.Unmarshal(spec, &doc); err != nil {
		if yerr := safeyaml.Unmarshal(spec, &doc); yerr != nil {
			return spec, filename
		}
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return spec, filename
	}
	return encoded, jsonSpecFilename(filename)
}

// jsonSpecFilename gives a spec filename a .json extension so Parse reads the
// stored bytes as the JSON they now are.
func jsonSpecFilename(filename string) string {
	if filename == "" {
		return "deck.json"
	}
	if strings.HasSuffix(strings.ToLower(filename), ".json") {
		return filename
	}
	if i := strings.LastIndex(filename, "."); i > 0 {
		return filename[:i] + ".json"
	}
	return filename + ".json"
}

// newDeckHandleFor builds the handle to store for a spec.
func newDeckHandleFor(spec []byte, filename, template string) *deckHandle {
	canonical, name := canonicalSpec(filename, spec)
	return &deckHandle{
		Spec:         canonical,
		Filename:     name,
		SlideDigests: slideDigests(canonical),
		Template:     template,
	}
}

// rememberDeck stores (or refreshes) the handle for a call's spec and returns
// the id to echo. An existing id is kept so one identifier covers the session.
func (mc *mcpConfig) rememberDeck(existingID string, spec []byte, filename, template string) string {
	h := newDeckHandleFor(spec, filename, template)
	if existingID != "" {
		mc.deckHandles.Update(existingID, h)
		return existingID
	}
	return mc.deckHandles.Save(h)
}

// --- tool parameters ---

const deckIDParamDescription = "Optional handle for a spec this server already holds, returned as deck_id by a previous validate_deck_spec or render_deck_spec call. Send it INSTEAD of spec to act on the stored deck without re-uploading it. Combine it with patch to change part of the deck — a one-line title fix costs a ~120-byte call instead of the whole spec. Handles are per-process and expire after 1 hour; an unknown or expired one is an error, not a silent miss, so send the spec again to start a fresh handle."

const deckPatchParamDescription = `Optional edits to apply to the deck named by deck_id before this call acts on it: [{op, path, value}] where op is replace | add | remove and path is a JSON Pointer into the spec (e.g. /slides/3/title, /meta/template, /slides/6 with add to insert a slide, "-" as the last segment to append). Requires deck_id. The stored deck is updated, so the next call sees the edit; the response's changed_slides names the slides that differ.`

// deckHandleToolParams declares deck_id and patch on a spec tool.
func deckHandleToolParams() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithString("deck_id", mcp.Description(deckIDParamDescription)),
		mcp.WithArray("patch", mcp.Description(deckPatchParamDescription)),
	}
}
