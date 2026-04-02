package generator

import (
	"bytes"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/pptx"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// =============================================================================
// Panel Native Shapes — EMU Specification
// =============================================================================
//
// Source: reference panel template slide2.xml (4 panels) and slide3.xml (3 panels).
// Audit date: 2026-03-24 (pptx-03l).
//
// CANVAS SIZE: 12,855,575 x 7,231,063 EMU (14.059" x 7.908")
//
// ─────────────────────────────────────────────────────────────────────────────
// STRUCTURE OVERVIEW
// ─────────────────────────────────────────────────────────────────────────────
//
// Each panel is one p:grpSp containing exactly 2 child p:sp elements:
//
//   1. HEADER RECT — solidFill rectangle with centered bold title text.
//   2. BODY RECT   — noFill rectangle with thin black border and bulleted text.
//
// There are NO icon shapes (p:pic) inside any panel group.
//   → OQ-2 resolved: icons do not exist in the reference file.
//   → OQ-3 resolved: 2 shapes per panel group (not 5).
//
// Slide 2 (4 panels): 4 independent p:grpSp elements directly in p:spTree.
// Slide 3 (3 panels): 3 p:grpSp wrapped in an outer p:grpSp that rescales
//   them. The inner panels use identical child-space coordinates as slide 2
//   panels 1–3. The outer group just maps them to a different slide region.
//   → For generation, use the flat (slide 2) approach.
//
// ─────────────────────────────────────────────────────────────────────────────
// EMU COORDINATE SYSTEM — SLIDE 2 (4 PANELS, CANONICAL)
// ─────────────────────────────────────────────────────────────────────────────
//
// All 4 panels share the same Y, height, and child-space dimensions.
// Only the X position varies per panel.
//
// Group-level transform (p:grpSpPr > a:xfrm):
//
//   Panel  │ off.x      │ off.y    │ ext.cx   │ ext.cy   │ chOff.x    │ chOff.y  │ chExt.cx │ chExt.cy
//   ───────┼────────────┼──────────┼──────────┼──────────┼────────────┼──────────┼──────────┼──────────
//   1      │    329,610 │2,129,246 │2,860,575 │4,197,531 │    329,610 │2,129,246 │2,822,831 │4,197,531
//   2      │  3,392,626 │2,129,246 │2,860,575 │4,197,531 │  3,454,118 │2,129,246 │2,822,831 │4,197,531
//   3      │  6,455,643 │2,129,246 │2,860,575 │4,197,531 │  6,578,625 │2,129,246 │2,822,831 │4,197,531
//   4      │  9,518,659 │2,129,246 │2,860,575 │4,197,531 │  9,703,133 │2,129,246 │2,822,831 │4,197,531
//
//   Note: ext.cx (2,860,575) > chExt.cx (2,822,831), giving a ~1.34% horizontal
//   scale factor. This is a PowerPoint layout artifact; for generation we can set
//   chOff = off and chExt = ext (identity transform, no scaling).
//
// Gap between panels (off.x[n+1] − off.x[n] − ext.cx):
//   1→2: 202,441   2→3: 202,442   3→4: 202,441  (~0.221", consistent)
//
// ─────────────────────────────────────────────────────────────────────────────
// CHILD SHAPE LAYOUT (relative to child coordinate origin)
// ─────────────────────────────────────────────────────────────────────────────
//
// Within each panel group, the two child shapes are positioned as:
//
//   ┌──────────────────────────────┐  ← chOff.y (= group top)
//   │        HEADER RECT           │  height: 714,103 EMU
//   │  (solidFill, centered text)  │  width:  chExt.cx (full group width)
//   └──────────────────────────────┘
//            gap: 95,794 EMU
//   ┌──────────────────────────────┐  ← chOff.y + 809,897
//   │                              │
//   │         BODY RECT            │  height: 3,387,634 EMU
//   │  (noFill, border, bullets)   │  width:  chExt.cx − 4,292
//   │                              │  left inset: 4,292 EMU from chOff.x
//   │                              │  right edge: aligned with header
//   └──────────────────────────────┘  ← chOff.y + chExt.cy
//
//   Sanity: 714,103 + 95,794 + 3,387,634 = 4,197,531 = chExt.cy  ✓
//
//   The body's left inset of 4,292 EMU (~0.047") accommodates the border
//   stroke width. For generation, this can be approximated as borderWidth/2
//   or ignored entirely (the visual difference is negligible).
//
// ─────────────────────────────────────────────────────────────────────────────
// HEIGHT PROPORTIONS (for dynamic computation from placeholder bounds)
// ─────────────────────────────────────────────────────────────────────────────
//
//   Header : 714,103  / 4,197,531 = 17.01%
//   Gap    :  95,794  / 4,197,531 =  2.28%
//   Body   : 3,387,634 / 4,197,531 = 80.71%
//
// ─────────────────────────────────────────────────────────────────────────────
// STYLING — HEADER RECT
// ─────────────────────────────────────────────────────────────────────────────
//
//   Geometry:  prstGeom prst="rect"
//   Fill:      solidFill srgbClr "EDF5FF" (light blue tint)
//              → For theme-aware generation, use a:schemeClr with lumMod/lumOff
//                instead of hardcoded hex. See table.go:672 for the pattern.
//   Line:      w="6350" (0.5pt), noFill (invisible border line)
//   Autofit:   noAutofit
//
//   Text bodyPr:
//     wrap="square"
//     lIns="0"  tIns="0"  rIns="0"  bIns="0"  (zero internal margins)
//     anchor="ctr"  anchorCtr="0"  (vertically centered)
//
//   Paragraph:
//     algn="ctr"
//     buNone (no bullets)
//
//   Run properties:
//     sz="1600"  (16pt)
//     b="1"      (bold)
//     solidFill srgbClr "003CB4" (accent blue)
//       → For generation, use schemeClr for the text color too.
//     latin typeface="Frutiger Light"
//       → For generation, OMIT explicit typeface to inherit from theme.
//
// ─────────────────────────────────────────────────────────────────────────────
// STYLING — BODY RECT
// ─────────────────────────────────────────────────────────────────────────────
//
//   Geometry:  prstGeom prst="rect"
//   Fill:      noFill
//   Line:      w="6350" (0.5pt), solidFill srgbClr "000000" (black), solid dash
//              → For generation, consider using schemeClr val="tx1" for the
//                border color (maps to dark text color in all themes).
//   Autofit:   noAutofit (reference uses noAutofit; generation may prefer
//              normAutofit for text-heavy panels)
//
//   Text bodyPr:
//     wrap="square"
//     lIns="108000"  (~0.118", ~8.5pt)
//     tIns="108000"
//     rIns="108000"
//     bIns="144000"  (~0.157", ~11.3pt, slightly larger bottom margin)
//     anchor="t"  anchorCtr="0"  (top-aligned)
//
//   Paragraph (bulleted):
//     marL="177800"       (~0.194", bullet indent)
//     indent="-177800"    (hanging indent = bullet character hangs left)
//     defTabSz="284221"
//     spcAft spcPts="600" (6pt space after each paragraph)
//     buClr srgbClr "003CB4" (bullet color = accent blue)
//     buFont typeface="Arial"
//     buChar char="•"
//
//   Run properties:
//     sz="1400"  (14pt)
//     solidFill srgbClr "003CB4" (accent blue)
//     latin typeface="Frutiger Light"
//       → Same as header: OMIT typeface for theme inheritance.
//
// ─────────────────────────────────────────────────────────────────────────────
// SLIDE 3 DIFFERENCES (3 panels with outer scaling group)
// ─────────────────────────────────────────────────────────────────────────────
//
// Slide 3 uses the same 3 inner panel groups (panels 1–3 from slide 2) wrapped
// in an outer p:grpSp that rescales them:
//
//   Outer group:
//     off  x=305,816  y=2,943,498  ext  cx=12,027,853  cy=2,899,954
//     chOff x=329,610  y=2,129,246  chExt cx=8,986,608   cy=4,197,531
//
//   Inner panels use IDENTICAL child coordinates as slide 2 panels 1–3.
//   The outer transform stretches horizontally (×1.338) and compresses
//   vertically (×0.691), plus repositions the group lower on the slide.
//
//   chExt.cx = 8,986,608 = rightEdgePanel3 − leftEdgePanel1
//            = (6,455,643 + 2,860,575) − 329,610 = 8,986,608  ✓
//
//   Takeaway: The inner coordinate system is reusable across panel counts.
//   For generation, compute positions directly from placeholder bounds
//   without nesting groups. Use flat p:grpSp elements per slide 2.
//
// ─────────────────────────────────────────────────────────────────────────────
// CANONICAL p:grpSp TEMPLATE (single panel, simplified for generation)
// ─────────────────────────────────────────────────────────────────────────────
//
//   <p:grpSp>
//     <p:nvGrpSpPr>
//       <p:cNvPr id="{groupID}" name="Panel {n}"/>
//       <p:cNvGrpSpPr/>
//       <p:nvPr/>
//     </p:nvGrpSpPr>
//     <p:grpSpPr>
//       <a:xfrm>
//         <a:off x="{panelX}" y="{panelY}"/>
//         <a:ext cx="{panelCX}" cy="{panelCY}"/>
//         <a:chOff x="{panelX}" y="{panelY}"/>
//         <a:chExt cx="{panelCX}" cy="{panelCY}"/>
//       </a:xfrm>
//     </p:grpSpPr>
//     <!-- HEADER RECT -->
//     <p:sp>
//       <p:nvSpPr>
//         <p:cNvPr id="{headerID}" name="Panel {n} Header"/>
//         <p:cNvSpPr/>
//         <p:nvPr/>
//       </p:nvSpPr>
//       <p:spPr>
//         <a:xfrm>
//           <a:off x="{panelX}" y="{panelY}"/>
//           <a:ext cx="{panelCX}" cy="{headerCY}"/>
//         </a:xfrm>
//         <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
//         <a:solidFill>
//           <a:schemeClr val="{accentColor}">
//             <a:lumMod val="{lumMod}"/>
//             <a:lumOff val="{lumOff}"/>
//           </a:schemeClr>
//         </a:solidFill>
//         <a:ln w="6350"><a:noFill/></a:ln>
//       </p:spPr>
//       <p:txBody>
//         <a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0"
//                   anchor="ctr" anchorCtr="0">
//           <a:noAutofit/>
//         </a:bodyPr>
//         <a:lstStyle/>
//         <a:p>
//           <a:pPr algn="ctr"><a:buNone/></a:pPr>
//           <a:r>
//             <a:rPr lang="en-US" sz="1600" b="1" dirty="0">
//               <a:solidFill><a:schemeClr val="bg1"/></a:solidFill>
//             </a:rPr>
//             <a:t>{title}</a:t>
//           </a:r>
//         </a:p>
//       </p:txBody>
//     </p:sp>
//     <!-- BODY RECT -->
//     <p:sp>
//       <p:nvSpPr>
//         <p:cNvPr id="{bodyID}" name="Panel {n} Body"/>
//         <p:cNvSpPr/>
//         <p:nvPr/>
//       </p:nvSpPr>
//       <p:spPr>
//         <a:xfrm>
//           <a:off x="{panelX}" y="{panelY + headerCY + headerBodyGap}"/>
//           <a:ext cx="{panelCX}" cy="{bodyCY}"/>
//         </a:xfrm>
//         <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
//         <a:noFill/>
//         <a:ln w="6350" cap="flat" cmpd="sng" algn="ctr">
//           <a:solidFill><a:schemeClr val="tx1"/></a:solidFill>
//           <a:prstDash val="solid"/>
//           <a:round/>
//         </a:ln>
//       </p:spPr>
//       <p:txBody>
//         <a:bodyPr wrap="square" lIns="108000" tIns="108000" rIns="108000"
//                   bIns="144000" anchor="t" anchorCtr="0">
//           <a:normAutofit/>
//         </a:bodyPr>
//         <a:lstStyle/>
//         {bulletParagraphs}
//       </p:txBody>
//     </p:sp>
//   </p:grpSp>
//
// ─────────────────────────────────────────────────────────────────────────────
// BULLET PARAGRAPH TEMPLATE (one per bullet point)
// ─────────────────────────────────────────────────────────────────────────────
//
//   <a:p>
//     <a:pPr marL="177800" indent="-177800">
//       <a:spcAft><a:spcPts val="600"/></a:spcAft>
//       <a:buClr><a:schemeClr val="accent1"/></a:buClr>
//       <a:buFont typeface="Arial"/>
//       <a:buChar char="&#x2022;"/>
//     </a:pPr>
//     <a:r>
//       <a:rPr lang="en-US" sz="1400" dirty="0"/>
//       <a:t>{bulletText}</a:t>
//     </a:r>
//   </a:p>
//
// =============================================================================

// Panel shape EMU constants derived from reference panel template audit.
const (
	// panelGap is the horizontal gap between adjacent panels.
	// 202,441 EMU = ~0.221" = ~15.9pt
	panelGap int64 = 202441

	// panelBorderWidth is the line width for the body border and header line.
	// 6,350 EMU = 0.5pt
	panelBorderWidth int64 = 6350

	// panelHeaderFontSize is the header title font size in hundredths of a point.
	// 1600 = 16pt
	panelHeaderFontSize int = 1600

	// panelBodyFontSize is the body text font size in hundredths of a point.
	// 1400 = 14pt
	panelBodyFontSize int = 1400

	// panelBodyMarginLeft is the left text inset for the body text box.
	// 108,000 EMU = ~0.118"
	panelBodyMarginLeft int64 = 108000

	// panelBodyMarginTop is the top text inset for the body text box.
	// 108,000 EMU = ~0.118"
	panelBodyMarginTop int64 = 108000

	// panelBodyMarginRight is the right text inset for the body text box.
	// 108,000 EMU = ~0.118"
	panelBodyMarginRight int64 = 108000

	// panelBodyMarginBottom is the bottom text inset for the body text box.
	// 144,000 EMU = ~0.157" (slightly larger than other margins)
	panelBodyMarginBottom int64 = 144000

	// panelBulletSpaceAfter is the space after each bullet paragraph.
	// 600 = 6pt in hundredths of a point
	panelBulletSpaceAfter int = 600

	// panelHeaderHeightRatio is the header height as a fraction of total height.
	// Used for dynamic computation from placeholder bounds.
	panelHeaderHeightRatio = 0.1701

	// panelGapHeightRatio is the header-body gap as a fraction of total height.
	panelGapHeightRatio = 0.0228
)

// Panel scheme color constants for theme-aware rendering.
// These use OOXML scheme color references so panels inherit template theme colors.
const (
	// panelHeaderFillSchemeColor is the scheme color for header rectangle fill.
	// accent1 with luminance modifiers produces a tinted background.
	panelHeaderFillSchemeColor = "accent1"
	panelHeaderFillLumMod      = 15000
	panelHeaderFillLumOff      = 85000

	// panelHeaderTextSchemeColor is the scheme color for header title text.
	// dk1 (dark 1) maps to the primary dark text color in all themes.
	panelHeaderTextSchemeColor = "dk1"

	// panelBodyBorderSchemeColor is the scheme color for body rectangle border.
	// tx1 maps to the primary text color (typically black/dark).
	panelBodyBorderSchemeColor = "tx1"

	// panelBulletSchemeColor is the scheme color for bullet characters.
	panelBulletSchemeColor = "accent1"
)

// isPanelNativeLayout returns true if the diagram spec is a panel_layout
// with a layout mode supported by native OOXML shape generation (columns,
// rows, or stat_cards).
func isPanelNativeLayout(spec *types.DiagramSpec) bool {
	if spec.Type != "panel_layout" {
		return false
	}
	// Check explicit layout field in data; default is "columns"
	// (mirrors inferLayout logic in svggen/panel_layout.go).
	if layout, ok := spec.Data["layout"].(string); ok && layout != "" {
		switch layout {
		case "columns", "rows", "stat_cards":
			return true
		default:
			return false
		}
	}
	return true // default layout is "columns"
}

// panelLayoutMode extracts the layout mode string from a panel_layout DiagramSpec.
// Returns "columns" as the default if no explicit layout is set.
func panelLayoutMode(spec *types.DiagramSpec) string {
	if layout, ok := spec.Data["layout"].(string); ok && layout != "" {
		return layout
	}
	return "columns"
}

// panelBulletsParagraphs converts body text into pptx.Paragraph slices.
// Lines starting with "- " become bulleted paragraphs with accent1 bullet color;
// other lines become plain paragraphs.
func panelBulletsParagraphs(text string, fontSizeHundredths int) []pptx.Paragraph {
	if text == "" {
		return nil
	}
	return pptx.ParseBulletText(text, pptx.BulletTextOptions{
		FontSize:    fontSizeHundredths,
		Lang:        "en-US",
		Dirty:       true,
		BulletColor: pptx.SchemeFill(panelBulletSchemeColor),
		SpaceAfter:  panelBulletSpaceAfter,
	})
}

// panelBulletsToOOXML converts body text into OOXML paragraph elements.
// Lines starting with "- " become bulleted paragraphs with accent1 bullet color;
// other lines become plain paragraphs. Empty input returns an empty string.
func panelBulletsToOOXML(text string, fontSizeHundredths int) string {
	paras := panelBulletsParagraphs(text, fontSizeHundredths)
	if len(paras) == 0 {
		return ""
	}
	var buf bytes.Buffer
	for _, p := range paras {
		p.WriteXML(&buf)
	}
	return buf.String()
}

// generatePanelHeaderXML produces a p:sp element for a panel header rectangle.
// The header has a scheme-colored fill with luminance modifiers and centered bold text.
func generatePanelHeaderXML(title string, x, y, cx, cy int64, shapeID uint32, schemeColor string, lumMod, lumOff int) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Panel Header",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRect,
		Fill:     pptx.SchemeFill(schemeColor, pptx.LumMod(lumMod), pptx.LumOff(lumOff)),
		Line:     pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  [4]int64{0, 0, 0, 0},
			AutoFit: "noAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     title,
					Lang:     "en-US",
					FontSize: panelHeaderFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    pptx.SchemeFill(panelHeaderTextSchemeColor),
				}},
			}},
		},
	})
	if err != nil {
		slog.Warn("generatePanelHeaderXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generatePanelBodyXML produces a p:sp element for a panel body rectangle.
// The body has no fill, a scheme-colored border, and bulleted text content.
func generatePanelBodyXML(body string, x, y, cx, cy int64, shapeID uint32, borderSchemeColor string, fontSizeHundredths int) string {
	paras := panelBulletsParagraphs(body, fontSizeHundredths)
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Panel Body",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		Line: pptx.Line{
			Width:    panelBorderWidth,
			Fill:     pptx.SchemeFill(borderSchemeColor),
			Cap:      "flat",
			Compound: "sng",
			Align:    "ctr",
			Dash:     "solid",
			Join:     "round",
		},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "t",
			Insets:     [4]int64{panelBodyMarginLeft, panelBodyMarginTop, panelBodyMarginRight, panelBodyMarginBottom},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("generatePanelBodyXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generatePanelGroupXML produces the complete <p:grpSp> XML for a set of panels.
// Each panel gets a header rectangle and a body rectangle arranged as equal-width
// columns within the given bounding box. Uses identity child transform (chOff=off, chExt=ext).
func generatePanelGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
	n := len(panels)
	if n == 0 {
		return ""
	}

	totalWidth := bounds.Width
	totalHeight := bounds.Height

	// Layout: equal-width panels with gaps between them.
	// panelWidth = (totalWidth - (N-1)*gap) / N
	gapTotal := int64(n-1) * panelGap
	panelWidth := (totalWidth - gapTotal) / int64(n)

	// Height proportions from audit reference.
	headerCY := int64(float64(totalHeight) * panelHeaderHeightRatio)
	gapCY := int64(float64(totalHeight) * panelGapHeightRatio)
	bodyCY := totalHeight - headerCY - gapCY

	// Generate child shapes for each panel
	var children [][]byte
	for i, panel := range panels {
		panelX := bounds.X + int64(i)*(panelWidth+panelGap)
		panelY := bounds.Y

		headerID := shapeIDBase + uint32(i*2) + 1
		bodyID := shapeIDBase + uint32(i*2) + 2

		headerXML := generatePanelHeaderXML(
			panel.title,
			panelX, panelY, panelWidth, headerCY,
			headerID,
			panelHeaderFillSchemeColor, panelHeaderFillLumMod, panelHeaderFillLumOff,
		)
		children = append(children, []byte(headerXML))

		bodyY := panelY + headerCY + gapCY
		bodyXML := generatePanelBodyXML(
			panel.body,
			panelX, bodyY, panelWidth, bodyCY,
			bodyID,
			panelBodyBorderSchemeColor,
			panelBodyFontSize,
		)
		children = append(children, []byte(bodyXML))
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Panels",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generatePanelGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// =============================================================================
// Rows Layout — Horizontal rows with header left + body right
// =============================================================================

// Row layout constants.
const (
	// rowGap is the vertical gap between adjacent rows.
	rowGap int64 = 91440 // ~0.1" = ~7.2pt

	// rowHeaderWidthRatio is the header width as a fraction of total row width.
	rowHeaderWidthRatio = 0.25

	// rowHeaderBodyGap is the horizontal gap between header and body in a row.
	rowHeaderBodyGap int64 = 91440 // ~0.1"

	// rowBodyInset is the text inset for the body area.
	rowBodyInset int64 = 72000 // ~0.079"
)

// generatePanelRowsGroupXML produces the complete <p:grpSp> XML for panels arranged
// as horizontal rows. Each row has a scheme-colored header rect on the left and a
// bordered body rect on the right.
func generatePanelRowsGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
	n := len(panels)
	if n == 0 {
		return ""
	}

	totalWidth := bounds.Width
	totalHeight := bounds.Height

	// Layout: equal-height rows with gaps between them.
	gapTotal := int64(n-1) * rowGap
	rowHeight := (totalHeight - gapTotal) / int64(n)

	// Header/body width split.
	headerCX := int64(float64(totalWidth) * rowHeaderWidthRatio)
	bodyCX := totalWidth - headerCX - rowHeaderBodyGap

	var children [][]byte
	for i, panel := range panels {
		rowY := bounds.Y + int64(i)*(rowHeight+rowGap)

		headerID := shapeIDBase + uint32(i*3) + 1
		bodyID := shapeIDBase + uint32(i*3) + 2

		// Header rect — scheme-colored fill, vertically centered bold text
		headerXML := generatePanelHeaderXML(
			panel.title,
			bounds.X, rowY, headerCX, rowHeight,
			headerID,
			panelHeaderFillSchemeColor, panelHeaderFillLumMod, panelHeaderFillLumOff,
		)
		children = append(children, []byte(headerXML))

		// Body rect — bordered with bulleted text
		bodyXML := generatePanelBodyXML(
			panel.body,
			bounds.X+headerCX+rowHeaderBodyGap, rowY, bodyCX, rowHeight,
			bodyID,
			panelBodyBorderSchemeColor,
			panelBodyFontSize,
		)
		children = append(children, []byte(bodyXML))
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Panel Rows",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generatePanelRowsGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// =============================================================================
// Stat Cards Layout — Grid of stat cards with hero value + title + body
// =============================================================================

// Stat card layout constants.
const (
	// statCardGap is the gap between stat cards.
	statCardGap int64 = 137160 // ~0.15" = ~10.8pt

	// statCardValueFontSize is the hero value font size (hundredths of a point).
	statCardValueFontSize int = 3200 // 32pt

	// statCardLabelFontSize is the title label font size (hundredths of a point).
	statCardLabelFontSize int = 1200 // 12pt

	// statCardBodyFontSize is the body text font size (hundredths of a point).
	statCardBodyFontSize int = 1100 // 11pt

	// statCardInset is the text inset for stat card shapes.
	statCardInset int64 = 108000 // ~0.118"

	// statCardValueHeightRatio is the hero value zone as a fraction of card height.
	statCardValueHeightRatio = 0.45

	// statCardLabelHeightRatio is the title label zone as a fraction of card height.
	statCardLabelHeightRatio = 0.20

	// statCardMinCols is the minimum number of columns in the stat card grid.
	statCardMinCols = 2

	// statCardMaxCols is the maximum number of columns.
	statCardMaxCols = 4
)

// statCardGridLayout calculates columns and rows for n stat cards.
func statCardGridLayout(n int) (cols, rows int) {
	if n <= statCardMaxCols {
		return n, 1
	}
	// Pick number of columns that keeps cards roughly square.
	cols = statCardMaxCols
	if n <= 6 {
		cols = 3
	}
	if n <= 4 {
		cols = 2
	}
	rows = (n + cols - 1) / cols
	return cols, rows
}

// generateStatCardsGroupXML produces the complete <p:grpSp> XML for a grid of stat cards.
// Each card has a hero value (large accent-colored text), a title label, and optional body text.
// Cards with a body field that starts with "+" or "-" get an upArrow or downArrow delta indicator.
func generateStatCardsGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
	n := len(panels)
	if n == 0 {
		return ""
	}

	cols, rows := statCardGridLayout(n)

	totalWidth := bounds.Width
	totalHeight := bounds.Height

	hGapTotal := int64(cols-1) * statCardGap
	vGapTotal := int64(rows-1) * statCardGap
	cardW := (totalWidth - hGapTotal) / int64(cols)
	cardH := (totalHeight - vGapTotal) / int64(rows)

	var children [][]byte
	panelIdx := 0
	for row := range rows {
		for col := range cols {
			if panelIdx >= n {
				break
			}
			panel := panels[panelIdx]

			cardX := bounds.X + int64(col)*(cardW+statCardGap)
			cardY := bounds.Y + int64(row)*(cardH+statCardGap)

			// Each stat card uses up to 3 shape IDs: background, value text, label/body text
			baseID := shapeIDBase + uint32(panelIdx*3) + 1

			cardXML := generateStatCardXML(panel, cardX, cardY, cardW, cardH, baseID)
			children = append(children, []byte(cardXML))

			panelIdx++
		}
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Stat Cards",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generateStatCardsGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateStatCardXML produces the shapes for a single stat card.
// Returns a background rect with all text zones rendered as paragraphs in a single text body.
func generateStatCardXML(panel nativePanelData, x, y, cx, cy int64, shapeID uint32) string {
	// Determine the display value: prefer explicit value, fall back to title.
	displayValue := panel.value
	if displayValue == "" {
		displayValue = panel.title
	}

	// Determine if body indicates a delta (starts with + or -)
	var deltaColor string
	if len(panel.body) > 0 {
		switch panel.body[0] {
		case '+':
			deltaColor = "accent6" // green-ish in most themes
		case '-':
			deltaColor = "accent2" // red-ish in most themes
		}
	}

	// Build paragraphs: value, then title label (if value is separate), then body
	var paras []pptx.Paragraph

	// Hero value paragraph — large, bold, accent-colored, centered
	if displayValue != "" {
		paras = append(paras, pptx.Paragraph{
			Align:    "ctr",
			NoBullet: true,
			Runs: []pptx.Run{{
				Text:     displayValue,
				Lang:     "en-US",
				FontSize: statCardValueFontSize,
				Bold:     true,
				Dirty:    true,
				Color:    pptx.SchemeFill("accent1"),
			}},
		})
	}

	// Title label paragraph — shown when value is separate from title
	if panel.value != "" && panel.title != "" {
		paras = append(paras, pptx.Paragraph{
			Align:      "ctr",
			NoBullet:   true,
			SpaceAfter: 200,
			Runs: []pptx.Run{{
				Text:     panel.title,
				Lang:     "en-US",
				FontSize: statCardLabelFontSize,
				Bold:     true,
				Dirty:    true,
				Color:    pptx.SchemeFill(panelHeaderTextSchemeColor),
			}},
		})
	}

	// Body/delta paragraph
	if panel.body != "" {
		bodyRun := pptx.Run{
			Text:     panel.body,
			Lang:     "en-US",
			FontSize: statCardBodyFontSize,
			Dirty:    true,
		}
		if deltaColor != "" {
			bodyRun.Color = pptx.SchemeFill(deltaColor)
		}
		paras = append(paras, pptx.Paragraph{
			Align:    "ctr",
			NoBullet: true,
			Runs:     []pptx.Run{bodyRun},
		})
	}

	// Single rect with light fill and all text in one text body
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Stat Card",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: 5000}, // subtle rounding
		},
		Fill: pptx.SchemeFill(panelHeaderFillSchemeColor, pptx.LumMod(panelHeaderFillLumMod), pptx.LumOff(panelHeaderFillLumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "ctr",
			Insets:     [4]int64{statCardInset, statCardInset, statCardInset, statCardInset},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("generateStatCardXML failed", "error", err)
		return ""
	}
	return string(b)
}

// allocatePanelIconRelIDs allocates relationship IDs for panel icon images and
// generates the final group XML for each panelShapeInsert.
//
// This solves the chicken-and-egg problem: p:pic elements inside the group need
// r:embed relationship IDs, but IDs can only be allocated during writeOutput()
// after all other rel IDs are known. So processPanelNativeShapes() stores the
// raw icon bytes, and this function runs later to allocate IDs and build XML.
//
// For panels without icons (iconBytes == nil), no p:pic is emitted and no
// relationship is allocated — this is the normal case per the reference file.
func (ctx *singlePassContext) allocatePanelIconRelIDs() {
	// We need a global shape ID counter that avoids conflicts across slides.
	// Start at a high base to avoid conflicts with typical OOXML IDs.
	nextShapeID := uint32(10000)

	// Sort slide numbers for deterministic shape ID and rel ID allocation.
	// Map iteration order is non-deterministic in Go; without sorting,
	// nextShapeID and mediaCounter would vary between runs.
	slideNums := make([]int, 0, len(ctx.panelShapeInserts))
	for slideNum := range ctx.panelShapeInserts {
		slideNums = append(slideNums, slideNum)
	}
	sort.Ints(slideNums)

	for _, slideNum := range slideNums {
		inserts := ctx.panelShapeInserts[slideNum]
		// Compute next available rel ID for this slide.
		// rId1 = layout, then media, then native SVGs, then panel icons.
		nextRelID := 2

		// Account for regular media relationships
		if mediaRels, hasMedia := ctx.slideRelUpdates[slideNum]; hasMedia {
			nextRelID += len(mediaRels)
		}

		// Account for native SVG relationships (2 per insert: PNG + SVG)
		if nativeSVGs, hasSVG := ctx.nativeSVGInserts[slideNum]; hasSVG {
			nextRelID += len(nativeSVGs) * 2
		}

		for i := range inserts {
			for j := range inserts[i].panels {
				panel := &inserts[i].panels[j]
				if len(panel.iconBytes) == 0 {
					continue
				}

				// Allocate media filename
				panel.iconMediaFile = fmt.Sprintf("image%d.png", ctx.mediaCounter)
				ctx.mediaCounter++
				ctx.usedExtensions["png"] = true

				// Allocate relationship ID
				panel.iconRelID = fmt.Sprintf("rId%d", nextRelID)
				nextRelID++
			}

			// Generate the final group XML with real rel IDs (if any icons)
			// and the correct shape ID base.
			switch {
			case inserts[i].swotMode:
				inserts[i].groupXML = generateSWOTGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID,
				)
			case inserts[i].pestelMode:
				inserts[i].groupXML = generatePESTELGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID,
				)
			case inserts[i].valueChainMode:
				inserts[i].groupXML = generateValueChainGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID, inserts[i].valueChainMeta,
				)
			case inserts[i].nineBoxMode:
				inserts[i].groupXML = generateNineBoxGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID,
				)
			case inserts[i].kpiDashboardMode:
				inserts[i].groupXML = generateKPIDashboardGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID,
				)
			case inserts[i].portersFiveMode:
				inserts[i].groupXML = generatePortersFiveGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID,
				)
			case inserts[i].bmcMode:
				inserts[i].groupXML = generateBMCGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID,
				)
			case inserts[i].processFlowMode:
				inserts[i].groupXML = generateProcessFlowGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID, inserts[i].processFlowMeta,
				)
			case inserts[i].heatmapMode:
				inserts[i].groupXML = generateHeatmapGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID, inserts[i].heatmapMeta,
				)
			case inserts[i].pyramidMode:
				inserts[i].groupXML = generatePyramidGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID,
				)
			case inserts[i].houseDiagramMode:
				inserts[i].groupXML = generateHouseDiagramGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID, inserts[i].houseDiagramMeta,
				)
			case inserts[i].rowsMode:
				inserts[i].groupXML = generatePanelRowsGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID,
				)
			case inserts[i].statCardsMode:
				inserts[i].groupXML = generateStatCardsGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID,
				)
			default:
				inserts[i].groupXML = generatePanelGroupXML(
					inserts[i].panels, inserts[i].bounds, nextShapeID,
				)
			}
			// Advance shape ID: 1 (group) + N*shapes_per_panel.
			// Nine box mode uses more shapes (9 cells × 2 + up to 8 axis labels).
			// Stat cards mode: 1 shape per card (single rect with text body).
			// Rows mode: 2 shapes per panel (header + body), same as columns.
			switch {
			case inserts[i].valueChainMode:
				// 1 (group) + N support bars + N primary chevrons + 1 margin (optional)
				shapeCount := uint32(inserts[i].valueChainMeta.supportCount + inserts[i].valueChainMeta.primaryCount + 1)
				if inserts[i].valueChainMeta.marginLabel != "" {
					shapeCount++
				}
				nextShapeID += shapeCount
			case inserts[i].nineBoxMode:
				// 1 (group) + 9×2 (label+body) + 8 (axis shapes max)
				nextShapeID += 27
			case inserts[i].portersFiveMode:
				// 1 (group) + 5 force boxes + 4 connectors
				nextShapeID += 10
			case inserts[i].bmcMode:
				// 1 (group) + 9×2 (header+body per cell) = 19
				nextShapeID += 19
			case inserts[i].processFlowMode:
				// 1 (group) + N steps + M connectors + L labels
				nextShapeID += pfEstimateShapeCount(inserts[i].panels)
			case inserts[i].heatmapMode:
				// 1 (group) + R*C cells + R row labels + C col labels
				m := inserts[i].heatmapMeta
				nextShapeID += uint32(m.numRows*m.numCols + m.numRows + m.numCols + 1)
			case inserts[i].pyramidMode:
				// 1 (group) + N level shapes
				nextShapeID += pyramidEstimateShapeCount(inserts[i].panels)
			case inserts[i].houseDiagramMode:
				// 1 (group) + 1 (roof) + N (floor sections) + 1 (foundation)
				nextShapeID += houseDiagramEstimateShapeCount(inserts[i].panels)
			case inserts[i].statCardsMode, inserts[i].kpiDashboardMode:
				// 1 (group) + N×1 (single rect per card)
				nextShapeID += uint32(len(inserts[i].panels) + 1)
			default:
				nextShapeID += uint32(len(inserts[i].panels)*2 + 1)
			}
		}

		// Write back the modified slice
		ctx.panelShapeInserts[slideNum] = inserts
	}
}

