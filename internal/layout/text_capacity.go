package layout

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// mappedTextCapacity compares the text each placeholder will actually receive.
// Placeholder count remains a separate, stronger structural requirement.
func mappedTextCapacity(layout types.LayoutMetadata, slide types.SlideDefinition) float64 {
	score := 1.0
	consider := func(ph types.PlaceholderInfo, text string) {
		score = math.Min(score, placeholderTextCapacity(ph, text))
	}
	if slide.HasSlots() {
		var hosts []types.PlaceholderInfo
		for _, ph := range layout.Placeholders {
			if types.IsDisclosurePlaceholder(ph) {
				continue
			}
			switch ph.Type {
			case types.PlaceholderBody, types.PlaceholderContent, types.PlaceholderImage, types.PlaceholderChart, types.PlaceholderTable:
				hosts = append(hosts, ph)
			}
		}
		sortPlaceholdersByX(hosts)
		for number, slot := range slide.Slots {
			if slot == nil {
				continue
			}
			if slot.SlotNumber > 0 {
				number = slot.SlotNumber
			}
			if number <= 0 || number > len(hosts) {
				continue
			}
			parts := []string{slot.Text, slot.Body, strings.Join(slot.Bullets, "\n"), slot.BodyAfterBullets, slot.BodyAfterFence}
			for _, group := range slot.BulletGroups {
				parts = append(parts, group.Header, group.Body, strings.Join(group.Bullets, "\n"))
			}
			consider(hosts[number-1], strings.TrimSpace(strings.Join(parts, "\n")))
		}
		return score
	}
	c := slide.Content
	body := []string{c.Body, strings.Join(c.Bullets, "\n"), c.BodyAfterBullets}
	for _, group := range c.BulletGroups {
		body = append(body, group.Header, group.Body, strings.Join(group.Bullets, "\n"))
	}
	fields := map[string]string{"title": slide.Title, "body": c.Body, "bullets": strings.Join(body, "\n"), "body_and_bullets": strings.Join(body, "\n"), "left": strings.Join(c.Left, "\n"), "right": strings.Join(c.Right, "\n")}
	mappings, _ := buildMappings(layout, slide)
	for _, mapping := range mappings {
		for _, ph := range layout.Placeholders {
			if mapping.PlaceholderID == ph.ID || mapping.PlaceholderID == getUniquePlaceholderID(ph) {
				consider(ph, fields[mapping.ContentField])
				break
			}
		}
	}
	return score
}

func placeholderTextCapacity(ph types.PlaceholderInfo, text string) float64 {
	if strings.TrimSpace(text) == "" {
		return 1
	}
	ratio := 0.0
	if ph.Bounds.Width > 0 && ph.Bounds.Height > 0 {
		fontPt := float64(ph.FontSize) / 100
		if fontPt <= 0 {
			fontPt = 12
		}
		m, err := textfit.MeasureStyledRuns(textfit.StyledMeasureParams{Runs: []textfit.StyledRun{{Text: text}}, FontName: ph.FontFamily, FontPt: fontPt, WidthEMU: ph.Bounds.Width, InsetsPt: [4]float64{7.2, 3.6, 7.2, 3.6}})
		available := ph.Bounds.Height - 91440
		if err == nil && available > 0 {
			ratio = float64(m.RequiredEMU) / float64(available)
		}
	} else if ph.MaxChars > 0 {
		ratio = float64(utf8.RuneCountInString(text)) / float64(ph.MaxChars)
	}
	if ratio > 1 {
		return math.Max(capacityOverflowFloor, 1/ratio)
	}
	return 1
}
