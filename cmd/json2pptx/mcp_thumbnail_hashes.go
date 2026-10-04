package main

import (
	"sort"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/render"
)

// Thumbnail hashes the server has already handed out (go-slide-creator-wfhvv).
//
// render_deck_thumbnails takes known_hashes so a repeat pass does not resend
// the images of slides whose pixels did not change, but nothing told an agent
// to send them: the step after a patch render named the tool and the changed
// slides, and the full-deck pass over the final revision resent every image.
// The server knows which images it delivered for a file, so the thumbnails
// step after a re-render carries those hashes itself.
//
// A content_hash is a pixel hash, so it is only comparable at the density it
// was rendered at: the record keeps the density with the hashes.

// deliveredThumbnails remembers, per PPTX path, the content_hash of each slide
// image a render_deck_thumbnails call returned (as an image block, or as an
// unchanged entry the caller already held). Same lifetime and bound as the
// slide-id records.
var deliveredThumbnails = struct {
	mu      sync.Mutex
	entries map[string]*deliveredThumbnailRecord
	now     func() time.Time
}{entries: make(map[string]*deliveredThumbnailRecord), now: time.Now}

type deliveredThumbnailRecord struct {
	// byDensity maps a render density to slide index → content_hash.
	byDensity map[int]map[int]string
	expiresAt time.Time
}

// maxCarriedKnownHashes bounds the hashes one next_tool_call carries (about
// 70 bytes each). It is render_deck_thumbnails' own default slide cap; a
// larger deck's repeat pass falls back to naming the changed slides.
const maxCarriedKnownHashes = 50

// defaultThumbnailDensity is render_deck_thumbnails' default DPI.
const defaultThumbnailDensity = 50

// recordDeckThumbnails notes every slide of a render_deck_thumbnails response
// as one the caller now holds: the thumbnails step after the next render
// names these hashes.
func recordDeckThumbnails(request mcp.CallToolRequest, deck *render.DeckResult) {
	delivered := make(map[int]string, len(deck.Slides))
	for _, s := range deck.Slides {
		delivered[s.Index] = s.ContentHash
	}
	recordDeliveredThumbnails(request.GetString("pptx_path", ""), clampedRenderDensity(request.GetArguments(), defaultThumbnailDensity, 25, 150), delivered)
}

// recordDeliveredThumbnails notes the slide hashes a thumbnails call returned
// for pptxPath at density.
func recordDeliveredThumbnails(pptxPath string, density int, hashes map[int]string) {
	if pptxPath == "" || len(hashes) == 0 {
		return
	}
	t := &deliveredThumbnails
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	rec := t.entries[pptxPath]
	if rec == nil || now.After(rec.expiresAt) {
		if len(t.entries) >= maxRenderedSlideTOCs {
			for k, e := range t.entries {
				if now.After(e.expiresAt) {
					delete(t.entries, k)
				}
			}
			if len(t.entries) >= maxRenderedSlideTOCs {
				t.entries = make(map[string]*deliveredThumbnailRecord)
			}
		}
		rec = &deliveredThumbnailRecord{byDensity: map[int]map[int]string{}}
		t.entries[pptxPath] = rec
	}
	rec.expiresAt = now.Add(renderedSlideTOCTTL)
	slides := rec.byDensity[density]
	if slides == nil {
		slides = map[int]string{}
		rec.byDensity[density] = slides
	}
	for idx, h := range hashes {
		if h != "" {
			slides[idx] = h
		}
	}
}

// heldThumbnailHashes returns the hashes delivered for the first of paths that
// has any, in slide order without repeats, and the density they were rendered
// at: the density that covers the most slides (a one-slide zoom at 100 DPI
// does not displace the full pass at 50), the lower one on a tie. Nil when
// nothing was delivered for any path.
func heldThumbnailHashes(paths ...string) (hashes []string, density int) {
	t := &deliveredThumbnails
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, p := range paths {
		rec := t.entries[p]
		if p == "" || rec == nil || t.now().After(rec.expiresAt) {
			continue
		}
		best := -1
		for d, slides := range rec.byDensity {
			if best < 0 || len(slides) > len(rec.byDensity[best]) || (len(slides) == len(rec.byDensity[best]) && d < best) {
				best = d
			}
		}
		if best < 0 {
			continue
		}
		slides := rec.byDensity[best]
		indices := make([]int, 0, len(slides))
		for idx := range slides {
			indices = append(indices, idx)
		}
		sort.Ints(indices)
		seen := make(map[string]bool, len(indices))
		for _, idx := range indices {
			if h := slides[idx]; !seen[h] {
				seen[h] = true
				hashes = append(hashes, h)
			}
		}
		return hashes, best
	}
	return nil, 0
}
