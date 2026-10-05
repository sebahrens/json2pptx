package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/svggen/safeyaml"
)

// Server-side deck handles (go-slide-creator-voxp).
//
// Measured on a 15-slide deck: the DeckSpec loop (validate → render → thumbs →
// fix → validate → render → thumbs) resent the whole 3,695-byte spec on 5 of 8
// calls. A one-line title fix cost a full spec re-upload. Every tool was
// stateless, so the agent was the only place the deck lived, and it paid for
// that on every call.
//
// A deck_id names a TTL'd server-side DeckSpec or raw presentation. DeckSpec
// tools can patch a spec; raw tools can repair and regenerate a stored raw deck
// without re-uploading it. Stateless calls remain supported.

// deckHandleTTL bounds how long a handle stays loadable. It matches the loop
// session TTL: both are interactive-session scratch state, not persistence.
const deckHandleTTL = time.Hour

// deckHandle is the server's copy of one deck source and its slide digests, so
// a follow-up call can answer "what changed" without the agent re-sending it.
type deckHandle struct {
	// RawPresentation is a generated raw deck. A non-nil value distinguishes
	// it from a DeckSpec handle, even when the raw JSON happens to be empty.
	RawPresentation []byte
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
	// Template is the template the deck is bound to: the first template a call
	// named for it, or the spec's meta.template. A re-render without an
	// explicit template keeps the deck looking the same, and the binding
	// changes only by a patch to /meta/template (go-slide-creator-2dit4).
	Template string
	// TemplatePath is the vetted, absolute bring-your-own .pptx the last render
	// used (render_deck_spec template_path), and BaseDir the allowed root it was
	// vetted against — also the frame relative asset paths resolved in. Without
	// them a BYO deck_id re-rendered, validated or scored against template ""
	// (go-slide-creator-b7qqg.8). A handle holds a Template OR a TemplatePath.
	TemplatePath string
	BaseDir      string

	// Revision is the number of the stored spec revision, starting at 1, and
	// Revisions the kept history (go-slide-creator-rq1z9). NextSlideID is the
	// counter behind assigned slide ids (go-slide-creator-1w3uo). State is the
	// stored revision's per-slide digests; Rendered the same for the last
	// revision that rendered, which is what a render's changed_slides is
	// measured against (go-slide-creator-v5e9h). A stored handle is never
	// mutated: every store replaces it.
	Revision     int
	Revisions    []deckRevision
	NextSlideID  int
	State        *deckState
	Rendered     *deckState
	RenderedPptx string

	// storeTool / storeNote label the revision this handle creates when it is
	// stored; pendingRenderPptx marks it as rendered. All three are consumed
	// by inherit under the store lock.
	storeTool, storeNote  string
	storeMoved            map[string]bool
	pendingRenderPptx     string
	pendingRenderIdentity string
	// pendingRenderKeys is the rendered identity of each slide in the PPTX the
	// storing call wrote (go-slide-creator-o477e); nil when it rendered none.
	pendingRenderKeys []string
	// storeEvaluated is the template the storing call validated or rendered
	// on, recorded on the revision (go-slide-creator-oqu4a).
	storeEvaluated string
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
	pruneTTLEntries(s.entries, s.now(), maxSessionStoreEntries, func(e deckHandleEntry) time.Time { return e.expiresAt })
	h.inherit(nil, h.storeTool, h.storeNote, s.now())
	s.entries[id] = deckHandleEntry{handle: h, expiresAt: s.now().Add(s.ttl)}
	return id
}

