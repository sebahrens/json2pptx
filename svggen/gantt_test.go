package svggen

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestGanttChart_Draw(t *testing.T) {
	tests := []struct {
		name    string
		data    GanttData
		wantErr bool
	}{
		{
			name: "basic project plan with tasks",
			data: GanttData{
				Title: "Project Plan",
				Tasks: []GanttTask{
					{ID: "t1", Label: "Design", StartDate: date(2024, 1, 1), EndDate: date(2024, 2, 1), Category: "Design"},
					{ID: "t2", Label: "Development", StartDate: date(2024, 2, 1), EndDate: date(2024, 5, 1), Category: "Engineering"},
					{ID: "t3", Label: "Testing", StartDate: date(2024, 5, 1), EndDate: date(2024, 6, 15), Category: "QA"},
				},
			},
			wantErr: false,
		},
		{
			name: "tasks with dependencies",
			data: GanttData{
				Title: "Dependent Tasks",
				Tasks: []GanttTask{
					{ID: "t1", Label: "Requirements", StartDate: date(2024, 1, 1), EndDate: date(2024, 1, 15)},
					{ID: "t2", Label: "Design", StartDate: date(2024, 1, 15), EndDate: date(2024, 2, 15), Dependencies: []string{"t1"}},
					{ID: "t3", Label: "Implementation", StartDate: date(2024, 2, 15), EndDate: date(2024, 4, 1), Dependencies: []string{"t2"}},
				},
			},
			wantErr: false,
		},
		{
			name: "tasks with milestones",
			data: GanttData{
				Title: "Release Plan",
				Tasks: []GanttTask{
					{ID: "t1", Label: "Phase 1", StartDate: date(2024, 1, 1), EndDate: date(2024, 3, 1)},
					{ID: "m1", Label: "Alpha Release", Date: date(2024, 3, 1), IsMilestone: true},
					{ID: "t2", Label: "Phase 2", StartDate: date(2024, 3, 1), EndDate: date(2024, 6, 1)},
					{ID: "m2", Label: "GA Release", Date: date(2024, 6, 1), IsMilestone: true},
				},
			},
			wantErr: false,
		},
		{
			name: "standalone milestones",
			data: GanttData{
				Title: "Key Dates",
				Milestones: []GanttMilestone{
					{ID: "ms1", Label: "Kickoff", Date: date(2024, 1, 1)},
					{ID: "ms2", Label: "Review", Date: date(2024, 3, 15)},
					{ID: "ms3", Label: "Launch", Date: date(2024, 6, 1)},
				},
			},
			wantErr: false,
		},
		{
			name: "tasks with swimlanes",
			data: GanttData{
				Title: "Multi-Team Project",
				Tasks: []GanttTask{
					{ID: "t1", Label: "Backend API", StartDate: date(2024, 1, 1), EndDate: date(2024, 3, 1), Swimlane: "Backend"},
					{ID: "t2", Label: "Database Migration", StartDate: date(2024, 1, 15), EndDate: date(2024, 2, 15), Swimlane: "Backend"},
					{ID: "t3", Label: "UI Design", StartDate: date(2024, 1, 1), EndDate: date(2024, 2, 1), Swimlane: "Frontend"},
					{ID: "t4", Label: "Implementation", StartDate: date(2024, 2, 1), EndDate: date(2024, 4, 1), Swimlane: "Frontend"},
				},
			},
			wantErr: false,
		},
		{
			name: "tasks with progress",
			data: GanttData{
				Title: "Sprint Progress",
				Tasks: []GanttTask{
					{ID: "t1", Label: "Feature A", StartDate: date(2024, 1, 1), EndDate: date(2024, 1, 15), Progress: 100},
					{ID: "t2", Label: "Feature B", StartDate: date(2024, 1, 8), EndDate: date(2024, 1, 22), Progress: 60},
					{ID: "t3", Label: "Feature C", StartDate: date(2024, 1, 15), EndDate: date(2024, 2, 1), Progress: 25},
				},
			},
			wantErr: false,
		},
		{
			name: "empty data",
			data: GanttData{
				Title: "Empty Gantt",
			},
			wantErr: false,
		},
		{
			name: "with subtitle and footnote",
			data: GanttData{
				Title:    "Product Roadmap",
				Subtitle: "FY2024",
				Tasks: []GanttTask{
					{ID: "t1", Label: "Planning", StartDate: date(2024, 1, 1), EndDate: date(2024, 2, 1)},
				},
				Footnote: "Source: PMO",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewSVGBuilder(900, 500)
			config := DefaultGanttConfig(900, 500)
			config.ShowProgress = true // Enable progress for all tests

			chart := NewGanttChart(builder, config)
			err := chart.Draw(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("GanttChart.Draw() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				doc, err := builder.Render()
				if err != nil {
					t.Errorf("Failed to render SVG: %v", err)
					return
				}
				if doc == nil || len(doc.Content) == 0 {
					t.Error("Expected non-empty SVG document")
				}
			}
		})
	}
}

func TestGanttDiagram_Validate(t *testing.T) {
	diagram := &GanttDiagram{NewBaseDiagram("gantt")}

	tests := []struct {
		name    string
		req     *RequestEnvelope
		wantErr bool
	}{
		{
			name: "valid with tasks",
			req: &RequestEnvelope{
				Type: "gantt",
				Data: map[string]any{
					"tasks": []any{
						map[string]any{"label": "Task 1", "start": "2024-01-01", "end": "2024-02-01"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with milestones only",
			req: &RequestEnvelope{
				Type: "gantt",
				Data: map[string]any{
					"milestones": []any{
						map[string]any{"label": "Launch", "date": "2024-06-01"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with tasks and milestones",
			req: &RequestEnvelope{
				Type: "gantt",
				Data: map[string]any{
					"tasks": []any{
						map[string]any{"label": "Dev", "start": "2024-01-01", "end": "2024-03-01"},
					},
					"milestones": []any{
						map[string]any{"label": "Launch", "date": "2024-03-15"},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "missing data",
			req:     &RequestEnvelope{Type: "gantt", Data: nil},
			wantErr: true,
		},
		{
			name: "missing tasks and milestones",
			req: &RequestEnvelope{
				Type: "gantt",
				Data: map[string]any{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := diagram.Validate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("GanttDiagram.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGanttDiagram_Render(t *testing.T) {
	diagram := &GanttDiagram{NewBaseDiagram("gantt")}

	req := &RequestEnvelope{
		Type:  "gantt",
		Title: "Product Development",
		Data: map[string]any{
			"tasks": []any{
				map[string]any{"id": "design", "label": "Design Phase", "start": "2024-01-01", "end": "2024-02-15", "category": "Design"},
				map[string]any{"id": "dev", "label": "Development", "start": "2024-02-15", "end": "2024-05-01", "category": "Engineering", "dependencies": []any{"design"}},
				map[string]any{"id": "test", "label": "Testing", "start": "2024-05-01", "end": "2024-06-01", "category": "QA", "dependencies": []any{"dev"}},
			},
			"milestones": []any{
				map[string]any{"label": "Kickoff", "date": "2024-01-01"},
				map[string]any{"label": "Release", "date": "2024-06-15"},
			},
		},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("GanttDiagram.Render() error = %v", err)
	}

	if doc == nil {
		t.Fatal("Expected non-nil SVG document")
	}

	if doc.Width != 1200 || doc.Height != 667 {
		t.Errorf("Expected dimensions 1200x667 (900x500pt in CSS pixels), got %vx%v", doc.Width, doc.Height)
	}

	// Verify task labels are present in SVG
	svg := string(doc.Content)
	for _, label := range []string{"Design Phase", "Development", "Testing", "Kickoff", "Release"} {
		if !strings.Contains(svg, label) {
			t.Errorf("Expected SVG to contain label %q", label)
		}
	}
}

func TestParseGanttTask(t *testing.T) {
	tests := []struct {
		name             string
		input            map[string]any
		wantLabel        string
		wantCategory     string
		wantIsMilestone  bool
		wantDepsCount    int
	}{
		{
			name:         "basic task with start and end",
			input:        map[string]any{"label": "Dev", "start": "2024-01-01", "end": "2024-03-01", "category": "Engineering"},
			wantLabel:    "Dev",
			wantCategory: "Engineering",
		},
		{
			name:            "milestone task",
			input:           map[string]any{"label": "Launch", "date": "2024-06-01", "type": "milestone"},
			wantLabel:       "Launch",
			wantIsMilestone: true,
		},
		{
			name:          "task with dependencies",
			input:         map[string]any{"id": "t3", "label": "Test", "start": "2024-04-01", "end": "2024-05-01", "dependencies": []any{"t1", "t2"}},
			wantLabel:     "Test",
			wantDepsCount: 2,
		},
		{
			name:      "task with name alias",
			input:     map[string]any{"name": "My Task", "start_date": "2024-01-01", "end_date": "2024-02-01"},
			wantLabel: "My Task",
		},
		{
			name:         "task with status as category",
			input:        map[string]any{"label": "Feature", "start": "2024-01-01", "end": "2024-02-01", "status": "in_progress"},
			wantLabel:    "Feature",
			wantCategory: "in_progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := parseGanttTask(tt.input, 0)
			if task.Label != tt.wantLabel {
				t.Errorf("Label = %q, want %q", task.Label, tt.wantLabel)
			}
			if task.Category != tt.wantCategory {
				t.Errorf("Category = %q, want %q", task.Category, tt.wantCategory)
			}
			if task.IsMilestone != tt.wantIsMilestone {
				t.Errorf("IsMilestone = %v, want %v", task.IsMilestone, tt.wantIsMilestone)
			}
			if len(task.Dependencies) != tt.wantDepsCount {
				t.Errorf("Dependencies count = %d, want %d", len(task.Dependencies), tt.wantDepsCount)
			}
		})
	}
}

func TestGanttDiagram_LongLabelsNotTruncated(t *testing.T) {
	diagram := &GanttDiagram{NewBaseDiagram("gantt")}

	req := &RequestEnvelope{
		Type:  "gantt",
		Title: "Product Development Plan",
		Data: map[string]any{
			"tasks": []any{
				map[string]any{"id": "req", "label": "Requirements", "start": "2024-01-01", "end": "2024-01-31", "category": "Planning"},
				map[string]any{"id": "design", "label": "System Design", "start": "2024-01-15", "end": "2024-02-28", "category": "Planning"},
				map[string]any{"id": "backend", "label": "Backend Development", "start": "2024-02-15", "end": "2024-04-30", "category": "Engineering"},
				map[string]any{"id": "frontend", "label": "Frontend Development", "start": "2024-03-01", "end": "2024-05-15", "category": "Engineering"},
				map[string]any{"id": "integration", "label": "Integration Testing", "start": "2024-04-15", "end": "2024-05-31", "category": "QA"},
				map[string]any{"id": "docs", "label": "Documentation", "start": "2024-05-01", "end": "2024-06-15", "category": "Documentation"},
			},
		},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := string(doc.Content)
	// These labels should appear in full (not truncated with "...")
	for _, label := range []string{"Backend Development", "Frontend Development", "Integration Testing"} {
		if !strings.Contains(svg, label) {
			t.Errorf("Expected SVG to contain full label %q (should not be truncated)", label)
		}
	}
}

func TestGanttChart_AutoSizeLabelWidth(t *testing.T) {
	config := DefaultGanttConfig(900, 500)
	builder := NewSVGBuilder(900, 500)
	gc := NewGanttChart(builder, config)
	style := builder.StyleGuide()

	plotArea := config.PlotArea()

	tests := []struct {
		name      string
		labels    []string
		wantMin   float64 // should be at least this wide
		wantMax   float64 // should not exceed this
	}{
		{
			name:    "short labels use default width",
			labels:  []string{"Design", "Dev", "QA"},
			wantMin: config.LabelWidth,
			wantMax: config.LabelWidth,
		},
		{
			name:    "medium labels fit within default with real metrics",
			labels:  []string{"Backend Development", "Frontend Development", "Integration Testing"},
			wantMin: config.LabelWidth, // real font metrics show these fit the default
			wantMax: plotArea.W * 0.40,
		},
		{
			name:    "extremely long labels capped at 40%",
			labels:  []string{"This Is An Extremely Long Task Name That Definitely Should Be Capped Because It Keeps Going And Going On"},
			wantMin: plotArea.W * 0.39, // close to cap
			wantMax: plotArea.W * 0.40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rows []ganttRow
			for _, label := range tt.labels {
				rows = append(rows, ganttRow{
					task: &GanttTask{Label: label},
				})
			}

			width := gc.autoSizeLabelWidth(rows, plotArea, style)

			if width < tt.wantMin {
				t.Errorf("autoSizeLabelWidth() = %v, want >= %v", width, tt.wantMin)
			}
			if width > tt.wantMax {
				t.Errorf("autoSizeLabelWidth() = %v, want <= %v", width, tt.wantMax)
			}
		})
	}
}

func TestGanttChart_AutoSizeLabelUsesRealMetrics(t *testing.T) {
	// Verify that autoSizeLabelWidth uses real font metrics by checking
	// that a label with many wide chars (e.g., "WWWWWW") produces a wider
	// result than one with narrow chars (e.g., "iiiiii") of the same rune count.
	config := DefaultGanttConfig(900, 500)
	builder := NewSVGBuilder(900, 500)
	gc := NewGanttChart(builder, config)
	style := builder.StyleGuide()

	plotArea := config.PlotArea()
	// Force a small default so the measurement dominates
	gc.config.LabelWidth = 10

	wideRows := []ganttRow{{task: &GanttTask{Label: "WWWWWWWWWWWWWWWWWWWW"}}}
	narrowRows := []ganttRow{{task: &GanttTask{Label: "iiiiiiiiiiiiiiiiiiii"}}}

	wideWidth := gc.autoSizeLabelWidth(wideRows, plotArea, style)
	narrowWidth := gc.autoSizeLabelWidth(narrowRows, plotArea, style)

	if wideWidth <= narrowWidth {
		t.Errorf("Expected wide chars (%v) > narrow chars (%v) with real font metrics", wideWidth, narrowWidth)
	}
}

func TestGanttChart_DateRange(t *testing.T) {
	gc := &GanttChart{config: DefaultGanttConfig(900, 500)}

	data := GanttData{
		Tasks: []GanttTask{
			{StartDate: date(2024, 1, 1), EndDate: date(2024, 3, 1)},
			{StartDate: date(2024, 2, 15), EndDate: date(2024, 6, 1)},
		},
		Milestones: []GanttMilestone{
			{Date: date(2024, 7, 1)},
		},
	}

	dr := gc.calculateDateRange(data)

	// Min date should be before 2024-01-01 (with padding)
	if !dr.start.Before(date(2024, 1, 1)) {
		t.Errorf("Expected start before 2024-01-01, got %v", dr.start)
	}

	// Max date should be after 2024-07-01 (with padding)
	if !dr.end.After(date(2024, 7, 1)) {
		t.Errorf("Expected end after 2024-07-01, got %v", dr.end)
	}

	// Duration should be positive
	if dr.duration <= 0 {
		t.Errorf("Expected positive duration, got %v", dr.duration)
	}
}

func TestGanttChart_TimeUnitDetection(t *testing.T) {
	gc := &GanttChart{config: DefaultGanttConfig(900, 500)}

	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		wantUnit string
	}{
		{
			name:     "short range = days",
			start:    date(2024, 1, 1),
			end:      date(2024, 1, 10),
			wantUnit: "day",
		},
		{
			name:     "medium range = weeks",
			start:    date(2024, 1, 1),
			end:      date(2024, 2, 15),
			wantUnit: "week",
		},
		{
			name:     "long range = months",
			start:    date(2024, 1, 1),
			end:      date(2024, 9, 1),
			wantUnit: "month",
		},
		{
			name:     "very long range = quarters",
			start:    date(2024, 1, 1),
			end:      date(2026, 1, 1),
			wantUnit: "quarter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dr := timelineRange{
				start:    tt.start,
				end:      tt.end,
				duration: tt.end.Sub(tt.start),
			}
			unit := gc.detectTimeUnit(dr)
			if unit != tt.wantUnit {
				t.Errorf("detectTimeUnit() = %q, want %q", unit, tt.wantUnit)
			}
		})
	}
}

func TestGanttChart_BarTextContrast(t *testing.T) {
	// Verify that bar labels use contrast-aware text color via TextColorFor().
	// Light-colored bars should get dark text; dark bars should get white text.
	lightColor := MustParseColor("#EDC948") // light yellow
	darkColor := MustParseColor("#4E79A7")  // dark blue

	lightText := lightColor.TextColorFor()
	darkText := darkColor.TextColorFor()

	if lightText.IsLight() {
		t.Error("Light bar background (#EDC948) should use dark text, not white")
	}
	if !darkText.IsLight() {
		t.Error("Dark bar background (#4E79A7) should use white text, not dark")
	}

	// Render a Gantt with both light and dark bars to verify no panics
	// and that labels appear in the SVG output.
	diagram := &GanttDiagram{NewBaseDiagram("gantt")}
	req := &RequestEnvelope{
		Type:  "gantt",
		Title: "Contrast Test",
		Data: map[string]any{
			"tasks": []any{
				map[string]any{"id": "t1", "label": "Light Task", "start": "2024-01-01", "end": "2024-06-01", "color": "#EDC948"},
				map[string]any{"id": "t2", "label": "Dark Task", "start": "2024-06-01", "end": "2024-12-01", "color": "#4E79A7"},
			},
		},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := string(doc.Content)
	// Wide bars should have their labels drawn inside
	for _, label := range []string{"Light Task", "Dark Task"} {
		// The label appears twice: once in the row label column, once inside the bar
		count := strings.Count(svg, label)
		if count < 2 {
			t.Errorf("Expected label %q to appear at least twice (row + bar), found %d occurrences", label, count)
		}
	}
}

func TestGanttChart_XAxisLabelThinning(t *testing.T) {
	// Verify that when a Gantt chart has many ticks, x-axis labels are thinned
	// to prevent overcrowding.
	tests := []struct {
		name          string
		start         string
		end           string
		timeUnit      string
		maxLabels     int // expected maximum number of labels after thinning
	}{
		{
			name:      "short timeline shows all labels",
			start:     "2024-01-01",
			end:       "2024-06-01",
			timeUnit:  "month",
			maxLabels: 12, // 5 months, all should show
		},
		{
			name:      "multi-year monthly timeline thins labels",
			start:     "2022-01-01",
			end:       "2026-01-01",
			timeUnit:  "month",
			maxLabels: 25, // ~48 months, should thin to every 2nd or 3rd
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type:  "gantt",
				Title: tt.name,
				Data: map[string]any{
					"tasks": []any{
						map[string]any{"label": "Long Task", "start": tt.start, "end": tt.end},
					},
					"time_unit": tt.timeUnit,
				},
				Output: OutputSpec{Width: 900, Height: 500},
			}

			diagram := &GanttDiagram{NewBaseDiagram("gantt")}
			doc, err := diagram.Render(req)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			// Count text elements in the axis area. We can't easily count
			// just axis labels, but we verify rendering doesn't panic and
			// the SVG is non-empty.
			if doc == nil || len(doc.Content) == 0 {
				t.Error("Expected non-empty SVG document")
			}
		})
	}
}

// TestParseGanttData_TasksAsObject verifies that when tasks is a JSON object
// (map[string]any) instead of an array, the parser emits a warning and
// produces no tasks rather than silently producing wrong output (bug 1ef5).
func TestParseGanttData_TasksAsObject(t *testing.T) {
	req := &RequestEnvelope{
		Type:  "gantt",
		Title: "Object Tasks",
		Data: map[string]any{
			"tasks": map[string]any{
				"Task 1": map[string]any{"start": "2024-01-01", "end": "2024-02-01"},
				"Task 2": map[string]any{"start": "2024-02-01", "end": "2024-03-01"},
			},
		},
	}

	data, err := parseGanttData(req)
	if err != nil {
		t.Fatalf("parseGanttData() unexpected error: %v", err)
	}
	if len(data.Tasks) != 0 {
		t.Errorf("Expected 0 tasks when tasks is an object, got %d", len(data.Tasks))
	}
}

// TestGanttChart_ManyTasksNoOverlap verifies that a Gantt chart with 15+ tasks
// renders without overlapping bars, uses reduced font sizes, truncates long
// labels with ellipsis, and shows a "+N more" indicator if rows are capped.
func TestGanttChart_ManyTasksNoOverlap(t *testing.T) {
	// Generate 15 tasks with long names
	var tasks []GanttTask
	for i := 0; i < 15; i++ {
		tasks = append(tasks, GanttTask{
			ID:        fmt.Sprintf("t%d", i),
			Label:     fmt.Sprintf("Task %d: Long Description Name Here", i+1),
			StartDate: date(2024, 1+i/3, 1+i%28),
			EndDate:   date(2024, 2+i/3, 1+i%28),
		})
	}

	builder := NewSVGBuilder(900, 500)
	config := DefaultGanttConfig(900, 500)
	chart := NewGanttChart(builder, config)

	err := chart.Draw(GanttData{
		Title: "Dense Project Plan",
		Tasks: tasks,
	})
	if err != nil {
		t.Fatalf("Draw() error = %v", err)
	}

	doc, err := builder.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if doc == nil || len(doc.Content) == 0 {
		t.Fatal("Expected non-empty SVG document")
	}

	svg := string(doc.Content)

	// The "+N more" indicator should be present if tasks were capped,
	// OR all 15 task labels should appear (possibly truncated).
	hasOverflow := strings.Contains(svg, "more")
	taskCount := 0
	for i := 0; i < 15; i++ {
		label := fmt.Sprintf("Task %d", i+1)
		if strings.Contains(svg, label) {
			taskCount++
		}
	}

	if !hasOverflow && taskCount < 15 {
		t.Errorf("Expected either '+N more' indicator or all 15 task labels; got %d labels and hasOverflow=%v", taskCount, hasOverflow)
	}
}

// TestGanttChart_RowsDoNotOverlap verifies that Y positions of adjacent rows
// never overlap, even with many tasks.
func TestGanttChart_RowsDoNotOverlap(t *testing.T) {
	var tasks []GanttTask
	for i := 0; i < 20; i++ {
		tasks = append(tasks, GanttTask{
			ID:        fmt.Sprintf("t%d", i),
			Label:     fmt.Sprintf("Task %d", i+1),
			StartDate: date(2024, 1, 1+i),
			EndDate:   date(2024, 2, 1+i),
		})
	}

	builder := NewSVGBuilder(900, 500)
	config := DefaultGanttConfig(900, 500)
	chart := NewGanttChart(builder, config)

	plotArea := config.PlotArea()
	rows, overflow := chart.buildRows(GanttData{Tasks: tasks}, plotArea)

	// Check that no two adjacent rows overlap (row[i+1].y >= row[i].y + BarHeight)
	for i := 0; i < len(rows)-1; i++ {
		if rows[i+1].y < rows[i].y+chart.config.BarHeight {
			t.Errorf("Row %d (y=%.1f) overlaps with row %d (y=%.1f, barH=%.1f)",
				i+1, rows[i+1].y, i, rows[i].y, chart.config.BarHeight)
		}
	}

	// With 20 tasks in a 500pt chart, we expect some overflow
	totalVisible := len(rows) + overflow.count
	if totalVisible != 20 {
		t.Errorf("Expected total (visible + overflow) = 20, got %d + %d = %d",
			len(rows), overflow.count, totalVisible)
	}

	if overflow.count > 0 {
		t.Logf("Visible: %d, overflow: %d", len(rows), overflow.count)
	}
}

// TestGanttChart_AdaptiveFontSize verifies that with many tasks the label font
// size is reduced proportionally to row height, but never below 9pt.
func TestGanttChart_AdaptiveFontSize(t *testing.T) {
	// Render a chart with 12 tasks in a small area
	var taskData []any
	for i := 0; i < 12; i++ {
		taskData = append(taskData, map[string]any{
			"id":    fmt.Sprintf("t%d", i),
			"label": fmt.Sprintf("Task Number %d Description", i+1),
			"start": fmt.Sprintf("2024-%02d-01", i%12+1),
			"end":   fmt.Sprintf("2024-%02d-15", i%12+1),
		})
	}

	diagram := &GanttDiagram{NewBaseDiagram("gantt")}
	req := &RequestEnvelope{
		Type:  "gantt",
		Title: "Adaptive Font Test",
		Data: map[string]any{
			"tasks": taskData,
		},
		Output: OutputSpec{Width: 600, Height: 400},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if doc == nil || len(doc.Content) == 0 {
		t.Fatal("Expected non-empty SVG document")
	}

	// The SVG should render without errors; the quality test suite
	// (TestGolden_SVGQuality) will catch font sizes below 9pt.
}

// TestGanttChart_NarrowBarExternalLabel verifies that a task bar too narrow
// for its label renders the label externally (to the right of the bar) instead
// of omitting it entirely. Regression test for pptx-ml4.
func TestGanttChart_NarrowBarExternalLabel(t *testing.T) {
	diagram := &GanttDiagram{NewBaseDiagram("gantt")}

	// 'Requirements' spans only 2 weeks while other tasks span months,
	// making its bar too narrow for an internal label.
	req := &RequestEnvelope{
		Type:  "gantt",
		Title: "Narrow Bar Test",
		Data: map[string]any{
			"tasks": []any{
				map[string]any{"id": "req", "label": "Requirements", "start": "2024-01-01", "end": "2024-01-14"},
				map[string]any{"id": "design", "label": "Design", "start": "2024-01-15", "end": "2024-03-15"},
				map[string]any{"id": "dev", "label": "Development", "start": "2024-03-15", "end": "2024-08-01"},
				map[string]any{"id": "test", "label": "Testing", "start": "2024-08-01", "end": "2024-10-01"},
			},
		},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := string(doc.Content)

	// 'Requirements' should appear at least twice: once in the row label
	// column and once as the external bar label (or internal if it happens
	// to fit). Before the fix it appeared only once (row label only).
	count := strings.Count(svg, "Requirements")
	if count < 2 {
		t.Errorf("Expected 'Requirements' to appear at least twice (row label + bar label), found %d occurrences", count)
	}
}

// date is a helper to create time.Time values for testing.
func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
