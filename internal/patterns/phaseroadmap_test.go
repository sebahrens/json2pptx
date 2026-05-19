package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhaseRoadmap_Registration(t *testing.T) {
	p, ok := Default().Get("phase-roadmap")
	if !ok {
		t.Fatal("expected phase-roadmap to be registered in default registry")
	}
	if p.Name() != "phase-roadmap" {
		t.Errorf("Name() = %q, want %q", p.Name(), "phase-roadmap")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
	if !strings.Contains(p.UseWhen(), "prefer") {
		t.Errorf("UseWhen() should contain contrastive language: %q", p.UseWhen())
	}
}

func validPhaseRoadmapValues() *PhaseRoadmapValues {
	return &PhaseRoadmapValues{
		Phases: []PhaseRoadmapPhase{
			{Name: "Plan", DateLabel: "Mar–Apr 2025", Description: "Define scope and governance."},
			{Name: "Build", DateLabel: "May–Jul 2025", Description: "Implement pilot.", Active: true},
			{Name: "Scale", DateLabel: "Aug–Oct 2025", Description: "Roll out to BUs."},
			{Name: "Optimize", DateLabel: "Nov–Dec 2025", Description: "Tune cost, capture lessons."},
		},
	}
}

func TestPhaseRoadmap_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	if err := p.Validate(validPhaseRoadmapValues(), nil, nil); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestPhaseRoadmap_Validate_TooFewPhases(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	v := &PhaseRoadmapValues{
		Phases: []PhaseRoadmapPhase{{Name: "Plan"}, {Name: "Build"}},
	}
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 3 phases")
	}
	if !strings.Contains(err.Error(), "at least 3 items") {
		t.Errorf("expected min_items error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "timeline-horizontal") {
		t.Errorf("expected sibling hint mentioning timeline-horizontal, got: %v", err)
	}
}

func TestPhaseRoadmap_Validate_TooManyPhases(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	v := &PhaseRoadmapValues{
		Phases: []PhaseRoadmapPhase{
			{Name: "1"}, {Name: "2"}, {Name: "3"}, {Name: "4"}, {Name: "5"}, {Name: "6"}, {Name: "7"},
		},
	}
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for more than 6 phases")
	}
	if !strings.Contains(err.Error(), "at most 6") {
		t.Errorf("expected max_items error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "roadmap-phased") {
		t.Errorf("expected sibling hint mentioning roadmap-phased, got: %v", err)
	}
}

func TestPhaseRoadmap_Validate_MissingName(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	v := &PhaseRoadmapValues{
		Phases: []PhaseRoadmapPhase{
			{Name: ""},
			{Name: "Build"},
			{Name: "Scale"},
		},
	}
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for missing phase name")
	}
	if !strings.Contains(err.Error(), "phases[0].name is required") {
		t.Errorf("expected required-field error, got: %v", err)
	}
}

func TestPhaseRoadmap_Validate_FieldLengthLimits(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	cases := []struct {
		name    string
		mutate  func(*PhaseRoadmapValues)
		wantErr string
	}{
		{
			"name_too_long",
			func(v *PhaseRoadmapValues) { v.Phases[0].Name = strings.Repeat("x", 41) },
			"exceeds maxLength 40",
		},
		{
			"date_label_too_long",
			func(v *PhaseRoadmapValues) { v.Phases[0].DateLabel = strings.Repeat("d", 31) },
			"exceeds maxLength 30",
		},
		{
			"description_too_long",
			func(v *PhaseRoadmapValues) { v.Phases[0].Description = strings.Repeat("b", 161) },
			"exceeds maxLength 160",
		},
		{
			"milestone_too_long",
			func(v *PhaseRoadmapValues) { v.Phases[0].Milestone = strings.Repeat("m", 61) },
			"exceeds maxLength 60",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := validPhaseRoadmapValues()
			tc.mutate(v)
			err := p.Validate(v, nil, nil)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestPhaseRoadmap_Validate_MultipleActive(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	v := validPhaseRoadmapValues()
	v.Phases[0].Active = true
	v.Phases[1].Active = true
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error when multiple phases are active")
	}
	if !strings.Contains(err.Error(), "at most one phase") {
		t.Errorf("expected active-count error, got: %v", err)
	}
}

func TestPhaseRoadmap_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	v := validPhaseRoadmapValues()
	// 4 phases, no milestones: 1 + 3*4 = 13 cells (indices 0..12).
	overrides := map[int]any{99: &PhaseRoadmapCellOverride{AccentBar: true}}
	err := p.Validate(v, nil, overrides)
	if err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected out_of_range error, got: %v", err)
	}
}

