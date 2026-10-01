package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// solidAccentBlockMinEMU is the smaller side a solid-accent shape must reach
// to count as a "block" (0.5in). Rules, connector arrows, timeline dots,
// milestone diamonds and step-number discs are accent marks, not blocks.
const solidAccentBlockMinEMU int64 = 457200

// accentIsTheMessage lists the patterns whose exemplars legitimately carry
// more than one solid-accent block: the fill IS the data rather than
// structure.
var accentIsTheMessage = map[string]string{
	"capability-heatmap": "tier fills encode the rating",
	"waterfall-bridge":   "total and subtotal bars are the data",
}

// accentRestraintPending lists patterns that still default to a row of solid
// accent blocks and have no fix yet. The list only shrinks: a pattern that
// starts passing must be removed (the test says so), and a pattern not listed
// here may not regress.
var accentRestraintPending = map[string]string{
	"agenda-with-images":  "numbered accent squares per row",
	"labeled-rows":        "label_style filled (default) is a solid accent block per row",
	"numbered-step-strip": "stacked-box number lanes are solid accent blocks",
	"process-grid-2row":   "phase boxes are solid accent / accent-shade blocks",
}

// pStyleTheme is p-style's palette: a light orange accent1 on white paper,
// the template the review's walls of colour were most visible on.
func pStyleTheme() types.ThemeInfo {
	return types.ThemeInfo{Colors: []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk2", RGB: "#000000"}, {Name: "lt2", RGB: "#FFFFFF"},
		{Name: "accent1", RGB: "#FD5108"}, {Name: "accent2", RGB: "#FE7C39"},
		{Name: "accent3", RGB: "#FFAA72"}, {Name: "accent4", RGB: "#A1A8B3"},
		{Name: "accent5", RGB: "#B5BCC4"}, {Name: "accent6", RGB: "#CBD1D6"},
	}}
}

// solidAccentBlocks counts the resolved shapes filled with an unmodified,
// opaque accent slot and large enough to read as a block.
func solidAccentBlocks(cells []shapegrid.ResolvedCell) (n int, where []string) {
	for _, c := range cells {
		if c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil {
			continue
		}
		if !isSolidAccentFill(c.ShapeSpec.Fill) {
			continue
		}
		if c.Bounds.CX < solidAccentBlockMinEMU || c.Bounds.CY < solidAccentBlockMinEMU {
			continue
		}
		n++
		where = append(where, fmt.Sprintf("%s %.2fx%.2fin", c.ShapeSpec.Geometry, float64(c.Bounds.CX)/914400, float64(c.Bounds.CY)/914400))
	}
	return n, where
}

func isSolidAccentFill(raw json.RawMessage) bool {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.HasPrefix(s, "accent")
	}
	var obj struct {
		Color  string  `json:"color"`
		Alpha  float64 `json:"alpha"`
		LumMod int     `json:"lumMod"`
		LumOff int     `json:"lumOff"`
		Tint   int     `json:"tint"`
		Shade  int     `json:"shade"`
	}
	if json.Unmarshal(raw, &obj) != nil || !strings.HasPrefix(obj.Color, "accent") {
		return false
	}
	if obj.Alpha != 0 && obj.Alpha < 90 {
		return false
	}
	return obj.LumMod == 0 && obj.LumOff == 0 && obj.Tint == 0 && obj.Shade == 0
}

// TestExemplarGallery_AccentRestraint is the go-slide-creator-fl11f epic
// acceptance: every pattern exemplar renders with at most one solid-accent
// block by default, except where the accent fill is the data itself.
func TestExemplarGallery_AccentRestraint(t *testing.T) {
	reg := patterns.Default()
	var names []string
	for _, p := range reg.List() {
		if _, ok := p.(patterns.Exemplar); ok {
			names = append(names, p.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		pat, _ := reg.Get(name)
		values, err := json.Marshal(pat.(patterns.Exemplar).ExemplarValues())
		if err != nil {
			t.Fatal(err)
		}
		ctx := patterns.ExpandContext{
			SlideWidth:   12192000,
			SlideHeight:  6858000,
			LayoutBounds: patterns.LayoutBounds{X: contentRect.X, Y: contentRect.Y, Width: contentRect.CX, Height: contentRect.CY},
			Theme:        pStyleTheme(),
		}
		grid, _, err := expandPattern(&PatternInput{Name: name, Values: values}, ctx, reg)
		if err != nil {
			t.Errorf("%s: expand: %v", name, err)
			continue
		}
		b := contentRect
		res, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), &b, nil, 12192000, 6858000, nil)
		if err != nil {
			t.Errorf("%s: resolve: %v", name, err)
			continue
		}
		n, where := solidAccentBlocks(res.Cells)
		if _, ok := accentIsTheMessage[name]; ok {
			continue
		}
		if _, pending := accentRestraintPending[name]; pending {
			if n <= 1 {
				t.Errorf("%s now renders %d solid-accent block(s); remove it from accentRestraintPending", name, n)
			}
			continue
		}
		if n > 1 {
			t.Errorf("%s: %d solid-accent blocks by default %v, want <= 1 (go-slide-creator-fl11f)", name, n, where)
		}
	}
}
