package generator

import (
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/tokens"
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

	// vcBarPadEMU is a support bar's top / bottom text margin (7pt): the bar
	// is a one-line strip, set with row padding like a ruled list's rows.
	vcBarPadEMU int64 = 7 * 12700

	// vcSectionMinShare is the least of the height either section — the
	// support bars or the primary chevrons — keeps when the other needs more.
	vcSectionMinShare = 0.2

	// vcMaxBudgetItems bounds the search for how many items a chevron holds.
	vcMaxBudgetItems = 8

	// vcLabelFontSize is the font size for activity labels (hundredths of a point).
	// 1200 = 12pt
	vcLabelFontSize int = 1200

	// vcBodyFontSize is the activity item text size: the 12pt body step, the
	// smallest size a projected slide carries. It was 10pt
	// (go-slide-creator-6ne1m).
	vcBodyFontSize int = tokens.TypeScaleBodyHPt

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

// Laying the chain out from its text (go-slide-creator-6ne1m).
//
// The support bars took 40% of the height and the primary chevrons 52%,
// whatever they held, with the last 8% left empty; activity items were written
// at 10pt, and a chevron reserved half its tip twice — once in the preset's
// own text rectangle, once more as a right inset — so a 150pt chevron wrapped
// its bullets at 70pt. The chain is now sized in this order:
//
//  1. Width first. A chevron's text takes the preset's text rectangle, with
//     the uniform margin on both sides and nothing reserved twice.
//  2. Height from measured text. A support bar is a strip one line of text
//     tall at row padding (vcBarPadEMU); the chevrons, which hold the lists,
//     take the rest of the height, so the chain fills its region.
//  3. Padding before type. A chain that does not fit steps the chevrons' top
//     and bottom text margins, and the space between their bullets, down
//     nativeVerticalPadSteps.
//  4. Only then is a shrink stored, which generation reports with the budget
//     valueChainFitBudget measured.

// vcLayout is the chain's geometry: the support bars' and the chevrons'
// heights, the top / bottom text margin they are written with, and whether
// every shape holds its text at the authored size.
type vcLayout struct {
	contentWidth, marginW int64
	barH, supportH        int64
	primaryY, primaryH    int64
	chevronW              int64
	// pad is the chevrons' top / bottom text margin, barPad the bars'.
	pad, barPad int64
	fits        bool
}

// vcBarPadAt is a support bar's top / bottom text margin when the chevrons'
// is pad: row padding, never more than the chevrons keep.
func vcBarPadAt(pad int64) int64 { return min(pad, vcBarPadEMU) }

// vcBulletSpaceAfterAt is the space after a chevron bullet at a top / bottom
// margin of pad (hundredths of a point): the air between bullets steps down
// with the margin, before type.
func vcBulletSpaceAfterAt(pad int64) int {
	switch {
	case pad >= vcTextInset:
		return 400
	case pad >= nativeVerticalPadSteps[1]:
		return 200
	}
	return 0
}

// vcSupportText is the text body of a support bar: its bold title and, on the
// same line, its items.
func vcSupportText(panel nativePanelData, tint taxonomyTint, pad int64, fontName string) pptx.TextBody {
	runs := []pptx.Run{{
		Text:     panel.title,
		Lang:     "en-US",
		FontSize: vcLabelFontSize,
		Bold:     true,
		Dirty:    true,
		Color:    tint.titleFill(),
	}}
	var items []string
	for _, line := range strings.Split(panel.body, "\n") {
		if trimmed := strings.TrimPrefix(strings.TrimSpace(line), "- "); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	if len(items) > 0 {
		runs = append(runs, pptx.Run{
			Text:     " — " + strings.Join(items, " · "),
			Lang:     "en-US",
			FontSize: vcBodyFontSize,
			Dirty:    true,
			Color:    diagramPanelTextFill(tint.scheme),
		})
	}
	return pptx.TextBody{
		Wrap:       "square",
		Anchor:     "ctr",
		Insets:     [4]int64{vcTextInset, pad, vcTextInset, pad},
		AutoFit:    "normAutofit",
		Paragraphs: []pptx.Paragraph{{Align: "l", NoBullet: true, Runs: runs}},
		ThemeFonts: pptx.ThemeFonts{Major: fontName, Minor: fontName},
	}
}

// vcPrimaryText is the text body of a primary chevron: a centred bold title
// over its bulleted items.
func vcPrimaryText(panel nativePanelData, tint taxonomyTint, pad int64, fontName string) pptx.TextBody {
	paras := []pptx.Paragraph{{
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
	}}
	if panel.body != "" {
		paras[0].SpaceAfter = vcBulletSpaceAfterAt(pad)
		bodyParas := pptx.ParseBulletText(panel.body, pptx.BulletTextOptions{
			FontSize:    vcBodyFontSize,
			Lang:        "en-US",
			Dirty:       true,
			BulletColor: pptx.SchemeFill(panelBulletSchemeColor),
			SpaceAfter:  vcBulletSpaceAfterAt(pad),
		})
		bodyParas[len(bodyParas)-1].SpaceAfter = 0
		bulletColor := pptx.ResolveColorString(tint.scheme)
		for i := range bodyParas {
			if bodyParas[i].Bullet != nil {
				bodyParas[i].Bullet.Color = bulletColor
			}
		}
		diagramPanelBodyColors(bodyParas, tint.scheme)
		paras = append(paras, bodyParas...)
	}
	return pptx.TextBody{
		Wrap:       "square",
		Anchor:     "ctr",
		Insets:     [4]int64{vcTextInset, pad, vcTextInset, pad},
		AutoFit:    "normAutofit",
		Paragraphs: paras,
		ThemeFonts: pptx.ThemeFonts{Major: fontName, Minor: fontName},
	}
}

// vcChevronTextWidth is the width of a primary shape's own text rectangle: a
// homePlate gives up half its tip, the closing rect nothing. height is the
// shape's height, which sets the tip of a chevron wider than it is tall.
func vcChevronTextWidth(chevronW, height int64, isLast bool) int64 {
	if isLast {
		return chevronW
	}
	w, _ := pptx.PresetTextRectSize(string(pptx.GeomHomePlate), vcHomePlateAdj, pptx.RectEmu{CX: chevronW, CY: height})
	return w
}

// layoutValueChain sizes the chain in bounds.
func layoutValueChain(panels []nativePanelData, bounds types.BoundingBox, meta valueChainMeta, fontName string) vcLayout {
	var l vcLayout
	totalH := bounds.Height
	l.contentWidth = bounds.Width
	if meta.marginLabel != "" {
		l.marginW = max(int64(float64(bounds.Width)*vcMarginWidthRatio), 365760) // min ~0.4"
		l.contentWidth = bounds.Width - l.marginW - vcMarginGap
	}
	nSupport := min(meta.supportCount, len(panels))
	nPrimary := max(0, min(meta.primaryCount, len(panels)-nSupport))
	if nPrimary > 0 {
		l.chevronW = (l.contentWidth - int64(nPrimary-1)*vcGap) / int64(nPrimary)
	}
	sectionGap := vcSectionGap
	if nSupport == 0 || nPrimary == 0 {
		sectionGap = 0
	}
	supportGaps := int64(max(nSupport-1, 0)) * vcGap

	var barNeed, primaryNeed int64
	for _, pad := range nativeVerticalPadSteps {
		l.pad, l.barPad = pad, vcBarPadAt(pad)
		barNeed, primaryNeed = 0, 0
		for i := 0; i < nSupport; i++ {
			barNeed = max(barNeed, nativeTextNeedAtMarginEMU(vcSupportText(panels[i], taxonomyTint{}, l.barPad, fontName), l.contentWidth, totalH))
		}
		// The tallest the chevrons can be sets the deepest tip, and so the
		// narrowest text rectangle they are measured in.
		tallest := totalH - int64(nSupport)*barNeed - supportGaps - sectionGap
		for i := 0; i < nPrimary; i++ {
			w := vcChevronTextWidth(l.chevronW, max(tallest, 1), i == nPrimary-1)
			primaryNeed = max(primaryNeed, nativeTextNeedAtMarginEMU(vcPrimaryText(panels[nSupport+i], taxonomyTint{}, pad, fontName), w, totalH))
		}
		if int64(nSupport)*barNeed+supportGaps+sectionGap+primaryNeed <= totalH {
			l.fits = true
			break
		}
	}

	// The bars keep the height of their line; the chevrons take the rest. A
	// chain that fits at no step shares the shortfall in proportion to need.
	supportNeed := int64(nSupport)*barNeed + supportGaps
	room := totalH - sectionGap
	switch {
	case nPrimary == 0:
		l.supportH = room
	case nSupport == 0:
		l.primaryH = room
	case l.fits:
		l.supportH, l.primaryH = supportNeed, room-supportNeed
	default:
		l.supportH, l.primaryH = splitByNeed(room, supportNeed, primaryNeed, vcSectionMinShare)
	}
	if nSupport > 0 {
		l.barH = (l.supportH - supportGaps) / int64(nSupport)
	}
	l.primaryY = bounds.Y + l.supportH + sectionGap
	return l
}

// valueChainFitBudget measures what a primary activity holds at the authored
// size in bounds: how many one-line items every chevron can list, and how many
// characters one of those lines takes.
func valueChainFitBudget(panels []nativePanelData, bounds types.BoundingBox, meta valueChainMeta, fontName string) nativeFitBudget {
	nSupport := min(meta.supportCount, len(panels))
	probe := func(n int) vcLayout {
		filled := append([]nativePanelData{}, panels...)
		lines := make([]string, n)
		for i := range lines {
			lines[i] = "- Item"
		}
		for i := nSupport; i < len(filled); i++ {
			filled[i].body = strings.Join(lines, "\n")
		}
		return layoutValueChain(filled, bounds, meta, fontName)
	}
	items := 0
	for n := 1; n <= vcMaxBudgetItems; n++ {
		if !probe(n).fits {
			break
		}
		items = n
	}
	l := probe(1)
	chars := nativeOneLineChars(func(s string) pptx.TextBody {
		text := vcPrimaryText(nativePanelData{body: "- " + s}, taxonomyTint{}, pptx.ShapeTextInsetEMU, fontName)
		text.Paragraphs = text.Paragraphs[1:]
		return text
	}, vcChevronTextWidth(l.chevronW, max(l.primaryH, 1), false))
	return nativeFitBudget{
		item: "item", container: "activity", maxItems: items, maxChars: chars,
		sizePt: float64(vcBodyFontSize) / 100,
	}
}

// generateValueChainGroupXML produces the complete <p:grpSp> XML for a value chain.
func generateValueChainGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, meta valueChainMeta, surface nativeSurface, fontName string) string {
	if meta.primaryCount == 0 && meta.supportCount == 0 {
		slog.Warn("generateValueChainGroupXML: no activities provided")
		return ""
	}
	l := layoutValueChain(panels, bounds, meta, fontName)

	var children [][]byte
	nextID := shapeIDBase + 1

	// Generate support activity bars (stacked horizontal rects)
	for i, panel := range panels[:min(meta.supportCount, len(panels))] {
		tint := surface.tint(i, vcSupportTint, vcLabelFontSize)
		barY := bounds.Y + int64(i)*(l.barH+vcGap)
		xml := generateVCSupportBarXML(
			panel, bounds.X, barY, l.contentWidth, l.barH,
			nextID, tint, l.barPad, fontName,
		)
		children = append(children, []byte(xml))
		nextID++
	}

	// Generate primary activity chevrons (homePlate shapes in a row)
	for i := 0; i < meta.primaryCount && meta.supportCount+i < len(panels); i++ {
		panel := panels[meta.supportCount+i]
		chevronX := bounds.X + int64(i)*(l.chevronW+vcGap)
		isLast := i == meta.primaryCount-1
		// A step label is centred on a structural fill, not a card
		// title: it stays in the text ink.
		tint := surface.tint(i, vcPrimaryTint, vcLabelFontSize)
		tint.ink = ""

		xml := generateVCPrimaryChevronXML(
			panel, chevronX, l.primaryY, l.chevronW, l.primaryH,
			nextID, tint, isLast, l.pad, fontName,
		)
		children = append(children, []byte(xml))
		nextID++
	}

	// Generate margin section
	if meta.marginLabel != "" {
		marginX := bounds.X + l.contentWidth + vcMarginGap
		xml := generateVCMarginXML(
			meta.marginLabel, marginX, bounds.Y, l.marginW, bounds.Height,
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
func generateVCSupportBarXML(panel nativePanelData, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint, pad int64, fontName string) string {
	text := vcSupportText(panel, tint, pad, fontName)
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "VC Support " + panel.title,
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: nativeSurfaceGeometry,
		Fill:     tint.fill(),
		Line:     pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text:     &text,
	})
	if err != nil {
		slog.Warn("generateVCSupportBarXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateVCPrimaryChevronXML produces a homePlate or rect shape for a primary activity.
func generateVCPrimaryChevronXML(panel nativePanelData, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint, isLast bool, pad int64, fontName string) string {
	// Use homePlate for all but last, rect for last
	geom := pptx.GeomHomePlate
	var adjustments []pptx.AdjustValue
	if isLast {
		geom = nativeSurfaceGeometry
	} else {
		adjustments = []pptx.AdjustValue{{Name: "adj", Value: vcHomePlateAdj}}
	}

	// The text is laid out by the renderer in the preset's own text rectangle,
	// which already stops half a tip short of the point. The writer measures
	// the whole shape, so the shrink the text needs there — if any — is
	// measured here, in the rectangle it is drawn in.
	text := vcPrimaryText(panel, tint, pad, fontName)
	textRect := pptx.RectEmu{CX: vcChevronTextWidth(cx, cy, isLast), CY: cy}
	pptx.SetAutofitScale(&text, pptx.AutofitScaleFor(&text, textRect))

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:          shapeID,
		Name:        "VC Primary " + panel.title,
		Bounds:      pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry:    geom,
		Adjustments: adjustments,
		Fill:        tint.fill(),
		Line:        pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text:        &text,
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
