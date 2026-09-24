package main

import (
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestTeamBiosPhotoFramesResolveSquareAndCentered(t *testing.T) {
	pattern, ok := patterns.Default().Get("team-bios")
	if !ok {
		t.Fatal("team-bios pattern is not registered")
	}
	for _, count := range []int{4, 8} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			members := make([]patterns.TeamBiosMember, count)
			for i := range members {
				members[i] = patterns.TeamBiosMember{Name: fmt.Sprintf("Person %d", i), Role: "Partner"}
				if i%2 == 0 {
					members[i].Photo = &jsonschema.GridImageInput{Path: "headshot.jpg"}
				}
			}
			grid, err := pattern.Expand(patterns.ExpandContext{}, &patterns.TeamBiosValues{Members: members}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			bounds := &pptx.RectEmu{X: 200000, Y: 800000, CX: 11000000, CY: 4500000}
			resolved, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), bounds, nil, 12192000, 6858000, nil)
			if err != nil {
				t.Fatal(err)
			}
			var photoCells int
			for _, cell := range resolved.Cells {
				if cell.RowIdx%2 != 0 {
					continue
				}
				photoCells++
				if photoCells == 1 {
					t.Logf("photo frame for %d members: %.1fpt square within %.1fx%.1fpt cell", count,
						float64(cell.Bounds.CX)/12700, float64(cell.CellBounds.CX)/12700, float64(cell.CellBounds.CY)/12700)
				}
				wantSize := cell.CellBounds.CX
				if cell.CellBounds.CY < wantSize {
					wantSize = cell.CellBounds.CY
				}
				if cell.Bounds.CX != wantSize || cell.Bounds.CY != wantSize ||
					cell.Bounds.X != cell.CellBounds.X+(cell.CellBounds.CX-wantSize)/2 ||
					cell.Bounds.Y != cell.CellBounds.Y+(cell.CellBounds.CY-wantSize)/2 {
					t.Errorf("photo frame at row %d col %d is not a centered square: frame=%+v cell=%+v", cell.RowIdx, cell.ColIdx, cell.Bounds, cell.CellBounds)
				}
			}
			if photoCells != count {
				t.Errorf("resolved %d photo frames, want %d", photoCells, count)
			}
			if len(resolved.ImageInserts) != count/2 {
				t.Errorf("resolved %d real headshots, want %d", len(resolved.ImageInserts), count/2)
			}
			for _, insert := range resolved.ImageInserts {
				if insert.ExtentCX != insert.ExtentCY {
					t.Errorf("real headshot insert is not square: %+v", insert)
				}
			}
		})
	}
}