func TestPhaseRoadmap_Validate_CellOverrideBadKey(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	v := validPhaseRoadmapValues()
	overrides := map[int]any{
		0: &struct {
			BadKey string `json:"bad_key"`
		}{BadKey: "nope"},
	}
	err := p.Validate(v, nil, overrides)
	if err == nil {
		t.Fatal("expected validation error for bad cell override key")
	}
	if !strings.Contains(err.Error(), `unknown key "bad_key"`) {
		t.Errorf("expected unknown_key error, got: %v", err)
	}
}

func TestPhaseRoadmap_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	grid, err := p.Expand(ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, validPhaseRoadmapValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	// Without milestones: phase row + timeline + date row + description row = 4 rows.
	if got := len(grid.Rows); got != 4 {
		t.Fatalf("expected 4 rows without milestones, got %d", got)
	}
	if got := len(grid.Rows[0].Cells); got != 4 {
		t.Errorf("expected 4 phase cells, got %d", got)
	}
	// Timeline row spans all phases via a single colspan cell.
	if len(grid.Rows[1].Cells) != 1 {
		t.Errorf("expected 1 timeline cell (spanning all columns), got %d", len(grid.Rows[1].Cells))
	}
	if grid.Rows[1].Cells[0].ColSpan != 4 {
		t.Errorf("expected timeline ColSpan = 4, got %d", grid.Rows[1].Cells[0].ColSpan)
	}
	if got := len(grid.Rows[2].Cells); got != 4 {
		t.Errorf("expected 4 date cells, got %d", got)
	}
	if got := len(grid.Rows[3].Cells); got != 4 {
		t.Errorf("expected 4 description cells, got %d", got)
	}

	// Columns header set from phase count.
	var cols int
	if err := json.Unmarshal(grid.Columns, &cols); err != nil {
		t.Fatalf("columns unmarshal: %v", err)
	}
	if cols != 4 {
		t.Errorf("columns = %d, want 4", cols)
	}
}

