package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

// A configured deck grid can reduce the cell height enough to change fit and
// readability verdicts. Preflight must measure that rectangle as generation
// does, rather than the template's taller default (go-slide-creator-vrckb).
func TestFitAndReadabilityUseDeckRhythmBounds(t *testing.T) {
	reader, err := template.OpenTemplate(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	sw, sh := template.ParseSlideDimensions(reader)
	tightGrid := &GridConfig{TitleBaselinePct: 20, ContentTopPct: 24, ContentBottomPct: 43, LeftMarginPct: 8, RightMarginPct: 8}
	hasOverflow := func(input *PresentationInput) bool {
		for _, f := range generateFitReport(input, layouts, sw, sh) {
			if f.Code == patterns.ErrCodeFitOverflow && strings.Contains(f.Path, "shape_grid") {
				return true
			}
		}
		return false
	}
	text := strings.TrimSpace(strings.Repeat("Revenue ", 1000))
	shapeText := json.RawMessage(fmt.Sprintf(`{"content":%q,"size":18}`, text))
	slide := SlideInput{LayoutID: "content", ShapeGrid: &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect", Text: shapeText}}}}}}}
	base := &PresentationInput{Slides: []SlideInput{slide}}
	tight := &PresentationInput{Grid: tightGrid, Slides: []SlideInput{slide}}
	if hasOverflow(base) || !hasOverflow(tight) {
		t.Fatalf("fit_overflow should occur only in the tight rhythm area (template=%t, rhythm=%t)", hasOverflow(base), hasOverflow(tight))
	}
	baseRead := collectReadabilityFindings(base, layouts, sw, sh)
	tightRead := collectReadabilityFindings(tight, layouts, sw, sh)
	if len(baseRead) != 1 || len(tightRead) != 1 {
		t.Fatalf("expected one readability finding per deck, got template=%d rhythm=%d", len(baseRead), len(tightRead))
	}
	fontPt := regexp.MustCompile(`renders at ([0-9.]+)pt`)
	getFont := func(message string) float64 {
		m := fontPt.FindStringSubmatch(message)
		if len(m) != 2 {
			t.Fatalf("no measured font size in %q", message)
		}
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if basePt, tightPt := getFont(baseRead[0].Message), getFont(tightRead[0].Message); tightPt >= basePt {
		t.Errorf("tight rhythm should predict a smaller readable font: template %.1fpt, rhythm %.1fpt", basePt, tightPt)
	}
}
