package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/pipeline"
)

// Slide ids on the render tools (go-slide-creator-1w3uo).
//
// A DeckSpec slide has a stable id, and patches, findings and
// render_deck_spec's slides[] all speak it — but the image tools took only a
// 0-based index, so "show me the risk slide again" still meant looking its
// index up after every insert. render_deck_thumbnails' slide_indices and
// render_slide_image's slide_id (CLI: --slides / --slide-id) now take the id.
//
// The image tools see a .pptx, not a deck_id, so the ids come from what is
// known about that exact file: the table of contents render_deck_spec
// recorded when it wrote it, or the authoring sidecar `semantic render` wrote
// beside it. Both are bound to the file's sha256: a deck overwritten by
// something else has no ids, rather than another deck's.

// renderedSlideTOCs remembers, per artifact sha256, the slides of the DeckSpec
// render that wrote the artifact. Same lifetime as the deterministic gate
// records: the render cache drops an artifact after 24h unused.
var renderedSlideTOCs = struct {
	mu      sync.Mutex
	entries map[string]renderedSlideTOC
	now     func() time.Time
}{entries: make(map[string]renderedSlideTOC), now: time.Now}

type renderedSlideTOC struct {
	refs      []slideRef
	expiresAt time.Time
}

const (
	maxRenderedSlideTOCs = 1024
	renderedSlideTOCTTL  = 24 * time.Hour
)

// recordRenderedSlides remembers the slides of the deck just written to
// pptxPath. A table of contents without a single id is not worth keeping.
func recordRenderedSlides(pptxPath string, refs []slideRef) {
	hasID := false
	for _, r := range refs {
		hasID = hasID || r.ID != ""
	}
	if pptxPath == "" || !hasID {
		return
	}
	artifact, err := describeArtifact(pptxPath, "pptx")
	if err != nil {
		return
	}
	t := &renderedSlideTOCs
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	if len(t.entries) >= maxRenderedSlideTOCs {
		for k, e := range t.entries {
			if now.After(e.expiresAt) {
				delete(t.entries, k)
			}
		}
		// Still full: the ids are a convenience, so start over rather than
		// track which record is the least useful.
		if len(t.entries) >= maxRenderedSlideTOCs {
			t.entries = make(map[string]renderedSlideTOC)
		}
	}
	t.entries[artifact.SHA256] = renderedSlideTOC{refs: append([]slideRef(nil), refs...), expiresAt: now.Add(renderedSlideTOCTTL)}
}

// slideRefsForPptx returns the slides of the deck at pptxPath with their ids,
// or nil when nothing is known about this exact file.
func slideRefsForPptx(pptxPath string) []slideRef {
	artifact, err := describeArtifact(pptxPath, "pptx")
	if err != nil {
		return nil
	}
	t := &renderedSlideTOCs
	t.mu.Lock()
	entry, ok := t.entries[artifact.SHA256]
	if ok && t.now().After(entry.expiresAt) {
		delete(t.entries, artifact.SHA256)
		ok = false
	}
	t.mu.Unlock()
	if ok {
		return entry.refs
	}
	manifest, _ := pipeline.ReadAuthoringManifest(pptxPath + authoringManifestSuffix)
	if manifest == nil || manifest.PPTXSHA256 != artifact.SHA256 {
		return nil
	}
	return specDeckState("deck."+manifest.SourceFormat, []byte(manifest.Source), "").refs()
}

// slideIDIndex resolves one slide id to its 0-based index in the deck at
// pptxPath. The error says which ids the deck has, or how to get them.
func slideIDIndex(pptxPath, id string) (int, error) {
	refs := slideRefsForPptx(pptxPath)
	var known []string
	for _, r := range refs {
		if r.ID == id {
			return r.Index, nil
		}
		if r.ID != "" {
			known = append(known, r.ID)
		}
	}
	if len(known) == 0 {
		return 0, fmt.Errorf("slide id %q cannot be resolved: no slide ids are known for this file — ids come from the render_deck_spec call (or the `semantic render` sidecar) that wrote it; use a 0-based index instead", id)
	}
	return 0, fmt.Errorf("no slide has id %q (ids: %s)", id, strings.Join(known, ", "))
}

// resolveSlideIDSelectors rewrites the string entries of a slide_indices value
// as the indices of the slides with those ids, leaving numbers as they are.
// changed is false when the value holds no id.
func resolveSlideIDSelectors(pptxPath string, raw any) (resolved []any, changed bool, err error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, false, nil
	}
	out := make([]any, len(items))
	for i, item := range items {
		id, isID := item.(string)
		if !isID {
			out[i] = item
			continue
		}
		idx, err := slideIDIndex(pptxPath, id)
		if err != nil {
			return nil, false, err
		}
		out[i], changed = float64(idx), true
	}
	return out, changed, nil
}

// withResolvedSlideIDs returns request with the slide ids in slide_indices
// replaced by indices, so the index parser every tool shares reads it.
func withResolvedSlideIDs(tool string, request mcp.CallToolRequest, pptxPath string) (mcp.CallToolRequest, *mcp.CallToolResult) {
	args := request.GetArguments()
	resolved, changed, err := resolveSlideIDSelectors(pptxPath, args["slide_indices"])
	if err != nil {
		return request, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "slide_indices", err.Error(), "array", []int{4, 9}, nil)
	}
	if !changed {
		return request, nil
	}
	next := make(map[string]any, len(args))
	for k, v := range args {
		next[k] = v
	}
	next["slide_indices"] = resolved
	request.Params.Arguments = next
	return request, nil
}

// stampSlideIDs writes each rendered slide's id beside its index when the
// deck's ids are known.
func stampSlideIDs(pptxPath string, slides []renderedSlideMeta) {
	if pptxPath == "" || len(slides) == 0 {
		return
	}
	refs := slideRefsForPptx(pptxPath)
	for i := range slides {
		if idx := slides[i].Index; idx >= 0 && idx < len(refs) {
			slides[i].ID = refs[idx].ID
		}
	}
}
