package generator

import (
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// Value Chain Native Shapes — Chevron Primary + Stacked Support + Margin
// =============================================================================
//
// Replaces SVG-rendered Porter's Value Chain diagrams with native OOXML grouped
// shapes. Primary activities use homePlate preset geometry (arrow-shaped) for
// all but the last, which uses a rect. Support activities are horizontal rect
// bars stacked above. Margin is a vertical rect on the right.
//
// All shapes use scheme color references for theme-awareness.
//
// Layout:
//
//   ┌──────────────────────────────────────────────┐ ┌─────┐
//   │  Firm Infrastructure  (accent1, rect bar)    │ │     │
//   ├──────────────────────────────────────────────┤ │     │
//   │  HR Management        (accent2, rect bar)    │ │  M  │
//   ├──────────────────────────────────────────────┤ │  a  │
//   │  Technology Dev       (accent3, rect bar)    │ │  r  │
//   ├──────────────────────────────────────────────┤ │  g  │
//   │  Procurement          (accent4, rect bar)    │ │  i  │
//   └──────────────────────────────────────────────┘ │  n  │
//        gap                                         │     │
//   ┌────────┬────────┬────────┬────────┬────────┐   │     │
//   │Inbound │Operatn │Outbound│Marketng│Service │   │     │
//   │(homeP) │(homeP) │(homeP) │(homeP) │ (rect) │   │     │
//   └────────┴────────┴────────┴────────┴────────┘   └─────┘

// Value chain EMU constants.
const (
	// vcGap is the gap between support bars and between primary shapes.
	vcGap int64 = 45720 // ~0.05"

	// vcSectionGap is the gap between support and primary sections.
	vcSectionGap int64 = 73152 // ~0.08"

	// vcMarginWidthRatio is the margin section width as a fraction of total width.
	vcMarginWidthRatio = 0.08

	// vcMarginGap is the gap between content area and margin section.
	vcMarginGap int64 = 45720 // ~0.05"

	// vcSupportHeightRatio is the support section height as a fraction of total height.
	vcSupportHeightRatio = 0.40

	// vcPrimaryHeightRatio is the primary section height as a fraction of total height.
	vcPrimaryHeightRatio = 0.52

	// vcLabelFontSize is the font size for activity labels (hundredths of a point).
	// 1200 = 12pt
	vcLabelFontSize int = 1200

	// vcBodyFontSize is the font size for activity item text (hundredths of a point).
	// 1000 = 10pt
	vcBodyFontSize int = 1000

	// vcMarginFontSize is the font size for the margin label (hundredths of a point).
	// 1400 = 14pt
	vcMarginFontSize int = 1400

	// vcTextInset is the text inset for shapes (EMU): the uniform 0.5 cm
	// shape text margin.
	vcTextInset = pptx.ShapeTextInsetEMU

	// vcHomePlateAdj is the homePlate "adj" value controlling the arrow tip depth.
	// 50000 = default OOXML; we use a smaller value for a subtler arrow.
	vcHomePlateAdj int64 = 35000
)

// Value chain surfaces (go-slide-creator-amtkg). Support bars and primary
// chevrons used to rotate through accent1–6 at 40% and 60% — up to six hues on
// one diagram, none of them carrying information. They now take the shared
// neutral ladder, as the value-chain pattern does: support activities on the
// card surface, primary activities on the structural step, the margin between
// the two. The diagram's one accent is the ink of the support and margin
// titles. style.colors recolours the bars and the chevrons in order
// (["accent1",…,"accent6"] restores a per-activity rotation).
const (
	vcSupportTint = patterns.NeutralTint4
	vcMarginTint  = patterns.NeutralTint8
	vcPrimaryTint = patterns.NeutralTint16
)

// isValueChainDiagram returns true if the diagram spec is a value_chain diagram type.
func isValueChainDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "value_chain"
}

// valueChainMeta holds metadata about value chain structure beyond the panel list.
type valueChainMeta struct {
	primaryCount int    // Number of primary activities
	supportCount int    // Number of support activities
	marginLabel  string // Label for the margin section (empty = no margin)
}

// parseValueChainData extracts primary activities, support activities, and margin
// from the value chain diagram data map. Returns panels in order:
// [support0, support1, ..., primary0, primary1, ...] with metadata.
func parseValueChainData(data map[string]any) ([]nativePanelData, valueChainMeta) {
	var panels []nativePanelData
	var meta valueChainMeta

	// Parse support activities
	supportRaw := extractActivityList(data, "support", "support_activities")
	panels = append(panels, supportRaw...)
	meta.supportCount = len(supportRaw)

	// Parse primary activities
	primaryRaw := extractActivityList(data, "primary", "primary_activities")
	panels = append(panels, primaryRaw...)
	meta.primaryCount = len(primaryRaw)

	// Parse margin label
	if label, ok := data["margin_label"].(string); ok {
		meta.marginLabel = label
	} else if label, ok := data["margin"].(string); ok {
		meta.marginLabel = label
	}
	// Default margin label if not explicitly disabled
	if meta.marginLabel == "" {
		if showMargin, ok := data["show_margin"].(bool); !ok || showMargin {
			meta.marginLabel = "Margin"
		}
	}

	return panels, meta
}

