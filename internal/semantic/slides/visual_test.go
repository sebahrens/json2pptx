package slides

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// decodePattern decodes an emitted pattern's values into the supplied target.
func decodePattern(t *testing.T, raw json.RawMessage, target any) {
	t.Helper()
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode pattern values: %v", err)
	}
}

func TestCompileComparison_EmitsPattern(t *testing.T) {
	in := Input{
		Title:    "Build vs Buy",
		Takeaway: "Buy wins.",
		Body: map[string]any{
			"columns": []any{
				map[string]any{"title": "Build", "items": []any{"Slower", "Flexible"}},
				map[string]any{"title": "Buy", "items": []any{"Faster", "Rigid"}},
			},
		},
	}
	slide, links, err := CompileComparison(in)
	if err != nil {
		t.Fatalf("CompileComparison: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "comparison-2col" {
		t.Fatalf("expected comparison-2col pattern, got %+v", slide.Pattern)
	}
	var vals comparison2colValues
	decodePattern(t, slide.Pattern.Values, &vals)
	if len(vals.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(vals.Rows))
	}
	if vals.Rows[0].Left != "Slower" || vals.Rows[0].Right != "Faster" {
		t.Errorf("row 0 = %+v, want {Slower, Faster}", vals.Rows[0])
	}
	if len(vals.Headers) != 2 || vals.Headers[0] != "Build" || vals.Headers[1] != "Buy" {
		t.Errorf("headers = %v, want [Build Buy]", vals.Headers)
	}
	if slide.Takeaway != "Buy wins." {
		t.Errorf("takeaway not carried: %q", slide.Takeaway)
	}
	assertHasLink(t, links, "slides[0].columns")
}

func TestCompileComparison_DegradesWhenUnbalanced(t *testing.T) {
	in := Input{
		Title: "Lopsided",
		Body: map[string]any{
			"columns": []any{
				map[string]any{"title": "Left", "items": []any{"a", "b", "c"}},
				map[string]any{"title": "Right", "items": []any{"x"}},
			},
		},
	}
	slide, _, err := CompileComparison(in)
	if err != nil {
		t.Fatalf("CompileComparison: %v", err)
	}
	// go-slide-creator-3bgf: an unbalanced pair loses the side-by-side row
	// alignment comparison-2col gives, but it is still two titled columns of
	// content — card-grid, not a bullet list.
	if slide.Pattern == nil || slide.Pattern.Name != "card-grid" {
		t.Fatalf("pattern = %+v, want card-grid", slide.Pattern)
	}
	assertNoGoMapLeak(t, slide)
}

