package patterns

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// journeyRisks is the go-slide-creator-ec74l case: six named risks on
// likelihood × impact, four of them "medium" on one axis.
func journeyRisks() *RiskHeatmapValues {
	return &RiskHeatmapValues{Items: []RiskHeatmapItem{
		{Name: "Cyber", Likelihood: "high", Impact: "high"},
		{Name: "Third-party outage", Likelihood: "medium", Impact: "high"},
		{Name: "Model risk", Likelihood: "low", Impact: "high"},
		{Name: "Conduct", Likelihood: "medium", Impact: "medium"},
		{Name: "Climate", Likelihood: "low", Impact: "medium"},
		{Name: "Fraud", Likelihood: "medium", Impact: "low"},
	}}
}

func riskHeatmapPattern(t *testing.T) Pattern {
	t.Helper()
	p, ok := Default().Get("risk-heatmap")
	if !ok {
		t.Fatal("risk-heatmap is not registered")
	}
	return p
}

// rhmGridCell returns the heat-map cell at a 1-based likelihood / impact.
func rhmGridCell(t *testing.T, grid *jsonschema.ShapeGridInput, size, likelihood, impact int) *jsonschema.GridCellInput {
	t.Helper()
	row := grid.Rows[size-impact]
	// The first display row also carries the row-spanning impact title.
	offset := 1
	if impact == size {
		offset = 2
	}
	return row.Cells[offset+likelihood-1]
}

func TestRiskHeatmap_MetadataAndSchema(t *testing.T) {
	p := riskHeatmapPattern(t)
	if err := p.Validate(p.(Exemplar).ExemplarValues(), nil, nil); err != nil {
		t.Fatalf("exemplar rejected: %v", err)
	}
	for _, sibling := range []string{"matrix-2x2", "capability-heatmap", "table-highlight"} {
		if !strings.Contains(p.UseWhen()+p.NotWhen(), sibling) {
			t.Errorf("use_when/not_when does not contrast with %s", sibling)
		}
	}
	data, err := json.Marshal(p.Schema())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"items"`, `"likelihood"`, `"impact"`, `"size"`, `"likelihood_levels"`, `"impact_levels"`, `"show_legend"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("schema missing %s", want)
		}
	}
}

