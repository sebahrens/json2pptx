package slides

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/types"
)

func swotSections() map[string]any {
	return map[string]any{
		"strengths":     []any{"Two of three clearers migrated"},
		"weaknesses":    []any{"Reconciliation is still manual"},
		"opportunities": []any{"The T+1 mandate"},
		"threats":       []any{"A competitor is already live"},
	}
}

func frameworkBodyFor(name string, sections map[string]any) map[string]any {
	return map[string]any{"title": "Where we stand", "framework": name, "sections": sections}
}

// SWOT, the five forces and the Business Model Canvas are named things with
// fixed parts, and the spec had no kind for any of them. One kind takes all
// three and routes each to the visual that draws it (go-slide-creator-anzx).
func TestCompileFrameworkRoutesEachFramework(t *testing.T) {
	t.Run("swot compiles to the native diagram", func(t *testing.T) {
		slide, _, err := CompileFramework(Input{Body: frameworkBodyFor("swot", swotSections())})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		if slide.Pattern != nil {
			t.Fatalf("swot is a diagram, not a pattern: %s", slide.Pattern.Name)
		}
		diagram := findDiagram(t, slide)
		if diagram.Type != "swot" {
			t.Errorf("diagram type = %q, want swot", diagram.Type)
		}
		// The diagram needs a body placeholder to land in; blank-title has none.
		if slide.SlideType != "diagram" {
			t.Errorf("slide_type = %q, want diagram", slide.SlideType)
		}
	})

	t.Run("five forces carries each force's factors", func(t *testing.T) {
		body := frameworkBodyFor("porters_five_forces", map[string]any{
			"rivalry":      []any{"Three clearers, all migrating"},
			"new_entrants": []any{"High regulatory barrier"},
			"substitutes":  []any{"In-house settlement"},
			"suppliers":    []any{"Two viable vendors"},
			"buyers":       []any{"Ten banks are 80% of volume"},
		})
		slide, _, err := CompileFramework(Input{Body: body})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		diagram := findDiagram(t, slide)
		if diagram.Type != "porters_five_forces" {
			t.Fatalf("diagram type = %q", diagram.Type)
		}
		force, ok := diagram.Data["rivalry"].(map[string]any)
		if !ok {
			t.Fatalf("rivalry = %#v, want the force object the renderer reads", diagram.Data["rivalry"])
		}
		if force["label"] != "Competitive rivalry" {
			t.Errorf("label = %v", force["label"])
		}
		// An intensity nobody stated is left to the renderer rather than invented.
		if _, has := force["intensity"]; has {
			t.Errorf("an unstated intensity was invented: %v", force["intensity"])
		}
	})

	t.Run("bmc compiles to the canvas pattern with its own headings", func(t *testing.T) {
		sections := map[string]any{}
		for _, s := range frameworkSections[frameworkBMC] {
			sections[s.key] = []any{"Something for " + s.key}
		}
		body := frameworkBodyFor("bmc", sections)
		if got := FrameworkPattern(body); got != "bmc-canvas" {
			t.Fatalf("FrameworkPattern = %q, want bmc-canvas", got)
		}
		slide, _, err := CompileFramework(Input{Body: body})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		if slide.Pattern == nil || slide.Pattern.Name != "bmc-canvas" {
			t.Fatalf("pattern = %+v", slide.Pattern)
		}
		values := string(slide.Pattern.Values)
		// The canvas reads as the canvas, whatever the author called its boxes.
		for _, want := range []string{"Key partners", "Value propositions", "Revenue streams"} {
			if !strings.Contains(values, want) {
				t.Errorf("the canvas lost the heading %q: %s", want, values)
			}
		}
	})
}