// TestCompileComparison_ProsConsEmitsPattern is the regression for field-test
// 2.2: a comparison whose columns carry pros/cons arrays (rather than "items")
// must still render that content, not collapse to a flat list of column labels.
func TestCompileComparison_ProsConsEmitsPattern(t *testing.T) {
	in := Input{
		Title: "Build vs Buy",
		Body: map[string]any{
			"columns": []any{
				map[string]any{
					"label": "Build",
					"pros":  []any{"Full control", "Tailored"},
					"cons":  []any{"Slower"},
				},
				map[string]any{
					"label": "Buy",
					"pros":  []any{"Fast to deploy", "Supported"},
					"cons":  []any{"Vendor lock-in"},
				},
			},
		},
	}
	slide, _, err := CompileComparison(in)
	if err != nil {
		t.Fatalf("CompileComparison: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "comparison-2col" {
		t.Fatalf("expected comparison-2col pattern, got %+v", slide.Pattern)
	}
	var vals comparison2colValues
	decodePattern(t, slide.Pattern.Values, &vals)
	if len(vals.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (pros line + cons line)", len(vals.Rows))
	}
	if !strings.Contains(vals.Rows[0].Left, "Full control") || !strings.Contains(vals.Rows[0].Right, "Fast to deploy") {
		t.Errorf("pros row lost content: %+v", vals.Rows[0])
	}
	if !strings.Contains(vals.Rows[1].Left, "Slower") || !strings.Contains(vals.Rows[1].Right, "Vendor lock-in") {
		t.Errorf("cons row lost content: %+v", vals.Rows[1])
	}
}

// TestCompileComparison_ProsConsFallbackKeepsContent covers the degrade path
// (unbalanced pros/cons across >2 columns): pros/cons text must survive in the
// fallback bullets rather than being dropped to bare column labels.
func TestCompileComparison_ProsConsFallbackKeepsContent(t *testing.T) {
	in := Input{
		Title: "Three Options",
		Body: map[string]any{
			"columns": []any{
				map[string]any{"label": "A", "pros": []any{"cheap"}},
				map[string]any{"label": "B", "pros": []any{"robust"}, "cons": []any{"costly"}},
				map[string]any{"label": "C", "cons": []any{"slow"}},
			},
		},
	}
	slide, _, err := CompileComparison(in)
	if err != nil {
		t.Fatalf("CompileComparison: %v", err)
	}
	// go-slide-creator-3bgf: three pro/con columns are a three-panel visual, not
	// three bullets. The pros/cons content still has to survive the routing.
	if slide.Pattern == nil || slide.Pattern.Name != "stylish-panels" {
		t.Fatalf("pattern = %+v, want stylish-panels", slide.Pattern)
	}
	assertNoGoMapLeak(t, slide)
	values := string(slide.Pattern.Values)
	for _, want := range []string{"cheap", "robust", "costly", "slow"} {
		if !strings.Contains(values, want) {
			t.Errorf("routing dropped %q; values = %q", want, values)
		}
	}
}

// go-slide-creator-3bgf: three columns are a three-panel visual. A comparison
// degrades to bullets only past what any of the patterns can hold — six or more
// columns, or a column with no header.
func TestCompileComparison_DegradesPastEveryVisual(t *testing.T) {
	tests := []struct {
		name string
		cols []any
	}{
		{
			name: "six columns exceed card-grid's five",
			cols: []any{
				map[string]any{"title": "A", "items": []any{"1"}},
				map[string]any{"title": "B", "items": []any{"2"}},
				map[string]any{"title": "C", "items": []any{"3"}},
				map[string]any{"title": "D", "items": []any{"4"}},
				map[string]any{"title": "E", "items": []any{"5"}},
				map[string]any{"title": "F", "items": []any{"6"}},
			},
		},
		{
			name: "a headerless column has nothing to title a panel with",
			cols: []any{
				map[string]any{"title": "A", "items": []any{"1"}},
				map[string]any{"items": []any{"2"}},
				map[string]any{"title": "C", "items": []any{"3"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slide, _, err := CompileComparison(Input{Title: "Options", Body: map[string]any{"columns": tt.cols}})
			if err != nil {
				t.Fatalf("CompileComparison: %v", err)
			}
			if slide.Pattern != nil {
				t.Fatalf("expected the bullet fallback, got %+v", slide.Pattern)
			}
			assertNoGoMapLeak(t, slide)
		})
	}
}

// Three balanced columns are the standard consulting option slide, and the one
// the dogfood run found collapsing into three run-on bullets on a 60%-empty page.
func TestCompileComparison_ThreeColumnsEmitPanels(t *testing.T) {
	in := Input{
		Title: "Three Ways",
		Body: map[string]any{
			"columns": []any{
				map[string]any{"title": "Parcel automation", "items": []any{"EUR 60M capex", "Payback 3.5 yrs", "Low risk"}},
				map[string]any{"title": "Hub consolidation", "items": []any{"EUR 35M capex", "Payback 2.1 yrs", "Medium risk"}},
				map[string]any{"title": "Partner network", "items": []any{"EUR 8M capex", "Payback 1.2 yrs", "High risk"}},
			},
		},
	}
	slide, links, err := CompileComparison(in)
	if err != nil {
		t.Fatalf("CompileComparison: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "stylish-panels" {
		t.Fatalf("pattern = %+v, want stylish-panels", slide.Pattern)
	}
	var panels []stylishPanelsItem
	if jerr := json.Unmarshal(slide.Pattern.Values, &panels); jerr != nil {
		t.Fatalf("unmarshal panels: %v", jerr)
	}
	if len(panels) != 3 {
		t.Fatalf("panels = %d, want 3", len(panels))
	}
	// Each column keeps its own bullets rather than being joined into one line.
	if panels[0].Title != "Parcel automation" || len(panels[0].Body) != 3 {
		t.Errorf("panel 0 = %+v, want the column header and its three items", panels[0])
	}
	assertHasLink(t, links, in.semSlide()+".columns")
	assertNoGoMapLeak(t, slide)
}

func TestCompileProcess_EmitsPattern(t *testing.T) {
	in := Input{
		Title: "Process",
		Body: map[string]any{
			"steps": []any{
				map[string]any{"title": "Discover", "description": "Gather inputs"},
				map[string]any{"title": "Build", "description": "Produce deck"},
				map[string]any{"title": "Review", "description": "Check fit", "type": "decision"},
			},
		},
	}
	slide, _, err := CompileProcess(in)
	if err != nil {
		t.Fatalf("CompileProcess: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "process-flow" {
		t.Fatalf("expected process-flow pattern, got %+v", slide.Pattern)
	}
	var vals processFlowValues
	decodePattern(t, slide.Pattern.Values, &vals)
	if len(vals.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(vals.Steps))
	}
	if vals.Steps[0].Label != "Discover — Gather inputs" {
		t.Errorf("step 0 label = %q, want description-preserving label", vals.Steps[0].Label)
	}
	if vals.Steps[2].Type != "decision" {
		t.Errorf("step 2 type = %q, want decision", vals.Steps[2].Type)
	}
}

// TestCompileProcess_DegradesWithoutGoMapLeak is the direct regression for the
// field-report bug: a process slide whose step count is outside 3–8 falls back
// to a content slide, and the {title, description} step objects must render as
// readable bullets, never as Go "map[...]" strings.
func TestCompileProcess_DegradesWithoutGoMapLeak(t *testing.T) {
	in := Input{
		Title: "Two-Step",
		Body: map[string]any{
			"steps": []any{
				map[string]any{"title": "Discover", "description": "Gather inputs"},
				map[string]any{"title": "Build", "description": "Produce deck"},
			},
		},
	}
	slide, _, err := CompileProcess(in)
	if err != nil {
		t.Fatalf("CompileProcess: %v", err)
	}
	if slide.Pattern != nil {
		t.Fatalf("expected content fallback for 2 steps, got %+v", slide.Pattern)
	}
	assertNoGoMapLeak(t, slide)

	body := bulletBody(t, slide)
	if !strings.Contains(strings.Join(body, " "), "Discover — Gather inputs") {
		t.Errorf("expected readable 'Discover — Gather inputs' bullet, got %v", body)
	}
}

func TestCompileRoadmap_EmitsPattern(t *testing.T) {
	in := Input{
		Title: "Roadmap",
		Body: map[string]any{
			"phases": []any{
				map[string]any{"name": "Pilot", "date_label": "Q1", "description": "Prove value"},
				map[string]any{"name": "Expand", "date_label": "Q2", "description": "Scale", "active": true},
				map[string]any{"name": "GA", "date_label": "Q3", "milestone": "Launch"},
			},
		},
	}
	slide, _, err := CompileRoadmap(in)
	if err != nil {
		t.Fatalf("CompileRoadmap: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "phase-roadmap" {
		t.Fatalf("expected phase-roadmap pattern, got %+v", slide.Pattern)
	}
	var vals phaseRoadmapValues
	decodePattern(t, slide.Pattern.Values, &vals)
	if len(vals.Phases) != 3 {
		t.Fatalf("phases = %d, want 3", len(vals.Phases))
	}
	if vals.Phases[0].Name != "Pilot" || vals.Phases[0].DateLabel != "Q1" {
		t.Errorf("phase 0 = %+v", vals.Phases[0])
	}
	if !vals.Phases[1].Active {
		t.Error("phase 1 should be active")
	}
	if vals.Phases[2].Milestone != "Launch" {
		t.Errorf("phase 2 milestone = %q, want Launch", vals.Phases[2].Milestone)
	}
}

func TestCompileRoadmap_DegradesWithoutGoMapLeak(t *testing.T) {
	in := Input{
		Title: "Many Phases",
		Body: map[string]any{
			"phases": []any{
				map[string]any{"name": "P1", "description": "d1"},
				map[string]any{"name": "P2", "description": "d2"},
				map[string]any{"name": "P3", "description": "d3"},
				map[string]any{"name": "P4", "description": "d4"},
				map[string]any{"name": "P5", "description": "d5"},
				map[string]any{"name": "P6", "description": "d6"},
				map[string]any{"name": "P7", "description": "d7"},
			},
		},
	}
	slide, _, err := CompileRoadmap(in)
	if err != nil {
		t.Fatalf("CompileRoadmap: %v", err)
	}
	if slide.Pattern != nil {
		t.Fatalf("expected content fallback for 7 phases, got %+v", slide.Pattern)
	}
	assertNoGoMapLeak(t, slide)
}

// TestStringListNoGoMapLeak guards the shared helper that backs every content
// fallback: object list entries must never be stringified as Go maps.
func TestStringListNoGoMapLeak(t *testing.T) {
	body := map[string]any{
		"steps": []any{
			map[string]any{"title": "Discover", "description": "Gather inputs"},
			map[string]any{"label": "Build"},
			"Ship it",
		},
	}
	out, ok := stringList(body, "steps")
	if !ok {
		t.Fatal("stringList reported non-list")
	}
	for _, s := range out {
		if strings.Contains(s, "map[") {
			t.Errorf("stringList leaked a Go map string: %q", s)
		}
	}
	want := []string{"Discover — Gather inputs", "Build", "Ship it"}
	if strings.Join(out, "|") != strings.Join(want, "|") {
		t.Errorf("stringList = %v, want %v", out, want)
	}
}

// ---- test helpers ----

func assertHasLink(t *testing.T, links []SourceLink, wantSemantic string) {
	t.Helper()
	for _, l := range links {
		if l.SemanticPath == wantSemantic {
			return
		}
	}
	t.Errorf("expected a source link to %q, got %+v", wantSemantic, links)
}

// assertNoGoMapLeak fails if any text or bullet content on the slide contains a
// Go "map[...]" dump — the field-report symptom this fix exists to prevent.
func assertNoGoMapLeak(t *testing.T, slide *deckinput.SlideInput) {
	t.Helper()
	for _, c := range slide.Content {
		if c.TextValue != nil && strings.Contains(*c.TextValue, "map[") {
			t.Errorf("text content leaked a Go map string: %q", *c.TextValue)
		}
		if c.BulletsValue != nil {
			for _, b := range *c.BulletsValue {
				if strings.Contains(b, "map[") {
					t.Errorf("bullet leaked a Go map string: %q", b)
				}
			}
		}
	}
}

// bulletBody returns the bullets of the slide's first bullets content item.
func bulletBody(t *testing.T, slide *deckinput.SlideInput) []string {
	t.Helper()
	for _, c := range slide.Content {
		if c.Type == "bullets" && c.BulletsValue != nil {
			return *c.BulletsValue
		}
	}
	t.Fatal("no bullets content found")
	return nil
}
