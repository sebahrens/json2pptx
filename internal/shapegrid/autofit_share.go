package shapegrid

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Sibling cells share one autofit shrink (go-slide-creator-csclk.99).
//
// Every shape_grid text body is normAutofit and each shape measured its own
// fontScale, so in a row of timeline labels, waterfall labels or option-matrix
// cells the one long label shrank alone and the row rendered at uneven sizes.
// Cells in the same row that declare the same font size are siblings: they take
// the smallest shrink any of them needs.
//
// Peers beyond the row share it too (go-slide-creator-riyh7): the cells of a
// heatmap, the cards of a 3 x 2 grid and the labels of one column are one set
// (peerGroups), and a shrink taken row by row set the row with the one long
// activity at 10.8pt between rows at 12pt.

// shareRowAutofitScale sets AutofitScale on same-size sibling shape cells —
// those of one row, and those of one peer group — to the smallest shrink the
// group needs. Groups where every cell fits are left alone (AutofitScale 0),
// so a group that needs no shrink is unchanged.
func shareRowAutofitScale(cells []ResolvedCell) {
	type member struct {
		idx    int
		sizeHP int
		scale  float64
	}
	var members []member
	for i := range cells {
		c := &cells[i]
		// A layer is not a row sibling: it shares its cell, not a row slot.
		if c.Layer || c.Kind != CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
			continue
		}
		tb, err := ResolveTextInput(c.ShapeSpec.Text)
		if err != nil || tb == nil || tb.AutoFit != "normAutofit" {
			continue
		}
		size := firstRunSizeHP(tb)
		if size <= 0 {
			continue
		}
		for j := 0; j < 4; j++ {
			tb.Insets[j] += c.TextInsets[j]
		}
		tb.ThemeFonts = c.ShapeSpec.ThemeFonts
		members = append(members, member{idx: i, sizeHP: size, scale: pptx.AutofitScaleFor(tb, c.Bounds)})
	}
	if len(members) < 2 {
		return
	}
	shrinks := false
	for _, m := range members {
		if m.scale < 1 {
			shrinks = true
			break
		}
	}
	if !shrinks {
		return
	}
	peers := peerGroups(cells)
	group := make([]int, len(members))
	for i := range group {
		group[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if group[i] != i {
			group[i] = find(group[i])
		}
		return group[i]
	}
	for i := range members {
		for j := i + 1; j < len(members); j++ {
			a, b := members[i], members[j]
			if a.sizeHP != b.sizeHP {
				continue
			}
			sameRow := cells[a.idx].RowIdx == cells[b.idx].RowIdx
			samePeers := peers[a.idx] >= 0 && peers[a.idx] == peers[b.idx]
			if sameRow || samePeers {
				group[find(i)] = find(j)
			}
		}
	}
	least := map[int]float64{}
	count := map[int]int{}
	for i, m := range members {
		g := find(i)
		count[g]++
		if cur, ok := least[g]; !ok || m.scale < cur {
			least[g] = m.scale
		}
	}
	for i, m := range members {
		g := find(i)
		if count[g] < 2 || least[g] >= 1 {
			continue
		}
		cells[m.idx].AutofitScale = least[g]
		cells[m.idx].autofitGroup = g + 1
	}
}

// A shared shrink is written as sizes, not as a stored scale
// (go-slide-creator-5x4w4).
//
// PowerPoint applies the fontScale a shape stores; LibreOffice ignores it and
// fits every shape again on its own. A row whose cells shared a stored scale
// therefore rendered at one size in PowerPoint and, in LibreOffice, with only
// the long label shrunk — the uneven row the shared scale was meant to
// prevent. writeSharedShrink writes the group's shrink into the text sizes of
// the resolved copy instead, so every renderer draws the row at one size and
// no cell of it needs a stored scale to fit. Text the shrink would take under
// the renderer's size floor (MinTextSizePt) cannot be written that way, and
// neither can text whose runs carry sizes of their own: such a cell keeps its
// stored scale, and validation says so (fit_overflow at action "info": the
// renderer re-fits the text; TEXT_BELOW_READABLE_MIN when the result is too
// small to read).
//
// A cell with no same-size sibling in its row never shared a scale, so the
// writer measured it alone and stored the shrink on the shape: the lone "4"
// of an authored axis label was 24pt at 92% in PowerPoint and whatever
// LibreOffice made of the box. It takes the same route (go-slide-creator-217cd):
// its shrink is written into its sizes.

// writeSharedShrink replaces the AutofitScale shareRowAutofitScale assigned
// to a row's sibling group with shrunk text sizes, when every cell of the
// group can take them, and writes the shrink of a cell that shares none into
// its own sizes. It runs once on the final result: the composition and canvas
// trials compare cells at their designed sizes.
func writeSharedShrink(cells []ResolvedCell) {
	type key struct {
		row   int // a cell on its own (negative), or the row of a group set by hand
		group int // the peer group shareRowAutofitScale shared the scale across
		scale float64
	}
	groups := map[key][]int{}
	var order []key
	for i := range cells {
		c := &cells[i]
		if c.Kind != CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
			continue
		}
		k := key{row: -1 - i} // a cell on its own
		switch {
		case c.AutofitScale > 0 && c.AutofitScale < 1:
			k = key{row: c.RowIdx, scale: c.AutofitScale}
			if c.autofitGroup > 0 {
				k = key{group: c.autofitGroup, scale: c.AutofitScale}
			}
		case c.AutofitScale == 0:
			if k.scale = canvasAutofit(c); k.scale >= 1 {
				continue
			}
		default:
			continue
		}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], i)
	}
	for _, k := range order {
		idxs := groups[k]
		// Sizes are rounded down to a hundredth of a point and a shrunk
		// paragraph may break its lines elsewhere, so the measured scale can
		// leave a cell a hair over; a slightly smaller one is tried before the
		// group is left with its stored scale.
		for _, scale := range []float64{k.scale, k.scale - writtenShrinkStep, k.scale - 2*writtenShrinkStep} {
			specs := shrunkSpecs(cells, idxs, scale)
			if specs == nil {
				continue
			}
			for n, i := range idxs {
				cells[i].ShapeSpec = specs[n]
				cells[i].AutofitScale = 0
			}
			break
		}
	}
}