// An author says "high", not 0.85.
func TestFrameworkIntensityAcceptsWords(t *testing.T) {
	body := frameworkBodyFor("porters_five_forces", map[string]any{
		"rivalry":      map[string]any{"items": []any{"Three clearers"}, "intensity": "high"},
		"new_entrants": map[string]any{"items": []any{"High barrier"}, "intensity": 0.2},
		"substitutes":  []any{"In-house settlement"},
		"suppliers":    []any{"Two vendors"},
		"buyers":       []any{"Ten banks"},
	})
	slide, _, err := CompileFramework(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	diagram := findDiagram(t, slide)
	rivalry := diagram.Data["rivalry"].(map[string]any)
	if got, _ := rivalry["intensity"].(float64); got < 0.67 {
		t.Errorf("\"high\" became %v, want an intensity the renderer reads as high", rivalry["intensity"])
	}
	entrants := diagram.Data["new_entrants"].(map[string]any)
	if got, _ := entrants["intensity"].(float64); got != 0.2 {
		t.Errorf("intensity = %v, want 0.2", entrants["intensity"])
	}
}

// A framework missing a part is not that framework, so it degrades rather than
// drawing an empty quadrant — and the finding names what is missing.
func TestFrameworkDegradesWithAReason(t *testing.T) {
	cases := []struct {
		name   string
		body   map[string]any
		reason string
	}{
		{
			name:   "half a swot",
			body:   frameworkBodyFor("swot", map[string]any{"strengths": []any{"A"}, "weaknesses": []any{"B"}}),
			reason: "missing opportunities, threats",
		},
		{
			name:   "a framework this kind does not draw",
			body:   frameworkBodyFor("pestel", map[string]any{"political": []any{"A"}}),
			reason: `names "pestel"`,
		},
		{
			name:   "no framework named",
			body:   map[string]any{"sections": swotSections()},
			reason: "does not say which framework",
		},
		{
			name: "a section written as a paragraph",
			body: frameworkBodyFor("swot", map[string]any{
				"strengths": []any{strings.Repeat("a", 210)}, "weaknesses": []any{"B"},
				"opportunities": []any{"C"}, "threats": []any{"D"},
			}),
			reason: "210-character item under strengths",
		},
		{
			name: "a section with eleven items",
			body: frameworkBodyFor("swot", map[string]any{
				"strengths":     []any{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
				"weaknesses":    []any{"B"},
				"opportunities": []any{"C"}, "threats": []any{"D"},
			}),
			reason: "has 11 items under strengths",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if over := FrameworkOverBudget(c.body); !strings.Contains(over, c.reason) {
				t.Errorf("FrameworkOverBudget = %q, want it to mention %q", over, c.reason)
			}
			slide, _, err := CompileFramework(Input{Body: c.body})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if slide.Pattern != nil {
				t.Errorf("expected the bullet fallback, got pattern %s", slide.Pattern.Name)
			}
			if len(slide.Content) == 0 {
				t.Error("the fallback lost the sections entirely")
			}
		})
	}
}

// The degrade keeps each part under its own heading, so an incomplete
// framework still reads as the framework.
func TestFrameworkFallbackGroupsBySection(t *testing.T) {
	body := frameworkBodyFor("swot", map[string]any{
		"strengths":  []any{"Two clearers migrated", "Good regulator relationship"},
		"weaknesses": []any{"Manual reconciliation"},
	})
	slide, _, err := CompileFramework(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	encoded, err := json.Marshal(slide.Content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	for _, want := range []string{
		"Strengths — Two clearers migrated; Good regulator relationship",
		"Weaknesses — Manual reconciliation",
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the fallback dropped %q: %s", want, encoded)
		}
	}
}

// The spellings an author reaches for all resolve, and sections work at the top
// level as well as under "sections".
func TestFrameworkNameAndSectionResolution(t *testing.T) {
	for spelling, want := range map[string]string{
		"SWOT": frameworkSWOT, "swot_analysis": frameworkSWOT,
		"Porter's Five Forces": frameworkPorters, "five_forces": frameworkPorters,
		"BMC": frameworkBMC, "business model canvas": frameworkBMC,
	} {
		if got := FrameworkName(map[string]any{"framework": spelling}); got != want {
			t.Errorf("%q resolved to %q, want %q", spelling, got, want)
		}
	}
	for _, key := range []string{"framework", "type", "model"} {
		if got := FrameworkName(map[string]any{key: "swot"}); got != frameworkSWOT {
			t.Errorf("%s: resolved %q", key, got)
		}
	}
	// Sections at the top level, not nested.
	body := map[string]any{"framework": "swot"}
	for k, v := range swotSections() {
		body[k] = v
	}
	if n := UsableFrameworkSectionCount(body); n != 4 {
		t.Errorf("top-level sections resolved %d parts, want 4", n)
	}
	// A section written as one string rather than a list.
	single := frameworkBodyFor("swot", map[string]any{
		"strengths": "Two clearers migrated", "weaknesses": []any{"B"},
		"opportunities": []any{"C"}, "threats": []any{"D"},
	})
	if got := FrameworkContent(single)["strengths"]; len(got) != 1 || got[0] != "Two clearers migrated" {
		t.Errorf("a single-string section resolved to %v", got)
	}
}

// findDiagram returns the slide's diagram content block.
func findDiagram(t *testing.T, slide *deckinput.SlideInput) *types.DiagramSpec {
	t.Helper()
	for _, c := range slide.Content {
		if c.DiagramValue != nil {
			return c.DiagramValue
		}
	}
	t.Fatalf("slide carries no diagram: %+v", slide.Content)
	return nil
}
