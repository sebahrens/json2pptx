package template

import (
	"math"

	"github.com/sebahrens/json2pptx/internal/types"
)

// ResolvedGrid is a template's effective grid system (go-slide-creator-5ms8c):
// the metadata "grid" keys it declares, the engine defaults for the rest, and
// the content frame — title baseline, body line, left edge — read from its
// reference one-content layout. list_templates (full) and examine-template
// report it; grid_violation checks rendered content against the frame.
type ResolvedGrid struct {
	// MarginPct is the left / right content margin as a percentage of the
	// slide width: declared, else the reference layout's title left edge.
	MarginPct float64 `json:"margin_pct"`
	// Columns is the advisory column count (default 12).
	Columns int `json:"columns"`
	// GutterPt is the default shape_grid column / row gap (default 8pt).
	GutterPt float64 `json:"gutter_pt"`
	// TitleGapPt is the measured-title-to-body gap (default 18pt).
	TitleGapPt float64 `json:"title_gap_pt"`
	// Declared lists the keys the template's metadata sets; every other value
	// is the engine default.
	Declared []string `json:"declared,omitempty"`
	// Frame is the content frame, nil when the template has no reference
	// one-content layout with a title.
	Frame *GridFrame `json:"content_frame,omitempty"`
}

// GridFrame is the set of lines content slides align to, in points from the
// slide's top-left corner (EMU copies for the engine).
type GridFrame struct {
	TitleBottomPt float64 `json:"title_bottom_pt"`
	ContentTopPt  float64 `json:"content_top_pt"`
	LeftPt        float64 `json:"left_pt"`
	RightPt       float64 `json:"right_pt"`

	TitleBottomEMU int64 `json:"-"`
	ContentTopEMU  int64 `json:"-"`
	LeftEMU        int64 `json:"-"`
	RightEMU       int64 `json:"-"`
}

// ResolveTemplateGrid resolves the effective grid of a template from its
// parsed layouts (which carry the sanitized metadata grid, see ParseLayouts).
func ResolveTemplateGrid(layouts []types.LayoutMetadata, slideWidth, slideHeight int64) ResolvedGrid {
	grid := types.TemplateGridOf(layouts)
	out := ResolvedGrid{
		Columns:    grid.ColumnsOrDefault(),
		GutterPt:   grid.GutterPtOrDefault(),
		TitleGapPt: grid.TitleGapPtOrDefault(),
	}
	if grid != nil {
		if grid.MarginPct > 0 {
			out.Declared = append(out.Declared, "margin_pct")
			out.MarginPct = grid.MarginPct
		}
		if grid.Columns > 0 {
			out.Declared = append(out.Declared, "columns")
		}
		if grid.GutterPt > 0 {
			out.Declared = append(out.Declared, "gutter_pt")
		}
		if grid.TitleGapPt > 0 {
			out.Declared = append(out.Declared, "title_gap_pt")
		}
	}
	if slideWidth <= 0 {
		slideWidth = 12192000
	}
	ref := ChromeReferenceLayout(layouts)
	if ref == nil {
		return out
	}
	var title, body *types.PlaceholderInfo
	for i := range ref.Placeholders {
		ph := &ref.Placeholders[i]
		switch {
		case ph.Type == types.PlaceholderTitle && title == nil:
			title = ph
		case (ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent) && body == nil && !types.IsDisclosurePlaceholder(*ph):
			body = ph
		}
	}
	if title == nil {
		return out
	}
	f := &GridFrame{
		TitleBottomEMU: title.Bounds.Y + title.Bounds.Height,
		LeftEMU:        title.Bounds.X,
		RightEMU:       title.Bounds.X + title.Bounds.Width,
	}
	// Side artwork that moves the content column in moves the title with it
	// (go-slide-creator-oa0ru), so the frame's left edge moves too.
	if chrome := ResolveChromeFrame(ref, ref, slideWidth, slideHeight, false, false); chrome.SideDecorInset {
		f.LeftEMU = max(f.LeftEMU, chrome.Content.X)
		f.RightEMU = min(f.RightEMU, chrome.Content.X+chrome.Content.CX)
	}
	if body != nil {
		f.ContentTopEMU = body.Bounds.Y
		f.RightEMU = max(f.RightEMU, body.Bounds.X+body.Bounds.Width)
	} else {
		f.ContentTopEMU = f.TitleBottomEMU + int64(out.TitleGapPt*12700)
	}
	if out.MarginPct > 0 {
		margin := int64(out.MarginPct / 100 * float64(slideWidth))
		f.LeftEMU = max(f.LeftEMU, margin)
		f.RightEMU = min(f.RightEMU, slideWidth-margin)
	} else {
		out.MarginPct = math.Round(float64(f.LeftEMU)/float64(slideWidth)*1000) / 10
	}
	f.TitleBottomPt = emuToPt1(f.TitleBottomEMU)
	f.ContentTopPt = emuToPt1(f.ContentTopEMU)
	f.LeftPt = emuToPt1(f.LeftEMU)
	f.RightPt = emuToPt1(f.RightEMU)
	out.Frame = f
	return out
}

func emuToPt1(v int64) float64 { return math.Round(float64(v)/12700*10) / 10 }
