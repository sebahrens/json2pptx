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
	// 14pt x 1.2 line height + the uniform 0.5 cm top and bottom margin
	// (2 x 14.17pt) = 45.1pt.
	if got := float64(estimateCellTextHeightEMU(oneLine)) / 12700; got < 44 || got > 46 {
		t.Errorf("one 14pt line estimated at %.1fpt, want ~45.1", got)
	}

	stack := cellWith(`{"paragraphs":[{"content":"A","size":12},{"content":"B","size":12},{"content":"C","size":12}]}`)
	// Three 12pt lines: 3 x 14.4 + 28.35 = 71.5pt.
	if got := float64(estimateCellTextHeightEMU(stack)) / 12700; got < 70 || got > 73 {
		t.Errorf("three 12pt lines estimated at %.1fpt, want ~71.5", got)
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

	// Explicit insets replace the default padding rather than adding to it
	// (go-slide-creator-csclk.112): 14.4 + 8 + 8 = 30.4pt.
	inset := cellWith(`{"paragraphs":[{"content":"A","size":12}],"inset_left":8,"inset_right":8,"inset_top":8,"inset_bottom":8}`)
	if got := float64(estimateCellTextHeightEMU(inset)) / 12700; got < 30 || got > 31 {
		t.Errorf("12pt line with 8pt insets estimated at %.1fpt, want ~30.4", got)
	}

	// The single-content form: 14.4 + 28.35 = 42.7pt.
	if got := float64(estimateCellTextHeightEMU(cellWith(`{"content":"One line","size":12}`))) / 12700; got < 42 || got > 44 {
		t.Errorf("single-content form estimated at %.1fpt, want ~42.7", got)
	}

	// An authored side replaces only that side's margin: an 8pt top beside
	// the default 14.17pt bottom is 14.4 + 8 + 14.17 = 36.6pt.
	top := cellWith(`{"paragraphs":[{"content":"A","size":12}],"inset_top":8}`)
	if got := float64(estimateCellTextHeightEMU(top)) / 12700; got < 36 || got > 37.2 {
		t.Errorf("12pt line with an 8pt top inset estimated at %.1fpt, want ~36.6", got)
	}
}