// Update replaces the handle behind an existing id, keeping the id stable
// across a patch so an agent holds one identifier for the whole session. It
// refreshes the TTL: an actively edited deck should not expire mid-session.
//
// base is the spec the caller loaded before patching. When non-nil, Update is
// a compare-and-swap: it returns false without writing if another call has
// replaced the spec since, so concurrent patches cannot silently drop one
// another. An unknown id is a no-op that reports true.
func (s *deckHandleStore) Update(id string, base []byte, h *deckHandle) bool {
	if s == nil || id == "" || h == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	if !ok {
		return true
	}
	if base != nil && !bytes.Equal(entry.handle.Spec, base) {
		return false
	}
	// Validation and explanation refresh the spec but do not choose a new
	// template or filename. Keep the last render's metadata unless the caller
	// supplied a replacement. Do this under the store lock so updates cannot
	// briefly expose a handle with an empty template.
	if h.Template == "" && h.TemplatePath == "" {
		h.Template = entry.handle.Template
		h.TemplatePath = entry.handle.TemplatePath
	}
	if h.BaseDir == "" {
		h.BaseDir = entry.handle.BaseDir
	}
	if h.Filename == "" {
		h.Filename = entry.handle.Filename
	}
	h.inherit(entry.handle, h.storeTool, h.storeNote, s.now())
	s.entries[id] = deckHandleEntry{handle: h, expiresAt: s.now().Add(s.ttl)}
	return true
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

// UpdateRaw is a compare-and-swap for a raw deck. Two repairs starting from
// the same revision must not silently overwrite one another; only the first
// writer can replace the handle. Both the check and replacement hold the lock.
func (s *deckHandleStore) UpdateRaw(id string, previous, next []byte) bool {
	if s == nil || id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	if !ok || s.now().After(entry.expiresAt) || entry.handle.RawPresentation == nil || !bytes.Equal(entry.handle.RawPresentation, previous) {
		return false
	}
	s.entries[id] = deckHandleEntry{handle: &deckHandle{
		RawPresentation: append([]byte(nil), next...),
		SlideDigests:    slideDigests(next),
	}, expiresAt: s.now().Add(s.ttl)}
	return true
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
	// Handle is the stored handle DeckID named, as loaded for this call.
	Handle *deckHandle
	// DryRun asks the call not to store its result; Fork to store it under a
	// new deck_id; Restore names the kept revision the call started from (0
	// when it started from the current one). RawPatch is the patch as sent.
	DryRun   bool
	Fork     bool
	Restore  int
	RawPatch []any
	// NewDeck is fork on a spec sent in the call: store it under a new
	// deck_id even when it revises a stored deck (go-slide-creator-o477e).
	NewDeck bool
	// MovedIDs names the slides this call's move ops picked up.
	MovedIDs map[string]bool
	// Template is the template the handle is bound to (go-slide-creator-2dit4). A
	// handle-driven re-render that names no template falls back to it, so
	// patching a deck cannot silently restyle it.
	Template string
	// TemplatePath / BaseDir are the handle's bring-your-own template and the
	// root it (and relative assets) resolved against (go-slide-creator-b7qqg.8).
	TemplatePath string
	BaseDir      string
	// BaseSpec is the stored spec the call loaded before patching; storing
	// the result compares against it so concurrent patches cannot be lost.
	BaseSpec []byte
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
	if handle.RawPresentation != nil {
		return specSource{}, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "deck_id",
			"deck_id names a raw presentation, not a DeckSpec; use a raw-deck tool or send a DeckSpec", "string", nil, nil)
	}

	src := specSource{
		Filename:     handle.Filename,
		DeckID:       rawID,
		Handle:       handle,
		Template:     handle.Template,
		TemplatePath: handle.TemplatePath,
		BaseDir:      handle.BaseDir,
		BaseSpec:     handle.Spec,
	}
	var errRes *mcp.CallToolResult
	if src.DryRun, src.Fork, errRes = deckStoreFlags(tool, request); errRes != nil {
		return specSource{}, errRes
	}
	start := handle.Spec
	if src.Restore, errRes = restoreArg(tool, request); errRes != nil {
		return specSource{}, errRes
	}
	if src.Restore > 0 {
		rev, kept := handle.revision(src.Restore)
		if !kept {
			return specSource{}, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "restore",
				fmt.Sprintf("revision %d is not kept for this deck_id (kept: %s); read:\"history\" on validate_deck_spec lists them", src.Restore, handle.keptRevisions()),
				"integer", handle.Revision, nil)
		}
		start = rev.Spec
	}
	if src.Data, src.RawPatch, src.MovedIDs, errRes = applySpecPatchArg(tool, request, start, handle.NextSlideID); errRes != nil {
		return specSource{}, errRes
	}
	return src, nil
}

// mutated reports whether the call changed the spec its deck_id holds.
func (src specSource) mutated() bool {
	return src.DeckID != "" && !bytes.Equal(src.Data, src.BaseSpec)
}

// deckStoreFlags reads dry_run and fork.
func deckStoreFlags(tool string, request mcp.CallToolRequest) (dryRun, fork bool, errRes *mcp.CallToolResult) {
	if dryRun, errRes = semanticOptionalBool(tool, "dry_run", request); errRes != nil {
		return false, false, errRes
	}
	fork, errRes = semanticOptionalBool(tool, "fork", request)
	return dryRun, fork, errRes
}

