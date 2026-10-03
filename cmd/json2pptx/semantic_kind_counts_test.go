package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

// kindCountRanges are the item counts each kind documents as rendering its
// visual (docs/SEMANTIC_COMPILER.md, "Slide kind → compiled visual").
var kindCountRanges = []struct {
	kind     semantic.SlideKind
	list     string
	min, max int
}{
	{semantic.KindExecutiveSummary, "points", 3, 5},
	{semantic.KindKPISnapshot, "kpis", 2, 6},
	{semantic.KindChartInsight, "insights", 1, 6},
	{semantic.KindTimeline, "milestones", 3, 7},
	{semantic.KindDecision, "options", 3, 6},
	{semantic.KindNextSteps, "actions", 2, 6},
	{semantic.KindPillars, "pillars", 3, 5},
	{semantic.KindProcess, "steps", 3, 6},
	{semantic.KindRoadmap, "phases", 3, 6},
	{semantic.KindOptionMatrix, "options", 2, 6},
	{semantic.KindArchitecture, "tiers", 3, 6},
	{semantic.KindTeam, "members", 1, 8},
}

// entryNameKeys are the keys that name a list entry, in the order tried.
var entryNameKeys = []string{"label", "name", "title", "action", "lead", "value"}

// entryDetailKeys are the optional second lines of a list entry.
var entryDetailKeys = []string{"detail", "description", "body", "bio", "support", "items"}

// kindAtCount returns the kind's copy-ready example with its list resized to n
// entries: the example's own entries, repeated with a numbered name.
func kindAtCount(t *testing.T, kind semantic.SlideKind, list string, n int) map[string]any {
	t.Helper()
	slide := semantic.KindExample(kind)
	entries, ok := slide[list].([]any)
	if !ok || len(entries) == 0 {
		t.Fatalf("%s: the example has no %s list", kind, list)
	}
	out := make([]any, 0, n)
	for i := 0; i < n; i++ {
		entry := deepCopyJSON(entries[i%len(entries)])
		if m, isObject := entry.(map[string]any); isObject && n > len(entries) {
			// The largest count is documented for items that carry a name: the
			// optional second line of each has its own, tighter count.
			for _, key := range entryDetailKeys {
				delete(m, key)
			}
		}
		if m, isObject := entry.(map[string]any); isObject && i >= len(entries) {
			// A repeated entry is another item, never a second recommendation.
			delete(m, "recommended")
			delete(m, "active")
			delete(m, "milestone")
			for _, key := range entryNameKeys {
				if s, isText := m[key].(string); isText {
					m[key] = fmt.Sprintf("%s %d", s, i+1)
					break
				}
			}
		}
		out = append(out, entry)
	}
	slide[list] = out
	return slide
}

// knownCountRefusals are the documented counts a layout does not hold yet,
// by "kind count" and template ("*" for every template). Each is a slide the
// kind's schema promises and the pattern refuses with its leanest copy; they
// are listed so the test fails on a new one, and each is to be removed when
// its pattern learns to fit (go-slide-creator-vg73u, open).
var knownCountRefusals = map[string][]string{
	// Six numbered rows under a header, plus the decisions band, need 449pt.
	"next_steps 6": {"*"},
	// Six option rows under the criteria header, legend and takeaway need 354pt.
	"option_matrix 6": {"*"},
	// Five points and the bottom line on the two shortest content areas.
	"executive_summary 5": {"modern", "modern-template"},
	// The house's roof, pillars and foundation on the same two.
	"pillars 3": {"modern", "modern-template"},
	"pillars 5": {"modern"},
}

func knownCountRefusal(kind semantic.SlideKind, n int, template string) bool {
	for _, tpl := range knownCountRefusals[fmt.Sprintf("%s %d", kind, n)] {
		if tpl == "*" || tpl == template {
			return true
		}
	}
	return false
}

// go-slide-creator-vg73u: every kind renders at the smallest and the largest
// item count it documents, with its example's copy, on every shipped template
// — or the refusal is one of knownCountRefusals. The decision kind documents
// 3-6 options and refused five of them on modern-template because the layout's
// own "RECOMMENDED" badge shrank below the readable floor, and the finding
// told the author to shorten that badge: no refusal may ask an author to
// shorten text the layout wrote.
func TestEveryKindRendersAtItsDocumentedCounts(t *testing.T) {
	templates := []string{"modern-template"}
	if !testing.Short() {
		templates = shippedTemplateNames(t)
	}
	mc := refusalTestConfig(t)
	for _, tpl := range templates {
		for _, c := range kindCountRanges {
			for _, n := range []int{c.min, c.max} {
				spec := map[string]any{
					"meta": map[string]any{"title": "Count probe", "source": "Illustrative"},
					"slides": []any{
						map[string]any{"kind": "title", "title": "Counts every kind documents", "subtitle": "October 2026"},
						kindAtCount(t, c.kind, c.list, n),
					},
				}
				env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": tpl}))
				refused := false
				for _, f := range env.Findings {
					if f.Blocking == nil || !*f.Blocking || f.Path == nil || !strings.HasPrefix(*f.Path, "/slides/1") {
						continue
					}
					refused = true
					if !knownCountRefusal(c.kind, n, tpl) {
						t.Errorf("%s: %s with %d %s is refused: %s %s: %s", tpl, c.kind, n, c.list, f.Code, *f.Path, f.Message)
					}
					texts := []string{f.Message}
					symptoms, _ := f.Evidence[symptomsDetail].([]any)
					for _, s := range symptoms {
						if m, ok := s.(map[string]any); ok {
							msg, _ := m["message"].(string)
							texts = append(texts, msg)
						}
					}
					for _, text := range texts {
						if strings.Contains(text, "shorten the text") {
							t.Errorf("%s: %s with %d %s: the refusal asks to shorten text without saying whose: %s", tpl, c.kind, n, c.list, text)
						}
					}
				}
				if !refused && knownCountRefusal(c.kind, n, tpl) {
					t.Logf("%s: %s with %d %s now renders: remove it from knownCountRefusals", tpl, c.kind, n, c.list)
				}
			}
		}
	}
}
