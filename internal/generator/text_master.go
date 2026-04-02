// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// masterBodyStyleXML represents the bodyStyle element in a slide master's txStyles.
type masterBodyStyleXML struct {
	Lvl1pPr *masterLvlPPrXML `xml:"lvl1pPr"`
	Lvl2pPr *masterLvlPPrXML `xml:"lvl2pPr"`
	Lvl3pPr *masterLvlPPrXML `xml:"lvl3pPr"`
	Lvl4pPr *masterLvlPPrXML `xml:"lvl4pPr"`
	Lvl5pPr *masterLvlPPrXML `xml:"lvl5pPr"`
	Lvl6pPr *masterLvlPPrXML `xml:"lvl6pPr"`
	Lvl7pPr *masterLvlPPrXML `xml:"lvl7pPr"`
	Lvl8pPr *masterLvlPPrXML `xml:"lvl8pPr"`
	Lvl9pPr *masterLvlPPrXML `xml:"lvl9pPr"`
}

// masterLvlPPrXML represents a level paragraph property in the slide master.
type masterLvlPPrXML struct {
	Inner string `xml:",innerxml"` // Contains buNone, buChar, etc.
}

// masterTxStylesXML represents the txStyles element in a slide master.
type masterTxStylesXML struct {
	BodyStyle *masterBodyStyleXML `xml:"bodyStyle"`
}

// slideMasterForBulletsXML is a minimal parse of slide master to extract bullet info.
type slideMasterForBulletsXML struct {
	TxStyles *masterTxStylesXML `xml:"txStyles"`
}

// findFirstBulletLevelFromMaster parses a slide master and returns the first level
// that has bullets enabled (doesn't have <a:buNone/>). Returns -1 if not found or on error.
func findFirstBulletLevelFromMaster(masterData []byte) int {
	var master slideMasterForBulletsXML
	if err := xml.Unmarshal(masterData, &master); err != nil {
		return -1
	}

	if master.TxStyles == nil || master.TxStyles.BodyStyle == nil {
		return -1
	}

	bs := master.TxStyles.BodyStyle
	levels := []*masterLvlPPrXML{
		bs.Lvl1pPr, bs.Lvl2pPr, bs.Lvl3pPr, bs.Lvl4pPr, bs.Lvl5pPr,
		bs.Lvl6pPr, bs.Lvl7pPr, bs.Lvl8pPr, bs.Lvl9pPr,
	}

	for i, lvl := range levels {
		if lvl == nil {
			continue
		}
		// Check if this level has bullets disabled
		if !strings.Contains(lvl.Inner, "buNone") {
			return i
		}
	}

	return -1
}

// findMasterPathForLayoutFromZip finds the slide master path for a given layout.
// layoutID should be the layout filename (e.g., "slideLayout3").
func findMasterPathForLayoutFromZip(idx utils.ZipIndex, layoutID string) string {
	layoutRelsPath := fmt.Sprintf("%s_rels/%s.xml.rels", PathSlideLayouts, layoutID)

	relsData, err := utils.ReadFileFromZipIndex(idx, layoutRelsPath)
	if err != nil {
		return ""
	}

	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		return ""
	}

	for _, rel := range rels.Relationships {
		if rel.Type == pptx.RelTypeSlideMaster {
			// Target is relative (e.g., "../slideMasters/slideMaster1.xml")
			// Use "ppt/slideLayouts" (without trailing slash) as base directory
			return template.ResolveRelativePath("ppt/slideLayouts", rel.Target)
		}
	}

	return ""
}

// findMasterPathFromSyntheticRels finds the slide master path from synthetic
// layout rels files. Synthetic layouts (e.g., slideLayout99) aren't in the
// template ZIP, so their rels files must be read from ctx.syntheticFiles.
func findMasterPathFromSyntheticRels(syntheticFiles map[string][]byte, layoutID string) string {
	if len(syntheticFiles) == 0 {
		return ""
	}

	relsPath := fmt.Sprintf("%s_rels/%s.xml.rels", PathSlideLayouts, layoutID)
	relsData, ok := syntheticFiles[relsPath]
	if !ok {
		return ""
	}

	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		return ""
	}

	for _, rel := range rels.Relationships {
		if rel.Type == pptx.RelTypeSlideMaster {
			return template.ResolveRelativePath("ppt/slideLayouts", rel.Target)
		}
	}

	return ""
}

// getFirstBulletLevelForLayout returns the first bullet level for a given layout.
// This is determined by parsing the slide master's bodyStyle to find the first
// level that doesn't have buNone (bullets disabled).
// Returns 0 as the default if no master bullet info can be found.
func (ctx *singlePassContext) getFirstBulletLevelForLayout(layoutID string) int {
	// Handle nil template reader (e.g., in tests)
	if ctx.templateReader == nil {
		slog.Debug("getFirstBulletLevelForLayout: nil templateReader, returning 0", slog.String("layout_id", layoutID))
		return 0
	}

	// Find the master path for this layout
	masterPath := findMasterPathForLayoutFromZip(ctx.templateIndex, layoutID)
	if masterPath == "" {
		// Synthetic layouts (e.g., slideLayout99) aren't in the template ZIP.
		// Try finding the master path from the synthetic rels file instead.
		masterPath = findMasterPathFromSyntheticRels(ctx.syntheticFiles, layoutID)
	}
	if masterPath == "" {
		slog.Debug("getFirstBulletLevelForLayout: no master path found, returning 0",
			slog.String("layout_id", layoutID))
		return 0 // Default to level 0
	}

	// Check cache
	if level, ok := ctx.masterBulletLevelCache[masterPath]; ok {
		slog.Debug("getFirstBulletLevelForLayout: cached level",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.Int("level", level))
		return level
	}

	// Load and parse the master
	masterData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, masterPath)
	if err != nil {
		slog.Debug("getFirstBulletLevelForLayout: failed to read master, returning 0",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.String("error", err.Error()))
		ctx.masterBulletLevelCache[masterPath] = 0
		return 0
	}

	bulletLevel := findFirstBulletLevelFromMaster(masterData)
	slog.Debug("getFirstBulletLevelForLayout: found bullet level from master",
		slog.String("layout_id", layoutID),
		slog.String("master_path", masterPath),
		slog.Int("bullet_level", bulletLevel))
	if bulletLevel < 0 {
		bulletLevel = 0 // Default to level 0 if no bullet level found
	}

	ctx.masterBulletLevelCache[masterPath] = bulletLevel
	return bulletLevel
}
