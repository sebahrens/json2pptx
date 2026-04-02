package svggen

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestTimeline_Type(t *testing.T) {
	d := &Timeline{NewBaseDiagram("timeline")}
	if got := d.Type(); got != "timeline" {
		t.Errorf("Type() = %v, want timeline", got)
	}
}

func TestTimeline_Validate(t *testing.T) {
	d := &Timeline{NewBaseDiagram("timeline")}

	tests := []struct {
		name    string
		req     *RequestEnvelope
		wantErr bool
	}{
		{
			name: "valid request",
			req: &RequestEnvelope{
				Type: "timeline",
				Data: map[string]any{
					"activities": []any{
						map[string]any{
							"label":      "Task 1",
							"start_date": "2024-01-01",
							"end_date":   "2024-01-15",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing data",
			req: &RequestEnvelope{
				Type: "timeline",
				Data: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.Validate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTimeline_Render(t *testing.T) {
	d := &Timeline{NewBaseDiagram("timeline")}

	tests := []struct {
		name    string
		req     *RequestEnvelope
		wantErr bool
		checks  []string
	}{
		{
			name: "basic timeline with activities",
			req: &RequestEnvelope{
				Type:  "timeline",
				Title: "Project Timeline",
				Data: map[string]any{
					"activities": []any{
						map[string]any{
							"label":      "Planning",
							"start_date": "2024-01-01",
							"end_date":   "2024-01-15",
							"type":       "activity",
						},
						map[string]any{
							"label":      "Development",
							"start_date": "2024-01-10",
							"end_date":   "2024-02-15",
							"type":       "activity",
						},
						map[string]any{
							"label":      "Testing",
							"start_date": "2024-02-01",
							"end_date":   "2024-02-28",
							"type":       "activity",
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 400},
			},
			wantErr: false,
			checks:  []string{"<svg", "Project Timeline", "Planning", "Development", "Testing"},
		},
		{
			name: "timeline with milestones",
			req: &RequestEnvelope{
				Type:  "timeline",
				Title: "Release Schedule",
				Data: map[string]any{
					"activities": []any{
						map[string]any{
							"label":      "Sprint 1",
							"start_date": "2024-01-01",
							"end_date":   "2024-01-14",
						},
					},
					"milestones": []any{
						map[string]any{
							"label": "MVP Release",
							"date":  "2024-01-15",
						},
						map[string]any{
							"label": "Beta Release",
							"date":  "2024-02-01",
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 400},
			},
			wantErr: false,
			checks:  []string{"<svg", "Release Schedule", "Sprint 1", "MVP Release"},
		},
		{
			name: "timeline with phases",
			req: &RequestEnvelope{
				Type:  "timeline",
				Title: "Project Phases",
				Data: map[string]any{
					"activities": []any{
						map[string]any{
							"label":      "Phase 1",
							"start_date": "2024-01-01",
							"end_date":   "2024-03-31",
							"type":       "phase",
						},
						map[string]any{
							"label":      "Task A",
							"start_date": "2024-01-15",
							"end_date":   "2024-02-15",
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 400},
			},
			wantErr: false,
			checks:  []string{"<svg", "Project Phases", "Phase 1", "Task A"},
		},
		{
			name: "timeline with today line",
			req: &RequestEnvelope{
				Type:  "timeline",
				Title: "Current Status",
				Data: map[string]any{
					"show_today":  true,
					"today_label": "Now",
					"start_date":  "2024-01-01",
					"end_date":    "2025-12-31",
					"activities": []any{
						map[string]any{
							"label":      "Ongoing Task",
							"start_date": "2024-06-01",
							"end_date":   "2025-06-01",
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 400},
			},
			wantErr: false,
			checks:  []string{"<svg", "Current Status"},
		},
		{
			name: "timeline with progress",
			req: &RequestEnvelope{
				Type:  "timeline",
				Title: "Task Progress",
				Data: map[string]any{
					"show_progress": true,
					"activities": []any{
						map[string]any{
							"label":      "In Progress Task",
							"start_date": "2024-01-01",
							"end_date":   "2024-01-31",
							"progress":   75.0,
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 400},
			},
			wantErr: false,
			checks:  []string{"<svg", "Task Progress", "In Progress Task"},
		},
		{
			name: "timeline with custom colors",
			req: &RequestEnvelope{
				Type:  "timeline",
				Title: "Color Coded Timeline",
				Data: map[string]any{
					"activities": []any{
						map[string]any{
							"label":      "Red Task",
							"start_date": "2024-01-01",
							"end_date":   "2024-01-15",
							"color":      "#FF0000",
						},
						map[string]any{
							"label":      "Green Task",
							"start_date": "2024-01-16",
							"end_date":   "2024-01-31",
							"color":      "#00FF00",
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 400},
			},
			wantErr: false,
			checks:  []string{"<svg", "Red Task", "Green Task"},
		},
		{
			name: "timeline with different time units",
			req: &RequestEnvelope{
				Type:  "timeline",
				Title: "Yearly Timeline",
				Data: map[string]any{
					"time_unit": "year",
					"activities": []any{
						map[string]any{
							"label":      "Long Term Project",
							"start_date": "2024-01-01",
							"end_date":   "2027-12-31",
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 400},
			},
			wantErr: false,
			checks:  []string{"<svg", "Long Term Project"},
		},
		{
			name: "empty activities",
			req: &RequestEnvelope{
				Type:  "timeline",
				Title: "Empty Timeline",
				Data: map[string]any{
					"activities": []any{},
				},
				Output: OutputSpec{Width: 800, Height: 400},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svg, err := d.Render(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			content := svg.String()
			for _, check := range tt.checks {
				if !strings.Contains(content, check) {
					t.Errorf("Render() output missing %q", check)
				}
			}
		})
	}
}

func TestTimelineConfig_Defaults(t *testing.T) {
	config := DefaultTimelineConfig(800, 400)

	if config.Width != 800 {
		t.Errorf("Width = %v, want 800", config.Width)
	}
	if config.Height != 400 {
		t.Errorf("Height = %v, want 400", config.Height)
	}
	if config.RowHeight <= 0 {
		t.Errorf("RowHeight should be positive")
	}
	if config.BarHeight <= 0 {
		t.Errorf("BarHeight should be positive")
	}
	if config.MilestoneSize <= 0 {
		t.Errorf("MilestoneSize should be positive")
	}
}

func TestTimelineChart_Draw(t *testing.T) {
	builder := NewSVGBuilder(800, 400)
	config := DefaultTimelineConfig(800, 400)
	chart := NewTimelineChart(builder, config)

	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	data := TimelineData{
		Title:    "Test Timeline",
		Subtitle: "Subtitle",
		Activities: []TimelineActivity{
			{
				ID:        "task1",
				Label:     "Task 1",
				Type:      TimelineActivityTypeActivity,
				StartDate: baseDate,
				EndDate:   baseDate.AddDate(0, 0, 14),
				Row:       -1,
			},
			{
				ID:        "task2",
				Label:     "Task 2",
				Type:      TimelineActivityTypeActivity,
				StartDate: baseDate.AddDate(0, 0, 7),
				EndDate:   baseDate.AddDate(0, 0, 21),
				Row:       -1,
			},
			{
				ID:    "milestone1",
				Label: "Milestone",
				Type:  TimelineActivityTypeMilestone,
				Date:  baseDate.AddDate(0, 0, 30),
				Row:   -1,
			},
		},
		Footnote: "Note: All dates are tentative",
	}

	err := chart.Draw(data)
	if err != nil {
		t.Errorf("Draw() error = %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Errorf("Render() error = %v", err)
	}

	content := svg.String()
	if !strings.Contains(content, "<svg") {
		t.Error("Output should contain SVG tag")
	}
}

func TestTimelineChart_RowAssignment(t *testing.T) {
	builder := NewSVGBuilder(800, 400)
	config := DefaultTimelineConfig(800, 400)
	chart := NewTimelineChart(builder, config)

	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create overlapping activities
	activities := []TimelineActivity{
		{
			ID:        "task1",
			Label:     "Task 1",
			Type:      TimelineActivityTypeActivity,
			StartDate: baseDate,
			EndDate:   baseDate.AddDate(0, 0, 14),
			Row:       -1,
		},
		{
			ID:        "task2",
			Label:     "Task 2 (overlaps)",
			Type:      TimelineActivityTypeActivity,
			StartDate: baseDate.AddDate(0, 0, 7), // Overlaps with task1
			EndDate:   baseDate.AddDate(0, 0, 21),
			Row:       -1,
		},
		{
			ID:        "task3",
			Label:     "Task 3 (no overlap)",
			Type:      TimelineActivityTypeActivity,
			StartDate: baseDate.AddDate(0, 0, 25), // No overlap
			EndDate:   baseDate.AddDate(0, 1, 0),
			Row:       -1,
		},
	}

	dateRange := chart.calculateDateRange(TimelineData{Activities: activities})
	plotArea := chart.config.PlotArea()
	rows := chart.assignRows(activities, dateRange, plotArea)

	// Task 1 and 2 should be on different rows (they overlap)
	if rows[0] == rows[1] {
		t.Error("Overlapping tasks should be on different rows")
	}

	// Task 3 could share a row with task 1 (no overlap)
	// This is implementation-dependent, just verify no error
	if len(rows) != 3 {
		t.Errorf("Expected 3 row assignments, got %d", len(rows))
	}
}

func TestTimelineChart_DateRange(t *testing.T) {
	builder := NewSVGBuilder(800, 400)
	config := DefaultTimelineConfig(800, 400)
	chart := NewTimelineChart(builder, config)

	baseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)

	data := TimelineData{
		Activities: []TimelineActivity{
			{
				Label:     "Task",
				StartDate: baseDate,
				EndDate:   endDate,
			},
		},
	}

	dateRange := chart.calculateDateRange(data)

	// Range should include the activity dates with some padding
	if dateRange.start.After(baseDate) {
		t.Error("Date range start should be at or before first activity")
	}
	if dateRange.end.Before(endDate) {
		t.Error("Date range end should be at or after last activity")
	}
}

func TestTimelineChart_TimeUnitDetection(t *testing.T) {
	builder := NewSVGBuilder(800, 400)
	config := DefaultTimelineConfig(800, 400)
	chart := NewTimelineChart(builder, config)

	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"short (days)", 7 * 24 * time.Hour, "day"},
		{"medium (weeks)", 30 * 24 * time.Hour, "week"},
		{"months", 180 * 24 * time.Hour, "month"},
		{"years", 500 * 24 * time.Hour, "quarter"},
		{"multi-year", 1500 * 24 * time.Hour, "year"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			dateRange := timelineRange{
				start:    start,
				end:      start.Add(tt.duration),
				duration: tt.duration,
			}

			unit := chart.detectTimeUnit(dateRange)
			if unit != tt.expected {
				t.Errorf("detectTimeUnit() = %v, want %v", unit, tt.expected)
			}
		})
	}
}

func TestTimelineChart_TickFormatting(t *testing.T) {
	builder := NewSVGBuilder(800, 400)
	config := DefaultTimelineConfig(800, 400)
	chart := NewTimelineChart(builder, config)

	testDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		unit     string
		expected string
	}{
		{"day", "Jun 15"},
		{"week", "Jun 15"},
		{"month", "Jun '24"},
		{"quarter", "Q2 '24"},
		{"year", "2024"},
	}

	for _, tt := range tests {
		t.Run(tt.unit, func(t *testing.T) {
			label := chart.formatTickLabel(testDate, tt.unit)
			if label != tt.expected {
				t.Errorf("formatTickLabel() = %v, want %v", label, tt.expected)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"2024-01-15", false},
		{"2024-01-15T10:30:00Z", false},
		{"2024-01-15T10:30:00", false},
		{"Jan 15, 2024", false},
		{"January 15, 2024", false},
		{"2024/01/15", false},
		{"15-Jan-2024", false},
		{"2026 Q1", false},  // Quarter format
		{"Q2 2026", false},  // Quarter format (reversed)
		{"2026Q3", false},   // Quarter format (no space)
		{"Q1", false},       // Bare quarter format (current year)
		{"Q4", false},       // Bare quarter format (current year)
		{"2026 H1", false},  // Half-year format
		{"H2 2026", false},  // Half-year format (reversed)
		{"2026H1", false},   // Half-year format (no space)
		{"Mar 2026", false}, // Month-year (abbreviated)
		{"March 2026", false}, // Month-year (full)
		{"Dec 2025", false},   // Month-year (abbreviated)
		{"invalid date", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseQuarterDate(t *testing.T) {
	tests := []struct {
		input     string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			input:     "2026 Q1",
			wantStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:     "Q2 2026",
			wantStart: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:     "2026Q3",
			wantStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:     "2026 Q4",
			wantStart: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:   "2026 Q5", // Invalid quarter
			wantErr: true,
		},
		{
			input:     "Q1",
			wantStart: time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(time.Now().Year(), 3, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:     "Q4",
			wantStart: time.Date(time.Now().Year(), 10, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(time.Now().Year(), 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:   "not a date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			start, end, err := parseQuarterDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseQuarterDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !start.Equal(tt.wantStart) {
					t.Errorf("parseQuarterDate(%q) start = %v, want %v", tt.input, start, tt.wantStart)
				}
				if !end.Equal(tt.wantEnd) {
					t.Errorf("parseQuarterDate(%q) end = %v, want %v", tt.input, end, tt.wantEnd)
				}
			}
		})
	}
}

func TestParseHalfDate(t *testing.T) {
	tests := []struct {
		input     string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			input:     "2026 H1",
			wantStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:     "2026 H2",
			wantStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:     "H1 2027",
			wantStart: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:     "2026H2",
			wantStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:   "2026 H3", // Invalid half
			wantErr: true,
		},
		{
			input:   "not a date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			start, end, err := parseHalfDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHalfDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !start.Equal(tt.wantStart) {
					t.Errorf("parseHalfDate(%q) start = %v, want %v", tt.input, start, tt.wantStart)
				}
				if !end.Equal(tt.wantEnd) {
					t.Errorf("parseHalfDate(%q) end = %v, want %v", tt.input, end, tt.wantEnd)
				}
			}
		})
	}
}

func TestParseMonthDate(t *testing.T) {
	tests := []struct {
		input     string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			input:     "Mar 2026",
			wantStart: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:     "June 2026",
			wantStart: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:     "Feb 2024", // Leap year
			wantStart: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:     "December 2025",
			wantStart: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			input:   "not a date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			start, end, err := parseMonthDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMonthDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !start.Equal(tt.wantStart) {
					t.Errorf("parseMonthDate(%q) start = %v, want %v", tt.input, start, tt.wantStart)
				}
				if !end.Equal(tt.wantEnd) {
					t.Errorf("parseMonthDate(%q) end = %v, want %v", tt.input, end, tt.wantEnd)
				}
			}
		})
	}
}

func TestParseTimelineActivity(t *testing.T) {
	tests := []struct {
		name      string
		raw       any
		wantType  TimelineActivityType
		wantLabel string
	}{
		{
			name:      "string form",
			raw:       "Simple Task",
			wantType:  TimelineActivityTypeActivity,
			wantLabel: "Simple Task",
		},
		{
			name: "activity type",
			raw: map[string]any{
				"label": "Task",
				"type":  "activity",
			},
			wantType:  TimelineActivityTypeActivity,
			wantLabel: "Task",
		},
		{
			name: "milestone type",
			raw: map[string]any{
				"label": "Milestone",
				"type":  "milestone",
			},
			wantType:  TimelineActivityTypeMilestone,
			wantLabel: "Milestone",
		},
		{
			name: "phase type",
			raw: map[string]any{
				"label": "Phase",
				"type":  "phase",
			},
			wantType:  TimelineActivityTypePhase,
			wantLabel: "Phase",
		},
		{
			name: "title field maps to label (markdown format)",
			raw: map[string]any{
				"title": "Phase 1: Discovery",
				"date":  "2026 Q1",
			},
			wantType:  TimelineActivityTypeActivity,
			wantLabel: "Phase 1: Discovery",
		},
		{
			name: "name field maps to label (visual deck format)",
			raw: map[string]any{
				"name":  "Phase 1: Discovery",
				"start": "2026-01",
				"end":   "2026-03",
			},
			wantType:  TimelineActivityTypeActivity,
			wantLabel: "Phase 1: Discovery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activity := parseTimelineActivity(tt.raw, 0)
			if activity.Type != tt.wantType {
				t.Errorf("parseTimelineActivity() Type = %v, want %v", activity.Type, tt.wantType)
			}
			if activity.Label != tt.wantLabel {
				t.Errorf("parseTimelineActivity() Label = %v, want %v", activity.Label, tt.wantLabel)
			}
		})
	}
}

func TestTimeline_MarkdownFormat(t *testing.T) {
	// Test that markdown format with "items" and "title" fields works
	d := &Timeline{NewBaseDiagram("timeline")}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Implementation Roadmap",
		Data: map[string]any{
			"items": []any{
				map[string]any{
					"date":        "2026 Q1",
					"title":       "Phase 1: Discovery",
					"description": "Requirements gathering and analysis",
				},
				map[string]any{
					"date":        "2026 Q2",
					"title":       "Phase 2: Development",
					"description": "Core feature implementation",
				},
				map[string]any{
					"date":        "2026 Q3",
					"title":       "Phase 3: Testing",
					"description": "QA and user acceptance testing",
				},
				map[string]any{
					"date":        "2026 Q4",
					"title":       "Phase 4: Launch",
					"description": "Production deployment and rollout",
				},
			},
		},
		Output: OutputSpec{Width: 800, Height: 400},
	}

	svg, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()

	// Verify the SVG contains the expected content
	checks := []string{
		"<svg",
		"Implementation Roadmap",
		"Phase 1: Discovery",
		"Phase 2: Development",
		"Phase 3: Testing",
		"Phase 4: Launch",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("Output missing %q", check)
		}
	}
}

// TestTimeline_NarrowColumnDropsDescriptions verifies that descriptions are
// dropped in narrow two-column layouts where vertical space is too tight,
// preventing illegible <8pt text. Labels should still render.
func TestTimeline_NarrowColumnDropsDescriptions(t *testing.T) {
	d := &Timeline{NewBaseDiagram("timeline")}

	// Simulate a narrow half-width placeholder (~315pt wide, ~250pt tall).
	// The 2:1 natural aspect ratio with contain mode produces a ~315×157 SVG.
	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Product Roadmap",
		Data: map[string]any{
			"items": []any{
				map[string]any{
					"date":        "2026 Q1",
					"title":       "Foundation",
					"description": "New architecture launch",
				},
				map[string]any{
					"date":        "2026 Q2",
					"title":       "Advanced",
					"description": "Advanced features rollout",
				},
				map[string]any{
					"date":        "2026 Q3",
					"title":       "Ecosystem",
					"description": "Ecosystem expansion",
				},
				map[string]any{
					"date":        "2026 Q4",
					"title":       "Intelligence",
					"description": "Intelligent automation",
				},
			},
		},
		Output: OutputSpec{Width: 315, Height: 250},
	}

	svg, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()

	// Labels should still be present
	for _, label := range []string{"Foundation", "Advanced", "Ecosystem", "Intelligence"} {
		if !strings.Contains(content, label) {
			t.Errorf("Label %q should be rendered even in narrow layout", label)
		}
	}

	// Descriptions should be dropped in narrow layout to prevent illegible text
	for _, desc := range []string{"New architecture launch", "Advanced features rollout", "Ecosystem expansion", "Intelligent automation"} {
		if strings.Contains(content, desc) {
			t.Errorf("Description %q should be dropped in narrow layout to prevent illegible text", desc)
		}
	}
}

// TestTimeline_WideLayoutKeepsDescriptions verifies that descriptions are
// kept in full-width layouts where there's sufficient vertical space.
func TestTimeline_WideLayoutKeepsDescriptions(t *testing.T) {
	d := &Timeline{NewBaseDiagram("timeline")}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Product Roadmap",
		Data: map[string]any{
			"items": []any{
				map[string]any{
					"date":        "2026 Q1",
					"title":       "Foundation",
					"description": "New architecture launch",
				},
				map[string]any{
					"date":        "2026 Q2",
					"title":       "Advanced",
					"description": "Advanced features rollout",
				},
			},
		},
		Output: OutputSpec{Width: 800, Height: 400},
	}

	svg, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()

	// Both labels and descriptions should be present in wide layout
	for _, text := range []string{"Foundation", "New architecture launch", "Advanced", "Advanced features rollout"} {
		if !strings.Contains(content, text) {
			t.Errorf("Text %q should be present in wide layout", text)
		}
	}
}

func TestTimelineChart_LabelPositions(t *testing.T) {
	positions := []string{"left", "right", "above", "below", "inside"}

	for _, pos := range positions {
		t.Run(pos, func(t *testing.T) {
			builder := NewSVGBuilder(800, 400)
			config := DefaultTimelineConfig(800, 400)
			config.LabelPosition = pos
			chart := NewTimelineChart(builder, config)

			baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			data := TimelineData{
				Activities: []TimelineActivity{
					{
						Label:     "Test Task",
						StartDate: baseDate,
						EndDate:   baseDate.AddDate(0, 0, 14),
					},
				},
			}

			err := chart.Draw(data)
			if err != nil {
				t.Errorf("Draw() with labelPosition=%s error = %v", pos, err)
			}
		})
	}
}

func TestTimeline_JSONRoundtrip(t *testing.T) {
	// Test that we can parse JSON input correctly
	jsonInput := `{
		"type": "timeline",
		"title": "Project Schedule",
		"data": {
			"activities": [
				{
					"label": "Design Phase",
					"start_date": "2024-01-01",
					"end_date": "2024-01-31",
					"color": "#4E79A7"
				},
				{
					"label": "Development",
					"start_date": "2024-02-01",
					"end_date": "2024-03-31",
					"progress": 50
				}
			],
			"milestones": [
				{
					"label": "Release",
					"date": "2024-04-01"
				}
			],
			"show_progress": true
		}
	}`

	var req RequestEnvelope
	if err := json.Unmarshal([]byte(jsonInput), &req); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	d := &Timeline{NewBaseDiagram("timeline")}
	svg, err := d.Render(&req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()
	checks := []string{"<svg", "Project Schedule", "Design Phase", "Development"}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("Output missing %q", check)
		}
	}
}

func TestRegisterTimeline(t *testing.T) {
	// Create a new registry to avoid conflicts
	registry := NewRegistry()
	registry.Register(&Timeline{NewBaseDiagram("timeline")})

	if d := registry.Get("timeline"); d == nil {
		t.Error("Timeline should be registered")
	}

	// Test rendering through registry
	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Registry Test",
		Data: map[string]any{
			"activities": []any{
				map[string]any{
					"label":      "Task",
					"start_date": "2024-01-01",
					"end_date":   "2024-01-31",
				},
			},
		},
		Output: OutputSpec{Width: 800, Height: 400},
	}

	svg, err := registry.Render(req)
	if err != nil {
		t.Errorf("Registry Render() error = %v", err)
	}
	if svg == nil {
		t.Error("SVG should not be nil")
	}
}

func TestTimelineRange_DateToX(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	tr := timelineRange{
		start:    start,
		end:      end,
		duration: end.Sub(start),
	}

	plotArea := Rect{X: 100, Y: 50, W: 600, H: 300}

	// Start should map to left edge
	xStart := tr.dateToX(start, plotArea)
	if xStart != plotArea.X {
		t.Errorf("Start date should map to X=%v, got %v", plotArea.X, xStart)
	}

	// End should map to right edge
	xEnd := tr.dateToX(end, plotArea)
	if xEnd != plotArea.X+plotArea.W {
		t.Errorf("End date should map to X=%v, got %v", plotArea.X+plotArea.W, xEnd)
	}

	// Middle should map to center
	midDate := start.Add(tr.duration / 2)
	xMid := tr.dateToX(midDate, plotArea)
	expectedMid := plotArea.X + plotArea.W/2
	if xMid != expectedMid {
		t.Errorf("Mid date should map to X=%v, got %v", expectedMid, xMid)
	}
}

func TestDrawTimelineFromActivities(t *testing.T) {
	builder := NewSVGBuilder(800, 400)
	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	activities := []TimelineActivity{
		{
			ID:        "task1",
			Label:     "Task 1",
			Type:      TimelineActivityTypeActivity,
			StartDate: baseDate,
			EndDate:   baseDate.AddDate(0, 0, 14),
		},
		{
			ID:        "task2",
			Label:     "Task 2",
			Type:      TimelineActivityTypeActivity,
			StartDate: baseDate.AddDate(0, 0, 15),
			EndDate:   baseDate.AddDate(0, 0, 30),
		},
	}

	err := DrawTimelineFromActivities(builder, "Test Timeline", activities)
	if err != nil {
		t.Errorf("DrawTimelineFromActivities() error = %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Errorf("Render() error = %v", err)
	}

	if !strings.Contains(svg.String(), "Test Timeline") {
		t.Error("Output should contain title")
	}
}

func TestTimeline_BareQuarterDates(t *testing.T) {
	// Test that bare "Q1", "Q2" etc. dates (no year) produce a valid timeline
	// spanning the current year instead of collapsing to zero-width.
	d := &Timeline{NewBaseDiagram("timeline")}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "FY Product Roadmap",
		Data: map[string]any{
			"items": []any{
				map[string]any{"date": "Q1", "title": "Platform 2.0"},
				map[string]any{"date": "Q2", "title": "Enterprise Suite"},
				map[string]any{"date": "Q3", "title": "Partner APIs"},
				map[string]any{"date": "Q4", "title": "AI Features"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 400},
	}

	svg, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()
	for _, check := range []string{"Platform 2.0", "Enterprise Suite", "Partner APIs", "AI Features"} {
		if !strings.Contains(content, check) {
			t.Errorf("Output missing %q — bare quarter dates may not be parsed", check)
		}
	}
}

func TestTimeline_HalfYearDates(t *testing.T) {
	// Test that half-year dates like "2026 H1" render correctly
	d := &Timeline{NewBaseDiagram("timeline")}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Three-Year Transformation Plan",
		Data: map[string]any{
			"items": []any{
				map[string]any{"date": "2026 H1", "title": "Foundation", "description": "Cloud migration"},
				map[string]any{"date": "2026 H2", "title": "Modernization", "description": "Legacy retirement"},
				map[string]any{"date": "2027 H1", "title": "Intelligence", "description": "ML pipelines"},
				map[string]any{"date": "2027 H2", "title": "Scale", "description": "Multi-region"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 400},
	}

	svg, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()
	for _, check := range []string{"Foundation", "Modernization", "Intelligence", "Scale"} {
		if !strings.Contains(content, check) {
			t.Errorf("Output missing %q — half-year dates may not be parsed", check)
		}
	}
}

func TestTimeline_MonthYearDates(t *testing.T) {
	// Test that month-year dates like "Mar 2026" render correctly
	d := &Timeline{NewBaseDiagram("timeline")}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Key Milestones",
		Data: map[string]any{
			"items": []any{
				map[string]any{"date": "Mar 2026", "title": "M1: Foundation", "description": "Core platform live"},
				map[string]any{"date": "Jun 2026", "title": "M2: Migration", "description": "First workloads moved"},
				map[string]any{"date": "Oct 2026", "title": "M3: Retirement", "description": "Legacy decommissioned"},
				map[string]any{"date": "Dec 2026", "title": "M4: Operational", "description": "Full platform ready"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 400},
	}

	svg, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()
	for _, check := range []string{"M1: Foundation", "M2: Migration", "M3: Retirement", "M4: Operational"} {
		if !strings.Contains(content, check) {
			t.Errorf("Output missing %q — month-year dates may not be parsed", check)
		}
	}
}

func TestTimelineChart_InsideLabelContrastColor(t *testing.T) {
	// Test that labels inside bars use contrast-aware text colors
	// based on the background color luminance

	tests := []struct {
		name          string
		bgColor       string
		expectedText  string // The hex color we expect for text
		expectedLight bool   // Is the background light?
	}{
		{
			name:          "light pastel background uses dark text",
			bgColor:       "#E8F4FD", // Light blue pastel
			expectedText:  "#212529", // Dark text
			expectedLight: true,
		},
		{
			name:          "light yellow background uses dark text",
			bgColor:       "#FFF3CD", // Light yellow
			expectedText:  "#212529", // Dark text
			expectedLight: true,
		},
		{
			name:          "white background uses dark text",
			bgColor:       "#FFFFFF",
			expectedText:  "#212529", // Dark text
			expectedLight: true,
		},
		{
			name:          "dark blue background uses white text",
			bgColor:       "#4E79A7", // Default activity color (darker blue)
			expectedText:  "#FFFFFF", // White text
			expectedLight: false,
		},
		{
			name:          "red background uses dark text (better contrast)",
			bgColor:       "#E15759", // Red (mid-tone: dark 4.2:1 > white 3.7:1)
			expectedText:  "#212529", // Dark text
			expectedLight: false,
		},
		{
			name:          "black background uses white text",
			bgColor:       "#000000",
			expectedText:  "#FFFFFF", // White text
			expectedLight: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bgColor := MustParseColor(tt.bgColor)

			// Verify background is classified correctly
			if bgColor.IsLight() != tt.expectedLight {
				t.Errorf("IsLight() = %v, want %v for color %s",
					bgColor.IsLight(), tt.expectedLight, tt.bgColor)
			}

			// Verify TextColorFor returns correct color
			textColor := bgColor.TextColorFor()
			if textColor.Hex() != tt.expectedText {
				t.Errorf("TextColorFor() = %s, want %s for background %s",
					textColor.Hex(), tt.expectedText, tt.bgColor)
			}
		})
	}
}

func TestTimelineChart_InsideLabelRendering(t *testing.T) {
	// Test that rendering with inside labels and light colors
	// produces dark text in the SVG output

	builder := NewSVGBuilder(800, 400)
	config := DefaultTimelineConfig(800, 400)
	config.LabelPosition = "inside"
	chart := NewTimelineChart(builder, config)

	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	lightColor := MustParseColor("#E8F4FD") // Light pastel blue

	data := TimelineData{
		Activities: []TimelineActivity{
			{
				ID:        "light-task",
				Label:     "Light Background Task",
				StartDate: baseDate,
				EndDate:   baseDate.AddDate(0, 0, 14),
				Color:     &lightColor,
			},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Draw() error = %v", err)
	}

	svg, err := builder.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()

	// The SVG should contain the label text
	if !strings.Contains(content, "Light Background Task") {
		t.Error("Output should contain the label text")
	}

	// With a light background, the text should use dark color (#212529)
	// The SVG output should contain fill="#212529" for the text element
	if !strings.Contains(content, "#212529") {
		t.Error("Light background should result in dark text color (#212529)")
	}
}

func TestTimelineChart_MilestoneInsideLabelNotClipped(t *testing.T) {
	// Regression test: milestone labels with LabelPosition="inside" (the default)
	// were falling through to the "left" case, drawing text at
	// plotArea.X - spacing with right alignment, causing text to extend
	// beyond the left edge of the SVG canvas.
	// The fix adds an explicit "inside" case that draws above the diamond.

	d := &Timeline{NewBaseDiagram("timeline")}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Product Launch Roadmap",
		Data: map[string]any{
			"activities": []any{
				map[string]any{"id": "design", "label": "UX Research & Design", "type": "activity", "start_date": "2024-01-15", "end_date": "2024-03-15"},
				map[string]any{"id": "m1", "label": "Alpha Release", "type": "milestone", "date": "2024-05-01"},
				map[string]any{"id": "m2", "label": "Beta Release", "type": "milestone", "date": "2024-06-15"},
				map[string]any{"id": "m3", "label": "GA Launch", "type": "milestone", "date": "2024-08-01"},
			},
			"time_unit": "month",
		},
		Output: OutputSpec{Width: 1000, Height: 400},
	}

	svg, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()

	// All milestone labels must appear in the SVG output
	for _, label := range []string{"Alpha Release", "Beta Release", "GA Launch"} {
		if !strings.Contains(content, label) {
			t.Errorf("Output missing milestone label %q", label)
		}
	}

	// With inside positioning, milestone labels should use center alignment
	// (text-anchor="middle") not right alignment (text-anchor="end") which
	// was the old buggy behavior placing labels in the left margin.
	// We verify that there is no text-anchor="end" directly before any of
	// the milestone label text, which would indicate the old "left" fallthrough.
	if strings.Contains(content, `text-anchor="end"`) {
		// Check if any of the milestone labels are right-aligned (old bug)
		for _, label := range []string{"Alpha Release", "Beta Release", "GA Launch"} {
			idx := strings.Index(content, label)
			if idx < 0 {
				continue
			}
			// Look at the ~200 chars before the label for text-anchor
			start := idx - 200
			if start < 0 {
				start = 0
			}
			snippet := content[start:idx]
			if strings.Contains(snippet, `text-anchor="end"`) {
				t.Errorf("Milestone label %q is right-aligned (text-anchor=end), which places it in the left margin and risks clipping", label)
			}
		}
	}
}

// TestTimeline_DateAsStartForActivity verifies that a timeline item using
// "date" + "end_date" (instead of "start_date" + "end_date") is rendered
// correctly as an activity bar.  This is the exact data format used by
// the LLM-generated JSON in deck3-healthcare and deck6-defense which
// previously caused the Gantt/timeline to show only 1-3 items instead
// of all specified items (the bars had zero StartDate which positioned
// them far off-screen).
func TestTimeline_DateAsStartForActivity(t *testing.T) {
	diagram := &Timeline{NewBaseDiagram("timeline")}

	// Reproduce the exact JSON shape from deck3-healthcare slide 9
	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "3-Year Digital Transformation Roadmap",
		Data: map[string]any{
			"items": []any{
				map[string]any{
					"date":     "2026-01-01",
					"end_date": "2026-06-30",
					"title":    "Phase 1: EHR Core",
					"type":     "activity",
				},
				map[string]any{
					"date":     "2026-07-01",
					"end_date": "2026-12-31",
					"title":    "Phase 2: Telemedicine",
					"type":     "activity",
				},
				map[string]any{
					"date":     "2027-01-01",
					"end_date": "2027-06-30",
					"title":    "Phase 3: AI Diagnostics",
					"type":     "activity",
				},
				map[string]any{
					"date":  "2027-07-01",
					"title": "Full System Go-Live",
					"type":  "milestone",
				},
			},
		},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := string(doc.Content)

	// All four labels must appear in the SVG output.
	// Before the fix, activities with "date" instead of "start_date"
	// had zero StartDate, causing their bars to render off-screen.
	// Note: SVG XML-escapes "&" to "&amp;", so we check both forms.
	expectedLabels := []string{
		"Phase 1: EHR Core",
		"Phase 2: Telemedicine",
		"Phase 3: AI Diagnostics",
		"Full System Go-Live",
	}
	for _, label := range expectedLabels {
		escaped := strings.ReplaceAll(label, "&", "&amp;")
		if !strings.Contains(svg, label) && !strings.Contains(svg, escaped) {
			t.Errorf("Expected SVG to contain label %q", label)
		}
	}

	// Verify the activities have properly parsed StartDate by
	// checking that there are multiple path elements in the SVG
	// (the canvas-based renderer emits <path> elements for bars).
	pathCount := strings.Count(svg, "<path")
	if pathCount < 3 {
		t.Errorf("Expected at least 3 <path> elements (activity bars + milestones), got %d", pathCount)
	}
}

// TestTimeline_DateFieldStartDateParsing verifies the date parsing logic
// in parseTimelineActivity for various "date" field combinations.
func TestTimeline_DateFieldStartDateParsing(t *testing.T) {
	tests := []struct {
		name          string
		input         map[string]any
		wantStart     time.Time
		wantEnd       time.Time
		wantDate      time.Time
		wantStartZero bool
	}{
		{
			name: "date+end_date: date used as StartDate",
			input: map[string]any{
				"title":    "Phase 1",
				"date":     "2026-01-01",
				"end_date": "2026-06-30",
				"type":     "activity",
			},
			wantStart: date(2026, 1, 1),
			wantEnd:   date(2026, 6, 30),
			wantDate:  date(2026, 1, 1),
		},
		{
			name: "start_date+end_date: normal case unchanged",
			input: map[string]any{
				"title":      "Phase 2",
				"start_date": "2026-03-01",
				"end_date":   "2026-09-30",
				"type":       "activity",
			},
			wantStart: date(2026, 3, 1),
			wantEnd:   date(2026, 9, 30),
		},
		{
			name: "start_date+date: start_date takes priority",
			input: map[string]any{
				"title":      "Phase 3",
				"start_date": "2026-01-15",
				"date":       "2026-02-01",
				"end_date":   "2026-06-30",
				"type":       "activity",
			},
			wantStart: date(2026, 1, 15),
			wantEnd:   date(2026, 6, 30),
			wantDate:  date(2026, 2, 1),
		},
		{
			name: "milestone with date only",
			input: map[string]any{
				"title": "Go-Live",
				"date":  "2026-07-01",
				"type":  "milestone",
			},
			wantDate:  date(2026, 7, 1),
			wantStart: date(2026, 7, 1), // date also populates StartDate
		},
		{
			name: "start alias accepted",
			input: map[string]any{
				"title": "Phase A",
				"start": "2026-04-01",
				"end":   "2026-08-31",
				"type":  "activity",
			},
			wantStart: date(2026, 4, 1),
			wantEnd:   date(2026, 8, 31),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activity := parseTimelineActivity(tt.input, 0)

			if !tt.wantStart.IsZero() && !activity.StartDate.Equal(tt.wantStart) {
				t.Errorf("StartDate = %v, want %v", activity.StartDate, tt.wantStart)
			}
			if tt.wantStartZero && !activity.StartDate.IsZero() {
				t.Errorf("StartDate = %v, want zero", activity.StartDate)
			}
			if !tt.wantEnd.IsZero() && !activity.EndDate.Equal(tt.wantEnd) {
				t.Errorf("EndDate = %v, want %v", activity.EndDate, tt.wantEnd)
			}
			if !tt.wantDate.IsZero() && !activity.Date.Equal(tt.wantDate) {
				t.Errorf("Date = %v, want %v", activity.Date, tt.wantDate)
			}
		})
	}
}

// TestTimeline_DefenseTimelineAllItemsRendered reproduces the exact JSON
// from deck6-defense slide 4 which showed only 1 bar + 1 milestone.
func TestTimeline_DefenseTimelineAllItemsRendered(t *testing.T) {
	diagram := &Timeline{NewBaseDiagram("timeline")}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Major Milestones",
		Data: map[string]any{
			"items": []any{
				map[string]any{
					"date":     "2026-01-01",
					"end_date": "2026-06-30",
					"title":    "Planning & Requirements",
					"type":     "activity",
				},
				map[string]any{
					"date":     "2026-07-01",
					"end_date": "2026-12-31",
					"title":    "Development Phase 1",
					"type":     "activity",
				},
				map[string]any{
					"date":  "2027-06-30",
					"title": "Prototype Complete",
					"type":  "milestone",
				},
				map[string]any{
					"date":     "2028-01-01",
					"end_date": "2028-06-30",
					"title":    "Testing & Validation",
					"type":     "activity",
				},
			},
		},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := string(doc.Content)

	// All four labels must be present in the SVG.
	// Note: SVG XML-escapes "&" to "&amp;", so check both forms.
	for _, label := range []string{
		"Planning & Requirements",
		"Development Phase 1",
		"Prototype Complete",
		"Testing & Validation",
	} {
		escaped := strings.ReplaceAll(label, "&", "&amp;")
		if !strings.Contains(svg, label) && !strings.Contains(svg, escaped) {
			t.Errorf("Expected SVG to contain label %q -- item was not rendered", label)
		}
	}
}

// TestTimeline_DescriptionsNotTruncated verifies that activity descriptions
// with moderate-length text are allocated enough space (4 lines) to avoid
// truncation with ellipsis.
func TestTimeline_DescriptionsNotTruncated(t *testing.T) {
	d := &Timeline{NewBaseDiagram("timeline")}

	descriptions := []string{
		"Cloud migration, data platform, API gateway modernization",
		"Legacy system retirement and microservices architecture rollout",
		"ML pipelines, predictive analytics, and real-time dashboards",
		"Multi-region deployment with disaster recovery and auto-scaling",
	}

	items := make([]any, len(descriptions))
	for i, desc := range descriptions {
		items[i] = map[string]any{
			"title":       fmt.Sprintf("Phase %d", i+1),
			"start_date":  fmt.Sprintf("2026-%02d-01", i*3+1),
			"end_date":    fmt.Sprintf("2026-%02d-28", i*3+3),
			"type":        "activity",
			"description": desc,
		}
	}

	req := &RequestEnvelope{
		Type:   "timeline",
		Title:  "Digital Transformation Roadmap",
		Data:   map[string]any{"items": items},
		Output: OutputSpec{Width: 900, Height: 500},
	}

	// Rendering should succeed without error — descriptions have enough space.
	doc, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(doc.Content) == 0 {
		t.Fatal("Render() produced empty SVG")
	}

	// Verify DescriptionLabelFit allows 4 lines (regression guard).
	style := DefaultStyleGuide()
	strat := DescriptionLabelFit(style.Typography)
	if strat.MaxLines < 4 {
		t.Errorf("DescriptionLabelFit.MaxLines = %d, want >= 4", strat.MaxLines)
	}

	// Verify description text fits at the expected width for 4 activities
	// sharing a full-width (900px) timeline. Each activity gets ~225px budget,
	// description uses 95% of that ≈ 214px.
	b := NewSVGBuilder(900, 500)
	b.SetStyleGuide(style)
	descW := (900.0 / 4.0) * 0.95
	descH := strat.PreferredSize * float64(strat.MaxLines) * style.Typography.LineHeight
	for _, desc := range descriptions {
		fit := strat.Fit(b, desc, descW, descH)
		if fit.DisplayText != desc {
			t.Errorf("Description truncated: got %q, want %q", fit.DisplayText, desc)
		}
	}
}

func TestTimeline_MilestoneLabelStaggerNarrowColumn(t *testing.T) {
	// Regression test for go-slide-creator-b9aub: milestone labels overlapping
	// in narrow two-column layouts. When milestones are close together, labels
	// should alternate above/below to prevent horizontal overlap.
	d := &Timeline{NewBaseDiagram("timeline")}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Key Milestones",
		Data: map[string]any{
			"items": []any{
				map[string]any{"date": "Mar 2026", "title": "M1: Foundation", "description": "Core platform live", "type": "milestone"},
				map[string]any{"date": "Jun 2026", "title": "M2: Migration", "description": "First workloads moved", "type": "milestone"},
				map[string]any{"date": "Oct 2026", "title": "M3: Retirement", "description": "Legacy decommissioned", "type": "milestone"},
				map[string]any{"date": "Dec 2026", "title": "M4: Operational", "description": "Full platform ready", "type": "milestone"},
			},
		},
		// Narrow column typical of two-column layouts.
		Output: OutputSpec{Width: 400, Height: 300},
	}

	svg, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()

	// All milestone labels must appear in the SVG output.
	for _, label := range []string{"M1: Foundation", "M2: Migration", "M3: Retirement", "M4: Operational"} {
		if !strings.Contains(content, label) && !strings.Contains(content, label[:8]) {
			t.Errorf("Output missing milestone label %q (or truncation of it)", label)
		}
	}

	// Verify stagger: milestone labels should have at least 2 distinct Y positions
	// (some above the diamonds, some below). Extract Y values from tspan elements
	// that contain milestone labels.
	milestoneLabels := []string{"M1: Foundation", "M2: Migration", "M3: Retirement", "M4: Operational"}
	var yValues []float64
	for _, label := range milestoneLabels {
		idx := strings.Index(content, label)
		if idx < 0 {
			continue
		}
		// Find the enclosing <tspan> and extract its y attribute.
		tspanStart := strings.LastIndex(content[:idx], "<tspan")
		if tspanStart < 0 {
			continue
		}
		snippet := content[tspanStart:idx]
		yIdx := strings.Index(snippet, ` y="`)
		if yIdx < 0 {
			continue
		}
		yStr := snippet[yIdx+4:]
		endQuote := strings.Index(yStr, `"`)
		if endQuote < 0 {
			continue
		}
		var yVal float64
		fmt.Sscanf(yStr[:endQuote], "%f", &yVal)
		yValues = append(yValues, yVal)
	}

	if len(yValues) < 2 {
		t.Fatalf("Could not extract enough label Y positions; got %d", len(yValues))
	}

	// Check that labels use at least 2 distinct Y values (staggered above/below).
	uniqueY := make(map[float64]bool)
	for _, y := range yValues {
		rounded := math.Round(y*10) / 10
		uniqueY[rounded] = true
	}
	if len(uniqueY) < 2 {
		t.Errorf("Expected staggered milestone labels with different Y positions, but all at same Y; values: %v", yValues)
	}
}

func TestTimeline_ActivityLabelOverlapPrevention(t *testing.T) {
	// Regression test for go-slide-creator-02jtc: activity labels overlap when
	// 7+ items have adjacent date ranges. Labels should be sized based on
	// actual neighbor spacing and staggered above/below when needed.
	d := &Timeline{NewBaseDiagram("timeline")}

	// Create 7 activities clustered in a narrow date range (all in Jan 2026).
	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Sprint Tasks",
		Data: map[string]any{
			"items": []any{
				map[string]any{"title": "Task Alpha", "start": "2026-01-01", "end": "2026-01-03"},
				map[string]any{"title": "Task Beta", "start": "2026-01-04", "end": "2026-01-06"},
				map[string]any{"title": "Task Gamma", "start": "2026-01-07", "end": "2026-01-09"},
				map[string]any{"title": "Task Delta", "start": "2026-01-10", "end": "2026-01-12"},
				map[string]any{"title": "Task Epsilon", "start": "2026-01-13", "end": "2026-01-15"},
				map[string]any{"title": "Task Zeta", "start": "2026-01-16", "end": "2026-01-18"},
				map[string]any{"title": "Task Eta", "start": "2026-01-19", "end": "2026-01-21"},
			},
		},
		// Narrow canvas to trigger overlap.
		Output: OutputSpec{Width: 400, Height: 300},
	}

	svg, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := svg.String()

	// All labels (or truncations of them) must appear.
	for _, label := range []string{"Task Alpha", "Task Beta", "Task Gamma", "Task Delta", "Task Epsilon", "Task Zeta", "Task Eta"} {
		if !strings.Contains(content, label) && !strings.Contains(content, label[:6]) {
			t.Errorf("Output missing activity label %q (or truncation of it)", label)
		}
	}

	// Verify stagger: labels should have at least 2 distinct Y positions
	// (some above bars, some below) when there's overlap.
	labels := []string{"Task A", "Task B", "Task G", "Task D", "Task E", "Task Z", "Task Eta"}
	var yValues []float64
	for _, label := range labels {
		idx := strings.Index(content, label)
		if idx < 0 {
			continue
		}
		tspanStart := strings.LastIndex(content[:idx], "<tspan")
		if tspanStart < 0 {
			continue
		}
		snippet := content[tspanStart:idx]
		yIdx := strings.Index(snippet, ` y="`)
		if yIdx < 0 {
			continue
		}
		yStr := snippet[yIdx+4:]
		endQuote := strings.Index(yStr, `"`)
		if endQuote < 0 {
			continue
		}
		var yVal float64
		fmt.Sscanf(yStr[:endQuote], "%f", &yVal)
		yValues = append(yValues, yVal)
	}

	if len(yValues) < 2 {
		t.Fatalf("Could not extract enough label Y positions; got %d", len(yValues))
	}

	// Check that labels use at least 2 distinct Y values (staggered above/below).
	uniqueY := make(map[float64]bool)
	for _, y := range yValues {
		rounded := math.Round(y*10) / 10
		uniqueY[rounded] = true
	}
	if len(uniqueY) < 2 {
		t.Errorf("Expected staggered activity labels with different Y positions, but all at same Y; values: %v", yValues)
	}
}

// TestTimeline_PhasesKeyWithNameField reproduces the visual deck generator format
// which uses "phases" key and "name" field (not "label" or "title").
func TestTimeline_PhasesKeyWithNameField(t *testing.T) {
	diagram := &Timeline{NewBaseDiagram("timeline")}

	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Implementation Roadmap",
		Data: map[string]any{
			"phases": []any{
				map[string]any{"name": "Phase 1: Discovery", "start": "2026-01", "end": "2026-03"},
				map[string]any{"name": "Phase 2: Design", "start": "2026-04", "end": "2026-06"},
				map[string]any{"name": "Phase 3: Build", "start": "2026-07", "end": "2026-10"},
				map[string]any{"name": "Phase 4: Launch", "start": "2026-11", "end": "2026-12"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 400},
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := string(doc.Content)

	for _, label := range []string{
		"Phase 1: Discovery",
		"Phase 2: Design",
		"Phase 3: Build",
		"Phase 4: Launch",
	} {
		if !strings.Contains(svg, label) {
			t.Errorf("Expected SVG to contain label %q -- phase label not rendered", label)
		}
	}
}

func TestTimeline_MilestonesWithEventField(t *testing.T) {
	// Regression: "event" field should be recognized as a label alias
	diagram := &Timeline{NewBaseDiagram("timeline")}
	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Release Milestones",
		Data: map[string]any{
			"milestones": []any{
				map[string]any{
					"date":  "2024-06-15",
					"event": "Beta Launch",
				},
				map[string]any{
					"date":  "2024-09-01",
					"event": "GA Release",
				},
			},
		},
		Output: OutputSpec{Width: 800, Height: 400},
	}

	if err := diagram.Validate(req); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := string(doc.Content)
	for _, label := range []string{"Beta Launch", "GA Release"} {
		if !strings.Contains(svg, label) {
			t.Errorf("Expected SVG to contain %q from 'event' field", label)
		}
	}
}

func TestTimeline_MilestonesWithUnparseableDates(t *testing.T) {
	// Regression: milestones with descriptive text in "date" field
	// (not parseable as dates) should still render with labels and
	// auto-assigned dates.
	diagram := &Timeline{NewBaseDiagram("timeline")}
	req := &RequestEnvelope{
		Type:  "timeline",
		Title: "Implementation Roadmap",
		Data: map[string]any{
			"milestones": []any{
				map[string]any{
					"date":  "Phase 1: Foundation (2026-2027)",
					"event": "Establish governance entity, procure first 2 data centers",
				},
				map[string]any{
					"date":  "Phase 2: Scale (2027-2029)",
					"event": "Deploy 50K GPUs, expand to 5 data centers",
				},
				map[string]any{
					"date":  "Phase 3: Autonomy (2029-2031)",
					"event": "Achieve full sovereign AI capability",
				},
			},
		},
		Output: OutputSpec{Width: 800, Height: 400},
	}

	if err := diagram.Validate(req); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	doc, err := diagram.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := string(doc.Content)
	// The "event" text should appear as labels
	for _, label := range []string{"Establish governance", "Deploy 50K", "sovereign AI"} {
		if !strings.Contains(svg, label) {
			t.Errorf("Expected SVG to contain %q from milestone event text", label)
		}
	}
	// Should produce a valid SVG (not crash on zero dates)
	if !strings.Contains(svg, "<svg") {
		t.Error("Expected valid SVG output")
	}
}

func TestAutoAssignDatelessActivities(t *testing.T) {
	// All dateless milestones should get evenly spaced dates
	activities := []TimelineActivity{
		{Label: "Phase 1", Type: TimelineActivityTypeMilestone},
		{Label: "Phase 2", Type: TimelineActivityTypeMilestone},
		{Label: "Phase 3", Type: TimelineActivityTypeMilestone},
	}

	result := autoAssignDatelessActivities(activities)

	for i, act := range result {
		if act.Date.IsZero() {
			t.Errorf("activity[%d] still has zero date after auto-assign", i)
		}
	}

	// Dates should be in chronological order
	for i := 1; i < len(result); i++ {
		if !result[i].Date.After(result[i-1].Date) {
			t.Errorf("activity[%d].Date (%v) should be after activity[%d].Date (%v)",
				i, result[i].Date, i-1, result[i-1].Date)
		}
	}
}

func TestAutoAssignDatelessActivities_MixedDates(t *testing.T) {
	// Some activities have dates, some don't — dateless ones should
	// be spaced within the existing range.
	activities := []TimelineActivity{
		{Label: "Task A", Type: TimelineActivityTypeActivity,
			StartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)},
		{Label: "Phase 2", Type: TimelineActivityTypeMilestone},
		{Label: "Task C", Type: TimelineActivityTypeActivity,
			StartDate: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC)},
	}

	result := autoAssignDatelessActivities(activities)

	// The dateless milestone (index 1) should now have a date
	if result[1].Date.IsZero() {
		t.Error("dateless milestone should have been assigned a date")
	}
	// It should fall within the overall range
	if result[1].Date.Before(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) ||
		result[1].Date.After(time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("assigned date %v should be within Jan-Sep 2024 range", result[1].Date)
	}
}
