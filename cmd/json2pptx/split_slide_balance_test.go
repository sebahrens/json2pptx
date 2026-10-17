package main

import (
	"github.com/sebahrens/json2pptx/internal/deckinput"
	"testing"
)

func TestSplitSlideBalancedRowsRetainOriginalPaths(t *testing.T) {
	input := baseSplitSlide(makeRows(7), 3)
	pages, err := expandSplitSlide(input)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{0, 3, 5} {
		if pages[i].Origin.RowOffset != want {
			t.Errorf("page%d offset=%d want%d", i, pages[i].Origin.RowOffset, want)
		}
	}
	for i, want := range []int{3, 2, 2} {
		_, table := findTableContent(pages[i].Content)
		if len(table.Rows) != want {
			t.Errorf("page%d has%d rows want%d", i, len(table.Rows), want)
		}
	}
}

func TestSplitSlideBalancePreservesMergedRows(t *testing.T) {
	rows := makeRows(9)
	rows[2][0].RowSpan = 2 // balanced boundary3 would cut this cell; authored4 is legal.
	pages, err := expandSplitSlide(baseSplitSlide(rows, 4))
	if err != nil {
		t.Fatal(err)
	}
	offsets := []int{0, 4, 6}
	for i, want := range []int{4, 2, 3} {
		if pages[i].Origin.RowOffset != offsets[i] {
			t.Fatalf("page%d offset=%d want%d", i, pages[i].Origin.RowOffset, offsets[i])
		}
		_, table := findTableContent(pages[i].Content)
		if len(table.Rows) != want {
			t.Fatalf("page%d cuts merged group: %d rows", i, len(table.Rows))
		}
	}
}

func TestSplitSlideBalancedChunksPreserveRowsAndMergedGroups(t *testing.T) {
	for _, tc := range []struct{ rows, ceiling int }{{7, 3}, {9, 4}, {10, 4}, {17, 6}, {6, 100}} {
		rows := makeRows(tc.rows)
		chunks := balancedTableChunks(rows, tc.ceiling)
		offset := 0
		for _, chunk := range chunks {
			if len(chunk) == 0 || len(chunk) > tc.ceiling {
				t.Fatalf("bad page size: %d", len(chunk))
			}
			for _, row := range chunk {
				if row[0].Content != rows[offset][0].Content {
					t.Fatalf("row order changed at%d", offset)
				}
				offset++
			}
			if offset < len(rows) && tableBoundaryCrossesSpan(rows, offset) {
				t.Fatalf("cut merged row at%d", offset)
			}
		}
		if offset != len(rows) {
			t.Fatalf("kept%d/%d rows", offset, len(rows))
		}
	}
	rows := makeRows(9)
	rows[2][0].RowSpan = 2
	chunks := balancedTableChunks(rows, 4)
	offset := 0
	for _, chunk := range chunks {
		offset += len(chunk)
		if offset < len(rows) && tableBoundaryCrossesSpan(rows, offset) {
			t.Fatal("balanced boundary cuts rowspan")
		}
	}
}

func TestSplitTargetedTableRetainsCumulativeOriginOffsets(t *testing.T) {
	input := baseSplitSlide(makeRows(9), 4)
	input.Base.Origin = &deckinput.SlideOrigin{Pointer: "/slides/2", SplitPage: true, TableContent: 1, TableField: "table_value", RowOffset: 20}
	input.Base.Content[1].TableValue.Rows[2][0].RowSpan = 2
	pages, err := expandSplitSlideAtTable(input, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{20, 24, 26} {
		if pages[i].Origin == nil || pages[i].Origin.RowOffset != want || pages[i].Origin.Pointer != "/slides/2" {
			t.Fatalf("page%d origin=%+v wantoffset%d", i, pages[i].Origin, want)
		}
	}
	if input.Base.Origin.RowOffset != 20 {
		t.Fatal("base origin mutated")
	}
}
