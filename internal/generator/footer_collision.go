package generator

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// FooterCollisionInput describes a JSON-authored shape and the footer reserved
// area on a slide. Only shapes authored in the JSON input (shape_grid cells)
// should be checked — shapes inherited from the layout master are excluded by
// the caller.
type FooterCollisionInput struct {
	// SlideIndex is the zero-based slide index.
	SlideIndex int
	// Path is the JSON path, e.g. "slides[0].shape_grid.rows[1].cells[0]".
	Path string
	// ShapeX is the shape's horizontal offset from the slide left edge (EMU).
	ShapeX int64
	// ShapeY is the shape's vertical offset from the slide top edge (EMU).
	ShapeY int64
	// ShapeCX is the shape width (EMU).
	ShapeCX int64
	// ShapeCY is the shape height (EMU).
	ShapeCY int64
	// Role is the shape's semantic role tag. Shapes with role "background"
	// or "decor" are skipped.
	Role string
	// FooterY is the top edge of the footer reserved area (EMU from top).
	FooterY int64
	// FooterCY is the height of the footer reserved area (EMU).
	FooterCY int64
	// LayoutDeclaresFooter indicates whether the slide's resolved layout
	// declares a footer placeholder (dt, ftr, or sldNum). When false, no
	// finding is emitted regardless of geometry — this prevents false
	// positives on layouts that use heuristic fallback positioning.
	LayoutDeclaresFooter bool
	// StrictFit controls the action severity: "strict" -> refuse,
	// "warn" -> review, "off" -> skip entirely.
	StrictFit string
}

// DetectFooterCollision checks whether a JSON-authored shape intrudes into
// the footer reserved area on a slide whose layout declares a footer
// placeholder.
//
// The detector only fires when LayoutDeclaresFooter is true. Shapes tagged
// with role "background" or "decor" are skipped — they are decorative and
// intentionally placed at the edges.
//
// Returns nil when there is no collision or when the check is not applicable.
func DetectFooterCollision(input FooterCollisionInput) *patterns.FitFinding {
	// Off mode: skip entirely.
	if input.StrictFit == "off" {
		return nil
	}

	// Only fire when the layout declares a footer placeholder.
	if !input.LayoutDeclaresFooter {
		return nil
	}

	// Skip decorative shapes.
	if input.Role == "background" || input.Role == "decor" {
		return nil
	}

	// Guard against degenerate inputs.
	if input.ShapeCX <= 0 || input.ShapeCY <= 0 || input.FooterCY <= 0 {
		return nil
	}

	// Check axis-aligned rectangle intersection on the Y axis.
	// The footer occupies [FooterY, FooterY+FooterCY).
	// The shape occupies [ShapeY, ShapeY+ShapeCY).
	// They intersect when shape bottom > footer top AND shape top < footer bottom.
	shapeBottom := input.ShapeY + input.ShapeCY
	footerBottom := input.FooterY + input.FooterCY

	if shapeBottom <= input.FooterY || input.ShapeY >= footerBottom {
		return nil // No vertical overlap.
	}

	// Compute the vertical intrusion in EMU.
	overlapTop := input.ShapeY
	if overlapTop < input.FooterY {
		overlapTop = input.FooterY
	}
	overlapBottom := shapeBottom
	if overlapBottom > footerBottom {
		overlapBottom = footerBottom
	}
	intrusionEMU := overlapBottom - overlapTop

	action := "review"
	if input.StrictFit == "strict" {
		action = "refuse"
	}

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "shape_grid",
			Path:    input.Path,
			Code:    patterns.ErrCodeFooterCollision,
			Message: fmt.Sprintf(
				"shape bottom edge (%d EMU) intrudes %d EMU into footer area (top=%d EMU)",
				shapeBottom, intrusionEMU, input.FooterY,
			),
			Fix: &patterns.FixSuggestion{Kind: "reposition_shape"},
		},
		Action: action,
		Measured: &patterns.Extent{
			WidthEMU:  input.ShapeCX,
			HeightEMU: input.ShapeCY,
		},
		Allowed: &patterns.Extent{
			WidthEMU:  input.ShapeCX,
			HeightEMU: input.FooterY - input.ShapeY, // available height above footer
		},
	}
}

// footerObstacle is visible, non-placeholder art inherited from a layout or
// master. Broad backgrounds and footer placeholders are deliberately excluded.
type footerObstacle struct {
	name string
	box  transformXML
}

