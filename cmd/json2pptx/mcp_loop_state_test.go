package main

import (
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// --- session store ---

// TestLoopSessionStore_SaveLoadRoundTrip asserts a saved checkpoint is loadable
// by its minted token and that the token is non-empty.
func TestLoopSessionStore_SaveLoadRoundTrip(t *testing.T) {
	s := newLoopSessionStore(time.Hour)
	cp := &loopCheckpoint{Tool: "auto_repair", NextPass: 2}

	token := s.Save(cp)
	if token == "" {
		t.Fatal("Save returned an empty token")
	}

	got, ok := s.Load(token)
	if !ok {
		t.Fatalf("Load(%q) returned ok=false for a just-saved checkpoint", token)
	}
	if got != cp {
		t.Errorf("Load returned a different checkpoint pointer than was saved")
	}
}

// TestLoopSessionStore_LoadUnknownToken asserts unknown tokens miss cleanly.
func TestLoopSessionStore_LoadUnknownToken(t *testing.T) {
	s := newLoopSessionStore(time.Hour)
	if _, ok := s.Load("rs_does-not-exist"); ok {
		t.Error("Load of an unknown token returned ok=true")
	}
	if _, ok := s.Load(""); ok {
		t.Error("Load of an empty token returned ok=true")
	}
}

// TestLoopSessionStore_Expiry asserts an entry past its TTL is treated as a miss
// and evicted.
func TestLoopSessionStore_Expiry(t *testing.T) {
	s := newLoopSessionStore(time.Hour)
	now := time.Unix(1000, 0)
	s.now = func() time.Time { return now }

	token := s.Save(&loopCheckpoint{Tool: "make_deck"})
	if _, ok := s.Load(token); !ok {
		t.Fatal("entry should be live immediately after Save")
	}

	now = now.Add(2 * time.Hour) // past the 1h TTL
	if _, ok := s.Load(token); ok {
		t.Error("expired entry should miss")
	}
	// A second Load after eviction must also miss (entry deleted).
	if _, ok := s.Load(token); ok {
		t.Error("evicted entry should stay missing")
	}
}

// TestLoopSessionStore_NilReceiver asserts a nil store degrades to no-op so
// tests and code paths that never wire it don't crash.
func TestLoopSessionStore_NilReceiver(t *testing.T) {
	var s *loopSessionStore
	if token := s.Save(&loopCheckpoint{Tool: "auto_repair"}); token != "" {
		t.Errorf("nil-store Save should return an empty token, got %q", token)
	}
	if _, ok := s.Load("rs_x"); ok {
		t.Error("nil-store Load should return ok=false")
	}
}

// TestNewResumeToken_Unique asserts minted tokens are unique and carry the
// expected prefix.
func TestNewResumeToken_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 256; i++ {
		tok := newResumeToken()
		if tok == "" {
			t.Fatal("newResumeToken returned empty")
		}
		if len(tok) < 8 || tok[:3] != "rs_" {
			t.Errorf("token %q lacks the rs_ prefix", tok)
		}
		if seen[tok] {
			t.Fatalf("duplicate token minted: %q", tok)
		}
		seen[tok] = true
	}
}

// --- completion classification ---

// TestDeriveLoopCompletion covers every branch of the terminal-state classifier
// so a partial/degraded result is never mislabeled as converged.
func TestDeriveLoopCompletion(t *testing.T) {
	cases := []struct {
		name                                     string
		gatePassed, evidenceComplete, renderDone bool
		stalled                                  bool
		want                                     string
	}{
		{"clean converge", true, true, true, false, loopCompletionConverged},
		{"degraded converge", true, false, true, false, loopCompletionConvergedDegraded},
		{"degraded converge ignores render", true, false, false, false, loopCompletionConvergedDegraded},
		{"render incomplete", false, false, false, false, loopCompletionRenderIncomplete},
		{"stalled", false, false, true, true, loopCompletionNoProgress},
		{"budget exhausted", false, false, true, false, loopCompletionExhausted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveLoopCompletion(tc.gatePassed, tc.evidenceComplete, tc.renderDone, tc.stalled)
			if got != tc.want {
				t.Errorf("deriveLoopCompletion(%v,%v,%v,%v) = %q, want %q",
					tc.gatePassed, tc.evidenceComplete, tc.renderDone, tc.stalled, got, tc.want)
			}
		})
	}
}

// TestNextActionForCompletion asserts every completion status maps to a
// non-empty instruction (agents rely on it always being populated).
func TestNextActionForCompletion(t *testing.T) {
	for _, c := range []string{
		loopCompletionConverged, loopCompletionConvergedDegraded,
		loopCompletionExhausted, loopCompletionNoProgress, loopCompletionRenderIncomplete,
		"unknown-status",
	} {
		if got := nextActionForCompletion(c); got == "" {
			t.Errorf("nextActionForCompletion(%q) returned empty", c)
		}
	}
}

// TestSummarizeRemainingFindings_Caps asserts the projection caps at
// maxNextStateFindings and copies the salient fields.
func TestSummarizeRemainingFindings_Caps(t *testing.T) {
	if got := summarizeRemainingFindings(nil); got != nil {
		t.Errorf("nil findings should summarize to nil, got %v", got)
	}

	many := make([]patterns.FitFinding, maxNextStateFindings+10)
	for i := range many {
		many[i].Code = "BODY_TOO_LONG"
		many[i].Action = "review"
	}
	got := summarizeRemainingFindings(many)
	if len(got) != maxNextStateFindings {
		t.Errorf("expected the summary to cap at %d, got %d", maxNextStateFindings, len(got))
	}

	one := []patterns.FitFinding{{
		ValidationError: patterns.ValidationError{
			Code:    "CHART_PLACEHOLDER_EMPTY",
			Path:    "slides[0]",
			Message: "boom",
		},
		Action: "refuse",
	}}
	s := summarizeRemainingFindings(one)
	if len(s) != 1 || s[0].Code != "CHART_PLACEHOLDER_EMPTY" || s[0].Path != "slides[0]" || s[0].Message != "boom" || s[0].Action != "refuse" {
		t.Errorf("finding fields not copied through: %+v", s)
	}
}
