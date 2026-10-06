// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"encoding/xml"
	"fmt"
	"log/slog"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// findFirstBulletLevelFromMaster returns the first bodyStyle level of a slide
// master that has bullets enabled (doesn't have <a:buNone/>), -1 if there is
// none. The template analysis records the same level per placeholder
// (PlaceholderInfo.BulletBaseLevel), so both read it through one function.
func findFirstBulletLevelFromMaster(masterData []byte) int {
	return template.FirstBulletLevel(masterData)
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
