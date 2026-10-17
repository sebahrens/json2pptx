package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestPortableNativeReviewLayoutsAcrossTemplates(t *testing.T) {
	for _, name := range testutil.AllTestTemplateNames() {
		t.Run(name, func(t *testing.T) {
			layouts, w, h := schemaMaximaLayouts(t, name)
			for _, requested := range []string{"two-column-60-40", "two-column-40-60", "two-column-70-30", "two-column-30-70", "sidebar-left", "sidebar-right", "stacked", "stacked-60-40", "stacked-40-60"} {
				t.Run(requested, func(t *testing.T) {
					slides := []SlideInput{{LayoutID: requested}}
					resolveCanonicalLayoutIDs(slides, layouts)
					var base *types.LayoutMetadata
					for i := range layouts {
						if layouts[i].ID == slides[0].LayoutID {
							base = &layouts[i]
							break
						}
					}
					if base == nil {
						t.Fatalf("%s resolved to absent layout %s", requested, slides[0].LayoutID)
					}
					derived, err := deriveLayoutMetadata(slides[0], *base, layouts, w, h)
					if err != nil {
						t.Fatal(err)
					}
					boxes := map[string]types.BoundingBox{}
					for _, ph := range derived.Placeholders {
						boxes[ph.ID] = ph.Bounds
					}
					a, b := boxes["body"], boxes["body_2"]
					if slides[0].StackedSplit {
						if b.Y-a.Y-a.Height < layout.MinTwoColumnGutterEMU || a.X != b.X || a.Width != b.Width {
							t.Fatalf("bad stacked geometry %v %v", a, b)
						}
					} else {
						if b.X-a.X-a.Width < layout.MinTwoColumnGutterEMU {
							t.Fatalf("gutter too small %v %v", a, b)
						}
					}
				})
			}
			for _, alias := range []string{"three-column", "3-col", "four-column", "4-col", "five-column"} {
				id, ok := layout.ResolveCanonicalLayoutID(alias, layouts)
				if ok {
					found := false
					for _, candidate := range layouts {
						if candidate.ID == id {
							found = true
						}
					}
					if !found {
						t.Fatalf("%s resolved to absent %s", alias, id)
					}
				} else if layout.UnsupportedNativeAliasHint(alias) == "" {
					t.Fatalf("missing alternatives for %s", alias)
				}
			}
			for _, side := range []string{"left", "right"} {
				requested := "image-" + side
				id, ok := layout.ResolveCanonicalLayoutID(requested, layouts)
				if !ok {
					t.Fatalf("portable %s unavailable", requested)
				}
				if layout.ImageColumnFallback(requested, layouts) != "" {
					slides := []SlideInput{{LayoutID: requested, SlideType: "image", Content: []ContentInput{{PlaceholderID: "body", Type: "image"}, {PlaceholderID: "body_2", Type: "text"}}}}
					resolveCanonicalLayoutIDs(slides, layouts)
					if slides[0].ImageColumnSide != side || slides[0].ColumnLeftPercent != 50 {
						t.Fatalf("lost image side: %+v (%s)", slides[0], id)
					}
					wantImage := "body"
					if side == "right" {
						wantImage = "body_2"
					}
					if slides[0].Content[0].PlaceholderID != wantImage {
						t.Fatalf("image on wrong side: %+v", slides[0].Content)
					}
				}
			}
		})
	}
}

func TestQuadAndTilesRegionsReadableAcrossTemplates(t *testing.T) {
	for _, name := range testutil.AllTestTemplateNames() {
		t.Run(name, func(t *testing.T) {
			layouts, w, h := schemaMaximaLayouts(t, name)
			for _, arrangement := range []string{"quad", "tiles_over_panels"} {
				t.Run(arrangement, func(t *testing.T) {
					chart := map[string]any{"kind": "chart", "chart": map[string]any{"type": "pie_chart", "data": map[string]any{"categories": []any{"A", "B"}, "values": []any{60, 40}}}}
					text := map[string]any{"kind": "text", "body": "One owner per decision."}
					regions := []any{chart, map[string]any{"kind": "stat", "value": "32%", "label": "Margin"}, map[string]any{"kind": "table", "headers": []any{"Team", "Owner"}, "rows": []any{[]any{"Pilot", "Finance"}}}, text}
					if arrangement == "tiles_over_panels" {
						regions = []any{map[string]any{"kind": "kpis", "kpis": []any{map[string]any{"value": "32%", "label": "Margin"}, map[string]any{"value": "12", "label": "Owners"}}}, chart, text}
					}
					spec := &semantic.DeckSpec{Meta: semantic.DeckMeta{Title: "Review", Template: name, Source: "Finance ledger"}, Slides: []semantic.SlideSpec{{Kind: semantic.KindRegions, Body: map[string]any{"title": "Data supports an accountable decision", "takeaway": "Every decision has one owner.", "arrangement": arrangement, "regions": regions, "gap": 8, "divider": true}}}}
					input, result, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessStrict})
					if err != nil {
						t.Fatalf("compile: %v %+v", err, result.Diagnostics)
					}
					for _, finding := range collectFitFindings(input, layouts, w, h, nil) {
						if strings.Contains(finding.Code, "TEXT_BELOW_READABLE_MIN") {
							t.Errorf("unreadable %s: %s", finding.Path, finding.Message)
						}
					}
				})
			}
		})
	}
}