const footerArtGap int64 = 91440 // 0.1in of breathing room around artwork

type footerArtKind uint8

const (
	footerArtText footerArtKind = iota
	footerArtFill
	footerArtPictureKind
	footerArtFrameKind
)

type footerArtShape struct {
	NV nonVisualPropertiesXML `xml:"nvSpPr"`
	SP struct {
		Xfrm  *transformXML `xml:"xfrm"`
		Solid *struct{}     `xml:"solidFill"`
		Grad  *struct{}     `xml:"gradFill"`
	} `xml:"spPr"`
	Text struct {
		Paragraphs []struct {
			Runs []struct {
				Text string `xml:"t"`
			} `xml:"r"`
			Fields []struct {
				Text string `xml:"t"`
			} `xml:"fld"`
		} `xml:"p"`
	} `xml:"txBody"`
}

type footerArtPicture struct {
	NV struct {
		CNV connectionNonVisualXML `xml:"cNvPr"`
		NVP nvPrXML                `xml:"nvPr"`
	} `xml:"nvPicPr"`
	SP struct {
		Xfrm *transformXML `xml:"xfrm"`
	} `xml:"spPr"`
}

type footerArtFrame struct {
	NV struct {
		CNV connectionNonVisualXML `xml:"cNvPr"`
		NVP nvPrXML                `xml:"nvPr"`
	} `xml:"nvGraphicFramePr"`
	Xfrm *transformXML `xml:"xfrm"`
}

type footerGroupTransform struct {
	Offset      offsetXML `xml:"off"`
	Extent      extentXML `xml:"ext"`
	ChildOffset offsetXML `xml:"chOff"`
	ChildExtent extentXML `xml:"chExt"`
}

type footerArtGroup struct {
	Properties struct {
		Transform *footerGroupTransform `xml:"xfrm"`
	} `xml:"grpSpPr"`
	Shapes   []footerArtShape   `xml:"sp"`
	Pictures []footerArtPicture `xml:"pic"`
	Frames   []footerArtFrame   `xml:"graphicFrame"`
	Groups   []footerArtGroup   `xml:"grpSp"`
}

type footerArtTree struct {
	Shapes   []footerArtShape   `xml:"sp"`
	Pictures []footerArtPicture `xml:"pic"`
	Frames   []footerArtFrame   `xml:"graphicFrame"`
	Groups   []footerArtGroup   `xml:"grpSp"`
}

