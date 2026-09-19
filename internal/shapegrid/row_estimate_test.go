package shapegrid

import (
	"encoding/json"
	"testing"
)

// The row-overflow estimate used to unmarshal a cell's text into a
// {content,size} struct and nothing else. A paragraphs-form cell matched that
// struct with an empty content, so it fell through to the one-line-at-11pt
// default and every such row measured ~27pt whatever it held: rows sized to
// their own text reported a phantom overflow, and genuinely tall stacks
// reported none (go-slide-creator-wrsb).
func TestEstimateCellTextHeightReadsParagraphs(t *testing.T) {
	cellWith := func(text string) Cell {
		return Cell{Shape: &ShapeSpec{Text: json.RawMessage(text)}}
	}

	oneLine := cellWith(`{"paragraphs":[{"content":"Mandate published","size":14,"bold":true}]}`)
	// 14pt x 1.2 line height + 7.2pt of top/bottom inset = 24pt.
	if got := float64(estimateCellTextHeightEMU(oneLine)) / 12700; got < 23 || got > 25 {
		t.Errorf("one 14pt line estimated at %.1fpt, want ~24", got)
	}

	stack := cellWith(`{"paragraphs":[{"content":"A","size":12},{"content":"B","size":12},{"content":"C","size":12}]}`)
	// Three 12pt lines: 3 x 14.4 + 7.2 = 50.4pt.
	if got := float64(estimateCellTextHeightEMU(stack)) / 12700; got < 49 || got > 52 {
		t.Errorf("three 12pt lines estimated at %.1fpt, want ~50", got)
	}
	if estimateCellTextHeightEMU(stack) <= estimateCellTextHeightEMU(oneLine) {
		t.Error("a three-line stack must estimate taller than one line")
	}

	// space_after is part of the block a row has to hold.
	spaced := cellWith(`{"paragraphs":[{"content":"A","size":12,"space_after":10},{"content":"B","size":12}]}`)
	plain := cellWith(`{"paragraphs":[{"content":"A","size":12},{"content":"B","size":12}]}`)
	if estimateCellTextHeightEMU(spaced) <= estimateCellTextHeightEMU(plain) {
		t.Error("space_after must add to the estimate")
	}

	// An empty paragraph list carries no text, so it asks for no height.
	if got := estimateCellTextHeightEMU(cellWith(`{"paragraphs":[{"content":""}]}`)); got != 0 {
		t.Errorf("an empty paragraph estimated %d EMU, want 0", got)
	}

	// The single-content form is unchanged.
	if got := float64(estimateCellTextHeightEMU(cellWith(`{"content":"One line","size":12}`))) / 12700; got < 20 || got > 23 {
		t.Errorf("single-content form estimated at %.1fpt, want ~21.6", got)
	}
}
