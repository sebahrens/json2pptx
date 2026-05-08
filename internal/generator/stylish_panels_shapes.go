package generator

import (
	"log/slog"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// Stylish Panels Layout
// =============================================================================
//
// Based on reference: stylish-panels.pptx slide 1.
//
// Structure per panel (N equal-width columns):
//   1. ACCENT BAND — solidFill accent2, vertically centered (decorative header bar)
//   2. BODY RECT   — solidFill accent2 lumMod=20000/lumOff=80000 (light tint),
//                     bulleted text with large top inset
//
// Shared across all panels:
//   3. RIBBON — full-width rectangle at accent/body junction, fill bg1 lumMod=95000
//   4. HEADER TEXT BOXES — one per panel, centered on the ribbon, containing title
//
// Height proportions (from reference, total content height = 4,512,405 EMU):
//   Accent band:  21.7%
//   Gap:           2.4%
//   Body rect:    75.9%
//   Ribbon offset: 24.1% (from top, overlaps body top)
//   Ribbon height: 20.5%
//   Header text y: 26.6% (from top)
//   Header height: 15.7%
//
// Panel gap: 283,175 EMU (~0.31")
// =============================================================================

// Stylish panels layout constants derived from reference file audit.
const (
	// stylishPanelGap is the horizontal gap between adjacent panels.
	stylishPanelGap int64 = 283175

	// Height ratios (fraction of total bounding box height).
	stylishAccentHeightRatio  = 0.217
	stylishGapHeightRatio     = 0.024
	stylishRibbonOffsetRatio  = 0.241 // ribbon starts at accent + gap
	stylishRibbonHeightRatio  = 0.205
	stylishHeaderOffsetRatio  = 0.266
	stylishHeaderHeightRatio  = 0.157

	// Styling constants.
	stylishAccentSchemeColor = "accent2"

	stylishBodyFillSchemeColor = "accent2"
	stylishBodyFillLumMod      = 20000
	stylishBodyFillLumOff      = 80000

	stylishRibbonSchemeColor = "bg1"
	stylishRibbonLumMod      = 95000
	stylishRibbonLumOff      = 5000

	// Text sizing.
	stylishHeaderFontSize = 1600 // 16pt
	stylishBodyFontSize   = 1400 // 14pt

	// Body text insets (from reference).
	stylishBodyLIns int64 = 91440
	stylishBodyTIns int64 = 1044000 // large top inset to clear ribbon area
	stylishBodyRIns int64 = 91440
	stylishBodyBIns int64 = 45720
)

// generateStylishPanelsGroupXML produces the complete <p:grpSp> XML for the
// stylish panels layout. Each panel has an accent band and a body rect, plus
// a shared full-width ribbon with per-panel header text boxes on top.
func generateStylishPanelsGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
	n := len(panels)
	if n == 0 {
		return ""
	}

	totalWidth := bounds.Width
	totalHeight := bounds.Height

	// Panel widths: equal columns with gaps.
	gapTotal := int64(n-1) * stylishPanelGap
	panelWidth := (totalWidth - gapTotal) / int64(n)

	// Vertical zones.
	accentCY := int64(float64(totalHeight) * stylishAccentHeightRatio)
	gapCY := int64(float64(totalHeight) * stylishGapHeightRatio)
	bodyCY := totalHeight - accentCY - gapCY
	ribbonOffY := int64(float64(totalHeight) * stylishRibbonOffsetRatio)
	ribbonCY := int64(float64(totalHeight) * stylishRibbonHeightRatio)
	headerOffY := int64(float64(totalHeight) * stylishHeaderOffsetRatio)
	headerCY := int64(float64(totalHeight) * stylishHeaderHeightRatio)

	var children [][]byte
	nextID := shapeIDBase + 1

	// --- Per-panel accent bands ---
	for i := range panels {
		panelX := bounds.X + int64(i)*(panelWidth+stylishPanelGap)
		b, err := pptx.GenerateShape(pptx.ShapeOptions{
			ID:       nextID,
			Name:     "Accent Band",
			Bounds:   pptx.RectEmu{X: panelX, Y: bounds.Y, CX: panelWidth, CY: accentCY},
			Geometry: pptx.GeomRect,
			Fill:     pptx.SchemeFill(stylishAccentSchemeColor),
			Line:     pptx.Line{Width: 0, Fill: pptx.NoFill()},
		})
		if err != nil {
			slog.Warn("stylish panels: accent band failed", "error", err)
			continue
		}
		children = append(children, b)
		nextID++
	}

	// --- Per-panel body rects ---
	bodyY := bounds.Y + accentCY + gapCY
	for i, panel := range panels {
		panelX := bounds.X + int64(i)*(panelWidth+stylishPanelGap)
		paras := panelBulletsParagraphs(panel.body, stylishBodyFontSize)
		b, err := pptx.GenerateShape(pptx.ShapeOptions{
			ID:       nextID,
			Name:     "Panel Body",
			Bounds:   pptx.RectEmu{X: panelX, Y: bodyY, CX: panelWidth, CY: bodyCY},
			Geometry: pptx.GeomRect,
			Fill:     pptx.SchemeFill(stylishBodyFillSchemeColor, pptx.LumMod(stylishBodyFillLumMod), pptx.LumOff(stylishBodyFillLumOff)),
			Line: pptx.Line{
				Width: 0,
				Fill:  pptx.SchemeFill(stylishBodyFillSchemeColor, pptx.LumMod(stylishBodyFillLumMod), pptx.LumOff(stylishBodyFillLumOff)),
			},
			Text: &pptx.TextBody{
				Wrap:       "square",
				Anchor:     "t",
				Insets:     [4]int64{stylishBodyLIns, stylishBodyTIns, stylishBodyRIns, stylishBodyBIns},
				AutoFit:    "normAutofit",
				Paragraphs: paras,
			},
		})
		if err != nil {
			slog.Warn("stylish panels: body rect failed", "error", err)
			continue
		}
		children = append(children, b)
		nextID++
	}

	// --- Full-width ribbon ---
	ribbonY := bounds.Y + ribbonOffY
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       nextID,
		Name:     "Ribbon",
		Bounds:   pptx.RectEmu{X: bounds.X, Y: ribbonY, CX: totalWidth, CY: ribbonCY},
		Geometry: pptx.GeomRect,
		Fill:     pptx.SchemeFill(stylishRibbonSchemeColor, pptx.LumMod(stylishRibbonLumMod), pptx.LumOff(stylishRibbonLumOff)),
		Line:     pptx.Line{Width: 0, Fill: pptx.NoFill()},
	})
	if err != nil {
		slog.Warn("stylish panels: ribbon failed", "error", err)
	} else {
		children = append(children, b)
	}
	nextID++

	// --- Per-panel header text boxes (on the ribbon) ---
	headerY := bounds.Y + headerOffY
	for i, panel := range panels {
		panelX := bounds.X + int64(i)*(panelWidth+stylishPanelGap)
		b, err := pptx.GenerateShape(pptx.ShapeOptions{
			ID:       nextID,
			Name:     "Panel Header",
			Bounds:   pptx.RectEmu{X: panelX, Y: headerY, CX: panelWidth, CY: headerCY},
			Geometry: pptx.GeomRect,
			Fill:     pptx.NoFill(),
			Line:     pptx.Line{Width: 0, Fill: pptx.NoFill()},
			Text: &pptx.TextBody{
				Wrap:    "square",
				Anchor:  "ctr",
				Insets:  [4]int64{91440, 45720, 91440, 45720},
				AutoFit: "normAutofit",
				Paragraphs: []pptx.Paragraph{{
					Align:    "ctr",
					NoBullet: true,
					Runs: []pptx.Run{{
						Text:     panel.title,
						Lang:     "en-US",
						FontSize: stylishHeaderFontSize,
						Bold:     true,
						Dirty:    true,
						Color:    pptx.SchemeFill(panelHeaderTextSchemeColor),
					}},
				}},
			},
		})
		if err != nil {
			slog.Warn("stylish panels: header text failed", "error", err)
			continue
		}
		children = append(children, b)
		nextID++
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	grp, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Stylish Panels",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("stylish panels: group failed", "error", err)
		return ""
	}
	return string(grp)
}

// stylishPanelsEstimateShapeCount returns the number of shape IDs consumed by
// a stylish panels group: N accent bands + N body rects + 1 ribbon + N headers + 1 group.
func stylishPanelsEstimateShapeCount(panels []nativePanelData) uint32 {
	n := uint32(len(panels))
	return 3*n + 2 // N accents + N bodies + 1 ribbon + N headers + 1 group
}