// restoreArg reads the optional revision number to start from.
func restoreArg(tool string, request mcp.CallToolRequest) (int, *mcp.CallToolResult) {
	raw, ok := request.GetArguments()["restore"]
	if !ok || raw == nil {
		return 0, nil
	}
	n, isNumber := raw.(float64)
	if !isNumber || n < 1 || n != float64(int(n)) {
		return 0, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "restore",
			fmt.Sprintf("restore must be a revision number (an integer of at least 1), got %v", raw), "integer", 1, nil)
	}
	return int(n), nil
}

// specSourceFromSpec handles the stateless form: a spec in the call itself.
func (mc *mcpConfig) specSourceFromSpec(tool string, request mcp.CallToolRequest, hasSpec bool) (specSource, *mcp.CallToolResult) {
	args := request.GetArguments()
	if !hasSpec {
		return specSource{}, argRequired(request, tool, "spec", "object|string", map[string]any{
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
	if raw, restoring := args["restore"]; restoring && raw != nil {
		return specSource{}, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "restore",
			"restore starts from a revision this server kept, so it needs deck_id", "integer", nil, nextCallRetry(tool, "deck_id"))
	}
	data, name, errRes := semanticSpecBytes(tool, request)
	if errRes != nil {
		return specSource{}, errRes
	}
	src := specSource{Data: data, Filename: name}
	// A spec sent in the call starts a new handle unless it revises a deck
	// the server holds (deckHandleStore.Continued); fork asks for a new one
	// regardless. dry_run still means "do not store".
	if src.DryRun, src.NewDeck, errRes = deckStoreFlags(tool, request); errRes != nil {
		return specSource{}, errRes
	}
	return src, nil
}

// applySpecPatchArg applies the call's patch (when present) to spec — the
// stored revision the call starts from — and returns the resulting bytes plus
// the patch as sent. With no patch it returns spec unchanged. Slides the patch
// added get their id here (counting on from nextSlideID), so the response can
// name them even when the result is not stored.
func applySpecPatchArg(tool string, request mcp.CallToolRequest, spec []byte, nextSlideID int) ([]byte, []any, map[string]bool, *mcp.CallToolResult) {
	rawPatch, ok := request.GetArguments()["patch"]
	if !ok || rawPatch == nil {
		return spec, nil, nil, nil
	}
	ops, errRes := parseSpecPatchOps(tool, rawPatch)
	if errRes != nil {
		return nil, nil, nil, errRes
	}
	if len(ops) == 0 {
		return spec, nil, nil, nil
	}

	var doc any
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, nil, nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "deck_id",
			fmt.Sprintf("the stored spec could not be decoded for patching: %v", err), "string", nil, nil)
	}
	moved := map[string]bool{}
	for i, op := range ops {
		var err error
		if op.Op == specPatchMove {
			if from, perr := parsePointer(op.From); perr == nil {
				if slide, verr := pointerValue(doc, from); verr == nil {
					if m, ok := slide.(map[string]any); ok {
						if id, ok := m["id"].(string); ok {
							moved[id] = true
						}
					}
				}
			}
		}
		doc, err = op.apply(doc)
		if err != nil {
			path := fmt.Sprintf("patch[%d].path", i)
			var fromErr *patchFromError
			if errors.As(err, &fromErr) {
				path = fmt.Sprintf("patch[%d].from", i)
			}
			return nil, nil, nil, argError(argErrorEnvelope{
				Code:         diagnostics.CodeInvalidParameter,
				Path:         path,
				Message:      fmt.Sprintf("patch[%d] %s %s: %v", i, op.Op, op.Path, err),
				ExpectedType: "string",
				ExampleValue: "/slides/3/title",
			})
		}
		if id, dup := duplicateSlideID(doc); dup {
			return nil, nil, nil, argError(argErrorEnvelope{
				Code:         diagnostics.CodeInvalidParameter,
				Path:         fmt.Sprintf("patch[%d].value", i),
				Message:      fmt.Sprintf("patch[%d] %s %s: slide id %q is already used by another slide; ids must be unique, so drop the id (one is assigned) or choose another", i, op.Op, op.Path, id),
				ExpectedType: "string",
			})
		}
	}
	assignSlideIDs(doc, nextSlideID)
	patched, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "patch",
			fmt.Sprintf("the patched spec could not be re-encoded: %v", err), "array", nil, nil)
	}
	sent, _ := rawPatch.([]any)
	return patched, sent, moved, nil
}

