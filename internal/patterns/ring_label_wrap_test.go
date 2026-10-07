package patterns

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// No one-word label wraps (go-slide-creator-y21pc): a word broken across two
// lines is not a wrapped label, it is a damaged one. Each ring pattern gives
// the widest one-word label its natural width before the ring keeps its
// size: the ring gives way, the label does not.
// ---------------------------------------------------------------------------

// ringWrapBodies are the content areas the rule is checked in: the three of
// ringGapBodies and the narrower areas a split layout or a compose segment
// hands a ring.
var ringWrapBodies = []struct {
	name string
	w, h float64
}{
	{"abstract", 687, 294},
	{"midnight-blue", 828, 349},
	{"p-style", 899, 360},
	{"two-thirds", 600, 349},
	{"sixty", 540, 349},
	{"half", 440, 349},
	{"narrow half", 340, 294},
}

// ringWrapWords are one-word labels from the shortest to the longest a deck
// plausibly carries; item i takes word i modulo their number, so the long
// words land on both sides of every ring.
var ringWrapWords = []string{"Plan", "Standardisation", "Check", "Implementation", "Institutionalisation", "Act", "Transformation", "Governance", "Rationalisation", "Communication"}

func ringWrapLabels(n, offset int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = ringWrapWords[(i+offset)%len(ringWrapWords)]
	}
	return out
}

// ringWrapCheck fails for every text cell of the resolved grid whose first
// paragraph is one of labels and is set in a cell narrower than the word.
func ringWrapCheck(t *testing.T, name string, res *shapegrid.ResolveResult, labels []string) {
	t.Helper()
	want := map[string]bool{}
	for _, l := range labels {
		want[l] = true
	}
	seen := map[string]bool{}
	for _, c := range res.Cells {
		if c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
			continue
		}
		var obj struct {
			Paragraphs []struct {
				Content string  `json:"content"`
				Size    float64 `json:"size"`
				Bold    bool    `json:"bold"`
			} `json:"paragraphs"`
			InsetLeft  *float64 `json:"inset_left"`
			InsetRight *float64 `json:"inset_right"`
		}
		if err := json.Unmarshal(c.ShapeSpec.Text, &obj); err != nil || len(obj.Paragraphs) == 0 || !want[obj.Paragraphs[0].Content] {
			continue
		}
		p := obj.Paragraphs[0]
		seen[p.Content] = true
		left, right := pptx.ShapeTextInsetPt, pptx.ShapeTextInsetPt
		if obj.InsetLeft != nil {
			left = *obj.InsetLeft
		}
		if obj.InsetRight != nil {
			right = *obj.InsetRight
		}
		inner := float64(c.Bounds.CX)/12700 - left - right
		if lines := measuredLines(p.Content, "", p.Bold, p.Size, inner); lines > 1 {
			t.Errorf("%s: %q at %vpt breaks onto %d lines in a %.0fpt label cell (%.0fpt of text width)", name, p.Content, p.Size, lines, float64(c.Bounds.CX)/12700, inner)
		}
	}
	for _, l := range labels {
		if !seen[l] {
			t.Errorf("%s: label %q is not drawn in a text cell of its own", name, l)
		}
	}
}

func TestRingOneWordLabelsNeverWrap(t *testing.T) {
	drawn := 0
	for _, body := range ringWrapBodies {
		for _, described := range []bool{true, false} {
			desc := ""
			if described {
				desc = "Compare results against the baseline"
			}
			for _, n := range []int{3, 4, 6, 8} {
				for offset := 0; offset < 2; offset++ {
					labels := ringWrapLabels(n, offset*3)
					name := fmt.Sprintf("%s/%d/described=%t/%d", body.name, n, described, offset)

					ring := &CycleRingValues{Center: &CycleRingCenter{Label: "Operating model"}}
					nodes := &CycleNodesValues{}
					hub := &RadialHubValues{Center: RadialHubCenter{Label: "Platform"}}
					eight := &CycleFigureEightValues{LeftLabel: "Build", RightLabel: "Run"}
					for _, l := range labels {
						ring.Phases = append(ring.Phases, CycleRingPhase{Label: l, Description: desc})
						nodes.Steps = append(nodes.Steps, CycleNodesStep{Label: l, Description: desc})
						hub.Spokes = append(hub.Spokes, RadialHubSpoke{Label: l, Description: desc})
						eight.Phases = append(eight.Phases, CycleRingPhase{Label: l, Description: desc})
					}
					ctx := cycleRingCtx(body.w, body.h)
					cases := []struct {
						pattern Pattern
						values  any
						ok      bool
					}{
						{&cycleRing{}, ring, true},
						{&cycleNodes{}, nodes, true},
						{&radialHub{}, hub, true},
						{&cycleFigureEight{}, eight, n%2 == 0 && n >= 4},
					}
					for _, tc := range cases {
						if !tc.ok {
							continue
						}
						if err := tc.pattern.Validate(tc.values, nil, nil); err != nil {
							continue // a count or a label this pattern does not take
						}
						grid, err := tc.pattern.Expand(ctx, tc.values, nil, nil)
						if err != nil {
							continue // an area the pattern refuses outright
						}
						drawn++
						ringWrapCheck(t, tc.pattern.Name()+" "+name, ringWrapResolve(t, grid, body.w, body.h), labels)
					}
				}
			}
		}
	}
	if drawn < ringWrapMinDrawn {
		t.Errorf("only %d layouts were drawn; the sweep is not testing the family", drawn)
	}
}

// ringWrapMinDrawn is the least number of layouts the sweep must have drawn.
const ringWrapMinDrawn = 250

// ringWrapResolve resolves a ring pattern's grid in a w × h area.
func ringWrapResolve(t *testing.T, grid *jsonschema.ShapeGridInput, w, h float64) *shapegrid.ResolveResult {
	t.Helper()
	return cycleNodesResolveAt(t, grid, w, h)
}