// processPanelNativeShapes parses panel data from a DiagramSpec and registers
// a panelShapeInsert for native OOXML shape generation. The actual XML generation
// is deferred to generatePanelGroupXML, which may be called later during
// icon rel-ID allocation (pptx-z27).
//
// KNOWN LIMITATION: themeOverride/brand_color from frontmatter does NOT modify
// ppt/theme/theme1.xml, so scheme color refs won't pick up overrides.
func (ctx *singlePassContext) processPanelNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("panel native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	// Warn if themeOverride is set — scheme colors won't reflect overrides.
	if ctx.themeOverride != nil {
		slog.Warn("panel native shapes: themeOverride is set but scheme color refs in panel shapes will not reflect overrides",
			"slide", slideNum)
	}

	// Parse panels from DiagramSpec.Data
	panelsRaw, ok := diagramSpec.Data["panels"].([]any)
	if !ok {
		slog.Warn("panel native shapes: missing or invalid 'panels' data", "slide", slideNum)
		return
	}

	// Determine layout mode for routing.
	layoutMode := panelLayoutMode(diagramSpec)

	var panels []nativePanelData
	for _, item := range panelsRaw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		panel := nativePanelData{}
		if title, ok := m["title"].(string); ok {
			panel.title = title
		}
		if value, ok := m["value"].(string); ok {
			panel.value = value
		}
		if body, ok := m["body"].(string); ok {
			panel.body = body
		}
		// Icon bytes are not loaded here — that's handled by the caller or
		// deferred to allocatePanelIconRelIDs (pptx-z27).
		panels = append(panels, panel)
	}

	if len(panels) == 0 {
		slog.Warn("panel native shapes: no panels parsed", "slide", slideNum)
		return
	}

	// Get placeholder bounds from the shape being replaced.
	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native panel shapes: registered",
		"slide", slideNum,
		"panels", len(panels),
		"layout", layoutMode,
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		placeholderIdx: shapeIdx,
		bounds:         placeholderBounds,
		panels:         panels,
		rowsMode:       layoutMode == "rows",
		statCardsMode:  layoutMode == "stat_cards",
	})
}