// writtenShrinkStep is the step by which writeSharedShrink lowers a scale
// whose written sizes the writer would still shrink: the step of the
// writer's own scale search.
const writtenShrinkStep = 0.02

// shrunkSpecs returns, for each cell of idxs, a copy of its shape with the
// text sizes at scale; nil when a cell cannot take them (shrunkText) or the
// writer would still shrink one at the written sizes.
func shrunkSpecs(cells []ResolvedCell, idxs []int, scale float64) []*ShapeSpec {
	if scale <= 0 {
		return nil
	}
	specs := make([]*ShapeSpec, 0, len(idxs))
	for _, i := range idxs {
		c := &cells[i]
		text, ok := shrunkText(c.ShapeSpec.Text, scale)
		if !ok {
			return nil
		}
		spec := *c.ShapeSpec
		spec.Text = text
		trial := *c
		trial.ShapeSpec = &spec
		if canvasAutofit(&trial) < 1 {
			return nil
		}
		specs = append(specs, &spec)
	}
	return specs
}

// shrunkText returns raw with every paragraph's size (and suffix size) times
// scale, rounded down to a hundredth of a point. ok is false when a size
// would fall under MinTextSizePt or the text does not resolve to exactly the
// shrunk sizes (inline runs with sizes of their own).
func shrunkText(raw json.RawMessage, scale float64) (json.RawMessage, bool) {
	before, err := ResolveTextInput(raw)
	if err != nil || before == nil {
		return nil, false
	}
	shrink := func(obj map[string]json.RawMessage, content string) bool {
		var size, suffix float64
		_ = json.Unmarshal(obj["size"], &size)
		if strings.TrimSpace(content) == "" && size <= 0 {
			return true
		}
		if size <= 0 {
			size = DefaultTextSizePt
		}
		next := scaledSize(EffectiveTextSizePt(size), scale)
		if next < MinTextSizePt {
			return false
		}
		obj["size"], _ = json.Marshal(next)
		if json.Unmarshal(obj["suffix_size"], &suffix) == nil && suffix > 0 {
			obj["suffix_size"], _ = json.Marshal(math.Max(scaledSize(suffix, scale), MinTextSizePt))
		}
		return true
	}
	var s string
	var out json.RawMessage
	if json.Unmarshal(raw, &s) == nil {
		obj := map[string]json.RawMessage{}
		obj["content"], _ = json.Marshal(s)
		if !shrink(obj, s) {
			return nil, false
		}
		out, _ = json.Marshal(obj)
	} else {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			return nil, false
		}
		if rawParas, ok := obj["paragraphs"]; ok {
			var defs []map[string]json.RawMessage
			if json.Unmarshal(rawParas, &defs) != nil {
				return nil, false
			}
			for i := range defs {
				var content string
				_ = json.Unmarshal(defs[i]["content"], &content)
				if !shrink(defs[i], content) {
					return nil, false
				}
			}
			obj["paragraphs"], _ = json.Marshal(defs)
		} else {
			var content string
			_ = json.Unmarshal(obj["content"], &content)
			if !shrink(obj, content) {
				return nil, false
			}
		}
		out, _ = json.Marshal(obj)
	}
	after, err := ResolveTextInput(out)
	if err != nil || after == nil || !shrunkBy(before, after, scale) {
		return nil, false
	}
	return out, true
}

// shrunkBy reports whether after is before with every sized run at scale
// times its size (to the hundredth of a point the writer keeps).
func shrunkBy(before, after *pptx.TextBody, scale float64) bool {
	if len(after.Paragraphs) != len(before.Paragraphs) {
		return false
	}
	for i, p := range before.Paragraphs {
		if len(after.Paragraphs[i].Runs) != len(p.Runs) {
			return false
		}
		for j, r := range p.Runs {
			if strings.TrimSpace(r.Text) == "" || r.FontSize <= 0 {
				continue
			}
			got := after.Paragraphs[i].Runs[j].FontSize
			if got >= r.FontSize || math.Abs(float64(got)-float64(r.FontSize)*scale) > 1.5 {
				return false
			}
		}
	}
	return true
}

// firstRunSizeHP returns the font size (hundredths of a point) of the body's
// first sized run, or 0 when none declares one.
func firstRunSizeHP(tb *pptx.TextBody) int {
	for _, p := range tb.Paragraphs {
		for _, r := range p.Runs {
			if r.Text != "" && r.FontSize > 0 {
				return r.FontSize
			}
		}
	}
	return 0
}

// GenerateCellShapeXML renders a resolved shape cell, applying the row-shared
// autofit shrink where one is still stored (writeSharedShrink).
func GenerateCellShapeXML(cell ResolvedCell) ([]byte, error) {
	return generateShapeXML(cell.ShapeSpec, cell.ID, cell.Bounds, cell.AutofitScale, cell.TextInsets)
}
