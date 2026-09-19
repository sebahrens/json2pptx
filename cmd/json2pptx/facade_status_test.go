package main

import (
	"strings"
	"testing"
)

// publishable was false for every deck ever run through the default path — the
// mode renders no pixels, so the visual-verdict reason is always present. A flag
// that is constant false decides nothing, so the deterministic half is reported
// on its own (go-slide-creator-z0cx).
func TestDeterministicReadyIsReachableWithoutAVisualVerdict(t *testing.T) {
	status := deriveFacadeStatus(true, true, true, false, nil, contentProvenanceAuthorSupplied)

	if !status.DeterministicReady {
		t.Errorf("a deck that passes every deterministic check is not deterministic_ready: %v", status.DeterministicBlockingReasons)
	}
	if len(status.DeterministicBlockingReasons) != 0 {
		t.Errorf("deterministic_blocking_reasons = %v, want empty", status.DeterministicBlockingReasons)
	}
	if status.Publishable {
		t.Error("publishable must stay false without a visual verdict")
	}
	if len(status.BlockingReasons) != 1 || !strings.Contains(status.BlockingReasons[0], "vision") {
		t.Errorf("blocking_reasons = %v, want the vision reason alone", status.BlockingReasons)
	}
	if !status.ManualReviewRequired {
		t.Error("manual_review_required must stay the inverse of publishable")
	}
}

// With the verdict supplied, both flags are true and nothing blocks.
func TestVisualVerdictCompletesPublishable(t *testing.T) {
	status := deriveFacadeStatus(true, true, true, true, nil, contentProvenanceAuthorSupplied)
	if !status.DeterministicReady || !status.Publishable {
		t.Errorf("deterministic_ready=%v publishable=%v, want both true: %v",
			status.DeterministicReady, status.Publishable, status.BlockingReasons)
	}
	if len(status.BlockingReasons) != 0 {
		t.Errorf("blocking_reasons = %v, want empty", status.BlockingReasons)
	}
}

// Anything an agent can fix by editing blocks BOTH flags, and appears in both
// lists — the split is only about the visual verdict.
func TestDeterministicBlockersBlockBothFlags(t *testing.T) {
	cases := []struct {
		name     string
		status   facadeStatus
		fragment string
	}{
		{
			name:     "gate failed",
			status:   deriveFacadeStatus(false, true, true, true, []string{"min_score 82 < 85"}, contentProvenanceAuthorSupplied),
			fragment: "min_score",
		},
		{
			name:     "evidence incomplete",
			status:   deriveFacadeStatus(true, false, true, true, nil, contentProvenanceAuthorSupplied),
			fragment: "evidence incomplete",
		},
		{
			name:     "exemplar content",
			status:   deriveFacadeStatus(true, true, true, true, nil, contentProvenanceExemplarSkeleton),
			fragment: "exemplar skeleton",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.status.DeterministicReady {
				t.Error("a deterministic blocker must clear deterministic_ready")
			}
			if c.status.Publishable {
				t.Error("a deterministic blocker must clear publishable")
			}
			if !containsFragment(c.status.DeterministicBlockingReasons, c.fragment) {
				t.Errorf("deterministic_blocking_reasons = %v, want one mentioning %q", c.status.DeterministicBlockingReasons, c.fragment)
			}
			if !containsFragment(c.status.BlockingReasons, c.fragment) {
				t.Errorf("blocking_reasons = %v, want one mentioning %q", c.status.BlockingReasons, c.fragment)
			}
		})
	}
}

// The vision reason says what supplies the verdict, so an agent reading it knows
// the next call rather than only what is missing.
func TestVisionBlockingReasonNamesItsRemedy(t *testing.T) {
	status := deriveFacadeStatus(true, true, true, false, nil, contentProvenanceAuthorSupplied)
	if len(status.BlockingReasons) == 0 {
		t.Fatal("expected the vision reason")
	}
	for _, want := range []string{"render_deck_thumbnails", "submit_visual_review"} {
		if !strings.Contains(status.BlockingReasons[0], want) {
			t.Errorf("vision reason %q does not name %s", status.BlockingReasons[0], want)
		}
	}
}

func containsFragment(list []string, fragment string) bool {
	for _, v := range list {
		if strings.Contains(v, fragment) {
			return true
		}
	}
	return false
}
