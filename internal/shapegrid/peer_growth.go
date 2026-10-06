package shapegrid

import "github.com/sebahrens/json2pptx/internal/pptx"

// Peers share one type size (go-slide-creator-riyh7).
//
// Grow-to-fill (type_scale "comfortable" / "presentation") measured every
// shape on its own: the card with two words grew to 14pt, its neighbour with
// two lines stayed at 12pt, and the people of one directory row, the rows of
// one SCQA and the columns of one framework were set at different sizes. A
// designer picks ONE size for a set of peers, the largest that the fullest of
// them can take. sharePeerGrowth gives every peer group the smallest growth
// any of its members measured, so a group grows together or not at all.
//
// Peers are read off the resolved geometry, so the rule holds for every
// pattern and for authored grids alike, with nothing to declare:
//
//   - two text shapes are peers when they have the same preset geometry, their
//     first paragraphs are set alike (same size and weight) and their frames
//     stand as peers (peerAligned): the same size, or a shared row or column
//     edge with one dimension in common;
//   - a peer group is a connected set of that relation: the cards of a 3 x 2
//     grid are one group, as are the row labels of a table.
//
// Fill is deliberately not part of it: the highlighted card of a row is still
// a peer of the cards beside it. A shape that does not grow (its pattern
// pinned it "compact", or its text already fills its box) holds its whole
// group at the size it was given.

// peerGrowth is one resolved cell's own answer to "how far may this text
// grow": the scale and the paragraphs it was measured with.
type peerGrowth struct {
	scale float64
	paras []scaleParagraph
}

// peerBandTolEMU is how far two edges may differ and still be the same row or
// column: one point, well under any gap a grid resolves.
const peerBandTolEMU = 12700

// peerKey is what two shapes must have in common, besides a row or a column,
// to be peers.
type peerKey struct {
	geometry string
	sizeHP   int
	bold     bool
}

// peerAligned reports whether two frames stand as peers: they are the same
// size (the steps of a swimlane, wherever their lanes put them), or they share
// an edge of a row or a column and one dimension with it — the cards of a row
// (same top, same height), the steps of a staircase (same foot, same width),
// the cells of a column (same left, same width), the bars of a ranked list
// (same left, same height).
func peerAligned(a, b pptx.RectEmu) bool {
	near := func(p, q int64) bool {
		d := p - q
		if d < 0 {
			d = -d
		}
		return d <= peerBandTolEMU
	}
	sameW, sameH := near(a.CX, b.CX), near(a.CY, b.CY)
	if sameW && sameH {
		return true
	}
	if !sameW && !sameH {
		return false
	}
	row := near(a.Y, b.Y) || near(a.Y+a.CY, b.Y+b.CY)
	col := near(a.X, b.X) || near(a.X+a.CX, b.X+b.CX)
	return row || col
}

// peerGroups returns, for each cell, the index of its peer group's
// representative; -1 for a cell that carries no sized text.
func peerGroups(cells []ResolvedCell) []int {
	keys := make([]peerKey, len(cells))
	keyed := make([]bool, len(cells))
	for i := range cells {
		keys[i], keyed[i] = peerKeyOf(&cells[i])
	}
	set := newDisjointSet(len(cells))
	for i := range cells {
		if !keyed[i] {
			continue
		}
		for j := i + 1; j < len(cells); j++ {
			if keyed[j] && keys[i] == keys[j] && peerAligned(cells[i].Bounds, cells[j].Bounds) {
				set.union(i, j)
			}
		}
	}
	groups := make([]int, len(cells))
	for i := range groups {
		groups[i] = -1
		if keyed[i] {
			groups[i] = set.find(i)
		}
	}
	return groups
}

// peerKeyOf is the key of a cell that carries sized text in a real frame: its
// geometry and the size and weight of its first sized run.
func peerKeyOf(c *ResolvedCell) (peerKey, bool) {
	if c.Kind != CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 || c.Bounds.CX <= 0 || c.Bounds.CY <= 0 {
		return peerKey{}, false
	}
	tb, err := ResolveTextInput(c.ShapeSpec.Text)
	if err != nil || tb == nil {
		return peerKey{}, false
	}
	geometry := c.ShapeSpec.Geometry
	if geometry == "" {
		geometry = "rect"
	}
	for _, p := range tb.Paragraphs {
		for _, run := range p.Runs {
			if run.FontSize > 0 {
				return peerKey{geometry: geometry, sizeHP: run.FontSize, bold: run.Bold}, true
			}
		}
	}
	return peerKey{}, false
}

// disjointSet is a union-find over n items.
type disjointSet []int

func newDisjointSet(n int) disjointSet {
	set := make(disjointSet, n)
	for i := range set {
		set[i] = i
	}
	return set
}

func (s disjointSet) find(i int) int {
	for s[i] != i {
		s[i] = s[s[i]]
		i = s[i]
	}
	return i
}

func (s disjointSet) union(a, b int) { s[s.find(a)] = s.find(b) }

// sharePeerGrowth lowers every cell's growth to the smallest its peer group
// measured.
func sharePeerGrowth(cells []ResolvedCell, growth []peerGrowth) {
	grows := false
	for i := range growth {
		if growth[i].scale > 1 {
			grows = true
			break
		}
	}
	if !grows {
		return
	}
	groups := peerGroups(cells)
	least := map[int]float64{}
	for i, g := range groups {
		if g < 0 {
			continue
		}
		if cur, ok := least[g]; !ok || growth[i].scale < cur {
			least[g] = growth[i].scale
		}
	}
	for i, g := range groups {
		if g < 0 {
			continue
		}
		if s := least[g]; s < growth[i].scale {
			growth[i].scale = s
		}
	}
}