// extractActivityList parses activities from a data map using the given keys.
func extractActivityList(data map[string]any, keys ...string) []nativePanelData {
	var raw []any
	for _, key := range keys {
		if v, ok := data[key].([]any); ok {
			raw = v
			break
		}
	}
	if raw == nil {
		return nil
	}

	var panels []nativePanelData
	for _, item := range raw {
		switch v := item.(type) {
		case string:
			panels = append(panels, nativePanelData{title: v})
		case map[string]any:
			panel := nativePanelData{}
			if label, ok := v["label"].(string); ok {
				panel.title = label
			} else if name, ok := v["name"].(string); ok {
				panel.title = name
			} else if title, ok := v["title"].(string); ok {
				panel.title = title
			}
			if items, ok := v["items"].([]any); ok {
				strs := parseSWOTStringList(items)
				if len(strs) > 0 {
					bulletLines := make([]string, len(strs))
					for j, s := range strs {
						bulletLines[j] = "- " + s
					}
					panel.body = strings.Join(bulletLines, "\n")
				}
			}
			if desc, ok := v["description"].(string); ok && panel.body == "" {
				panel.body = desc
			}
			panels = append(panels, panel)
		}
	}
	return panels
}

// generateValueChainGroupXML produces the complete <p:grpSp> XML for a value chain.
func generateValueChainGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, meta valueChainMeta, surface nativeSurface) string {
	if meta.primaryCount == 0 && meta.supportCount == 0 {
		slog.Warn("generateValueChainGroupXML: no activities provided")
		return ""
	}

	totalWidth := bounds.Width
	totalHeight := bounds.Height

	// Calculate margin area
	hasMargin := meta.marginLabel != ""
	var marginW int64
	if hasMargin {
		marginW = int64(float64(totalWidth) * vcMarginWidthRatio)
		if marginW < 365760 { // min ~0.4"
			marginW = 365760
		}
	}
	contentWidth := totalWidth
	if hasMargin {
		contentWidth = totalWidth - marginW - vcMarginGap
	}

	// Calculate section heights
	supportH := int64(float64(totalHeight) * vcSupportHeightRatio)
	primaryH := int64(float64(totalHeight) * vcPrimaryHeightRatio)

	// Adjust if only one section exists
	if meta.supportCount == 0 {
		primaryH = totalHeight
		supportH = 0
	} else if meta.primaryCount == 0 {
		supportH = totalHeight
		primaryH = 0
	}

	primaryY := bounds.Y + supportH + vcSectionGap
	if meta.supportCount == 0 {
		primaryY = bounds.Y
	}

	var children [][]byte
	nextID := shapeIDBase + 1

	// Generate support activity bars (stacked horizontal rects)
	if meta.supportCount > 0 {
		barH := (supportH - int64(meta.supportCount-1)*vcGap) / int64(meta.supportCount)
		for i := 0; i < meta.supportCount; i++ {
			if i >= len(panels) {
				break
			}
			panel := panels[i] //nolint:gosec // bounds checked by i >= len(panels) break above
			barY := bounds.Y + int64(i)*(barH+vcGap)
			xml := generateVCSupportBarXML(
				panel, bounds.X, barY, contentWidth, barH,
				nextID, surface.tint(i, vcSupportTint, vcLabelFontSize),
			)
			children = append(children, []byte(xml))
			nextID++
		}
	}

	// Generate primary activity chevrons (homePlate shapes in a row)
	if meta.primaryCount > 0 {
		totalGaps := int64(meta.primaryCount-1) * vcGap
		chevronW := (contentWidth - totalGaps) / int64(meta.primaryCount)

		for i := 0; i < meta.primaryCount; i++ {
			panel := panels[meta.supportCount+i]
			chevronX := bounds.X + int64(i)*(chevronW+vcGap)
			isLast := i == meta.primaryCount-1
			// A step label is centred on a structural fill, not a card
			// title: it stays in the text ink.
			tint := surface.tint(i, vcPrimaryTint, vcLabelFontSize)
			tint.ink = ""

			xml := generateVCPrimaryChevronXML(
				panel, chevronX, primaryY, chevronW, primaryH,
				nextID, tint, isLast,
			)
			children = append(children, []byte(xml))
			nextID++
		}
	}

	// Generate margin section
	if hasMargin {
		marginX := bounds.X + contentWidth + vcMarginGap
		xml := generateVCMarginXML(
			meta.marginLabel, marginX, bounds.Y, marginW, totalHeight,
			nextID, nativeSurface{colors: surface.colors}.tint(0, vcMarginTint, vcMarginFontSize),
		)
		children = append(children, []byte(xml))
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Value Chain",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generateValueChainGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateVCSupportBarXML produces a horizontal rect bar for a support activity.
func generateVCSupportBarXML(panel nativePanelData, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint) string {
	// Build paragraphs: bold title left-aligned, then items inline if space
	var paras []pptx.Paragraph

	// Title paragraph
	titleRun := pptx.Run{
		Text:     panel.title,
		Lang:     "en-US",
		FontSize: vcLabelFontSize,
		Bold:     true,
		Dirty:    true,
		Color:    tint.titleFill(),
	}
	bodyInk := diagramPanelTextFill(tint.scheme)

	// If there are body items, append them as a lighter suffix on the same line
	if panel.body != "" {
		lines := strings.Split(panel.body, "\n")
		var items []string
		for _, line := range lines {
			trimmed := strings.TrimPrefix(strings.TrimSpace(line), "- ")
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		if len(items) > 0 {
			suffix := " — " + strings.Join(items, " · ")
			paras = append(paras, pptx.Paragraph{
				Align:    "l",
				NoBullet: true,
				Runs: []pptx.Run{
					titleRun,
					{
						Text:     suffix,
						Lang:     "en-US",
						FontSize: vcBodyFontSize,
						Dirty:    true,
						Color:    bodyInk,
					},
				},
			})
		} else {
			paras = append(paras, pptx.Paragraph{
				Align:    "l",
				NoBullet: true,
				Runs:     []pptx.Run{titleRun},
			})
		}
	} else {
		paras = append(paras, pptx.Paragraph{
			Align:    "l",
			NoBullet: true,
			Runs:     []pptx.Run{titleRun},
		})
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "VC Support " + panel.title,
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: nativeSurfaceGeometry,
		Fill:     tint.fill(),
		Line:     pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "ctr",
			Insets:     pptx.ShapeTextInsets(),
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("generateVCSupportBarXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateVCPrimaryChevronXML produces a homePlate or rect shape for a primary activity.
func generateVCPrimaryChevronXML(panel nativePanelData, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint, isLast bool) string {
	// Use homePlate for all but last, rect for last
	geom := pptx.GeomHomePlate
	var adjustments []pptx.AdjustValue
	if isLast {
		geom = nativeSurfaceGeometry
	} else {
		adjustments = []pptx.AdjustValue{{Name: "adj", Value: vcHomePlateAdj}}
	}

	// Build paragraphs: centered bold title, then bulleted items below
	var paras []pptx.Paragraph

	// Title paragraph
	paras = append(paras, pptx.Paragraph{
		Align:    "ctr",
		NoBullet: true,
		Runs: []pptx.Run{{
			Text:     panel.title,
			Lang:     "en-US",
			FontSize: vcLabelFontSize,
			Bold:     true,
			Dirty:    true,
			Color:    tint.titleFill(),
		}},
	})

	// Body bullets
	if panel.body != "" {
		bodyParas := panelBulletsParagraphs(panel.body, vcBodyFontSize)
		bulletColor := pptx.ResolveColorString(tint.scheme)
		for i := range bodyParas {
			if bodyParas[i].Bullet != nil {
				bodyParas[i].Bullet.Color = bulletColor
			}
		}
		diagramPanelBodyColors(bodyParas, tint.scheme)
		paras = append(paras, bodyParas...)
	}

	// Reduce right inset for homePlate shapes to account for the arrow tip
	rInset := vcTextInset
	if !isLast {
		// homePlate tip eats into usable text area; add extra right inset
		tipDepth := cx * vcHomePlateAdj / 100000
		rInset = vcTextInset + tipDepth/2
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:          shapeID,
		Name:        "VC Primary " + panel.title,
		Bounds:      pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry:    geom,
		Adjustments: adjustments,
		Fill:        tint.fill(),
		Line:        pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "ctr",
			Insets:     [4]int64{vcTextInset, vcTextInset, rInset, vcTextInset},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("generateVCPrimaryChevronXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateVCMarginXML produces a vertical rect for the margin/profit section.
func generateVCMarginXML(label string, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "VC Margin",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: nativeSurfaceGeometry,
		Fill:     tint.fill(),
		Line:     pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Vert:    "vert270", // Vertical text, bottom-to-top
			Insets:  pptx.ShapeTextInsets(),
			AutoFit: "noAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     label,
					Lang:     "en-US",
					FontSize: vcMarginFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    tint.titleFill(),
				}},
			}},
		},
	})
	if err != nil {
		slog.Warn("generateVCMarginXML failed", "error", err)
		return ""
	}
	return string(b)
}
