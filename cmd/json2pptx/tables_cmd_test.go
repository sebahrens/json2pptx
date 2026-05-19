package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTablesGuide(t *testing.T) {
	bin := buildJSON2PPTX(t)

	t.Run("human", func(t *testing.T) {
		out, err := runBin(bin, "tables", "guide")
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		output := string(out)
		if !strings.Contains(output, "Table Density Reference") {
			t.Errorf("expected human header, got: %s", output)
		}
		if !strings.Contains(output, "Multiline cells eat budget") {
			t.Errorf("expected multiline note in human output, got: %s", output)
		}
	})

	t.Run("json", func(t *testing.T) {
		out, err := runBin(bin, "tables", "guide", "--json")
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		var resp densityGuideResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if len(resp.Tiers) == 0 {
			t.Fatal("expected at least one density tier")
		}
		if resp.Limits.MaxRows == 0 {
			t.Error("expected limits.max_rows to be populated")
		}
		if resp.MultilineNote == "" {
			t.Error("expected non-empty multiline_note")
		}
		// Sanity check: first tier should match buildDensityTiers() output.
		want := buildDensityTiers()
		if resp.Tiers[0].DataRows != want[0].DataRows {
			t.Errorf("tier[0].data_rows = %q, want %q", resp.Tiers[0].DataRows, want[0].DataRows)
		}
		if resp.Tiers[0].MaxColumns != want[0].MaxColumns {
			t.Errorf("tier[0].max_columns = %d, want %d", resp.Tiers[0].MaxColumns, want[0].MaxColumns)
		}
	})
}
