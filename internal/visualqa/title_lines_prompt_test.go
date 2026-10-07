package visualqa

import (
	"strings"
	"testing"
)

// go-slide-creator-s49nz: the vision prompt asks for touching title lines by
// name, in the general quality targets and again on the two display types.
// A raster check was tried and dropped: at the 60-80dpi the review renders
// use, the clear row between two correctly set lines is one anti-aliased
// pixel, so it flagged good titles.
func TestPromptsAskForTouchingTitleLines(t *testing.T) {
	for _, slideType := range []string{"title", "section"} {
		if p := PromptForSlideType(slideType); !strings.Contains(p, "lines must not touch") {
			t.Errorf("%s prompt does not ask for touching title lines:\n%s", slideType, p)
		}
	}
	if !strings.Contains(systemPrompt, "descenders touching or crossing the line below is overlap (P1)") {
		t.Error("the general quality targets do not name touching title lines")
	}
}
