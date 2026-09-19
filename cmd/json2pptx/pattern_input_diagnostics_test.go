package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-20jm. The dogfood run (.claude/review-evidence/r2/dogfood-c,
// finding d.3) sent six pattern slides with the field names an agent naturally
// reaches for. Every one came back as a single INPUT.INVALID_SLIDE whose message
// was "invalid slide specification: slide 1: pattern: pattern \"process-flow\":
// invalid values: json: cannot unmarshal string into Go struct field
// ProcessFlowValues.steps of type patterns.ProcessFlowStep" — Go type names, no
// path, no suggestion, no next step. These tests replay all six through the
// generate path and pin what comes back.

// patternSlideDeck builds a one-pattern deck the way an MCP caller would.
func patternSlideDeck(t *testing.T, name, values string) []SlideInput {
	t.Helper()
	raw := `[{"layout_id":"content-slide","content":[{"placeholder_id":"title","type":"text","text_value":"T"}],` +
		`"pattern":{"name":"` + name + `","values":` + values + `}}]`
	var slides []SlideInput
	if err := json.Unmarshal([]byte(raw), &slides); err != nil {
		t.Fatalf("unmarshal slides: %v", err)
	}
	return slides
}

func patternDiagLayouts() []types.LayoutMetadata {
	return []types.LayoutMetadata{{
		ID: "content-slide", Name: "Content",
		Placeholders: []types.PlaceholderInfo{{ID: "title", Type: types.PlaceholderTitle}},
	}}
}

// generatePatternDiagnostics runs the same conversion generate_presentation runs
// and returns the diagnostics its handler would report.
func generatePatternDiagnostics(t *testing.T, name, values string) []diagnostics.Diagnostic {
	t.Helper()
	slides := patternSlideDeck(t, name, values)
	_, _, _, err := convertPresentationSlides(slides, patternDiagLayouts(),
		validationDefaultSlideWidthEMU, validationDefaultSlideHeightEMU,
		nil, nil, "", &GridDiagramContext{}, false)
	if err == nil {
		t.Fatalf("pattern %q with values %s was accepted; the repro is stale", name, values)
	}
	ds := slidePatternInputDiagnostics(err)
	if len(ds) == 0 {
		t.Fatalf("no per-field diagnostics for %q: %v", name, err)
	}
	return ds
}

func TestGeneratePatternFailuresArePathAddressed(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		values     string
		wantCode   string
		wantPath   string
		wantSubstr string
		wantDidYou string
		wantTool   string
	}{
		{
			name:       "team-bios title not role",
			pattern:    "team-bios",
			values:     `{"members":[{"name":"Dana","title":"VP"}]}`,
			wantCode:   patterns.ErrCodePatternUnknownField,
			wantPath:   "/slides/0/pattern/values/members/0/title",
			wantDidYou: "role",
			wantTool:   "show_pattern",
		},
		{
			name:       "process-flow steps as strings",
			pattern:    "process-flow",
			values:     `{"steps":["A","B","C"]}`,
			wantCode:   patterns.ErrCodeInvalidShape,
			wantPath:   "/slides/0/pattern/values/steps/0",
			wantSubstr: "must be an object",
			wantTool:   "show_pattern",
		},
		{
			name:       "timeline-horizontal values wrapped in an object",
			pattern:    "timeline-horizontal",
			values:     `{"stops":[{"label":"a"},{"label":"b"},{"label":"c"}]}`,
			wantCode:   patterns.ErrCodeInvalidShape,
			wantPath:   "/slides/0/pattern/values",
			wantSubstr: `"stops" is never read`,
			wantTool:   "show_pattern",
		},
		{
			name:       "phase-roadmap label not name",
			pattern:    "phase-roadmap",
			values:     `{"phases":[{"label":"P1","date":"Q1"},{"label":"P2","date":"Q2"},{"label":"P3","date":"Q3"}]}`,
			wantCode:   patterns.ErrCodePatternUnknownField,
			wantPath:   "/slides/0/pattern/values/phases/0/label",
			wantDidYou: "name",
			wantTool:   "show_pattern",
		},
		{
			name:       "unknown pattern name",
			pattern:    "kpi-four-up",
			values:     `[]`,
			wantCode:   diagnostics.CodeUnknownPattern,
			wantPath:   "/slides/0/pattern/name",
			wantDidYou: "kpi-4up",
			wantTool:   "show_pattern",
		},
		{
			name:       "comparison-2col columns not rows",
			pattern:    "comparison-2col",
			values:     `{"columns":[{"header":"A","items":["x"]},{"header":"B","items":["y"]}]}`,
			wantCode:   patterns.ErrCodePatternUnknownField,
			wantPath:   "/slides/0/pattern/values/columns",
			wantDidYou: "rows",
			wantTool:   "show_pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := generatePatternDiagnostics(t, tt.pattern, tt.values)

			var d *diagnostics.Diagnostic
			for i := range ds {
				if ds[i].Path == tt.wantPath {
					d = &ds[i]
				}
			}
			if d == nil {
				t.Fatalf("no finding at %s; got %v", tt.wantPath, diagPaths(ds))
			}
			if d.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", d.Code, tt.wantCode)
			}
			if d.Severity != diagnostics.SeverityError {
				t.Errorf("severity = %q, want error", d.Severity)
			}
			if !strings.HasPrefix(d.Message, "slide 1: ") {
				t.Errorf("message %q does not name the slide", d.Message)
			}
			if tt.wantSubstr != "" && !strings.Contains(d.Message, tt.wantSubstr) {
				t.Errorf("message %q does not mention %q", d.Message, tt.wantSubstr)
			}
			if tt.wantDidYou != "" {
				if d.Fix == nil {
					t.Fatalf("no fix on %s", d.Code)
				}
				if got := d.Fix.Params["did_you_mean"]; got != tt.wantDidYou {
					t.Errorf("did_you_mean = %v, want %q", got, tt.wantDidYou)
				}
			}
			if d.NextToolCall == nil || d.NextToolCall.Tool != tt.wantTool {
				t.Errorf("next_tool_call = %+v, want tool %q", d.NextToolCall, tt.wantTool)
			}
			// Every finding in the set must be addressable and Go-type-free.
			for _, dd := range ds {
				if !strings.HasPrefix(dd.Path, "/slides/0/pattern") {
					t.Errorf("finding path %q is not under the pattern", dd.Path)
				}
				for _, leak := range []string{"patterns.", "Go struct", "Go value", "json: cannot"} {
					if strings.Contains(dd.Message, leak) {
						t.Errorf("message leaks %q: %s", leak, dd.Message)
					}
				}
			}
		})
	}
}

