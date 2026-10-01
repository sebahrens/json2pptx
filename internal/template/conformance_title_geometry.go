package template

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/types"
)

// titleGeometryToleranceEMU is the drift a title placeholder may show between
// content-family layouts before the headline visibly jumps: 0.1in.
const titleGeometryToleranceEMU int64 = 91440

// titleGeometryCheck is the conformance check name for the content-family
// title baseline rule (go-slide-creator-kyk01).
const titleGeometryCheck = "Content title geometry consistent"

// checkContentTitleGeometry requires the title placeholder of One Content,
// Two Content and Blank + Title layouts to share one offset, size and
// vertical anchor (±0.1in). A deck mixes bullet slides (One Content), side by
// side slides (Two Content) and pattern canvases (Blank + Title); when their
// titles differ the headline jumps from slide to slide — abstract's One
// Content title sat 0.5in right and ~1in lower than the other two. The
// reference is the first One Content layout. Layouts without a measurable
// title are skipped.
func checkContentTitleGeometry(layouts []types.LayoutMetadata) []ConformanceCheck {
	type titled struct {
		layout types.LayoutMetadata
		title  types.PlaceholderInfo
	}
	var family []titled
	refIdx := -1
	for _, layout := range layouts {
		ct := EffectiveCanonicalType(&layout)
		if ct != types.CanonicalLayoutOneContent && ct != types.CanonicalLayoutTwoContent && ct != types.CanonicalLayoutBlankTitle {
			continue
		}
		for _, ph := range layout.Placeholders {
			if ph.Type == types.PlaceholderTitle && ph.Bounds.Width > 0 && ph.Bounds.Height > 0 {
				if refIdx < 0 && ct == types.CanonicalLayoutOneContent {
					refIdx = len(family)
				}
				family = append(family, titled{layout, ph})
				break
			}
		}
	}
	if refIdx < 0 || len(family) < 2 {
		return []ConformanceCheck{{Category: "geometry", Check: titleGeometryCheck, Status: ConformanceStatusPass}}
	}
	ref := family[refIdx]
	var checks []ConformanceCheck
	for i, f := range family {
		if i == refIdx {
			continue
		}
		if diff := titleGeometryDiff(ref.title, f.title); diff != "" {
			checks = append(checks, ConformanceCheck{
				Category: "geometry", Check: titleGeometryCheck, Status: ConformanceStatusWarn,
				Detail: fmt.Sprintf("Layout %q (%s) title differs from %q (%s): %s; titles jump between slides — align the title xfrm and anchor (±0.1in)",
					f.layout.Name, f.layout.ID, ref.layout.Name, ref.layout.ID, diff),
			})
		}
	}
	if len(checks) == 0 {
		return []ConformanceCheck{{Category: "geometry", Check: titleGeometryCheck, Status: ConformanceStatusPass}}
	}
	return checks
}

// titleGeometryDiff describes how b's title differs from a's, or "" when the
// two agree within tolerance.
func titleGeometryDiff(a, b types.PlaceholderInfo) string {
	in := func(emu int64) float64 { return float64(emu) / 914400 }
	abs := func(v int64) int64 {
		if v < 0 {
			return -v
		}
		return v
	}
	var parts []string
	for _, d := range []struct {
		name string
		x, y int64
	}{
		{"x", a.Bounds.X, b.Bounds.X}, {"y", a.Bounds.Y, b.Bounds.Y},
		{"width", a.Bounds.Width, b.Bounds.Width}, {"height", a.Bounds.Height, b.Bounds.Height},
	} {
		if abs(d.x-d.y) > titleGeometryToleranceEMU {
			parts = append(parts, fmt.Sprintf("%s %.2fin vs %.2fin", d.name, in(d.y), in(d.x)))
		}
	}
	anchor := func(s string) string {
		if s == "" {
			return "t"
		}
		return s
	}
	if anchor(a.Anchor) != anchor(b.Anchor) {
		parts = append(parts, fmt.Sprintf("anchor %s vs %s", anchor(b.Anchor), anchor(a.Anchor)))
	}
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}
	return out
}