// removePanelPlaceholders removes placeholder shapes that are being replaced by
// native panel group shapes. Returns a modified copy of the slide.
func (ctx *singlePassContext) removePanelPlaceholders(slide *slideXML, inserts []panelShapeInsert) *slideXML {
	removeIdxs := make(map[int]bool)
	for _, ins := range inserts {
		removeIdxs[ins.placeholderIdx] = true
	}
	filteredShapes := make([]shapeXML, 0, len(slide.CommonSlideData.ShapeTree.Shapes))
	for i, shape := range slide.CommonSlideData.ShapeTree.Shapes {
		if !removeIdxs[i] {
			filteredShapes = append(filteredShapes, shape)
		}
	}
	slide.CommonSlideData.ShapeTree.Shapes = filteredShapes
	return slide
}

// insertPanelGroups inserts pre-generated <p:grpSp> elements for panel inserts
// before </p:spTree> in the marshaled slide XML.
func insertPanelGroups(slideData []byte, inserts []panelShapeInsert) []byte {
	insertPos := findLastClosingSpTree(slideData)
	if insertPos == -1 {
		return slideData // Cannot find insertion point
	}

	var groups []string
	for _, ins := range inserts {
		if ins.groupXML != "" {
			groups = append(groups, ins.groupXML)
		}
	}
	if len(groups) == 0 {
		return slideData
	}

	insertion := strings.Join(groups, "\n")
	return spliceBytes(slideData, insertPos, insertion)
}
