package shapegrid

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Peers that live in sibling grids — the labels of a ranked bar list, one
// sub-grid per bar — share one growth through a PeerScope
// (go-slide-creator-7ophx): measured apart, the short labels grew past the
// long one.
func TestPeerScopeSharesGrowthAcrossSiblingGrids(t *testing.T) {
	long := "A sentence long enough to fill its card at the size it was given, so that it cannot grow any further without leaving the share of the box the policy allows it to take, however the row beside it is filled"
	// Three sibling grids stacked as rows of one parent: same width, same
	// height, one label each.
	siblings := func(texts ...string) []*Grid {
		grids := make([]*Grid, len(texts))
		for i, text := range texts {
			g := peerTestGrid("presentation", text)
			g.Bounds = pptx.RectEmu{X: 0, Y: int64(i) * 100 * 12700, CX: 200 * 12700, CY: 90 * 12700}
			grids[i] = g
		}
		return grids
	}
	resolveAll := func(grids []*Grid, measure bool, scope *PeerScope) ([]*ResolveResult, []int) {
		var results []*ResolveResult
		var sizes []int
		for _, g := range grids {
			g.MeasurePeers, g.Peers = measure, scope
			res, err := Resolve(g, pptx.NewShapeIDAllocator(nil))
			if err != nil {
				t.Fatal(err)
			}
			results = append(results, res)
			tb, err := ResolveTextInput(res.Cells[0].ShapeSpec.Text)
			if err != nil {
				t.Fatal(err)
			}
			sizes = append(sizes, firstRunSizeHP(tb))
		}
		return results, sizes
	}

	measured, apart := resolveAll(siblings("Vendor A", long, "Vendor C"), true, nil)
	if apart[0] <= 1200 || apart[0] != apart[2] || apart[1] >= apart[0] {
		t.Fatalf("setup: resolved apart, the short labels should grow past the long one, got %v", apart)
	}
	scope := NewPeerScope(measured...)
	if scope == nil {
		t.Fatal("the long label holds its siblings back: want a scope")
	}
	_, together := resolveAll(siblings("Vendor A", long, "Vendor C"), false, scope)
	if together[0] != together[1] || together[1] != together[2] {
		t.Errorf("sibling peers are written at several sizes under the scope: %v", together)
	}
	if together[0] != apart[1] {
		t.Errorf("the group takes the smallest growth any member measured (%d), got %v", apart[1], together)
	}

	// Even siblings need no second pass, and a measuring resolve writes the
	// sizes a plain one does.
	even, evenSizes := resolveAll(siblings("Vendor A", "Vendor B", "Vendor C"), true, nil)
	if NewPeerScope(even...) != nil {
		t.Error("siblings that agree hold nothing: want a nil scope")
	}
	if _, plain := resolveAll(siblings("Vendor A", "Vendor B", "Vendor C"), false, nil); plain[0] != evenSizes[0] {
		t.Errorf("measuring changed the written size: %v vs %v", evenSizes, plain)
	}

	// A frame that is no peer (another size, no shared edge) is not held.
	other := peerTestGrid("presentation", "Vendor Z")
	other.Bounds = pptx.RectEmu{X: 400 * 12700, Y: 37 * 12700, CX: 260 * 12700, CY: 140 * 12700}
	alone, _ := resolveAll([]*Grid{other}, true, nil)
	withOther := NewPeerScope(append(measured, alone...)...)
	_, held := resolveAll([]*Grid{func() *Grid {
		g := peerTestGrid("presentation", "Vendor Z")
		g.Bounds = other.Bounds
		return g
	}()}, false, withOther)
	_, free := resolveAll([]*Grid{func() *Grid {
		g := peerTestGrid("presentation", "Vendor Z")
		g.Bounds = other.Bounds
		return g
	}()}, false, nil)
	if held[0] != free[0] {
		t.Errorf("an unrelated frame was held by the scope: %d, on its own %d", held[0], free[0])
	}
}
