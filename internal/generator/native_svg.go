// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/pptx"
	"github.com/ahrens/go-slide-creator/internal/utils"
)

// allocateMediaRelIDs pre-allocates relationship IDs for media inserts (images, charts, infographics).
// This must be called before writing slides so that the p:pic elements have correct rIds.
// For new slides, relationship IDs start at rId2 (rId1 is reserved for layout).
func (ctx *singlePassContext) allocateMediaRelIDs() error {
	for slideNum, mediaRels := range ctx.slideRelUpdates {
		if len(mediaRels) == 0 {
			continue
		}

		// For new slides, we know the layout relationship is rId1
		// so media starts at rId2
		nextRelID := 2

		// Allocate IDs for each media item
		for i := range mediaRels {
			mediaRels[i].relID = fmt.Sprintf("rId%d", nextRelID)
			nextRelID++
		}

		// Update the map with allocated IDs
		ctx.slideRelUpdates[slideNum] = mediaRels
	}

	return nil
}

// allocateBackgroundRelIDs pre-allocates relationship IDs for slide background images.
// Called after all other rel ID allocations. Computes the next available rId for each
// slide that has a background image by counting already-allocated relationships.
func (ctx *singlePassContext) allocateBackgroundRelIDs() {
	for slideNum, bgMedia := range ctx.slideBgMedia {
		// rId1 = layout
		nextRelID := 2

		// Count regular media rels
		if mediaRels, ok := ctx.slideRelUpdates[slideNum]; ok {
			nextRelID += len(mediaRels)
		}

		// Count native SVG rels (2 per insert: PNG + SVG)
		if nativeSVGs, ok := ctx.nativeSVGInserts[slideNum]; ok {
			nextRelID += len(nativeSVGs) * 2
		}

		// Count panel icon rels
		if panelInserts, ok := ctx.panelShapeInserts[slideNum]; ok {
			for _, ins := range panelInserts {
				for _, panel := range ins.panels {
					if panel.iconRelID != "" {
						nextRelID++
					}
				}
			}
		}

		bgMedia.relID = fmt.Sprintf("rId%d", nextRelID)
		ctx.slideBgMedia[slideNum] = bgMedia
	}
}

// allocateNativeSVGRelIDs pre-allocates relationship IDs for native SVG inserts.
// This must be called before writing slides so that the p:pic elements have correct rIds.
func (ctx *singlePassContext) allocateNativeSVGRelIDs() error {
	for slideNum, nativeSVGs := range ctx.nativeSVGInserts {
		if len(nativeSVGs) == 0 {
			continue
		}

		// Read existing relationships to find next available ID.
		// For new slides, rId1 is reserved for the layout relationship,
		// so media relationships start at rId2.
		relsFileName := SlideRelsPath(slideNum)
		nextRelID := 2

		relsData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, relsFileName)
		if err == nil {
			var existingRels pptx.RelationshipsXML
			if parseErr := xml.Unmarshal(relsData, &existingRels); parseErr == nil {
				for _, rel := range existingRels.Relationships {
					var num int
					if _, err := fmt.Sscanf(rel.ID, "rId%d", &num); err == nil {
						if num >= nextRelID {
							nextRelID = num + 1
						}
					}
				}
			}
		}

		// Also check if there are regular media relationships being added
		if mediaRels, hasMedia := ctx.slideRelUpdates[slideNum]; hasMedia {
			nextRelID += len(mediaRels)
		}

		// Allocate IDs for each native SVG insert (PNG first, then SVG)
		for i := range nativeSVGs {
			nativeSVGs[i].pngRelID = fmt.Sprintf("rId%d", nextRelID)
			nextRelID++
			nativeSVGs[i].svgRelID = fmt.Sprintf("rId%d", nextRelID)
			nextRelID++
		}

		// Update the map with allocated IDs
		ctx.nativeSVGInserts[slideNum] = nativeSVGs
	}

	return nil
}

// insertNativeSVGPics inserts p:pic elements for native SVG inserts into slide XML.
// It finds the closing </p:spTree> tag and inserts the p:pic elements before it.
// Relationship IDs must already be allocated via allocateNativeSVGRelIDs().
func (ctx *singlePassContext) insertNativeSVGPics(slideNum int, slideData []byte, nativeSVGs []nativeSVGInsert) ([]byte, error) {
	// Find the next shape ID using pre-compiled regex (faster than O(n) string scan)
	maxID := findMaxShapeID(slideData)
	nextShapeID := maxID + 1
	if nextShapeID < 100 {
		nextShapeID = 100 // Start high to avoid conflicts with typical OOXML IDs
	}

	// Generate p:pic elements for each native SVG
	var picsToInsert []string
	for i := range nativeSVGs {
		svg := &nativeSVGs[i]

		// Skip zero-size SVGs — these result from placeholders with unresolved
		// bounds (e.g., implicit body type not matched in master lookup).
		// Without this guard, PowerPoint renders them as invisible 0x0 elements.
		if svg.extentCX == 0 && svg.extentCY == 0 {
			slog.Warn("skipping zero-size native SVG insert",
				slog.Int("slide", slideNum),
				slog.Int("index", i),
			)
			continue
		}

		svg.shapeID = nextShapeID
		nextShapeID++

		// Generate p:pic XML using the pptx package
		// Relationship IDs were pre-allocated by allocateNativeSVGRelIDs()
		picXML, err := pptx.GeneratePicWithSVG(
			svg.shapeID,
			svg.pngRelID,
			svg.svgRelID,
			svg.offsetX,
			svg.offsetY,
			svg.extentCX,
			svg.extentCY,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to generate p:pic for SVG: %w", err)
		}
		picsToInsert = append(picsToInsert, string(picXML))
	}

	// Store the updated inserts back (with shape IDs)
	ctx.nativeSVGInserts[slideNum] = nativeSVGs

	// Find closing </p:spTree> and insert p:pic elements before it
	insertPos := findLastClosingSpTree(slideData)
	if insertPos == -1 {
		return nil, fmt.Errorf("could not find </p:spTree> in slide XML")
	}

	// Insert the p:pic elements
	insertion := strings.Join(picsToInsert, "\n")
	return spliceBytes(slideData, insertPos, insertion), nil
}
