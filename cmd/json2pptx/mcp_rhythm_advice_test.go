package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/rhythm"
)

// rhythmAdviceSpec is a cover plus body slides that alternate two kinds, so
// nothing but the deck's length is at issue. chrome is the meta.chrome object.
func rhythmAdviceSpec(body int, chrome string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `{"meta": {"title": "Atlas data platform", "template": "midnight-blue", "chrome": %s}, "slides": [`, chrome)
	b.WriteString(`{"kind": "title", "title": "Atlas data platform", "subtitle": "Steering committee"}`)
	for i := 0; i < body; i++ {
		if i%2 == 0 {
			fmt.Fprintf(&b, `,{"kind": "kpi_snapshot", "title": "Load time fell again in month %d", "takeaway": "The trend holds", "kpis": [{"value": "9.5h", "label": "Nightly load"}, {"value": "61", "label": "Missed SLAs"}]}`, i+1)
		} else {
			fmt.Fprintf(&b, `,{"kind": "executive_summary", "title": "Three findings from workstream %d", "takeaway": "Proceed", "points": ["Load misses its window", "Quality is uneven", "Cost is rising"]}`, i+1)
		}
	}
	b.WriteString("]}")
	return b.String()
}

func envelopeFindingCount(env deckSpecEnvelopeResponse, code string) int {
	n := 0
	for _, f := range env.Findings {
		if strings.HasSuffix(f.Code, code) {
			n++
		}
	}
	return n
}

// go-slide-creator-th6o9 (h-A12, f-A8), through the tools: a deck of 11 body
// slides is not asked for dividers by validate_deck_spec or by
// analyze_deck_rhythm; a deck of 13 is, at a slide the agent can find; and
// meta.chrome.tracker: false declines the advice on both.
func TestSectioningAdviceRespectsDeckLengthAndTrackerChoice(t *testing.T) {
	const chrome = `{"client_name": "Atlas Retail"}`
	const declined = `{"client_name": "Atlas Retail", "tracker": false}`
	for _, tc := range []struct {
		name   string
		body   int
		chrome string
		want   bool
	}{
		{"12-slide deck", 11, chrome, false},
		{"13 body slides", 13, chrome, true},
		{"13 body slides, tracker declined", 13, declined, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mc := handleTestConfig(t)
			env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": rhythmAdviceSpec(tc.body, tc.chrome)}))
			if got := envelopeFindingCount(env, "SEMANTIC_RHYTHM_SECTIONING") > 0; got != tc.want {
				t.Errorf("validate_deck_spec: sectioning advice = %v, want %v", got, tc.want)
			}
			res := mustCall(t, mc.handleAnalyzeDeckRhythm, map[string]any{"deck_id": env.DeckID})
			if res.IsError {
				t.Fatalf("analyze_deck_rhythm: %s", textContent(res))
			}
			var out rhythm.Result
			structuredInto(t, res.StructuredContent, &out)
			sections := 0
			for _, rec := range out.Recommendations {
				// h-A12: a recommendation at slide_index -1 names nothing to fix.
				if rec.SlideIndex < 0 || rec.SlideIndex > tc.body {
					t.Errorf("%s names no slide: slide_index %d (%s)", rec.Code, rec.SlideIndex, rec.Message)
				}
				if rec.Code == rhythm.CodeMissingSections {
					sections++
					if !strings.Contains(rec.Message, fmt.Sprintf("before slide %d", rec.SlideIndex+1)) {
						t.Errorf("missing_sections should say where the first divider goes: %+v", rec)
					}
				}
			}
			if got := sections > 0; got != tc.want {
				t.Errorf("analyze_deck_rhythm: missing_sections = %v, want %v (%+v)", got, tc.want, out.Recommendations)
			}
		})
	}

	// A raw presentation says the same with chrome.tracker: false.
	raw := func(tracker string) map[string]any {
		slides := []any{map[string]any{"slide_type": "title", "content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Deck"}}}}
		for i := 0; i < 13; i++ {
			slides = append(slides, map[string]any{"slide_type": "content", "content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": fmt.Sprintf("Point %d", i+1)},
				map[string]any{"placeholder_id": "body", "type": "text", "text_value": "One sentence of argument."},
			}})
		}
		var pres map[string]any
		if err := json.Unmarshal([]byte(`{"template": "midnight-blue", "chrome": {`+tracker+`}}`), &pres); err != nil {
			t.Fatal(err)
		}
		pres["slides"] = slides
		return pres
	}
	for tracker, want := range map[string]bool{``: true, `"tracker": false`: false} {
		res := mustCall(t, handleAnalyzeDeckRhythm, map[string]any{"presentation": raw(tracker)})
		if res.IsError {
			t.Fatalf("analyze_deck_rhythm(raw): %s", textContent(res))
		}
		var out rhythm.Result
		structuredInto(t, res.StructuredContent, &out)
		got := false
		for _, rec := range out.Recommendations {
			got = got || rec.Code == rhythm.CodeMissingSections
		}
		if got != want {
			t.Errorf("raw presentation, chrome {%s}: missing_sections = %v, want %v", tracker, got, want)
		}
	}
}

// go-slide-creator-th6o9 (i-A12): the density finding an agent reads names the
// run and where to break it, as JSON Pointers into the spec it sent.
func TestDensityAdviceCarriesAnInsertPosition(t *testing.T) {
	table := func(title string) string {
		return fmt.Sprintf(`{"kind": "table", "title": %q, "takeaway": "Read across", "headers": ["Control", "Result"], "rows": [["Access", "Effective"], ["Change", "Deficient"]]}`, title)
	}
	spec := `{"meta": {"title": "ITGC readout", "template": "midnight-blue"}, "slides": [
	  {"kind": "title", "title": "ITGC readout", "subtitle": "Audit committee"},
	  ` + table("Results board holds at two deficiencies") + `,
	  ` + table("Access findings fell to four") + `,
	  ` + table("Change findings rose to three") + `,
	  ` + table("Operations findings stayed at one") + `,
	  {"kind": "closing", "title": "Questions"}
	]}`
	mc := handleTestConfig(t)
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec}))
	found := false
	for _, f := range env.Findings {
		if !strings.HasSuffix(f.Code, "SEMANTIC_RHYTHM_DENSITY") {
			continue
		}
		found = true
		if got := f.Evidence["insert_before"]; got != "/slides/3" {
			t.Errorf("evidence.insert_before = %v, want /slides/3", got)
		}
		var run []string
		structuredInto(t, f.Evidence["run"], &run)
		if len(run) != 4 || run[0] != "/slides/1" || run[3] != "/slides/4" {
			t.Errorf("evidence.run = %v, want /slides/1 … /slides/4", run)
		}
		for _, want := range []string{"/slides/1 to /slides/4", "before /slides/3"} {
			if !strings.Contains(f.Message, want) {
				t.Errorf("message %q should contain %q", f.Message, want)
			}
		}
	}
	if !found {
		t.Fatalf("four dense slides in a row should be reported: %+v", env.Findings)
	}
}
