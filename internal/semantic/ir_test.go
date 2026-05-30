package semantic

import (
	"os"
	"path/filepath"
	"testing"
)

// loadBoardUpdateIR parses the shared fixture and normalizes it to a DeckIR.
func loadBoardUpdateIR(t *testing.T) *DeckIR {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "board_update.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, ds := Parse("board_update.yaml", data)
	if ds.HasErrors() {
		t.Fatalf("unexpected parse diagnostics: %v", ds)
	}
	return Normalize(spec)
}

func TestNormalizeProducesNonEmptyIR(t *testing.T) {
	ir := loadBoardUpdateIR(t)

	if ir == nil {
		t.Fatal("expected non-nil IR")
	}
	if ir.Title != "Q2 Board Update" {
		t.Errorf("ir.Title = %q, want %q", ir.Title, "Q2 Board Update")
	}
	if ir.Archetype != ArchetypeBoardUpdate {
		t.Errorf("ir.Archetype = %q, want %q", ir.Archetype, ArchetypeBoardUpdate)
	}
	if got, want := len(ir.Slides), 5; got != want {
		t.Fatalf("len(ir.Slides) = %d, want %d", got, want)
	}
	if ir.SourceMap == nil {
		t.Error("expected an initialized (non-nil) SourceMap on the IR")
	}
	if ir.Rhythm.SlideCount != 5 {
		t.Errorf("rhythm.SlideCount = %d, want 5", ir.Rhythm.SlideCount)
	}
}

func TestNormalizeSlidePlans(t *testing.T) {
	ir := loadBoardUpdateIR(t)

	// Source order: title, executive_summary, kpi_snapshot, chart_insight, closing.
	wantRole := []NarrativeRole{RoleOpening, RoleSummary, RoleEvidence, RoleEvidence, RoleClosing}
	wantFamily := []VisualFamily{FamilyStructural, FamilyText, FamilyKPI, FamilyChart, FamilyStructural}
	for i, s := range ir.Slides {
		if s.SourceIndex != i {
			t.Errorf("slides[%d].SourceIndex = %d, want %d", i, s.SourceIndex, i)
		}
		if s.Role != wantRole[i] {
			t.Errorf("slides[%d].Role = %q, want %q", i, s.Role, wantRole[i])
		}
		if s.Visual.Family != wantFamily[i] {
			t.Errorf("slides[%d].Visual.Family = %q, want %q", i, s.Visual.Family, wantFamily[i])
		}
	}

	// The KPI slide declares two KPIs, so it should plan kpi-2up.
	if got := ir.Slides[2].Visual.Pattern; got != "kpi-2up" {
		t.Errorf("kpi slide pattern = %q, want kpi-2up", got)
	}
	// The chart_insight slide carries its takeaway as "insight".
	if got := ir.Slides[3].Takeaway; got != "Sustained growth across all segments." {
		t.Errorf("chart slide takeaway = %q, want the insight text", got)
	}
	// Structural slides plan a layout but no pattern.
	if got := ir.Slides[0].Visual.Layout; got != "title" {
		t.Errorf("title slide layout = %q, want title", got)
	}
	if got := ir.Slides[0].Visual.Pattern; got != "" {
		t.Errorf("title slide pattern = %q, want empty", got)
	}
}

func TestKPIPatternScalesWithCount(t *testing.T) {
	cases := []struct {
		count int
		want  string
	}{
		{1, "kpi-2up"},
		{2, "kpi-2up"},
		{3, "kpi-3up"},
		{4, "kpi-4up"},
		{5, "kpi-5up"},
		{6, "kpi-6up"},
		{9, "kpi-6up"},
	}
	for _, tc := range cases {
		kpis := make([]any, tc.count)
		for i := range kpis {
			kpis[i] = map[string]any{"label": "x", "value": "1"}
		}
		got := kpiPattern(map[string]any{"kpis": kpis})
		if got != tc.want {
			t.Errorf("kpiPattern(%d) = %q, want %q", tc.count, got, tc.want)
		}
	}
}

func TestNormalizeUnknownKindIsPassthrough(t *testing.T) {
	spec := &DeckSpec{
		Meta:   DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: SlideKind("bogus"), Body: map[string]any{"title": "x"}}},
	}
	ir := Normalize(spec)
	if got := ir.Slides[0].Role; got != RolePassthrough {
		t.Errorf("unknown kind role = %q, want passthrough", got)
	}
	if got := ir.Slides[0].Visual.Family; got != FamilyRaw {
		t.Errorf("unknown kind family = %q, want raw", got)
	}
}

func TestNormalizeNilSpecIsSafe(t *testing.T) {
	ir := Normalize(nil)
	if ir == nil {
		t.Fatal("Normalize(nil) must return a non-nil IR")
	}
	if ir.SourceMap == nil {
		t.Error("Normalize(nil) must attach an initialized SourceMap")
	}
	if ir.Rhythm.Families == nil || ir.Rhythm.Densities == nil {
		t.Error("Normalize(nil) must initialize the rhythm maps")
	}
	// Explain must not panic on the empty IR.
	if exp := ir.Explain(); len(exp.Slides) != 0 {
		t.Errorf("empty IR explanation has %d slides, want 0", len(exp.Slides))
	}
}

func TestComputeRhythmTallies(t *testing.T) {
	ir := loadBoardUpdateIR(t)
	if got := ir.Rhythm.Families[FamilyStructural]; got != 2 {
		t.Errorf("structural family count = %d, want 2 (title + closing)", got)
	}
	if got := ir.Rhythm.Densities[DensityLight]; got != 2 {
		t.Errorf("light density count = %d, want 2", got)
	}
	total := 0
	for _, n := range ir.Rhythm.Families {
		total += n
	}
	if total != ir.Rhythm.SlideCount {
		t.Errorf("family counts sum to %d, want SlideCount %d", total, ir.Rhythm.SlideCount)
	}
}

func TestExplainReturnsPlannedDecisions(t *testing.T) {
	exp := ExplainSpec(mustParseBoardUpdate(t))

	if exp.Archetype != ArchetypeBoardUpdate {
		t.Errorf("explanation archetype = %q, want %q", exp.Archetype, ArchetypeBoardUpdate)
	}
	if len(exp.Slides) != 5 {
		t.Fatalf("explanation has %d slides, want 5", len(exp.Slides))
	}
	// Every slide explanation must carry kind, visual family, density, and a
	// planned pattern or layout.
	for i, s := range exp.Slides {
		if s.Kind == "" {
			t.Errorf("slides[%d] explanation missing kind", i)
		}
		if s.VisualFamily == "" {
			t.Errorf("slides[%d] explanation missing visual family", i)
		}
		if s.Density == "" {
			t.Errorf("slides[%d] explanation missing density", i)
		}
		if s.Pattern == "" && s.Layout == "" {
			t.Errorf("slides[%d] explanation has neither pattern nor layout", i)
		}
	}
}

// mustParseBoardUpdate parses the shared fixture, failing the test on error.
func mustParseBoardUpdate(t *testing.T) *DeckSpec {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "board_update.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, ds := Parse("board_update.yaml", data)
	if ds.HasErrors() {
		t.Fatalf("unexpected parse diagnostics: %v", ds)
	}
	return spec
}