// parseFooterObstacles reads the geometry of text, images, graphic frames and
// compact filled shapes. A full-bleed picture or broad color band is a
// background, not an obstacle to text drawn over it.
func parseFooterObstacles(data []byte, slideWidth, slideHeight int64) ([]footerObstacle, error) {
	var doc struct {
		Common struct {
			Tree footerArtTree `xml:"spTree"`
		} `xml:"cSld"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse footer artwork: %w", err)
	}
	if slideWidth <= 0 {
		slideWidth = 12192000
	}
	if slideHeight <= 0 {
		slideHeight = defaultSlideHeightEMU
	}
	var obstacles []footerObstacle
	collectFooterArt(doc.Common.Tree, nil, slideWidth, slideHeight, &obstacles)
	return obstacles, nil
}

func collectFooterArt(tree footerArtTree, parents []*footerGroupTransform, slideWidth, slideHeight int64, obstacles *[]footerObstacle) {
	for _, shape := range tree.Shapes {
		if shape.NV.NvPr.Placeholder != nil {
			continue
		}
		if footerArtShapeHasText(shape) {
			appendFooterObstacle(obstacles, shape.NV.ConnectionNonVisual.Name, shape.SP.Xfrm, footerArtText, parents, slideWidth, slideHeight)
		} else if shape.SP.Solid != nil || shape.SP.Grad != nil {
			appendFooterObstacle(obstacles, shape.NV.ConnectionNonVisual.Name, shape.SP.Xfrm, footerArtFill, parents, slideWidth, slideHeight)
		}
	}
	for _, picture := range tree.Pictures {
		if picture.NV.NVP.Placeholder == nil {
			appendFooterObstacle(obstacles, picture.NV.CNV.Name, picture.SP.Xfrm, footerArtPictureKind, parents, slideWidth, slideHeight)
		}
	}
	for _, frame := range tree.Frames {
		if frame.NV.NVP.Placeholder == nil {
			appendFooterObstacle(obstacles, frame.NV.CNV.Name, frame.Xfrm, footerArtFrameKind, parents, slideWidth, slideHeight)
		}
	}
	for _, group := range tree.Groups {
		chain := append(append([]*footerGroupTransform(nil), parents...), group.Properties.Transform)
		children := footerArtTree{Shapes: group.Shapes, Pictures: group.Pictures, Frames: group.Frames, Groups: group.Groups}
		collectFooterArt(children, chain, slideWidth, slideHeight, obstacles)
	}
}

func footerArtShapeHasText(shape footerArtShape) bool {
	for _, paragraph := range shape.Text.Paragraphs {
		for _, run := range paragraph.Runs {
			if strings.TrimSpace(run.Text) != "" {
				return true
			}
		}
		for _, field := range paragraph.Fields {
			if strings.TrimSpace(field.Text) != "" {
				return true
			}
		}
	}
	return false
}

func appendFooterObstacle(obstacles *[]footerObstacle, name string, box *transformXML, kind footerArtKind, parents []*footerGroupTransform, slideWidth, slideHeight int64) {
	if box == nil || box.Extent.CX <= 0 || box.Extent.CY <= 0 {
		return
	}
	mapped := *box
	for i := len(parents) - 1; i >= 0; i-- {
		mapped = mapFooterGroupBox(mapped, parents[i])
	}
	if mapped.Extent.CX <= 0 || mapped.Extent.CY <= 0 {
		return
	}
	// Do not treat a broad color band or full-bleed image as an obstruction:
	// footer text is intentionally drawn over those backgrounds. Content-bearing
	// text and tables remain obstacles regardless of size.
	if kind == footerArtFill && (mapped.Extent.CX >= slideWidth*3/4 || mapped.Extent.CY >= slideHeight*3/4) {
		return
	}
	if kind == footerArtPictureKind && mapped.Extent.CX >= slideWidth*4/5 && mapped.Extent.CY >= slideHeight*3/4 {
		return
	}
	*obstacles = append(*obstacles, footerObstacle{name: name, box: mapped})
}

// mapFooterGroupBox maps a child shape through one OOXML group transform.
// A missing/zero child extent is treated as identity, matching PowerPoint's
// default for an untransformed group.
func mapFooterGroupBox(box transformXML, group *footerGroupTransform) transformXML {
	if group == nil || group.ChildExtent.CX <= 0 || group.ChildExtent.CY <= 0 {
		return box
	}
	box.Offset.X = group.Offset.X + (box.Offset.X-group.ChildOffset.X)*group.Extent.CX/group.ChildExtent.CX
	box.Offset.Y = group.Offset.Y + (box.Offset.Y-group.ChildOffset.Y)*group.Extent.CY/group.ChildExtent.CY
	box.Extent.CX = box.Extent.CX * group.Extent.CX / group.ChildExtent.CX
	box.Extent.CY = box.Extent.CY * group.Extent.CY / group.ChildExtent.CY
	return box
}

// footerObstaclesForLayout caches inherited artwork because many slides share
// one layout. A layout can suppress master artwork via showMasterSp="0".
func (ctx *singlePassContext) footerObstaclesForLayout(layoutID string) []footerObstacle {
	if obstacles, ok := ctx.footerObstaclesByLayout[layoutID]; ok {
		return obstacles
	}
	var obstacles []footerObstacle
	layout, err := ctx.readLayoutFile(layoutID)
	if err != nil {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("footer artwork in layout %s: %v", layoutID, err))
	} else {
		if regions, parseErr := parseFooterObstacles(layout, ctx.slideWidth, ctx.slideHeight); parseErr == nil {
			obstacles = append(obstacles, regions...)
		} else {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf("footer artwork in layout %s: %v", layoutID, parseErr))
		}
	}
	var visibility struct {
		ShowMasterSp string `xml:"showMasterSp,attr"`
	}
	showMaster := true
	if err == nil {
		if parseErr := xml.Unmarshal(layout, &visibility); parseErr != nil {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf("footer master visibility in layout %s: %v", layoutID, parseErr))
		} else if visibility.ShowMasterSp == "0" || strings.EqualFold(visibility.ShowMasterSp, "false") {
			showMaster = false
		}
	}
	if showMaster {
		if masterPath := ctx.masterPathForLayout(layoutID); masterPath != "" {
			if master, readErr := ctx.readFileWithSyntheticFallback(masterPath); readErr == nil {
				if regions, parseErr := parseFooterObstacles(master, ctx.slideWidth, ctx.slideHeight); parseErr == nil {
					obstacles = append(obstacles, regions...)
				} else {
					ctx.warnings = append(ctx.warnings, fmt.Sprintf("footer artwork in master %s: %v", masterPath, parseErr))
				}
			} else {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("footer artwork in master %s: %v", masterPath, readErr))
			}
		}
	}
	if ctx.footerObstaclesByLayout == nil {
		ctx.footerObstaclesByLayout = make(map[string][]footerObstacle)
	}
	ctx.footerObstaclesByLayout[layoutID] = obstacles
	return obstacles
}

func footerBoxesOverlap(a, b transformXML) bool {
	return a.Offset.X < b.Offset.X+b.Extent.CX && b.Offset.X < a.Offset.X+a.Extent.CX &&
		a.Offset.Y < b.Offset.Y+b.Extent.CY && b.Offset.Y < a.Offset.Y+a.Extent.CY
}

// clearPageNumberBox keeps the sized box on its footer baseline and picks the
// closest clear horizontal position within the right footer's allowed space.
// A nil result means there is no safe place for the page number.
func clearPageNumberBox(box *transformXML, obstacles []footerObstacle, leftLimit, slideWidth int64) *transformXML {
	if box == nil {
		return nil
	}
	if slideWidth <= 0 {
		slideWidth = 12192000
	}
	maxX := slideWidth - box.Extent.CX
	if maxX < leftLimit {
		return nil
	}
	candidates := []int64{box.Offset.X, leftLimit, maxX}
	for _, obstacle := range obstacles {
		if box.Offset.Y >= obstacle.box.Offset.Y+obstacle.box.Extent.CY ||
			obstacle.box.Offset.Y >= box.Offset.Y+box.Extent.CY {
			continue
		}
		candidates = append(candidates,
			obstacle.box.Offset.X-box.Extent.CX-footerArtGap,
			obstacle.box.Offset.X+obstacle.box.Extent.CX+footerArtGap)
	}
	var best *transformXML
	var bestDistance int64
	for _, x := range candidates {
		if x < leftLimit || x > maxX {
			continue
		}
		candidate := *box
		candidate.Offset.X = x
		clear := true
		for _, obstacle := range obstacles {
			if footerBoxesOverlap(candidate, obstacle.box) {
				clear = false
				break
			}
		}
		if !clear {
			continue
		}
		distance := x - box.Offset.X
		if distance < 0 {
			distance = -distance
		}
		if best == nil || distance < bestDistance {
			best, bestDistance = &candidate, distance
		}
	}
	return best
}

// clearLeftFooterBox takes the widest clear interval within the left footer
// band. Text is refitted to that interval, or dropped if less than 1in remains.
func clearLeftFooterBox(box *transformXML, obstacles []footerObstacle) *transformXML {
	if box == nil {
		return nil
	}
	start, end := box.Offset.X, box.Offset.X+box.Extent.CX
	type interval struct{ start, end int64 }
	var blocked []interval
	for _, obstacle := range obstacles {
		if box.Offset.Y >= obstacle.box.Offset.Y+obstacle.box.Extent.CY ||
			obstacle.box.Offset.Y >= box.Offset.Y+box.Extent.CY {
			continue
		}
		left := max(start, obstacle.box.Offset.X-footerArtGap)
		right := min(end, obstacle.box.Offset.X+obstacle.box.Extent.CX+footerArtGap)
		if right > left {
			blocked = append(blocked, interval{left, right})
		}
	}
	if len(blocked) == 0 {
		return box
	}
	sort.Slice(blocked, func(i, j int) bool { return blocked[i].start < blocked[j].start })
	best := interval{}
	cursor := start
	for _, obstacle := range blocked {
		if obstacle.start > cursor && obstacle.start-cursor > best.end-best.start {
			best = interval{cursor, obstacle.start}
		}
		if obstacle.end > cursor {
			cursor = obstacle.end
		}
	}
	if end > cursor && end-cursor > best.end-best.start {
		best = interval{cursor, end}
	}
	if best.end-best.start < minLeftFooterWidth {
		return nil
	}
	adjusted := *box
	adjusted.Offset.X = best.start
	adjusted.Extent.CX = best.end - best.start
	return &adjusted
}
