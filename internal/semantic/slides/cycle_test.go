package slides

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

func cycleBody(style string, extra map[string]any) map[string]any {
	body := map[string]any{
		"title": "The loop",
		"phases": []any{
			map[string]any{"label": "Plan", "description": "Set the bets"},
			"Build",
			map[string]any{"name": "Measure", "detail": "Read adoption"},
			"Learn",
		},
	}
	if style != "" {
		body["style"] = style
	}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

// Every style compiles to its pattern with the payload mapped onto that
// pattern's own values: phases / steps / loop / spokes / layers, the highlight
// as a per-phase flag or a 1-based number, the centre where the pattern has
// one (go-slide-creator-53v5u).
func TestCompileCycleMapsEachStyleOntoItsPattern(t *testing.T) {
	cases := []struct {
		style, pattern string
		extra          map[string]any
		want           string
	}{
		{"", "cycle-ring", map[string]any{"center": map[string]any{"label": "Product", "sublabel": "Quarterly"}, "highlight": "Measure"},
			`{"phases":[{"label":"Plan","description":"Set the bets"},{"label":"Build"},{"label":"Measure","description":"Read adoption","highlight":true},{"label":"Learn"}],"center":{"label":"Product","sublabel":"Quarterly"}}`},
		{"nodes", "cycle-nodes", map[string]any{"center": "Product", "highlight": 3},
			`{"steps":[{"label":"Plan","description":"Set the bets"},{"label":"Build"},{"label":"Measure","description":"Read adoption"},{"label":"Learn"}],"center":{"label":"Product"},"highlight":3}`},
		{"intake", "cycle-intake", map[string]any{"intake": []any{"Sign", map[string]any{"label": "Onboard", "description": "Users trained"}}, "highlight": 2},
			`{"intake":[{"label":"Sign"},{"label":"Onboard","description":"Users trained"}],"loop":[{"label":"Plan","description":"Set the bets"},{"label":"Build","highlight":true},{"label":"Measure","description":"Read adoption"},{"label":"Learn"}]}`},
		{"figure_eight", "cycle-figure-eight", map[string]any{"left_label": "Build", "right_label": "Run", "left_count": 2},
			`{"phases":[{"label":"Plan","description":"Set the bets"},{"label":"Build"},{"label":"Measure","description":"Read adoption"},{"label":"Learn"}],"left_count":2,"left_label":"Build","right_label":"Run"}`},
		{"radial", "radial-hub", map[string]any{"center": "Customer", "highlight": "Build"},
			`{"center":{"label":"Customer"},"spokes":[{"label":"Plan","description":"Set the bets"},{"label":"Build"},{"label":"Measure","description":"Read adoption"},{"label":"Learn"}],"highlight":2}`},
		{"concentric", "concentric-rings", nil,
			`{"layers":[{"label":"Plan","description":"Set the bets"},{"label":"Build"},{"label":"Measure","description":"Read adoption"},{"label":"Learn"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			body := cycleBody(tc.style, tc.extra)
			if issues := CycleIssues(body); len(issues) > 0 {
				t.Fatalf("issues: %+v", issues)
			}
			slide, links, err := CompileCycle(Input{Title: "The loop", Takeaway: "It loops.", Body: body})
			if err != nil {
				t.Fatal(err)
			}
			if slide.Pattern == nil || slide.Pattern.Name != tc.pattern || slide.LayoutID != "blank-title" {
				t.Fatalf("slide = %+v, want pattern %s on blank-title", slide, tc.pattern)
			}
			if got := string(slide.Pattern.Values); got != tc.want {
				t.Errorf("values\n got %s\nwant %s", got, tc.want)
			}
			if err := deckinput.ValidatePattern(slide.Pattern, patterns.Default()); err != nil {
				t.Errorf("the pattern refuses the compiled values: %v", err)
			}
			if slide.Takeaway != "It loops." {
				t.Errorf("takeaway = %q", slide.Takeaway)
			}
			if CyclePattern(body) != tc.pattern {
				t.Errorf("CyclePattern = %q, want %s (explain and compile must agree)", CyclePattern(body), tc.pattern)
			}
			var listed bool
			for _, l := range links {
				listed = listed || strings.HasSuffix(l.SemanticPath, ".phases")
			}
			if !listed {
				t.Errorf("links = %+v, want the phases mapped back", links)
			}
		})
	}
}

// The phases may be written as steps or items, and a style left unset follows
// the payload: three phases are nodes, intake steps select intake.
func TestCycleAliasesAndDefaultStyle(t *testing.T) {
	for _, field := range []string{"steps", "items"} {
		body := map[string]any{field: []any{"Plan", "Do", "Check", "Act"}}
		if CyclePattern(body) != "cycle-ring" || CyclePhasesField(body) != field {
			t.Errorf("%s: pattern %q field %q", field, CyclePattern(body), CyclePhasesField(body))
		}
	}
	if got := CyclePattern(map[string]any{"phases": []any{"Commit", "Inspect", "Adjust"}}); got != "cycle-nodes" {
		t.Errorf("three unstyled phases compile to %q, want cycle-nodes", got)
	}
	if got := CyclePattern(map[string]any{"phases": []any{"Plan", "Do", "Check"}, "intake": []any{"Sign"}}); got != "cycle-intake" {
		t.Errorf("intake steps with no style compile to %q, want cycle-intake", got)
	}
	if got := CycleStyleOf(map[string]any{"style": "Figure-Eight", "phases": []any{"a", "b", "c", "d"}}); got != CycleStyleFigureEight {
		t.Errorf("style spelling: %q", got)
	}
}

// A count outside the style's range is reported at the list with the range
// and where the content belongs, in the kind's own vocabulary.
func TestCycleCountIssuesNameTheSibling(t *testing.T) {
	nine := []any{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	cases := []struct {
		name string
		body map[string]any
		min  int
		max  int
		hint string
	}{
		{"ring of nine", map[string]any{"steps": nine}, 4, 8, "split the loop across two cycle slides"},
		{"ring of three", map[string]any{"style": "ring", "phases": nine[:3]}, 4, 8, "style nodes"},
		{"figure eight of three", map[string]any{"style": "figure_eight", "phases": nine[:3]}, 4, 8, "style ring or nodes"},
		{"radial of three", map[string]any{"style": "radial", "center": "Hub", "phases": nine[:3]}, 4, 8, "pillars"},
		{"concentric of six", map[string]any{"style": "concentric", "phases": nine[:6]}, 3, 5, "architecture"},
		{"nodes of two", map[string]any{"style": "nodes", "phases": nine[:2]}, 3, 8, "comparison"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := CycleIssues(tc.body)
			if len(issues) != 1 {
				t.Fatalf("issues = %+v, want the count alone", issues)
			}
			is := issues[0]
			if is.Field != CyclePhasesField(tc.body) || is.Min != tc.min || is.Max != tc.max || !strings.Contains(is.Message, tc.hint) {
				t.Errorf("issue = %+v, want %d–%d at the list with %q", is, tc.min, tc.max, tc.hint)
			}
			if CyclePattern(tc.body) != "" {
				t.Error("a payload with an issue still claims its pattern")
			}
		})
	}
	four := map[string]any{"style": "intake", "phases": nine[:4], "intake": nine[:4]}
	if issues := CycleIssues(four); len(issues) != 1 || issues[0].Field != "intake" || issues[0].Max != 3 {
		t.Errorf("four intake steps: %+v", issues)
	}
}

// Every over-long text is reported at its own authored path with the length
// the style holds, and that length is the pattern's own.
func TestCycleBudgetIssuesPointAtTheAuthoredField(t *testing.T) {
	long := strings.Repeat("x", 61)
	body := map[string]any{
		"style":  "radial",
		"center": map[string]any{"label": strings.Repeat("c", 25)},
		"items": []any{
			"Sales",
			map[string]any{"name": strings.Repeat("n", 27)},
			map[string]any{"label": "Finance", "detail": long},
			strings.Repeat("s", 30),
		},
	}
	got := map[string]int{}
	for _, is := range CycleIssues(body) {
		got[is.Field] = is.Allowed
		if is.Measured <= is.Allowed {
			t.Errorf("%s: measured %d within allowed %d", is.Field, is.Measured, is.Allowed)
		}
	}
	want := map[string]int{"center.label": 24, "items[1].name": 26, "items[2].detail": 60, "items[3]": 26}
	if len(got) != len(want) {
		t.Fatalf("issues at %v, want %v", got, want)
	}
	for field, allowed := range want {
		if got[field] != allowed {
			t.Errorf("%s: allowed %d, want %d", field, got[field], allowed)
		}
	}

	// A figure eight whose loop holds four phases has the tighter description
	// budget the pattern reports at render time.
	eight := map[string]any{"style": "figure_eight", "phases": []any{
		"a", "b", "c", map[string]any{"label": "d", "description": strings.Repeat("d", 45)}, "e", "f", "g", "h",
	}}
	issues := CycleIssues(eight)
	if len(issues) != 1 || issues[0].Field != "phases[3].description" || issues[0].Allowed != 40 {
		t.Errorf("figure eight description: %+v", issues)
	}
}

// What a style needs and lacks, and what it carries and cannot draw.
func TestCycleRequiredAndDroppedFields(t *testing.T) {
	radial := map[string]any{"style": "radial", "phases": []any{"a", "b", "c", "d"}}
	if issues := CycleIssues(radial); len(issues) != 1 || issues[0].Field != "center" || !issues[0].Required {
		t.Errorf("radial without a center: %+v", issues)
	}
	intake := map[string]any{"style": "intake", "phases": []any{"a", "b", "c"}}
	if issues := CycleIssues(intake); len(issues) != 1 || issues[0].Field != "intake" || !issues[0].Required {
		t.Errorf("intake without intake steps: %+v", issues)
	}
	miss := map[string]any{"phases": []any{"a", "b", "c", "d"}, "highlight": "z"}
	if issues := CycleIssues(miss); len(issues) != 1 || issues[0].Field != "highlight" {
		t.Errorf("highlight naming no phase: %+v", issues)
	}
	two := map[string]any{"phases": []any{"a", map[string]any{"label": "b", "highlight": true}, "c", "d"}, "highlight": 3}
	if issues := CycleIssues(two); len(issues) != 1 || !strings.Contains(issues[0].Message, "2 phases are highlighted") {
		t.Errorf("two highlights: %+v", issues)
	}

	dropped := func(body map[string]any) []string {
		var out []string
		for _, d := range CycleDroppedFields(body) {
			if !d.Dropped || !strings.Contains(d.Message, "DROPPED") {
				t.Errorf("dropped field %+v does not say so", d)
			}
			out = append(out, d.Field)
		}
		return out
	}
	phases := []any{"a", "b", "c", "d"}
	for name, tc := range map[string]struct {
		body map[string]any
		want string
	}{
		"center on a figure eight":  {map[string]any{"style": "figure_eight", "phases": phases, "center": "x"}, "center"},
		"center on concentric":      {map[string]any{"style": "concentric", "phases": phases, "center": "x"}, "center"},
		"sublabel on nodes":         {map[string]any{"style": "nodes", "phases": phases, "center": map[string]any{"label": "x", "sublabel": "y"}}, "center.sublabel"},
		"lobe label on a ring":      {map[string]any{"phases": phases, "left_label": "x"}, "left_label"},
		"intake on a radial":        {map[string]any{"style": "radial", "center": "x", "phases": phases, "intake": []any{"s"}}, "intake"},
		"nothing dropped on a ring": {map[string]any{"phases": phases, "center": map[string]any{"label": "x", "sublabel": "y"}}, ""},
	} {
		got := strings.Join(dropped(tc.body), ",")
		if got != tc.want {
			t.Errorf("%s: dropped %q, want %q", name, got, tc.want)
		}
	}
}

// A payload the visual refuses keeps every word as a numbered list.
func TestCompileCycleFallbackKeepsEveryPhase(t *testing.T) {
	body := map[string]any{
		"style":  "intake",
		"center": "Monthly",
		"intake": []any{map[string]any{"label": "Sign", "description": "Scope agreed"}},
		"phases": []any{"a", "b", "c", "d", "e", "f", "g", "h", map[string]any{"label": "i", "description": "the ninth"}},
	}
	slide, links, err := CompileCycle(Input{Title: "Nine phases", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern != nil {
		t.Fatalf("nine phases compiled to %s", slide.Pattern.Name)
	}
	raw, _ := json.Marshal(slide)
	for _, want := range []string{"Monthly", "First: Sign — Scope agreed", "1. a", "9. i — the ninth"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("fallback lost %q: %s", want, raw)
		}
	}
	if len(links) < 2 {
		t.Errorf("links = %+v", links)
	}
}

// A cycle region hosts the style's pattern in its cell; a figure eight is
// refused wherever the region shares the slide's width.
func TestCycleRegion(t *testing.T) {
	loop := map[string]any{"kind": "cycle", "phases": []any{"Plan", "Do", "Check", "Act"}, "center": "Monthly", "highlight": "Check"}
	b, err := regionCycle(loop)
	if err != nil {
		t.Fatal(err)
	}
	if b.cell == nil || !strings.Contains(string(b.cell.Pattern), `"name":"cycle-ring"`) || !strings.Contains(string(b.cell.Pattern), `"highlight":true`) {
		t.Errorf("cell = %s", b.cell.Pattern)
	}
	if _, err := regionCycle(map[string]any{"kind": "cycle", "phases": []any{"a", "b"}}); err == nil {
		t.Error("a two-phase cycle region compiled")
	}

	eight := map[string]any{"kind": "cycle", "style": "figure_eight", "phases": []any{"a", "b", "c", "d"}}
	text := map[string]any{"kind": "text", "body": "x"}
	for arrangement, wantRefused := range map[string]bool{ArrangeColumns: true, ArrangeMainLeft: true, ArrangeRows: false, ArrangeMainTop: false} {
		regions := []any{eight, text}
		if IsMainArrangement(arrangement) {
			regions = append(regions, text)
		}
		msg := CycleRegionWidthIssue(map[string]any{"arrangement": arrangement, "regions": regions}, 0)
		if (msg != "") != wantRefused {
			t.Errorf("%s: refusal %q, want refused=%v", arrangement, msg, wantRefused)
		}
		if wantRefused && !strings.Contains(msg, "style ring") {
			t.Errorf("%s: the refusal names no style that fits: %s", arrangement, msg)
		}
	}
	if got := RegionPatterns(map[string]any{"regions": []any{loop, text}}); got[0] != "cycle-ring" {
		t.Errorf("RegionPatterns = %v", got)
	}
}

// The style table's counts are the patterns' own: one inside each bound is
// accepted and one outside refused by the pattern itself.
func TestCycleStyleCountsMatchThePatterns(t *testing.T) {
	labels := []any{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	for style, info := range cycleStyleTable {
		for n := info.min - 1; n <= info.max+1; n++ {
			body := map[string]any{"style": style, "phases": labels[:n], "center": "Hub", "intake": []any{"s"}}
			switch style {
			case CycleStyleFigureEight, CycleStyleConcentric:
				delete(body, "center")
			}
			if style != CycleStyleIntake {
				delete(body, "intake")
			}
			p := resolveCycle(body)
			block, err := p.pattern()
			if err != nil {
				t.Fatal(err)
			}
			accepted := deckinput.ValidatePattern(&block, patterns.Default()) == nil
			if inside := n >= info.min && n <= info.max; accepted != inside {
				t.Errorf("style %s with %d phases: the pattern accepts=%v, the table says inside=%v", style, n, accepted, inside)
			}
		}
	}
}