// A fix has to address the same place evidence.path does; a pattern-relative
// "values.members[0].title" is not something an agent holding the whole deck can
// apply.
func TestPatternFixPathIsDeckAbsolute(t *testing.T) {
	ds := generatePatternDiagnostics(t, "team-bios", `{"members":[{"name":"Dana","title":"VP"}]}`)
	for _, d := range ds {
		if d.Fix == nil {
			continue
		}
		p, ok := d.Fix.Params["path"].(string)
		if !ok {
			continue
		}
		if p != d.Path {
			t.Errorf("fix path %q != evidence path %q", p, d.Path)
		}
	}
}

// Several wrong fields means several findings — the old path newline-joined them
// into one message, so an agent could only fix one per round trip.
func TestPatternFailuresAreOnePerField(t *testing.T) {
	ds := generatePatternDiagnostics(t, "phase-roadmap",
		`{"phases":[{"label":"P1","date":"Q1"},{"label":"P2","date":"Q2"},{"label":"P3","date":"Q3"}]}`)
	if len(ds) != 6 {
		t.Errorf("want 6 findings (3 phases × label + date), got %d: %v", len(ds), diagPaths(ds))
	}
	seen := map[string]bool{}
	for _, d := range ds {
		if seen[d.Path] {
			t.Errorf("duplicate finding at %s", d.Path)
		}
		seen[d.Path] = true
		if strings.Contains(d.Message, "\n") {
			t.Errorf("message carries more than one problem: %q", d.Message)
		}
	}
}

// A pattern inside a shape_grid cell keeps its coordinates, so the finding
// addresses the cell rather than the whole grid.
func TestNestedCellPatternFindingsKeepCellCoordinates(t *testing.T) {
	grid := &ShapeGridInput{
		Rows: []jsonschema.GridRowInput{{
			Cells: []*jsonschema.GridCellInput{
				{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect"}},
				{Pattern: json.RawMessage(`{"name":"team-bios","values":{"members":[{"name":"Dana","title":"VP"}]}}`)},
			},
		}},
	}
	err := expandNestedCellPatterns(grid, patterns.ExpandContext{
		SlideWidth: validationDefaultSlideWidthEMU, SlideHeight: validationDefaultSlideHeightEMU,
	}, patterns.Default())
	if err == nil {
		t.Fatal("nested pattern with a dropped field was accepted")
	}
	ds := patternInputDiagnostics(err, "/slides/2/shape_grid", "slide 3")
	if len(ds) != 1 {
		t.Fatalf("want 1 finding, got %v", diagPaths(ds))
	}
	want := "/slides/2/shape_grid/rows/0/cells/1/pattern/values/members/0/title"
	if ds[0].Path != want {
		t.Errorf("path = %q, want %q", ds[0].Path, want)
	}
}

func TestDottedPathToPointer(t *testing.T) {
	tests := map[string]string{
		"values":                            "values",
		"values.members[0].role":            "values/members/0/role",
		"values[2].small":                   "values/2/small",
		"cell_overrides[3].align":           "cell_overrides/3/align",
		"overrides.rag_colors.green":        "overrides/rag_colors/green",
		"rows[1].cells[0].pattern.values.x": "rows/1/cells/0/pattern/values/x",
	}
	for in, want := range tests {
		if got := dottedPathToPointer(in); got != want {
			t.Errorf("dottedPathToPointer(%q) = %q, want %q", in, got, want)
		}
	}
}

// Pattern.Validate reports values-relative paths ("members[0].role"); they are
// rooted once, at harvest, so no consumer downstream has to special-case them.
func TestRootPatternFindingPaths(t *testing.T) {
	in := []*patterns.ValidationError{
		{Path: "members[0].role"},
		{Path: "values[2].big"},
		{Path: "overrides.style"},
		{Path: "cell_overrides[0]"},
		{Path: "name"},
		{Path: ""},
	}
	want := []string{"values.members[0].role", "values[2].big", "overrides.style", "cell_overrides[0]", "name", "values"}
	got := rootPatternFindingPaths(in)
	for i := range want {
		if got[i].Path != want[i] {
			t.Errorf("path[%d] = %q, want %q", i, got[i].Path, want[i])
		}
	}
	// The originals must not be mutated: the same findings also reach the CLI.
	if in[0].Path != "members[0].role" {
		t.Errorf("input finding was mutated: %q", in[0].Path)
	}
}

func diagPaths(ds []diagnostics.Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Code + "@" + d.Path
	}
	return out
}
