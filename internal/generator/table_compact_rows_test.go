package generator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// eightRowTable is a one-line-per-row price list: the table a consulting deck
// shows on one slide and the one the 0.4in row floor used to truncate
// (go-slide-creator-plg7r).
func eightRowTable() *types.TableSpec {
	rows := make([][]types.TableCell, 8)
	for i := range rows {
		rows[i] = []types.TableCell{
			{Content: fmt.Sprintf("Week %d", i/2+1), ColSpan: 1, RowSpan: 1},
			{Content: fmt.Sprintf("Deliverable %d", i+1), ColSpan: 1, RowSpan: 1},
			{Content: fmt.Sprintf("%d", 8+i), ColSpan: 1, RowSpan: 1},
		}
	}
	return &types.TableSpec{
		Headers: []string{"Phase", "Deliverable", "Days"},
		Rows:    rows,
		Style:   types.DefaultTableStyle,
	}
}

func compactFindingCodes(findings []patterns.FitFinding) []string {
	codes := make([]string, 0, len(findings))
	for _, f := range findings {
		codes = append(codes, f.Code)
	}
	return codes
}

func findingWithCode(findings []patterns.FitFinding, code string) *patterns.FitFinding {
	for i := range findings {
		if findings[i].Code == code {
			return &findings[i]
		}
	}
	return nil
}

var trHeightRE = regexp.MustCompile(`<a:tr h="(\d+)"`)

func emittedRowHeights(t *testing.T, xml string) []int64 {
	t.Helper()
	var out []int64
	for _, m := range trHeightRE.FindAllStringSubmatch(xml, -1) {
		h, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			t.Fatalf("row height %q: %v", m[1], err)
		}
		out = append(out, h)
	}
	return out
}

// A table that overflows its frame at the 0.4in row floor but fits at the
// compact pitch renders every row at the compact pitch — silently, like any
// other row-height decision — instead of truncating.
func TestGenerateTableXML_CompactsRowsBeforeTruncating(t *testing.T) {
	table := eightRowTable()
	// 3.0in: nine rows at 0.4in need 3.6in; at the ~0.3in compact pitch of
	// 12pt text they need about 2.7in.
	config := TableRenderConfig{
		Bounds: types.BoundingBox{X: 457200, Y: 914400, Width: 8229600, Height: 2743200},
		Style:  table.Style,
	}
	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f := findingWithCode(result.Findings, patterns.ErrCodeTableRowsTruncated); f != nil {
		t.Fatalf("table that fits at the compact pitch was truncated: %s (findings %v)", f.Message, compactFindingCodes(result.Findings))
	}
	if strings.Contains(result.XML, "more rows") {
		t.Fatal("summary row emitted although every row fits at the compact pitch")
	}
	heights := emittedRowHeights(t, result.XML)
	if len(heights) != 9 {
		t.Fatalf("expected 9 rows (header + 8), got %d", len(heights))
	}
	if !strings.Contains(result.XML, "Deliverable 8") {
		t.Error("last data row missing")
	}
	var total int64
	for i, h := range heights {
		total += h
		if h >= defaultRowHeight {
			t.Errorf("row %d is %d EMU, still at or above the 0.4in floor after compaction", i, h)
		}
	}
	if total > config.Bounds.Height {
		t.Errorf("compacted rows total %d EMU, over the %d EMU frame", total, config.Bounds.Height)
	}
}

// A table that fits at the 0.4in floor keeps today's row pitch: existing
// decks do not change.
func TestGenerateTableXML_FittingTableKeepsDefaultPitch(t *testing.T) {
	table := eightRowTable()
	config := TableRenderConfig{
		Bounds: types.BoundingBox{X: 457200, Y: 914400, Width: 8229600, Height: 4572000}, // 5in
		Style:  table.Style,
	}
	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, h := range emittedRowHeights(t, result.XML) {
		if h < defaultRowHeight {
			t.Errorf("row %d is %d EMU, under the 0.4in floor although the table fits", i, h)
		}
	}
}

// Past the compact pitch the table still truncates — with a refuse-class
// finding — but keeps the rows the compact pitch affords, not the 0.4in count.
func TestGenerateTableXML_TruncatesAfterCompaction(t *testing.T) {
	table := eightRowTable()
	config := TableRenderConfig{
		Bounds: types.BoundingBox{X: 457200, Y: 914400, Width: 8229600, Height: 1828800}, // 2in
		Style:  table.Style,
	}
	result, err := GenerateTableXML(table, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	trunc := findingWithCode(result.Findings, patterns.ErrCodeTableRowsTruncated)
	if trunc == nil {
		t.Fatalf("expected truncation in a 2in frame, got %v", compactFindingCodes(result.Findings))
	}
	if trunc.Action != "refuse" {
		t.Errorf("truncation action = %q, want refuse", trunc.Action)
	}
	// At 0.4in a 2in frame holds header + 3 rows + summary; the compact pitch
	// keeps at least one more.
	visible, _ := trunc.Fix.Params["visible_rows"].(int)
	if visible < 4 {
		t.Errorf("compact pitch should keep at least 4 rows before the summary, kept %d", visible)
	}
}

// Preflight predicts the same outcome as generation: no truncation for a
// table the compact pitch fits.
func TestDetectTablePreflight_PredictsCompaction(t *testing.T) {
	table := eightRowTable()
	findings := DetectTablePreflight(TablePreflightInput{
		Path:    "/slides/0/content/1",
		Headers: table.Headers,
		Rows:    table.Rows,
		Style:   table.Style,
		Bounds:  types.BoundingBox{X: 457200, Y: 914400, Width: 8229600, Height: 2743200},
	})
	if f := findingWithCode(findings, patterns.ErrCodeTableRowsTruncated); f != nil {
		t.Fatalf("preflight predicted truncation for a table generation compacts: %s", f.Message)
	}
}
