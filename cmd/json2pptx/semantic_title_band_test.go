package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// go-slide-creator-b7qqg.13: the copy-ready architecture example on
// blue-corporate (bold, spc="300", all-caps title) and the matrix-2x2 example
// on modern-yellow ("Segoe UI Semibold" 44pt, substituted when measured) both
// render two-line titles. The body zone was reserved for ONE line, so the top
// tier panel / the "Low effort" axis label were drawn into the second line.
// Every pattern shape must now start below two title lines.
func TestSemanticExamplesReserveTwoLineTitle(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	templatesDir := filepath.Join(projectRoot, "templates")
	cases := []struct {
		template, title string
		titleY          int64
		lineEMU         int64 // one rendered line: size x 1.2 x 90% lnSpc
		pattern         map[string]any
	}{
		{
			template: "blue-corporate", title: "Four tiers, two concerns that cut across them",
			titleY: 841248, lineEMU: int64(28 * 1.2 * 0.9 * 12700),
			pattern: map[string]any{"name": "arch-stack", "values": map[string]any{
				"tiers": []any{
					map[string]any{"label": "Experience", "description": "Web console, Mobile approvals, Partner portal"},
					map[string]any{"label": "Services", "description": "Orders, Pricing, Fulfilment, Identity"},
					map[string]any{"label": "Data", "description": "Event stream, warehouse, feature store"},
					map[string]any{"label": "Platform", "description": "Kubernetes, observability, secrets"},
				},
				"side_rails": []any{"Security & compliance", "Cost governance"},
			}},
		},
		{
			template: "modern-yellow", title: "Where to spend the next two quarters",
			titleY: 365125, lineEMU: int64(44 * 1.2 * 0.9 * 12700),
			pattern: map[string]any{"name": "matrix-2x2", "values": map[string]any{
				"x_axis_label": "Delivery effort", "y_axis_label": "Impact on settlement risk",
				"x_low": "Low effort", "x_high": "High effort", "y_low": "Low impact", "y_high": "High impact",
				"top_left":     map[string]any{"header": "Do first", "body": "Reconciliation alerts, cut-off automation."},
				"top_right":    map[string]any{"header": "Plan properly", "body": "Platform migration, wave 2 and 3."},
				"bottom_left":  map[string]any{"header": "Fill the gaps", "body": "Runbook tidy-up, dashboard polish."},
				"bottom_right": map[string]any{"header": "Defer", "body": "Reporting refresh, vendor consolidation."},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.template, func(t *testing.T) {
			dir := t.TempDir()
			deck := map[string]any{
				"template": tc.template, "design_mode": "constrained", "type_scale": "comfortable",
				"output_filename": "deck.pptx",
				"slides": []any{map[string]any{
					"layout_id": "blank-title", "slide_type": "content",
					"content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": tc.title}},
					"pattern": tc.pattern,
				}},
			}
			data, _ := json.Marshal(deck)
			in := filepath.Join(dir, "deck.json")
			if err := os.WriteFile(in, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := runJSONMode(in, filepath.Join(dir, "result.json"), templatesDir, dir, "", false, false, "", "off", false, "off", "", false); err != nil {
				t.Fatalf("runJSONMode: %v", err)
			}
			top := topmostPatternShapeY(t, filepath.Join(dir, "deck.pptx"))
			if min := tc.titleY + 2*tc.lineEMU; top < min {
				t.Errorf("first pattern shape at y=%d intrudes into the two-line title (text ends ≥ %d)", top, min)
			}
		})
	}
}

var shapeOffRe = regexp.MustCompile(`name="Shape \d+"[\s\S]*?<a:off x="\d+" y="(\d+)"/>`)

// topmostPatternShapeY returns the smallest Y of the generated "Shape N"
// pattern shapes on slide 1.
func topmostPatternShapeY(t *testing.T, path string) int64 {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.Name != "ppt/slides/slide1.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		top := int64(-1)
		for _, m := range shapeOffRe.FindAllStringSubmatch(string(b), -1) {
			y, _ := strconv.ParseInt(m[1], 10, 64)
			if top < 0 || y < top {
				top = y
			}
		}
		if top < 0 {
			t.Fatal("no pattern shapes on slide 1")
		}
		return top
	}
	t.Fatal("slide1.xml missing")
	return 0
}