func TestRiskHeatmap_PlacesEveryNamedRiskInItsCell(t *testing.T) {
	p := riskHeatmapPattern(t)
	v := journeyRisks()
	if err := p.Validate(v, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	levels := map[string]int{"low": 1, "medium": 2, "high": 3}
	for _, it := range v.Items {
		cell := rhmGridCell(t, grid, 3, levels[string(it.Likelihood)], levels[string(it.Impact)])
		if cell.Shape == nil || !strings.Contains(string(cell.Shape.Text), it.Name) {
			t.Errorf("%s is not in the likelihood %s / impact %s cell", it.Name, it.Likelihood, it.Impact)
		}
	}
	// "Third-party outage" and nothing else sits at medium / high: a medium
	// has a cell of its own instead of an axis line.
	mid := string(rhmGridCell(t, grid, 3, 2, 3).Shape.Text)
	if strings.Contains(mid, "Cyber") || strings.Contains(mid, "Model risk") {
		t.Errorf("medium / high cell holds a neighbour's risk: %s", mid)
	}
	// All nine cells are drawn, filled or not: the grid is the heat map.
	for impact := 1; impact <= 3; impact++ {
		for likelihood := 1; likelihood <= 3; likelihood++ {
			if c := rhmGridCell(t, grid, 3, likelihood, impact); c.Shape == nil || len(c.Shape.Fill) == 0 {
				t.Errorf("cell likelihood %d / impact %d has no fill", likelihood, impact)
			}
		}
	}
}

func TestRiskHeatmap_TierFillsFollowLikelihoodTimesImpact(t *testing.T) {
	p := riskHeatmapPattern(t)
	grid, err := p.Expand(ExpandContext{}, journeyRisks(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	fill := func(l, i int) string { return string(rhmGridCell(t, grid, 3, l, i).Shape.Fill) }
	low, medium, high := fill(1, 1), fill(2, 2), fill(3, 3)
	if low == medium || medium == high || low == high {
		t.Fatalf("tiers share a fill: low %s medium %s high %s", low, medium, high)
	}
	want := map[[2]int]string{
		{1, 1}: low, {2, 1}: low, {1, 2}: low,
		{3, 1}: medium, {2, 2}: medium, {1, 3}: medium,
		{3, 2}: high, {2, 3}: high, {3, 3}: high,
	}
	for at, f := range want {
		if got := fill(at[0], at[1]); got != f {
			t.Errorf("likelihood %d / impact %d fill = %s, want %s", at[0], at[1], got, f)
		}
	}
	// Tier colours come from the theme, never a literal.
	raw, _ := json.Marshal(grid)
	if hex := regexp.MustCompile(`#[0-9A-Fa-f]{6}`).FindString(string(raw)); hex != "" {
		t.Errorf("expansion carries a hard-coded colour %s", hex)
	}
}

func TestRiskHeatmap_TextNeverBelowTwelvePoint(t *testing.T) {
	p := riskHeatmapPattern(t)
	for _, size := range []int{3, 5} {
		v := journeyRisks()
		v.Size = size
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(grid)
		for _, m := range regexp.MustCompile(`"size":([0-9.]+)`).FindAllStringSubmatch(string(raw), -1) {
			if pt, _ := strconv.ParseFloat(m[1], 64); pt < 12 {
				t.Errorf("size %d: text set at %spt", size, m[1])
			}
		}
	}
}

func TestRiskHeatmap_LevelsAcceptNumbersWordsAndCustomLabels(t *testing.T) {
	p := riskHeatmapPattern(t)
	var v RiskHeatmapValues
	payload := `{"size":5,
		"likelihood_levels":["Rare","Unlikely","Possible","Likely","Almost certain"],
		"items":[
			{"name":"Cyber","likelihood":"Almost certain","impact":5},
			{"name":"Fraud","likelihood":2,"impact":"very low"},
			{"name":"Conduct","likelihood":"possible","impact":"Medium"}]}`
	if err := json.Unmarshal([]byte(payload), &v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := p.Validate(&v, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := p.Expand(ExpandContext{}, &v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, at := range map[string][2]int{"Cyber": {5, 5}, "Fraud": {2, 1}, "Conduct": {3, 3}} {
		if !strings.Contains(string(rhmGridCell(t, grid, 5, at[0], at[1]).Shape.Text), name) {
			t.Errorf("%s is not at likelihood %d / impact %d", name, at[0], at[1])
		}
	}
	raw, _ := json.Marshal(grid)
	if !strings.Contains(string(raw), "Almost certain") {
		t.Error("custom likelihood level label is not drawn on the axis")
	}
}

func TestRiskHeatmap_ValidateRejectsWhatItCannotPlace(t *testing.T) {
	p := riskHeatmapPattern(t)
	cases := []struct {
		name string
		mut  func(v *RiskHeatmapValues)
		want string
	}{
		{"no items", func(v *RiskHeatmapValues) { v.Items = nil }, "items"},
		{"unknown level", func(v *RiskHeatmapValues) { v.Items[0].Likelihood = "severe" }, "items[0].likelihood"},
		{"level off the grid", func(v *RiskHeatmapValues) { v.Items[1].Impact = "4" }, "items[1].impact"},
		{"size", func(v *RiskHeatmapValues) { v.Size = 4 }, "size"},
		{"level label count", func(v *RiskHeatmapValues) { v.ImpactLevels = []string{"Minor", "Major"} }, "impact_levels"},
		{"blank name", func(v *RiskHeatmapValues) { v.Items[2].Name = " " }, "items[2].name"},
		{"long name", func(v *RiskHeatmapValues) { v.Items[2].Name = strings.Repeat("x", 41) }, "items[2].name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := journeyRisks()
			tc.mut(v)
			err := p.Validate(v, nil, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want one naming %s", err, tc.want)
			}
		})
	}
}

func TestRiskHeatmap_CrowdedCellIsReportedNotShrunk(t *testing.T) {
	p := riskHeatmapPattern(t)
	warner := p.(PostExpandWarner)
	if w := warner.PostExpandWarnings(ExpandContext{}, journeyRisks(), nil); len(w) != 0 {
		t.Fatalf("six risks on a 3x3 warn: %v", w)
	}
	// Three risks in one cell still fit: the row takes the height it needs.
	three := journeyRisks()
	three.Items[2].Likelihood, three.Items[0].Likelihood = "medium", "medium"
	if w := warner.PostExpandWarnings(ExpandContext{}, three, nil); len(w) != 0 {
		t.Errorf("three risks in one cell warn: %v", w)
	}
	crowded := &RiskHeatmapValues{}
	for i := 0; i < 20; i++ {
		crowded.Items = append(crowded.Items, RiskHeatmapItem{Name: "A risk with a long descriptive name " + string(rune('A'+i)), Likelihood: "3", Impact: "3"})
	}
	if err := p.Validate(crowded, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	w := warner.PostExpandWarnings(ExpandContext{}, crowded, nil)
	if len(w) == 0 || !strings.HasPrefix(w[0], ErrCodeBodyTooLong+":") || !strings.Contains(w[0], "items[0].name") || !strings.Contains(w[0], "19 more risks") {
		t.Errorf("twenty risks in one cell: warnings = %v", w)
	}
}
