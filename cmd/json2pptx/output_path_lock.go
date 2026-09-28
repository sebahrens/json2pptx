package main

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

// Output-path serialization and naming (go-slide-creator-tngh).
//
// Every render used to write <output_dir>/output.pptx unless the deck itself
// named a file, so two render_deck_spec calls in one session silently destroyed
// each other's work: both returned success with different content_hash values
// and the same pptx_path, and the file on disk held only one of them. That is
// worse than losing a file — submit_visual_review requires pptx_revision to
// equal the artifact's sha256 and render_deck_thumbnails keys its cache on file
// content, so an agent whose hash matches nothing on disk cannot complete at
// all, and has no way to see why.
//
// Two defences. Distinct default names, so independent renders do not aim at
// the same path in the first place; and a lock per target path, so when two
// renders DO aim at the same file (the caller asked for it) the write and the
// hash-read that follows cannot interleave with another render's write.

// outputPathLocks holds one reference-counted mutex per absolute output path
// that some goroutine currently holds or waits for. An entry is removed when
// its last holder unlocks, so the map is bounded by the number of in-flight
// renders rather than growing with every distinct output path a long-lived
// MCP server ever wrote (go-slide-creator-tcxsq). Removing only at refcount
// zero is what keeps a waiter from being handed a different mutex than the
// holder's.
var outputPathLocks = struct {
	mu    sync.Mutex
	locks map[string]*outputPathLock
}{locks: make(map[string]*outputPathLock)}

type outputPathLock struct {
	mu   sync.Mutex
	refs int // holders plus waiters; guarded by outputPathLocks.mu
}

// lockOutputPath serializes writes to one output path, returning the unlock
// function. Paths are compared after cleaning and resolving to absolute form so
// two spellings of the same file share a lock. The unlock function must be
// called exactly once.
func lockOutputPath(path string) func() {
	key := path
	if abs, err := filepath.Abs(path); err == nil {
		key = filepath.Clean(abs)
	}
	outputPathLocks.mu.Lock()
	l := outputPathLocks.locks[key]
	if l == nil {
		l = &outputPathLock{}
		outputPathLocks.locks[key] = l
	}
	l.refs++
	outputPathLocks.mu.Unlock()

	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		outputPathLocks.mu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(outputPathLocks.locks, key)
		}
		outputPathLocks.mu.Unlock()
	}
}

// outputPathLockCount reports how many paths currently have a lock entry.
func outputPathLockCount() int {
	outputPathLocks.mu.Lock()
	defer outputPathLocks.mu.Unlock()
	return len(outputPathLocks.locks)
}

// deckSpecOutputFilename derives a collision-free default filename for a
// DeckSpec render: a slug of the deck title plus a short digest of the spec, so
// the same spec rendered twice lands on the same file (re-rendering after an
// edit-free retry is idempotent) while two different specs never do.
func deckSpecOutputFilename(title string, specBytes []byte) string {
	slug := slugForFilename(title)
	if slug == "" {
		slug = "deck"
	}
	sum := sha256.Sum256(specBytes)
	return slug + "-" + hex.EncodeToString(sum[:])[:8] + ".pptx"
}

// slugForFilename reduces a title to lowercase ASCII words joined by hyphens,
// capped at slugMaxLen runes so a long title cannot produce an unwieldy path.
func slugForFilename(title string) string {
	const slugMaxLen = 48
	var b strings.Builder
	lastHyphen := true // suppress a leading hyphen
	for _, r := range strings.ToLower(title) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen && b.Len() < slugMaxLen:
			b.WriteByte('-')
			lastHyphen = true
		}
		if b.Len() >= slugMaxLen {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}
