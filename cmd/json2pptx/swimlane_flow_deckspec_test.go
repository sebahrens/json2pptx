package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// go-slide-creator-v786r: a three-team swimlane authored the obvious way —
// every team's steps side by side — drew arrows down each column, and nothing
// said the order could be stated. The DeckSpec route (a raw_json2pptx slide;
// swimlane has no kind of its own) must carry values.flow through to the
// arrows, and must say so when the order is left to be guessed.

func swimlaneDeckSpec(t *testing.T, flow string) (*patterns.SwimlaneValues, patterns.Pattern) {
	t.Helper()
	spec := `{
	  "meta": {"title": "Support flow", "template": "midnight-blue"},
	  "slides": [{
	    "kind": "raw_json2pptx",
	    "slide": {
	      "slide_type": "content",
	      "content": [{"placeholder_id": "title", "type": "text", "text_value": "A support ticket crosses three teams before it is resolved"}],
	      "pattern": {"name": "swimlane", "values": {
	        "lanes": [
	          {"actor": "Customer", "steps": ["Submit request", "", "Approve fix"]},
	          {"actor": "Support", "steps": ["Triage", "Investigate", "Resolve"]},
	          {"actor": "Engineering", "steps": ["", "Fix bug", "Deploy"]}
	        ]` + flow + `
	      }}
	    }
	  }]
	}`
	parsed, diags := semantic.Parse("spec.json", []byte(spec))
	if parsed == nil {
		t.Fatalf("parse: %+v", diags)
	}
	input, _, err := semantic.Compile(parsed, semantic.CompileOptions{Strict: semantic.StrictnessWarn, DefaultTemplate: "midnight-blue"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(input.Slides) != 1 || input.Slides[0].Pattern == nil || input.Slides[0].Pattern.Name != "swimlane" {
		t.Fatalf("compiled slides = %+v, want one swimlane pattern slide", input.Slides)
	}
	pat, _ := patterns.Default().Get("swimlane")
	values := pat.NewValues().(*patterns.SwimlaneValues)
	if err := json.Unmarshal(input.Slides[0].Pattern.Values, values); err != nil {
		t.Fatalf("decode values: %v", err)
	}
	if err := pat.Validate(values, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	return values, pat
}

func TestSwimlaneDeckSpecFlowDrawsTheStatedOrder(t *testing.T) {
	values, pat := swimlaneDeckSpec(t, `, "flow": [[0,0],[1,0],[1,1],[2,1],[2,2],[1,2],[0,2]]`)
	grid, err := pat.Expand(patterns.ExpandContext{}, values, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	// The label a grid cell carries, by (grid row, grid column).
	label := func(ref [2]int) string {
		cell := grid.Rows[ref[0]].Cells[ref[1]]
		var text struct {
			Paragraphs []struct {
				Content string `json:"content"`
			} `json:"paragraphs"`
		}
		if cell.Shape == nil || json.Unmarshal(cell.Shape.Text, &text) != nil || len(text.Paragraphs) == 0 {
			t.Fatalf("link endpoint %v is not a step", ref)
		}
		return text.Paragraphs[0].Content
	}
	want := []string{"Submit request", "Triage", "Investigate", "Fix bug", "Deploy", "Resolve", "Approve fix"}
	if len(grid.Links) != len(want)-1 {
		t.Fatalf("%d arrows, want %d", len(grid.Links), len(want)-1)
	}
	for i, link := range grid.Links {
		if from, to := label(link.From), label(link.To); from != want[i] || to != want[i+1] {
			t.Errorf("arrow %d runs %q -> %q, want %q -> %q", i, from, to, want[i], want[i+1])
		}
		if link.Connector == nil || link.Connector.Style != "arrow" {
			t.Errorf("arrow %d has no arrowhead: %+v", i, link.Connector)
		}
	}
	if w := pat.(patterns.PostExpandWarner).PostExpandWarnings(patterns.ExpandContext{}, values, nil); len(w) != 0 {
		t.Errorf("a stated flow drew findings: %v", w)
	}
}

func TestSwimlaneDeckSpecWithoutFlowIsReported(t *testing.T) {
	values, pat := swimlaneDeckSpec(t, "")
	warnings := pat.(patterns.PostExpandWarner).PostExpandWarnings(patterns.ExpandContext{}, values, nil)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], patterns.ErrCodeSwimlaneFlowAmbiguous+": ") {
		t.Fatalf("warnings = %v, want one %s", warnings, patterns.ErrCodeSwimlaneFlowAmbiguous)
	}
	// The finding has to carry the remedy: the default tool profile shows an
	// agent nothing else about step order.
	for _, want := range []string{"values.flow", "[lane, step]", "own column"} {
		if !strings.Contains(warnings[0], want) {
			t.Errorf("finding does not mention %q: %s", want, warnings[0])
		}
	}
	// The pattern's schema says it too.
	raw, _ := json.Marshal(pat.Schema())
	for _, want := range []string{`"flow"`, "own column"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("swimlane schema does not mention %s", want)
		}
	}
}