func TestPhaseRoadmap_Expand_ActivePhaseGetsAccent(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	grid, err := p.Expand(ExpandContext{}, validPhaseRoadmapValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// In valid values, phase index 1 is active and should fill with accent1.
	// Non-active phases should fill with dk1.
	for i, cell := range grid.Rows[0].Cells {
		var fill string
		if err := json.Unmarshal(cell.Shape.Fill, &fill); err != nil {
			t.Fatalf("cell[%d] fill unmarshal: %v", i, err)
		}
		if i == 1 {
			if fill != "accent1" {
				t.Errorf("active cell[%d] fill = %q, want accent1", i, fill)
			}
		} else {
			if fill != "dk1" {
				t.Errorf("inactive cell[%d] fill = %q, want dk1", i, fill)
			}
		}
	}
}

func TestPhaseRoadmap_Expand_AccentOverride(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	ovr := &PhaseRoadmapOverrides{Accent: "accent3"}
	grid, err := p.Expand(ExpandContext{}, validPhaseRoadmapValues(), ovr, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	var fill string
	if err := json.Unmarshal(grid.Rows[0].Cells[1].Shape.Fill, &fill); err != nil {
		t.Fatalf("active cell fill unmarshal: %v", err)
	}
	if fill != "accent3" {
		t.Errorf("active cell fill = %q, want accent3", fill)
	}
}

func TestPhaseRoadmap_Expand_MinPhases(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	v := &PhaseRoadmapValues{
		Phases: []PhaseRoadmapPhase{
			{Name: "Plan", DateLabel: "Q1"},
			{Name: "Build", DateLabel: "Q2"},
			{Name: "Launch", DateLabel: "Q3"},
		},
	}
	if err := p.Validate(v, nil, nil); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("expected 3 phase cells, got %d", len(grid.Rows[0].Cells))
	}
	if grid.Rows[1].Cells[0].ColSpan != 3 {
		t.Errorf("expected timeline ColSpan = 3, got %d", grid.Rows[1].Cells[0].ColSpan)
	}
}

func TestPhaseRoadmap_Expand_MaxPhases(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	v := &PhaseRoadmapValues{
		Phases: []PhaseRoadmapPhase{
			{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"}, {Name: "F"},
		},
	}
	if err := p.Validate(v, nil, nil); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if len(grid.Rows[0].Cells) != 6 {
		t.Errorf("expected 6 phase cells, got %d", len(grid.Rows[0].Cells))
	}
	if grid.Rows[1].Cells[0].ColSpan != 6 {
		t.Errorf("expected timeline ColSpan = 6, got %d", grid.Rows[1].Cells[0].ColSpan)
	}
}

func TestPhaseRoadmap_Expand_MilestoneRowOnlyWhenSet(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")

	// No milestone -> 4 rows.
	grid, err := p.Expand(ExpandContext{}, validPhaseRoadmapValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if got := len(grid.Rows); got != 4 {
		t.Errorf("expected 4 rows without milestones, got %d", got)
	}

	// Add a milestone on one phase -> 5 rows including a milestone row whose
	// cells without milestone text use no-fill, and cells with text use an
	// accent-filled roundRect.
	v := validPhaseRoadmapValues()
	v.Phases[2].Milestone = "Pilot go-live"
	grid, err = p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if got := len(grid.Rows); got != 5 {
		t.Fatalf("expected 5 rows with milestones, got %d", got)
	}
	milestoneRow := grid.Rows[4]
	if len(milestoneRow.Cells) != 4 {
		t.Fatalf("expected 4 milestone cells, got %d", len(milestoneRow.Cells))
	}
	// Cell 2 has the milestone; others should be empty (no-fill placeholders).
	if milestoneRow.Cells[2].Shape.Geometry != "roundRect" {
		t.Errorf("milestone cell geometry = %q, want roundRect", milestoneRow.Cells[2].Shape.Geometry)
	}
	var milestoneFill string
	if err := json.Unmarshal(milestoneRow.Cells[2].Shape.Fill, &milestoneFill); err != nil {
		t.Fatalf("milestone fill unmarshal: %v", err)
	}
	if milestoneFill != "accent1" {
		t.Errorf("milestone cell fill = %q, want accent1", milestoneFill)
	}
	for _, idx := range []int{0, 1, 3} {
		var fill string
		if err := json.Unmarshal(milestoneRow.Cells[idx].Shape.Fill, &fill); err != nil {
			t.Fatalf("empty milestone cell[%d] fill unmarshal: %v", idx, err)
		}
		if fill != "none" {
			t.Errorf("empty milestone cell[%d] fill = %q, want none", idx, fill)
		}
	}
}

func TestPhaseRoadmap_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	// Indexes for 4 phases without milestones:
	//   0..3   phases, 4 timeline, 5..8 dates, 9..12 descriptions.
	cellOverrides := map[int]any{
		0: &PhaseRoadmapCellOverride{AccentBar: true},
	}
	grid, err := p.Expand(ExpandContext{}, validPhaseRoadmapValues(), nil, cellOverrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid.Rows[0].Cells[0].AccentBar == nil {
		t.Error("expected cell[0] (first phase) to have an accent bar")
	}
	if grid.Rows[0].Cells[1].AccentBar != nil {
		t.Error("cell[1] should not have an accent bar")
	}
}

func TestPhaseRoadmap_Schema(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	s := p.Schema()
	if s == nil {
		t.Fatal("Schema() returned nil")
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("Schema marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Schema unmarshal: %v", err)
	}
	if m["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("missing $schema draft 2020-12")
	}
	if m["type"] != "object" {
		t.Errorf("root type = %v, want object", m["type"])
	}
}

func TestPhaseRoadmap_Taxonomy(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	tax := p.Taxonomy()
	if tax.Category != "structural" {
		t.Errorf("Category = %q, want %q", tax.Category, "structural")
	}
	if tax.DensityClass == "" {
		t.Error("DensityClass should be set")
	}
	if tax.AccentWeight == "" {
		t.Error("AccentWeight should be set")
	}
	if len(tax.NarrativeRole) == 0 {
		t.Error("NarrativeRole should be non-empty")
	}
	if len(tax.PairsWith) == 0 {
		t.Error("PairsWith should be non-empty")
	}
}

func TestPhaseRoadmap_Golden_Default(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	grid, err := p.Expand(ExpandContext{}, validPhaseRoadmapValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	got, err := json.MarshalIndent(grid, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	goldenPath := filepath.Join("testdata", "phase-roadmap", "default.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Log("golden file updated")
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("golden mismatch.\ngot:\n%s\nwant:\n%s", got, want)
	}
}