// --- patch ops ---

// specPatchOp is one JSON-Pointer-addressed edit to a stored spec: the RFC
// 6902 operations a deck revision needs. move and copy arrived with the
// e-revise journey (go-slide-creator-83kru): without them a reorder meant
// removing a slide and re-sending it whole from the agent's memory. test is
// still left out — a patch is already all-or-nothing.
type specPatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	From  string `json:"from,omitempty"`
	Value any    `json:"value,omitempty"`
}

// deckPatchOps are the accepted operations.
const (
	specPatchReplace = "replace"
	specPatchAdd     = "add"
	specPatchRemove  = "remove"
	specPatchMove    = "move"
	specPatchCopy    = "copy"
)

// patchFromError marks a failure at an op's from pointer rather than its path.
type patchFromError struct{ err error }

func (e *patchFromError) Error() string { return "from " + e.err.Error() }
func (e *patchFromError) Unwrap() error { return e.err }

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
		reads := false
		switch op.Op {
		case specPatchReplace, specPatchAdd, specPatchRemove:
		case specPatchMove, specPatchCopy:
			reads = true
		default:
			return nil, argInvalidValue(tool, diagnostics.CodeUnknownEnum, fmt.Sprintf("patch[%d].op", i),
				fmt.Sprintf("unknown patch op %q; use replace, add, remove, move or copy", op.Op), "string", specPatchExample(), nil)
		}
		if !strings.HasPrefix(op.Path, "/") {
			return nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, fmt.Sprintf("patch[%d].path", i),
				fmt.Sprintf("path must be a JSON Pointer into the spec, e.g. /slides/3/title; got %q", op.Path), "string", specPatchExample(), nil)
		}
		if reads {
			if !strings.HasPrefix(op.From, "/") {
				return nil, argInvalidValue(tool, diagnostics.CodeMissingParameter, fmt.Sprintf("patch[%d].from", i),
					fmt.Sprintf("%s needs from: the JSON Pointer of the value to %s, e.g. /slides/5; got %q", op.Op, op.Op, op.From), "string",
					[]any{map[string]any{"op": op.Op, "from": "/slides/5", "path": "/slides/7"}}, nil)
			}
			continue
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
	switch op.Op {
	case specPatchMove, specPatchCopy:
		return op.applyFrom(doc, segments)
	}
	return applyPointer(doc, segments, op)
}

// applyFrom performs move and copy: read the value at from, then (for move)
// remove it there, then add it at path. As in RFC 6902 the path is resolved
// after the removal, so "move /slides/5 to /slides/7" lands the slide at index
// 7 of the resulting deck; a slide id as the last path segment inserts before
// that slide.
func (op specPatchOp) applyFrom(doc any, segments []string) (any, error) {
	from, err := parsePointer(op.From)
	if err != nil || len(from) == 0 {
		if err == nil {
			err = fmt.Errorf("must name a field, not the document root")
		}
		return nil, &patchFromError{err}
	}
	value, err := pointerValue(doc, from)
	if err != nil {
		return nil, &patchFromError{err}
	}
	if op.Op == specPatchMove {
		if strings.HasPrefix(op.Path+"/", op.From+"/") && op.Path != op.From {
			return nil, fmt.Errorf("a value cannot be moved into itself")
		}
		if doc, err = applyPointer(doc, from, specPatchOp{Op: specPatchRemove}); err != nil {
			return nil, &patchFromError{err}
		}
	} else {
		value = deepCopyJSON(value)
		// A copied slide is a new slide: it must not share the original's id.
		if slide, ok := value.(map[string]any); ok && len(segments) >= 2 && segments[len(segments)-2] == "slides" {
			delete(slide, "id")
		}
	}
	return applyPointer(doc, segments, specPatchOp{Op: specPatchAdd, Value: value})
}

// deepCopyJSON copies a decoded JSON value so a copy op shares nothing with
// its source.
func deepCopyJSON(v any) any {
	switch node := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(node))
		for k, child := range node {
			out[k] = deepCopyJSON(child)
		}
		return out
	case []any:
		out := make([]any, len(node))
		for i, child := range node {
			out[i] = deepCopyJSON(child)
		}
		return out
	}
	return v
}

