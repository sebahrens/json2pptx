package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Run with JSON2PPTX_BUDGET_PROBE=1 go test ./cmd/json2pptx
// -run TestPatternBudgetProbe -v. It measures the actual fit collector against
// every bundled template, without adding a slow combinatorial sweep to normal
// tests. A clean result (no below-floor finding) has budget >= the probe size.
func TestPatternBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_BUDGET_PROBE") != "1" {
		t.Skip("manual budget calibration probe")
	}
	for count := 1; count <= 10; count++ {
		bindingName, bindingBudget := "", 201
		for _, name := range bmcProbeCellNames() {
			budget := bmcReadableBudget(t, name, count)
			t.Logf("bmc-canvas %-21s %2d bullets: %3d chars per bullet", name, count, budget)
			if budget < bindingBudget {
				bindingName, bindingBudget = name, budget
			}
		}
		t.Logf("bmc-canvas %d bullets: binding cell %s at %d chars", count, bindingName, bindingBudget)
	}

	pat, _ := patterns.Default().Get("pull-quote")
	values, note := schemaMaximumValues(pat)
	if note != "" {
		t.Fatal(note)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, name)
		input := &PresentationInput{Template: name, Slides: []SlideInput{{
			SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: pat.Name(), Values: encoded},
		}}}
		for _, finding := range collectReadabilityFindings(input, layouts, width, height) {
			t.Logf("pull-quote %s: %s", name, finding.Message)
		}
	}
}

// JSON2PPTX_DRIVER_BUDGET_PROBE=1 measures the driver-tree copy target for
// each legal uniform tree shape. The shortest result across templates binds.
func TestDriverTreeBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_DRIVER_BUDGET_PROBE") != "1" {
		t.Skip("manual driver-tree budget calibration probe")
	}
	for branches := 2; branches <= 4; branches++ {
		for leaves := 1; leaves <= 4; leaves++ {
			counts := make([]int, branches)
			for i := range counts {
				counts[i] = leaves
			}
			for _, annotated := range []bool{false, true} {
				for _, field := range []string{"root", "branch", "leaf", "annotation"} {
					if field == "annotation" && !annotated {
						continue
					}
					budget := driverTreeReadableBudget(t, counts, annotated, field)
					t.Logf("driver-tree %d branches x %d leaves, annotated=%t, %s: %d chars", branches, leaves, annotated, field, budget)
				}
			}
		}
	}
	for _, total := range []int{5, 7, 10, 11, 13, 14, 15} {
		counts := []int{1, 1}
		if total > 8 {
			counts = []int{1, 1, 1}
		}
		if total > 12 {
			counts = []int{1, 1, 1, 1}
		}
		remaining := total - len(counts)
		for i := range counts {
			add := min(3, remaining)
			counts[i] += add
			remaining -= add
		}
		for _, annotated := range []bool{false, true} {
			for _, field := range []string{"root", "branch", "leaf", "annotation"} {
				if field == "annotation" && !annotated {
					continue
				}
				budget := driverTreeReadableBudget(t, counts, annotated, field)
				t.Logf("driver-tree %d leaves, annotated=%t, %s: %d chars", total, annotated, field, budget)
			}
		}
	}
}

func TestDriverTreeSpanProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_DRIVER_SPAN_PROBE") != "1" {
		t.Skip("manual driver-tree span calibration probe")
	}
	for total := 2; total <= 14; total++ {
		for span := 1; span <= 4; span++ {
			remaining := total - span
			if remaining < 1 || remaining > 12 {
				continue
			}
			var counts []int
			for remaining > 0 {
				n := min(4, remaining)
				counts = append(counts, n)
				remaining -= n
			}
			counts = append(counts, span)
			if len(counts) > 4 {
				continue
			}
			for _, annotated := range []bool{false, true} {
				branch := driverTreeReadableBudget(t, counts, annotated, "branch")
				if annotated {
					annotation := driverTreeReadableBudget(t, counts, true, "annotation")
					t.Logf("total=%d span=%d annotated=true branch=%d annotation=%d", total, span, branch, annotation)
				} else {
					t.Logf("total=%d span=%d annotated=false branch=%d", total, span, branch)
				}
			}
		}
	}
}

func TestComparisonBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_COMPARISON_BUDGET_PROBE") != "1" {
		t.Skip("manual comparison budget calibration probe")
	}
	for rows := 1; rows <= 10; rows++ {
		for _, headers := range []bool{false, true} {
			for _, field := range []string{"body", "header"} {
				if field == "header" && !headers {
					continue
				}
				budget := comparisonReadableBudget(t, rows, headers, field)
				t.Logf("comparison-2col rows=%d headers=%t field=%s budget=%d", rows, headers, field, budget)
			}
		}
	}
}

func TestRoadmapPhasedBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_ROADMAP_BUDGET_PROBE") != "1" {
		t.Skip("manual roadmap-phased budget calibration probe")
	}
	for phases := 2; phases <= 8; phases++ {
		for workstreams := 2; workstreams <= 6; workstreams++ {
			for _, field := range []string{"phase", "name", "item"} {
				budget := roadmapPhasedReadableBudget(t, phases, workstreams, field)
				t.Logf("roadmap-phased phases=%d workstreams=%d field=%s budget=%d", phases, workstreams, field, budget)
			}
		}
	}
}

func TestTeamBiosBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_TEAM_BIOS_BUDGET_PROBE") != "1" {
		t.Skip("manual team-bios budget calibration probe")
	}
	for members := 1; members <= 8; members++ {
		for _, field := range []string{"name", "role", "bio"} {
			budget := teamBiosReadableBudget(t, members, field)
			t.Logf("team-bios members=%d field=%s budget=%d", members, field, budget)
		}
	}
}

func teamBiosReadableBudget(t *testing.T, members int, field string) int {
	t.Helper()
	type geometry struct {
		name          string
		layouts       []types.LayoutMetadata
		width, height int64
	}
	var geometries []geometry
	for _, name := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, name)
		geometries = append(geometries, geometry{name, layouts, width, height})
	}
	limit := map[string]int{"name": 60, "role": 80, "bio": 220}[field]
	clean := func(length int) bool {
		v := &patterns.TeamBiosValues{}
		for i := 0; i < members; i++ {
			v.Members = append(v.Members, patterns.TeamBiosMember{Name: "Jane Doe", Role: "Lead", Bio: "Short biography"})
		}
		copy := strings.Repeat("word ", length/5) + strings.Repeat("w", length%5)
		switch field {
		case "name":
			v.Members[0].Name = copy
		case "role":
			v.Members[0].Role = copy
		case "bio":
			v.Members[0].Bio = copy
		}
		encoded, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, geom := range geometries {
			input := &PresentationInput{Template: geom.name, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "team-bios", Values: encoded},
			}}}
			if len(collectReadabilityFindings(input, geom.layouts, geom.width, geom.height)) > 0 {
				return false
			}
		}
		return true
	}
	lo, hi := 0, limit
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if clean(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func roadmapPhasedReadableBudget(t *testing.T, phases, workstreams int, field string) int {
	t.Helper()
	type geometry struct {
		name          string
		layouts       []types.LayoutMetadata
		width, height int64
	}
	var geometries []geometry
	for _, name := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, name)
		geometries = append(geometries, geometry{name, layouts, width, height})
	}
	limit := map[string]int{"phase": 20, "name": 40, "item": 80}[field]
	clean := func(length int) bool {
		v := &patterns.RoadmapPhasedValues{}
		for i := 0; i < phases; i++ {
			v.Phases = append(v.Phases, "Q1")
		}
		for i := 0; i < workstreams; i++ {
			ws := patterns.RoadmapWorkstream{Name: "Team"}
			for j := 0; j < phases; j++ {
				ws.Items = append(ws.Items, "Task")
			}
			v.Workstreams = append(v.Workstreams, ws)
		}
		copy := strings.Repeat("word ", length/5) + strings.Repeat("w", length%5)
		switch field {
		case "phase":
			v.Phases[0] = copy
		case "name":
			v.Workstreams[0].Name = copy
		case "item":
			v.Workstreams[0].Items[0] = copy
		}
		encoded, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, geom := range geometries {
			input := &PresentationInput{Template: geom.name, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "roadmap-phased", Values: encoded},
			}}}
			if len(collectReadabilityFindings(input, geom.layouts, geom.width, geom.height)) > 0 {
				return false
			}
		}
		return true
	}
	lo, hi := 0, limit
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if clean(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func comparisonReadableBudget(t *testing.T, rowCount int, headers bool, field string) int {
	t.Helper()
	type geometry struct {
		name          string
		layouts       []types.LayoutMetadata
		width, height int64
	}
	var geometries []geometry
	for _, name := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, name)
		geometries = append(geometries, geometry{name, layouts, width, height})
	}
	limit := 200
	if field == "header" {
		limit = 60
	}
	clean := func(length int) bool {
		v := &patterns.Comparison2colValues{}
		if headers {
			v.Headers = [2]string{"Left", "Right"}
		}
		for i := 0; i < rowCount; i++ {
			v.Rows = append(v.Rows, patterns.Comparison2colRow{Left: "Item", Right: "Other"})
		}
		copy := strings.Repeat("word ", length/5) + strings.Repeat("w", length%5)
		if field == "header" {
			v.Headers[0] = copy
		} else {
			v.Rows[0].Left = copy
		}
		encoded, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, geom := range geometries {
			input := &PresentationInput{Template: geom.name, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "comparison-2col", Values: encoded},
			}}}
			if len(collectReadabilityFindings(input, geom.layouts, geom.width, geom.height)) > 0 {
				return false
			}
		}
		return true
	}
	lo, hi := 0, limit
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if clean(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func driverTreeReadableBudget(t *testing.T, counts []int, annotated bool, field string) int {
	t.Helper()
	type geometry struct {
		name          string
		layouts       []types.LayoutMetadata
		width, height int64
	}
	var geometries []geometry
	for _, name := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, name)
		geometries = append(geometries, geometry{name, layouts, width, height})
	}
	maxLen := map[string]int{"root": 60, "branch": 60, "leaf": 120, "annotation": 140}[field]
	clean := func(length int) bool {
		v := &patterns.DriverTreeValues{Root: patterns.DriverTreeNode{Label: "Root"}}
		for _, leaves := range counts {
			branch := patterns.DriverTreeBranch{Label: "Branch"}
			for j := 0; j < leaves; j++ {
				branch.Leaves = append(branch.Leaves, "Item")
			}
			if annotated {
				branch.Annotation = "Note"
			}
			v.Branches = append(v.Branches, branch)
		}
		copy := strings.Repeat("word ", length/5) + strings.Repeat("w", length%5)
		switch field {
		case "root":
			v.Root.Label = copy
		case "branch":
			v.Branches[len(v.Branches)-1].Label = copy
		case "leaf":
			v.Branches[0].Leaves[0] = copy
		case "annotation":
			v.Branches[len(v.Branches)-1].Annotation = copy
		}
		encoded, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, geom := range geometries {
			input := &PresentationInput{Template: geom.name, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "driver-tree", Values: encoded},
			}}}
			if len(collectReadabilityFindings(input, geom.layouts, geom.width, geom.height)) > 0 {
				return false
			}
		}
		return true
	}
	lo, hi := 0, maxLen
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if clean(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func bmcProbeCellNames() []string {
	return []string{"key_partners", "key_activities", "key_resources", "value_propositions", "customer_relations", "channels", "customer_segments", "cost_structure", "revenue_streams"}
}

func bmcReadableBudget(t *testing.T, cellName string, bulletCount int) int {
	t.Helper()
	type templateGeometry struct {
		name          string
		layouts       []types.LayoutMetadata
		width, height int64
	}
	geometries := make([]templateGeometry, 0, len(schemaMaximaTemplates))
	for _, name := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, name)
		geometries = append(geometries, templateGeometry{name, layouts, width, height})
	}
	clean := func(length int) bool {
		values := bmcProbeValues()
		cell := bmcProbeCell(values, cellName)
		cell.Bullets = make([]string, bulletCount)
		for i := range cell.Bullets {
			cell.Bullets[i] = strings.Repeat("word ", length/5) + strings.Repeat("w", length%5)
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		for _, geom := range geometries {
			input := &PresentationInput{Template: geom.name, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "bmc-canvas", Values: encoded},
			}}}
			if len(collectReadabilityFindings(input, geom.layouts, geom.width, geom.height)) > 0 {
				return false
			}
		}
		return true
	}
	lo, hi := 0, 200
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if clean(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func bmcProbeValues() *patterns.BMCCanvasValues {
	cell := func(header string) patterns.BMCCell {
		return patterns.BMCCell{Header: header, Bullets: []string{"base"}}
	}
	return &patterns.BMCCanvasValues{
		KeyPartners: cell("Key Partners"), KeyActivities: cell("Key Activities"),
		KeyResources: cell("Key Resources"), ValuePropositions: cell("Value Propositions"),
		CustomerRelations: cell("Customer Relations"), Channels: cell("Channels"),
		CustomerSegments: cell("Customer Segments"), CostStructure: cell("Cost Structure"),
		RevenueStreams: cell("Revenue Streams"),
	}
}

func bmcProbeCell(values *patterns.BMCCanvasValues, name string) *patterns.BMCCell {
	switch name {
	case "key_partners":
		return &values.KeyPartners
	case "key_activities":
		return &values.KeyActivities
	case "key_resources":
		return &values.KeyResources
	case "value_propositions":
		return &values.ValuePropositions
	case "customer_relations":
		return &values.CustomerRelations
	case "channels":
		return &values.Channels
	case "customer_segments":
		return &values.CustomerSegments
	case "cost_structure":
		return &values.CostStructure
	case "revenue_streams":
		return &values.RevenueStreams
	default:
		return nil
	}
}
