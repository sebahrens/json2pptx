// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// SlotPopulationConfig provides configuration for slot-based content population.
type SlotPopulationConfig struct {
	Theme *types.ThemeInfo // Theme colors and fonts (optional)
}

// SlotPopulationResult contains the result of slot population.
type SlotPopulationResult struct {
	ContentItems []ContentItem // Generated content items for the generator
	Warnings     []string      // Non-fatal warnings encountered
}

// FilterContentPlaceholders returns only body/content placeholders from a layout.
// It excludes title, subtitle, date, footer, slide number, and header placeholders.
// The result is sorted by placeholder index for consistent slot mapping.
func FilterContentPlaceholders(placeholders []types.PlaceholderInfo) []types.PlaceholderInfo {
	var content []types.PlaceholderInfo
	for _, ph := range placeholders {
		// Include body and content placeholders
		// Exclude: title, subtitle, date, footer, slide number, header, other
		switch ph.Type {
		case types.PlaceholderBody, types.PlaceholderContent:
			content = append(content, ph)
		case types.PlaceholderImage, types.PlaceholderChart, types.PlaceholderTable:
			// Include visual placeholders as valid slot targets
			content = append(content, ph)
		}
	}

	// Sort by index to ensure consistent ordering
	slices.SortFunc(content, func(a, b types.PlaceholderInfo) int {
		return cmp.Compare(a.Index, b.Index)
	})

	return content
}

// routeTypesSlotContent converts types.SlotContent to generator ContentItems.
// For chart/infographic slots that also have body text (e.g., "**FY24 Mix**"
// preceding a chart code block), this returns both a ContentText item and a
// ContentDiagram item for the same placeholder. The text collision detection
// in processDiagramContent then positions text above the diagram.
func routeTypesSlotContent(slot *types.SlotContent, placeholder types.PlaceholderInfo) ([]ContentItem, error) {
	if slot == nil {
		return nil, nil
	}

	switch slot.Type {
	case types.SlotContentText:
		return []ContentItem{{
			PlaceholderID: placeholder.ID,
			Type:          ContentText,
			Value:         slot.Text,
		}}, nil

	case types.SlotContentBullets:
		return []ContentItem{{
			PlaceholderID: placeholder.ID,
			Type:          ContentBullets,
			Value:         slot.Bullets,
		}}, nil

	case types.SlotContentBodyAndBullets:
		return []ContentItem{{
			PlaceholderID: placeholder.ID,
			Type:          ContentBodyAndBullets,
			Value: BodyAndBulletsContent{
				Body:         slot.Body,
				Bullets:      slot.Bullets,
				TrailingBody: slot.BodyAfterBullets,
			},
		}}, nil

	case types.SlotContentBulletGroups:
		return []ContentItem{{
			PlaceholderID: placeholder.ID,
			Type:          ContentBulletGroups,
			Value: BulletGroupsContent{
				Body:         slot.Body,
				Groups:       slot.BulletGroups,
				TrailingBody: slot.BodyAfterBullets,
			},
		}}, nil

	case types.SlotContentTable:
		if slot.Table == nil {
			return nil, fmt.Errorf("no valid table found in slot %d", slot.SlotNumber)
		}
		return []ContentItem{{
			PlaceholderID: placeholder.ID,
			Type:          ContentTable,
			Value:         slot.Table,
		}}, nil

	case types.SlotContentChart:
		if slot.DiagramSpec == nil {
			return nil, fmt.Errorf("no valid chart found in slot %d", slot.SlotNumber)
		}
		return buildDiagramItems(slot, placeholder), nil

	case types.SlotContentInfographic:
		if slot.DiagramSpec == nil {
			return nil, fmt.Errorf("no valid infographic found in slot %d", slot.SlotNumber)
		}
		return buildDiagramItems(slot, placeholder), nil

	case types.SlotContentImage:
		if slot.ImagePath == "" {
			return nil, fmt.Errorf("no valid image path found in slot %d", slot.SlotNumber)
		}
		return []ContentItem{{
			PlaceholderID: placeholder.ID,
			Type:          ContentImage,
			Value: ImageContent{
				Path: slot.ImagePath,
				Alt:  "Slot image",
			},
		}}, nil

	default:
		return nil, fmt.Errorf("unknown content type: %s", slot.Type)
	}
}