// pointerValue returns the value a pointer addresses.
func pointerValue(doc any, segments []string) (any, error) {
	node := doc
	for _, seg := range segments {
		switch current := node.(type) {
		case map[string]any:
			child, ok := current[seg]
			if !ok {
				return nil, fmt.Errorf("%q does not exist", seg)
			}
			node = child
		case []any:
			idx, err := listIndex(seg, current, false)
			if err != nil {
				return nil, err
			}
			node = current[idx]
		default:
			return nil, fmt.Errorf("%q is not a container", seg)
		}
	}
	return node, nil
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
		idx, err := listIndex(head, node, false)
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
		idx, err := listIndex(segment, node, op.Op == specPatchAdd)
		if err != nil {
			return nil, err
		}
		switch op.Op {
		case specPatchRemove:
			return append(node[:idx], node[idx+1:]...), nil
		case specPatchReplace:
			node[idx] = keepSlideID(node[idx], op.Value)
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

// keepSlideID carries an element's id onto the object that replaces it when
// the replacement names none: replacing /slides/2 rewrites that slide, it does
// not swap it for a stranger.
func keepSlideID(old, replacement any) any {
	was, ok := old.(map[string]any)
	next, isObject := replacement.(map[string]any)
	if !ok || !isObject {
		return replacement
	}
	id, hasID := was["id"].(string)
	if _, named := next["id"]; hasID && !named {
		next["id"] = id
	}
	return replacement
}

// listIndex resolves an array segment: a 0-based index (one past the end is
// allowed for an insert), or the id of an element, so /slides/s4/title keeps
// addressing the same slide when an insert shifts its index
// (go-slide-creator-1w3uo). An id starts with a letter, so the two never clash.
func listIndex(segment string, node []any, allowAppend bool) (int, error) {
	length := len(node)
	idx, err := strconv.Atoi(segment)
	if err != nil {
		var ids []string
		for i, el := range node {
			m, ok := el.(map[string]any)
			if !ok {
				continue
			}
			id, ok := m["id"].(string)
			if !ok {
				continue
			}
			if id == segment {
				return i, nil
			}
			ids = append(ids, id)
		}
		if len(ids) > 0 {
			return 0, fmt.Errorf("no element with id %q here (ids: %s); address it by id or by 0-based index", segment, strings.Join(ids, ", "))
		}
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
	// A DeckSpec meta / structure edit (template, chrome, accent strategy, …)
	// restyles every slide without touching any slide's own content. Reporting
	// no changed slides told the agent there was nothing to re-inspect
	// (go-slide-creator-6p9mm).
	if handle.RawPresentation == nil && deckLevelDigest(handle.Spec) != deckLevelDigest(patched) {
		for i := range after {
			changed[i] = true
		}
	}
	out := make([]int, 0, len(changed))
	for i := range changed {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// deckLevelDigest digests everything in an encoded spec except its slides
// array: the deck-level settings that apply to every slide.
func deckLevelDigest(spec []byte) string {
	canonical, _ := canonicalSpec("spec.json", spec)
	var doc map[string]any
	if err := json.Unmarshal(canonical, &doc); err != nil {
		return ""
	}
	delete(doc, "slides")
	encoded, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	return diagnostics.ComputeInputSHA256(encoded)
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
// base is the stored spec the call started from (specSource.BaseSpec). It
// reports false when another call changed the stored deck in the meantime and
// this call's edit would overwrite it; a call that made no edit then leaves
// the newer stored deck in place and still reports true.
func (mc *mcpConfig) rememberDeck(existingID string, base, spec []byte, filename, template string) (string, bool) {
	return mc.rememberDeckSource(existingID, base, spec, filename, deckTemplateSource{Template: template})
}

// deckTemplateSource is the template identity a render resolved: a registered
// name, or a bring-your-own file plus the root it was vetted against.
type deckTemplateSource struct {
	Template     string
	TemplatePath string
	BaseDir      string
}

// rememberDeckSource is rememberDeck carrying the full template source, so a
// bring-your-own template survives on the handle (go-slide-creator-b7qqg.8).
func (mc *mcpConfig) rememberDeckSource(existingID string, base, spec []byte, filename string, src deckTemplateSource) (string, bool) {
	h := newDeckHandleFor(spec, filename, src.Template)
	h.TemplatePath = src.TemplatePath
	h.BaseDir = src.BaseDir
	if h.TemplatePath != "" {
		h.Template = ""
	}
	if existingID != "" {
		if filename == "" {
			// newDeckHandleFor gives an unnamed new deck a default filename;
			// for an existing deck omission instead means retain its name.
			h.Filename = ""
		}
		if !mc.deckHandles.Update(existingID, base, h) {
			return existingID, bytes.Equal(h.Spec, base)
		}
		return existingID, true
	}
	return mc.deckHandles.Save(h), true
}

// staleDeckSpecResult reports a deck_id patch that lost a race with another
// call on the same handle.
func staleDeckSpecResult(tool, deckID string) *mcp.CallToolResult {
	return argInvalidValue(tool, "STALE_REVISION", "deck_id",
		"another call changed this deck_id while this one was running, so its patch was not stored; reload with deck_id (no patch) and re-apply the edit", "string", deckID, nil)
}

// rememberRawDeck stores the effective raw presentation after defaults and
// structure expansion. A semantic source gets a NEW raw handle; an existing
// raw source keeps its id only if nobody revised it since the caller loaded it.
func (mc *mcpConfig) rememberRawDeck(existingID string, previous, presentation []byte) (string, bool) {
	if existingID != "" {
		old, ok := mc.deckHandles.Load(existingID)
		if !ok {
			return "", false
		}
		if old.RawPresentation != nil {
			return existingID, mc.deckHandles.UpdateRaw(existingID, previous, presentation)
		}
	}
	h := &deckHandle{RawPresentation: append([]byte(nil), presentation...), SlideDigests: slideDigests(presentation)}
	return mc.deckHandles.Save(h), true
}

// --- tool parameters ---

const deckIDParamDescription = "Handle of a spec this server holds: the deck_id a validate_deck_spec or render_deck_spec response returned. Send it INSTEAD of spec; add patch to edit part of the deck. Per-process, expires after 1 hour; an unknown or expired id is an error, so send the spec again."

const deckPatchParamDescription = `Edits to deck_id's spec, applied before the call acts: [{op, path, value | from}]. move and copy read from. path is a JSON Pointer; a slide is named by index or id (/slides/3/title, /slides/s4/title, /meta/template); add at /slides/6 inserts, "-" appends. All ops apply or none. The response has stored, revision, changed_slides (slides that look different) and slide_changes (edited | inserted | restyled | moved | renumbered | notes_only | removed). A refused render stores nothing.`

// deckHandleToolParams declares the deck-store arguments of a spec tool:
// deck_id and patch, plus how the result is stored. Every spec tool takes the
// same five, so the listing spells them out once — on render_deck_spec, the
// revise fast path — and the others point there: the default tools/list is
// budgeted (TestDeckSpecToolProfileBudget).
func deckHandleToolParams(tool string) []mcp.ToolOption {
	deckID, patch := deckIDParamDescription, deckPatchParamDescription
	dryRun := "true: run without storing anything."
	fork := "true: store the result under a NEW deck_id; this one stays as it is."
	restore := `Revision of deck_id to start from (validate_deck_spec read:"history" lists them); patch applies on top and the result is a new revision.`
	if tool != "render_deck_spec" {
		const see = "; see render_deck_spec."
		deckID = "Stored spec handle, sent INSTEAD of spec" + see
		patch = "Edits to deck_id's spec, applied before the call acts" + see
		if tool == "validate_deck_spec" {
			patch = "Edits to deck_id's spec, applied before the call acts and stored when the result parses" + see
		}
		dryRun, fork, restore = "Store nothing.", "Store under a new deck_id.", "Revision to start from."
	}
	return []mcp.ToolOption{
		mcp.WithString("deck_id", mcp.Description(deckID)),
		mcp.WithArray("patch", mcp.Description(patch),
			mcp.Items(map[string]any{
				"type": "object", "required": []string{"op", "path"}, "additionalProperties": false,
				"properties": map[string]any{
					"op":    map[string]any{"type": "string", "enum": []string{specPatchReplace, specPatchAdd, specPatchRemove, specPatchMove, specPatchCopy}},
					"path":  map[string]any{"type": "string"},
					"from":  map[string]any{"type": "string"},
					"value": map[string]any{},
				},
			}),
		),
		mcp.WithBoolean("dry_run", mcp.Description(dryRun)),
		mcp.WithBoolean("fork", mcp.Description(fork)),
		mcp.WithNumber("restore", mcp.Description(restore)),
	}
}

// withToolOptions appends option groups to a tool definition's own options.
func withToolOptions(base []mcp.ToolOption, groups ...[]mcp.ToolOption) []mcp.ToolOption {
	for _, g := range groups {
		base = append(base, g...)
	}
	return base
}
