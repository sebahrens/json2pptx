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
	// overrides.style "tiles" draws an arrow between every two steps.
	grid, err := pat.Expand(patterns.ExpandContext{}, values, &patterns.SwimlaneOverrides{Style: "tiles"}, nil)
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

	// The default look follows the same order and leaves out only the arrows
	// its pentagons make redundant: a step to the next column of its own lane
	// (go-slide-creator-vx7wk).
	bands, err := pat.Expand(patterns.ExpandContext{}, values, nil, nil)
	if err != nil {
		t.Fatalf("expand bands: %v", err)
	}
	var drawn [][2]string
	for _, link := range bands.Links {
		drawn = append(drawn, [2]string{label(link.From), label(link.To)})
	}
	wantDrawn := [][2]string{{"Submit request", "Triage"}, {"Investigate", "Fix bug"}, {"Deploy", "Resolve"}, {"Resolve", "Approve fix"}}
	if len(drawn) != len(wantDrawn) {
		t.Fatalf("bands arrows = %v, want %v", drawn, wantDrawn)
	}
	for i := range drawn {
		if drawn[i] != wantDrawn[i] {
			t.Errorf("bands arrow %d runs %v, want %v", i, drawn[i], wantDrawn[i])
		}
	}
}

// go-slide-creator-vx7wk: a lane band is the backdrop of its lane. It is
// written before the hand-off arrows, and the arrows before the tabs and the
// steps, so nothing is drawn over a label.
func TestSwimlaneBandsAreWrittenBehindArrowsAndSteps(t *testing.T) {
	pat, _ := patterns.Default().Get("swimlane")
	encoded, err := json.Marshal(pat.(patterns.Exemplar).ExemplarValues())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"midnight-blue", "warm-coral"} {
		geom := loadSchemaMaximaGeometry(t, name)
		input := &PresentationInput{Template: geom.name, Slides: []SlideInput{{
			SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "swimlane", Values: encoded},
		}}}
		specs, _, _, err := convertPresentationSlides(input.Slides, geom.layouts, geom.width, geom.height, nil, nil, "", nil, false)
		if err != nil || len(specs) != 1 {
			t.Fatalf("%s: convert: %v (%d slides)", name, err, len(specs))
		}
		var order []string
		for _, fragment := range specs[0].RawShapeXML {
			xml := string(fragment)
			switch {
			case strings.Contains(xml, "<p:cxnSp"):
				order = append(order, "arrow")
			case strings.Contains(xml, `prst="homePlate"`):
				order = append(order, "pentagon")
			case strings.Contains(xml, `prst="rect"`) && strings.Contains(xml, "<a:solidFill>"):
				order = append(order, "band") // an empty position is an unpainted rect
			}
		}
		// Three lanes; Report -> Log, Log -> Diagnose, Deploy -> Verify and
		// Verify -> Confirm change lane (Diagnose -> Deploy does not); three
		// actor tabs and six steps.
		want := strings.Repeat("band ", 3) + strings.Repeat("arrow ", 4) + strings.Repeat("pentagon ", 9)
		if got := strings.Join(order, " ") + " "; got != want {
			t.Errorf("%s: shapes are written as\n  %s\nwant\n  %s", name, got, want)
		}
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