// buildDiagramItems returns ContentItems for a chart/infographic slot.
// When the slot has pre-fence body text, a ContentText item is emitted first
// so that text collision detection positions it above the diagram.
func buildDiagramItems(slot *types.SlotContent, placeholder types.PlaceholderInfo) []ContentItem {
	var items []ContentItem
	if strings.TrimSpace(slot.Body) != "" {
		items = append(items, ContentItem{
			PlaceholderID: placeholder.ID,
			Type:          ContentText,
			Value:         slot.Body,
		})
	}
	items = append(items, ContentItem{
		PlaceholderID: placeholder.ID,
		Type:          ContentDiagram,
		Value:         slot.DiagramSpec,
	})
	return items
}

// BuildSlotContentItems creates ContentItem slice from slot content map.
// It maps slot numbers (1-indexed) to content placeholders (0-indexed).
// Returns content items and any warnings encountered.
func BuildSlotContentItems(
	slots map[int]*types.SlotContent,
	layout types.LayoutMetadata,
	config SlotPopulationConfig,
) (*SlotPopulationResult, error) {
	result := &SlotPopulationResult{
		ContentItems: make([]ContentItem, 0),
		Warnings:     make([]string, 0),
	}

	if len(slots) == 0 {
		return result, nil
	}

	// Get only content placeholders (filter out title, footer, date, slide number)
	contentPlaceholders := FilterContentPlaceholders(layout.Placeholders)
	maxSlot := len(contentPlaceholders)

	if maxSlot == 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("layout '%s' has no content placeholders for slot markers", layout.Name))
		return result, nil
	}

	// Sort slot numbers for deterministic iteration order
	slotNums := make([]int, 0, len(slots))
	for slotNum := range slots {
		slotNums = append(slotNums, slotNum)
	}
	slices.Sort(slotNums)

	// Route each slot to its placeholder
	for _, slotNum := range slotNums {
		if slotNum < 1 || slotNum > maxSlot {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("slot%d out of range: layout '%s' has %d content placeholders (slot ignored)",
					slotNum, layout.Name, maxSlot))
			continue
		}

		slotContent := slots[slotNum]
		placeholder := contentPlaceholders[slotNum-1] // slot1 -> index 0

		items, err := routeTypesSlotContent(slotContent, placeholder)
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("slot%d: %v", slotNum, err))
			continue
		}

		result.ContentItems = append(result.ContentItems, items...)
	}

	return result, nil
}

// ContentTable is a new content type for table data.
const ContentTable ContentType = "table"

// PopulateTableInShape generates table XML and prepares it for placeholder replacement.
// This function integrates the table generator with the existing content population flow.
func PopulateTableInShape(
	table *types.TableSpec,
	placeholder types.PlaceholderInfo,
	theme *types.ThemeInfo,
) (*TableRenderResult, error) {
	if table == nil {
		return nil, fmt.Errorf("table spec is nil")
	}

	// Build render config from placeholder bounds
	config := TableRenderConfig{
		Bounds: types.BoundingBox{
			X:      placeholder.Bounds.X,
			Y:      placeholder.Bounds.Y,
			Width:  placeholder.Bounds.Width,
			Height: placeholder.Bounds.Height,
		},
		Theme:            theme,
		Style:            table.Style,
		DefaultFont:      placeholder.FontFamily,
		DefaultSize:      placeholder.FontSize,
		ColumnAlignments: table.ColumnAlignments,
	}

	// Generate table XML
	return GenerateTableXML(table, config)
}
