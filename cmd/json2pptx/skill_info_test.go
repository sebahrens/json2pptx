package main

import (
	"sort"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/template"
)

func TestAnalyzeTemplateForSkillInfo_FiltersOtherPlaceholders(t *testing.T) {
	templatePath := "../../testdata/templates/forest-green.pptx"

	cache := template.NewMemoryCache(24 * time.Hour)
	info, err := analyzeTemplateForSkillInfo(templatePath, cache, "full")
	if err != nil {
		t.Fatalf("analyzeTemplateForSkillInfo failed: %v", err)
	}

	// Verify no placeholder has type "other" in any layout
	for _, layout := range info.Layouts {
		for _, ph := range layout.Placeholders {
			if ph.Type == "other" {
				t.Errorf("layout %q contains placeholder %q with type %q; "+
					"internal OOXML metadata placeholders should be filtered out",
					layout.Name, ph.ID, ph.Type)
			}
		}
	}

	// Verify we still have some placeholders (sanity check)
	totalPhs := 0
	for _, layout := range info.Layouts {
		totalPhs += len(layout.Placeholders)
	}
	if totalPhs == 0 {
		t.Error("expected at least one placeholder across all layouts after filtering")
	}
}

func TestBuildSupportedTypes_DataFormatHints(t *testing.T) {
	st := buildSupportedTypes()

	if st.DataFormatHints == nil {
		t.Fatal("DataFormatHints should not be nil")
	}

	// Every chart type must have a corresponding data format hint.
	for _, ct := range st.ChartTypes {
		if _, ok := st.DataFormatHints[ct]; !ok {
			t.Errorf("chart type %q missing from DataFormatHints", ct)
		}
	}

	// Every diagram type that is not an alias (icon_columns, icon_rows, stat_cards
	// are aliases for panel_layout) must have a data format hint.
	aliases := map[string]bool{
		"icon_columns": true,
		"icon_rows":    true,
		"stat_cards":   true,
	}
	for _, dt := range st.DiagramTypes {
		if aliases[dt] {
			continue
		}
		if _, ok := st.DataFormatHints[dt]; !ok {
			t.Errorf("diagram type %q missing from DataFormatHints", dt)
		}
	}

	// Spot-check a few entries for correct structure.
	tests := []struct {
		name         string
		wantRequired []string
		wantDesc     string
	}{
		{"bar", []string{"categories", "series"}, "categories: string array; series: [{name, values: number[]}]"},
		{"waterfall", []string{"points"}, "points: [{label, value, type: \"increase\"|\"decrease\"|\"total\"}]"},
		{"gauge", []string{"value"}, "value: number; min/max: number; thresholds: [{value, color, label}]"},
		{"fishbone", []string{"effect"}, "effect: string (problem label); categories: [{name, causes: string[]}]"},
		{"panel_layout", []string{"panels"}, "panels: [{title, body, icon?, color?}]; layout: \"columns\"|\"rows\"|\"stat_cards\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df, ok := st.DataFormatHints[tt.name]
			if !ok {
				t.Fatalf("missing DataFormatHints entry for %q", tt.name)
			}
			got := make([]string, len(df.RequiredKeys))
			copy(got, df.RequiredKeys)
			sort.Strings(got)
			want := make([]string, len(tt.wantRequired))
			copy(want, tt.wantRequired)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Errorf("RequiredKeys = %v, want %v", df.RequiredKeys, tt.wantRequired)
			} else {
				for i := range got {
					if got[i] != want[i] {
						t.Errorf("RequiredKeys = %v, want %v", df.RequiredKeys, tt.wantRequired)
						break
					}
				}
			}
			if df.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", df.Description, tt.wantDesc)
			}
		})
	}
}
