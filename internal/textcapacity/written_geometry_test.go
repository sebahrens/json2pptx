package textcapacity

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func TestResolvedGridDensityUsesWrittenGeometry(t *testing.T) {
	cell := shapegrid.ResolvedCell{
		Kind:       shapegrid.CellKindShape,
		Bounds:     pptx.RectEmu{CX: 400 * 12700, CY: 36 * 12700},
		CellBounds: pptx.RectEmu{CX: 500 * 12700, CY: 100 * 12700},
		ShapeSpec:  &shapegrid.ShapeSpec{Text: json.RawMessage(`{"content":"First line\nSecond line","size":13,"inset_left":2,"inset_right":2,"inset_top":2,"inset_bottom":2}`)},
	}
	d := ForResolvedGrid(&shapegrid.ResolveResult{Cells: []shapegrid.ResolvedCell{cell}})[0]
	if d.WidthEMU != 396*12700 || d.AvailableHeightPt != 32 || d.AutofitScale != 1 || !d.Fits {
		t.Fatalf("density disagrees with written shape's usable 396x32pt frame: %+v", d)
	}
}

func TestResolvedGridDensityPreservesWrittenParagraphSpacing(t *testing.T) {
	cell := shapegrid.ResolvedCell{Kind: shapegrid.CellKindShape, Bounds: pptx.RectEmu{CX: 400 * 12700, CY: 60 * 12700},
		ShapeSpec: &shapegrid.ShapeSpec{Text: json.RawMessage(`{"inset_top":2,"inset_bottom":2,"paragraphs":[{"content":"First line","size":13,"space_after":20},{"content":"Second line","size":13}]}`)},
	}
	cell.CellBounds = cell.Bounds
	d := ForResolvedGrid(&shapegrid.ResolveResult{Cells: []shapegrid.ResolvedCell{cell}})[0]
	if math.Abs(d.RequiredHeightPt-51.2) > .001 || d.AvailableHeightPt != 56 {
		t.Fatalf("written 20pt paragraph spacing lost: %+v", d)
	}
}

func TestResolvedGridDensityNeverCallsPopulatedEmptyFrameUnderfilled(t *testing.T) {
	cell := shapegrid.ResolvedCell{Kind: shapegrid.CellKindShape, Bounds: pptx.RectEmu{CX: 400 * 12700, CY: 5 * 12700},
		ShapeSpec: &shapegrid.ShapeSpec{Text: json.RawMessage(`"Required source text"`)},
	}
	cell.CellBounds = cell.Bounds
	d := ForResolvedGrid(&shapegrid.ResolveResult{Cells: []shapegrid.ResolvedCell{cell}})[0]
	if d.Status != StatusOverflow || d.Fits || d.MaxChars != 0 {
		t.Fatalf("empty usable frame mislabeled or given a text budget: %+v", d)
	}
}
