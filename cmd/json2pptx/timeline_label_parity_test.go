package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

// TestTimelineShortenedLabelValidateRenderParity pins go-slide-creator-ze6qs:
// a timeline label that cannot be drawn in full is a blocking finding on the
// item's own path, and validate (dry render) reports exactly what the render
// pass reports.
func TestTimelineShortenedLabelValidateRenderParity(t *testing.T) {
	var items []map[string]any
	for day := 1; day <= 16; day++ {
		items = append(items, map[string]any{
			"date":  fmt.Sprintf("2026-01-%02d", day),
			"title": fmt.Sprintf("Unbreakablemilestonelabelnumber%02d", day),
			"type":  "milestone",
		})
	}
	deckJSON := minimalDeck(
		map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Sixteen labels a day apart cannot all be shown"},
		map[string]any{"placeholder_id": "body", "type": "diagram", "diagram_value": map[string]any{
			"type": "timeline",
			"alt":  "Sixteen milestones in January 2026",
			"data": map[string]any{"items": items},
		}},
	)
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(deckJSON), &input); err != nil {
		t.Fatalf("unmarshal deck: %v", err)
	}
	applyDefaults(&input)

	layouts, w, h, synthetic, metadata, palette := renderEvidenceFixture(t)
	templatePath := filepath.Join("..", "..", "templates", "midnight-blue.pptx")
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	theme := template.ParseTheme(reader)
	_ = reader.Close()

	// Both passes must name the same labels. The paths differ by design:
	// validate addresses the item, the render pass keeps every renderer
	// finding on the authored content item.
	shortened := func(findings []patterns.FitFinding, pathPrefix string) []string {
		var messages []string
		for _, f := range findings {
			if f.Code != "diagram.label_truncated" {
				continue
			}
			if f.Action != "shrink_or_split" {
				t.Errorf("%s: action = %q, want shrink_or_split (a shortened label must block a clean render)", f.Path, f.Action)
			}
			if !strings.HasPrefix(f.Path, pathPrefix) {
				t.Errorf("path = %q, want prefix %q", f.Path, pathPrefix)
			}
			if f.Fix == nil {
				t.Fatalf("%s: finding carries no fix", f.Path)
			}
			messages = append(messages, fmt.Sprint(f.Fix.Params["original"]))
		}
		slices.Sort(messages)
		return messages
	}

	validated := shortened(collectChartDryRenderFindingsInFrames(&input, theme.Colors, theme.BodyFont, "warn", layouts, w, h),
		"/slides/0/content/1/diagram_value/data/items/")
	if len(validated) == 0 {
		t.Fatal("validate reports no shortened timeline label")
	}

	mc := repairMC(t)
	renderFindings, evidence := mc.collectRenderFindings(context.Background(), &input, templatePath, layouts, w, h, synthetic, metadata, palette)
	if !evidence.Complete {
		t.Fatalf("render incomplete: stage=%q detail=%q", evidence.Stage, evidence.Detail)
	}
	rendered := shortened(renderFindings, "/slides/0/content/1")
	if !slices.Equal(validated, rendered) {
		t.Errorf("validate and render disagree on the shortened labels:\n validate: %v\n render:   %v", validated, rendered)
	}
}
