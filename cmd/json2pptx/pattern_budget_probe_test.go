package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

type budgetProbeGeometry struct {
	name          string
	layouts       []types.LayoutMetadata
	width, height int64
}

func budgetProbeCopy(length int) string {
	return strings.Repeat("word ", length/5) + strings.Repeat("w", length%5)
}

// probeReadableBudget runs a pattern payload through the same collector used
// by fit reports on every bundled template, then finds the largest clean
// character count. Pattern-specific probes only need to build their payload.
func probeReadableBudget(t *testing.T, pattern string, maxChars int, payload func(int) any, overrides ...any) int {
	t.Helper()
	var encodedOverrides json.RawMessage
	if len(overrides) > 0 {
		var err error
		encodedOverrides, err = json.Marshal(overrides[0])
		if err != nil {
			t.Fatal(err)
		}
	}
	geometries := make([]budgetProbeGeometry, 0, len(schemaMaximaTemplates))
	for _, name := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, name)
		geometries = append(geometries, budgetProbeGeometry{name, layouts, width, height})
	}
	clean := func(length int) bool {
		encoded, err := json.Marshal(payload(length))
		if err != nil {
			t.Fatal(err)
		}
		for _, geom := range geometries {
			input := &PresentationInput{Template: geom.name, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: pattern, Values: encoded, Overrides: encodedOverrides},
			}}}
			if len(collectReadabilityFindings(input, geom.layouts, geom.width, geom.height)) > 0 {
				return false
			}
		}
		return true
	}
	lo, hi := 0, maxChars
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if clean(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// Run with JSON2PPTX_HORIZONTAL_BAR_BUDGET_PROBE=1 to measure row-count and
// callout-column effects against all bundled templates.
func TestHorizontalBarBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_HORIZONTAL_BAR_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_HORIZONTAL_BAR_BUDGET_PROBE=1")
	}
	pat, _ := patterns.Default().Get("horizontal-bar-with-callouts")
	maximum, note := schemaMaximumValues(pat)
	if note != "" {
		t.Fatal(note)
	}
	maximumJSON, err := json.Marshal(maximum)
	if err != nil {
		t.Fatal(err)
	}
	for _, templateName := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, templateName)
		deck := &PresentationInput{Template: templateName, Slides: []SlideInput{{
			SlideType: "content", LayoutID: "blank-title",
			Pattern: &PatternInput{Name: pat.Name(), Values: maximumJSON},
		}}}
		for _, finding := range collectReadabilityFindings(deck, layouts, width, height) {
			t.Logf("schema maximum on %s: %s", templateName, finding.Message)
		}
		withoutCallouts := *maximum.(*patterns.HorizontalBarCalloutsValues)
		withoutCallouts.Bars = append([]patterns.HorizontalBarCalloutsBar(nil), withoutCallouts.Bars...)
		for i := range withoutCallouts.Bars {
			withoutCallouts.Bars[i].Callout = ""
		}
		noCalloutJSON, err := json.Marshal(&withoutCallouts)
		if err != nil {
			t.Fatal(err)
		}
		deck.Slides[0].Pattern.Values = noCalloutJSON
		t.Logf("schema maximum without callouts on %s: readability findings=%d", templateName,
			len(collectReadabilityFindings(deck, layouts, width, height)))
	}
	for _, bars := range []int{3, 4, 5, 6, 7, 8} {
		for _, withCallouts := range []bool{false, true} {
			for _, field := range []string{"label", "unit", "callout"} {
				if field == "callout" && !withCallouts {
					continue
				}
				maxChars := map[string]int{"label": 40, "unit": 8, "callout": 200}[field]
				budget := probeReadableBudget(t, "horizontal-bar-with-callouts", maxChars, func(length int) any {
					values := &patterns.HorizontalBarCalloutsValues{MaxValue: 100, Unit: "%"}
					for i := 0; i < bars; i++ {
						bar := patterns.HorizontalBarCalloutsBar{Label: "Item", Value: 80}
						if withCallouts {
							bar.Callout = "Insight"
						}
						values.Bars = append(values.Bars, bar)
					}
					switch field {
					case "label":
						values.Bars[0].Label = budgetProbeCopy(length)
					case "unit":
						values.Unit = strings.Repeat("M", length)
					case "callout":
						values.Bars[0].Callout = budgetProbeCopy(length)
					}
					return values
				})
				t.Logf("bars=%d callouts=%t field=%s budget=%d", bars, withCallouts, field, budget)
			}
		}
	}
}

// Run with JSON2PPTX_AGENDA_IMAGES_BUDGET_PROBE=1 to measure each agenda row
// field by row count and whether image and subtitle content are present.
func TestAgendaWithImagesBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_AGENDA_IMAGES_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_AGENDA_IMAGES_BUDGET_PROBE=1")
	}
	for rows := 3; rows <= 6; rows++ {
		for _, images := range []bool{false, true} {
			for _, subtitles := range []bool{false, true} {
				for _, field := range []string{"title", "subtitle", "image_label"} {
					if field == "image_label" && !images {
						continue
					}
					maxChars := map[string]int{"title": 80, "subtitle": 160, "image_label": 60}[field]
					budget := probeReadableBudget(t, "agenda-with-images", maxChars, func(length int) any {
						v := &patterns.AgendaWithImagesValues{}
						for i := 0; i < rows; i++ {
							item := patterns.AgendaWithImagesItem{Title: "Title"}
							if subtitles {
								item.Subtitle = "Summary"
							}
							if images {
								item.ImageLabel = "Image"
							}
							v.Items = append(v.Items, item)
						}
						copy := budgetProbeCopy(length)
						switch field {
						case "title":
							v.Items[0].Title = copy
						case "subtitle":
							v.Items[0].Subtitle = copy
						case "image_label":
							v.Items[0].ImageLabel = copy
						}
						return v
					})
					t.Logf("rows=%d images=%t subtitles=%t field=%s budget=%d", rows, images, subtitles, field, budget)
				}
			}
		}
	}
	for _, rows := range []int{5, 6} {
		for _, subtitleChars := range []int{0, 40, 80, 120, 150} {
			budget := probeReadableBudget(t, "agenda-with-images", 80, func(length int) any {
				v := &patterns.AgendaWithImagesValues{}
				for i := 0; i < rows; i++ {
					v.Items = append(v.Items, patterns.AgendaWithImagesItem{Title: "Title", Subtitle: "Summary", ImageLabel: "Image"})
				}
				v.Items[0].Title = budgetProbeCopy(length)
				v.Items[0].Subtitle = budgetProbeCopy(subtitleChars)
				return v
			})
			t.Logf("rows=%d images=true subtitle_chars=%d title_budget=%d", rows, subtitleChars, budget)
		}
		for _, titleChars := range []int{5, 40, 65, 80} {
			budget := probeReadableBudget(t, "agenda-with-images", 160, func(length int) any {
				v := &patterns.AgendaWithImagesValues{}
				for i := 0; i < rows; i++ {
					v.Items = append(v.Items, patterns.AgendaWithImagesItem{Title: "Title", Subtitle: "Summary", ImageLabel: "Image"})
				}
				v.Items[0].Title = budgetProbeCopy(titleChars)
				v.Items[0].Subtitle = budgetProbeCopy(length)
				return v
			})
			t.Logf("rows=%d images=true title_chars=%d subtitle_budget=%d", rows, titleChars, budget)
		}
	}
	for rows := 3; rows <= 6; rows++ {
		for _, images := range []bool{false, true} {
			for _, field := range []string{"title", "subtitle"} {
				limit := map[string]int{"title": 80, "subtitle": 160}[field]
				budget := probeReadableBudget(t, "agenda-with-images", limit, func(length int) any {
					v := &patterns.AgendaWithImagesValues{}
					for i := 0; i < rows; i++ {
						item := patterns.AgendaWithImagesItem{Title: "Title", Subtitle: "Summary"}
						if images {
							item.ImageLabel = "Image"
						}
						v.Items = append(v.Items, item)
					}
					if field == "title" {
						v.Items[0].Title = budgetProbeCopy(length)
						v.Items[0].Subtitle = budgetProbeCopy(160)
					} else {
						v.Items[0].Title = budgetProbeCopy(80)
						v.Items[0].Subtitle = budgetProbeCopy(length)
					}
					return v
				})
				t.Logf("rows=%d images=%t other_max=true field=%s budget=%d", rows, images, field, budget)
			}
		}
	}
}

// Run with JSON2PPTX_BEFORE_AFTER_COMPACT_BUDGET_PROBE=1 to measure the
// height-capped body's per-bullet target as item count increases.
func TestBeforeAfterCompactBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_BEFORE_AFTER_COMPACT_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_BEFORE_AFTER_COMPACT_BUDGET_PROBE=1")
	}
	for items := 1; items <= 8; items++ {
		for _, all := range []bool{false, true} {
			budget := probeReadableBudget(t, "before-after-compact", 200, func(length int) any {
				v := &patterns.BeforeAfterValues{
					Before: patterns.BeforeAfterColumn{Header: "Current"},
					After:  patterns.BeforeAfterColumn{Header: "Future"},
				}
				for i := 0; i < items; i++ {
					v.Before.Items = append(v.Before.Items, "Short")
					v.After.Items = append(v.After.Items, "Short")
				}
				if all {
					for i := range v.Before.Items {
						v.Before.Items[i] = budgetProbeCopy(length)
						v.After.Items[i] = budgetProbeCopy(length)
					}
				} else {
					v.Before.Items[0] = budgetProbeCopy(length)
				}
				return v
			})
			t.Logf("items=%d all=%t per_bullet_budget=%d", items, all, budget)
		}
	}
	for items := 5; items <= 8; items++ {
		for longItems := 1; longItems <= items; longItems++ {
			budget := probeReadableBudget(t, "before-after-compact", 200, func(length int) any {
				v := &patterns.BeforeAfterValues{
					Before: patterns.BeforeAfterColumn{Header: "Current"},
					After:  patterns.BeforeAfterColumn{Header: "Future"},
				}
				for i := 0; i < items; i++ {
					item := "Short"
					if i < longItems-1 {
						item = budgetProbeCopy(200)
					}
					v.Before.Items = append(v.Before.Items, item)
					v.After.Items = append(v.After.Items, "Short")
				}
				v.Before.Items[longItems-1] = budgetProbeCopy(length)
				return v
			})
			t.Logf("items=%d long_items=%d last_long_budget=%d", items, longItems, budget)
		}
	}
	for items := 1; items <= 8; items++ {
		budget := probeReadableBudget(t, "before-after-compact", 200, func(length int) any {
			v := &patterns.BeforeAfterValues{
				Before: patterns.BeforeAfterColumn{Header: budgetProbeCopy(60)},
				After:  patterns.BeforeAfterColumn{Header: budgetProbeCopy(60)},
			}
			for i := 0; i < items; i++ {
				v.Before.Items = append(v.Before.Items, budgetProbeCopy(length))
				v.After.Items = append(v.After.Items, budgetProbeCopy(length))
			}
			return v
		})
		t.Logf("items=%d max_headers=true all=true per_bullet_budget=%d", items, budget)
	}
	for items := 5; items <= 8; items++ {
		for longItems := 1; longItems <= 3; longItems++ {
			budget := probeReadableBudget(t, "before-after-compact", 200, func(length int) any {
				v := &patterns.BeforeAfterValues{
					Before: patterns.BeforeAfterColumn{Header: budgetProbeCopy(60)},
					After:  patterns.BeforeAfterColumn{Header: budgetProbeCopy(60)},
				}
				for i := 0; i < items; i++ {
					item := "Short"
					if i < longItems-1 {
						item = budgetProbeCopy(200)
					}
					v.Before.Items = append(v.Before.Items, item)
					v.After.Items = append(v.After.Items, "Short")
				}
				v.Before.Items[longItems-1] = budgetProbeCopy(length)
				return v
			})
			t.Logf("items=%d max_headers=true long_items=%d last_long_budget=%d", items, longItems, budget)
		}
	}
	for _, slots := range []int{10, 11, 12} {
		budget := probeReadableBudget(t, "before-after-compact", 60, func(length int) any {
			v := &patterns.BeforeAfterValues{
				Before: patterns.BeforeAfterColumn{Header: budgetProbeCopy(length)},
				After:  patterns.BeforeAfterColumn{Header: budgetProbeCopy(length)},
			}
			for i := 0; i < 8; i++ {
				v.Before.Items = append(v.Before.Items, "Short")
				v.After.Items = append(v.After.Items, "Short")
			}
			v.Before.Items[0] = budgetProbeCopy(200)
			if slots == 11 {
				v.Before.Items[1] = budgetProbeCopy(133)
			} else if slots == 12 {
				v.Before.Items[1] = budgetProbeCopy(200)
			}
			return v
		})
		t.Logf("body_slots=%d header_budget=%d", slots, budget)
	}
}

// Run with JSON2PPTX_PHASE_ROADMAP_BUDGET_PROBE=1 to measure per-phase copy
// against all bundled templates, including the optional milestone row.
func TestPhaseRoadmapBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_PHASE_ROADMAP_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_PHASE_ROADMAP_BUDGET_PROBE=1")
	}
	for phases := 3; phases <= 6; phases++ {
		for _, milestones := range []bool{false, true} {
			for _, field := range []string{"name", "date_label", "description", "milestone"} {
				if field == "milestone" && !milestones {
					continue
				}
				limit := map[string]int{"name": 40, "date_label": 30, "description": 160, "milestone": 60}[field]
				budget := probeReadableBudget(t, "phase-roadmap", limit, func(length int) any {
					v := &patterns.PhaseRoadmapValues{}
					for i := 0; i < phases; i++ {
						phase := patterns.PhaseRoadmapPhase{Name: "Plan", DateLabel: "Q1", Description: "Brief"}
						if milestones {
							phase.Milestone = "Gate"
						}
						v.Phases = append(v.Phases, phase)
					}
					copy := budgetProbeCopy(length)
					switch field {
					case "name":
						v.Phases[0].Name = copy
					case "date_label":
						v.Phases[0].DateLabel = copy
					case "description":
						v.Phases[0].Description = copy
					case "milestone":
						v.Phases[0].Milestone = copy
					}
					return v
				})
				t.Logf("phases=%d milestones=%t field=%s budget=%d", phases, milestones, field, budget)
			}
		}
	}
}

// Run with JSON2PPTX_TIMELINE_BUDGET_PROBE=1 to measure each timeline style
// against all bundled templates as stop count grows.
func TestTimelineHorizontalBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_TIMELINE_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_TIMELINE_BUDGET_PROBE=1")
	}
	for _, style := range []string{"dots", "chevron", "gantt"} {
		for stops := 3; stops <= 7; stops++ {
			fields := []string{"label", "date", "body"}
			if style == "gantt" {
				fields = []string{"label", "date", "end_date"}
			}
			for _, field := range fields {
				limit := map[string]int{"label": 60, "date": 30, "end_date": 30, "body": 200}[field]
				budget := probeReadableBudget(t, "timeline-horizontal", limit, func(length int) any {
					v := patterns.TimelineHorizontalValues{}
					for i := 0; i < stops; i++ {
						stop := patterns.TimelineStop{Label: "Launch", Date: "Q1", Body: "Brief"}
						if style == "gantt" {
							stop.EndDate = "Q2"
						}
						v = append(v, stop)
					}
					copy := budgetProbeCopy(length)
					switch field {
					case "label":
						v[0].Label = copy
					case "date":
						v[0].Date = copy
					case "end_date":
						v[0].EndDate = copy
					case "body":
						v[0].Body = copy
					}
					return &v
				}, patterns.TimelineHorizontalOverrides{Style: style})
				t.Logf("style=%s stops=%d field=%s budget=%d", style, stops, field, budget)
			}
		}
	}
	for _, style := range []string{"dots", "chevron"} {
		for stops := 3; stops <= 7; stops++ {
			for _, field := range []string{"label", "body"} {
				limit := map[string]int{"label": 60, "body": 200}[field]
				budget := probeReadableBudget(t, "timeline-horizontal", limit, func(length int) any {
					v := patterns.TimelineHorizontalValues{}
					for i := 0; i < stops; i++ {
						v = append(v, patterns.TimelineStop{Label: "Launch", Date: "Q1", Body: "Brief"})
					}
					if field == "label" {
						v[0].Label = budgetProbeCopy(length)
						v[0].Body = budgetProbeCopy(200)
					} else {
						v[0].Label = budgetProbeCopy(60)
						v[0].Body = budgetProbeCopy(length)
					}
					return &v
				}, patterns.TimelineHorizontalOverrides{Style: style})
				t.Logf("style=%s stops=%d paired_max=true field=%s budget=%d", style, stops, field, budget)
			}
		}
	}
	for stops := 3; stops <= 7; stops++ {
		budget := probeReadableBudget(t, "timeline-horizontal", 30, func(length int) any {
			v := patterns.TimelineHorizontalValues{}
			for i := 0; i < stops; i++ {
				v = append(v, patterns.TimelineStop{Label: "Launch", Date: "Q1", EndDate: "Q2"})
			}
			v[0].Label = budgetProbeCopy(60)
			v[0].Date = budgetProbeCopy(30)
			v[0].EndDate = budgetProbeCopy(length)
			return &v
		}, patterns.TimelineHorizontalOverrides{Style: "gantt"})
		t.Logf("style=gantt stops=%d paired_dates=true end_date_budget=%d", stops, budget)
	}
	for _, style := range []string{"dots", "chevron"} {
		for stops := 3; stops <= 7; stops++ {
			for _, labelChars := range []int{5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 60} {
				budget := probeReadableBudget(t, "timeline-horizontal", 200, func(length int) any {
					v := patterns.TimelineHorizontalValues{}
					for i := 0; i < stops; i++ {
						v = append(v, patterns.TimelineStop{Label: "Launch", Date: "Q1", Body: "Brief"})
					}
					v[0].Label = budgetProbeCopy(labelChars)
					v[0].Body = budgetProbeCopy(length)
					return &v
				}, patterns.TimelineHorizontalOverrides{Style: style})
				t.Logf("style=%s stops=%d label_chars=%d body_budget=%d", style, stops, labelChars, budget)
			}
		}
	}
}

// Run with JSON2PPTX_STYLISH_PANELS_BUDGET_PROBE=1 to measure the ribbon title
// and shared bullet-list body budget by panel and bullet count.
func TestStylishPanelsBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_STYLISH_PANELS_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_STYLISH_PANELS_BUDGET_PROBE=1")
	}
	for panels := 3; panels <= 5; panels++ {
		titleBudget := probeReadableBudget(t, "stylish-panels", 80, func(length int) any {
			v := patterns.StylishPanelsValues{}
			for i := 0; i < panels; i++ {
				v = append(v, patterns.StylishPanelsItem{Title: "Panel", Body: []string{"Short"}})
			}
			v[0].Title = budgetProbeCopy(length)
			return &v
		})
		t.Logf("panels=%d field=title budget=%d", panels, titleBudget)
		for bullets := 1; bullets <= 8; bullets++ {
			for _, all := range []bool{false, true} {
				bodyBudget := probeReadableBudget(t, "stylish-panels", 200, func(length int) any {
					v := patterns.StylishPanelsValues{}
					for i := 0; i < panels; i++ {
						item := patterns.StylishPanelsItem{Title: "Panel"}
						for j := 0; j < bullets; j++ {
							item.Body = append(item.Body, "Short")
						}
						v = append(v, item)
					}
					if all {
						for i := range v[0].Body {
							v[0].Body[i] = budgetProbeCopy(length)
						}
					} else {
						v[0].Body[0] = budgetProbeCopy(length)
					}
					return &v
				})
				t.Logf("panels=%d bullets=%d all=%t body_budget=%d", panels, bullets, all, bodyBudget)
			}
		}
	}
	for panels := 3; panels <= 5; panels++ {
		for bullets := 1; bullets <= 8; bullets++ {
			bodyBudget := probeReadableBudget(t, "stylish-panels", 200, func(length int) any {
				v := patterns.StylishPanelsValues{}
				for i := 0; i < panels; i++ {
					item := patterns.StylishPanelsItem{Title: budgetProbeCopy(80)}
					for j := 0; j < bullets; j++ {
						item.Body = append(item.Body, budgetProbeCopy(length))
					}
					v = append(v, item)
				}
				return &v
			})
			t.Logf("panels=%d bullets=%d max_titles=true all=true body_budget=%d", panels, bullets, bodyBudget)
		}
	}
	for _, tc := range []struct{ panels, bullets, bodyChars int }{
		{3, 6, 121}, {4, 4, 120}, {5, 8, 42},
	} {
		titleBudget := probeReadableBudget(t, "stylish-panels", 80, func(length int) any {
			v := patterns.StylishPanelsValues{}
			for i := 0; i < tc.panels; i++ {
				item := patterns.StylishPanelsItem{Title: "Panel"}
				for j := 0; j < tc.bullets; j++ {
					item.Body = append(item.Body, budgetProbeCopy(tc.bodyChars))
				}
				v = append(v, item)
			}
			v[0].Title = budgetProbeCopy(length)
			return &v
		})
		t.Logf("panels=%d bullets=%d body_chars=%d title_budget=%d", tc.panels, tc.bullets, tc.bodyChars, titleBudget)
	}
	for panels := 3; panels <= 5; panels++ {
		for bullets := 1; bullets <= 8; bullets++ {
			for _, titleChars := range []int{5, 25, 45, 65, 80} {
				bodyBudget := probeReadableBudget(t, "stylish-panels", 200, func(length int) any {
					v := patterns.StylishPanelsValues{}
					for i := 0; i < panels; i++ {
						item := patterns.StylishPanelsItem{Title: budgetProbeCopy(titleChars)}
						for j := 0; j < bullets; j++ {
							item.Body = append(item.Body, budgetProbeCopy(length))
						}
						v = append(v, item)
					}
					return &v
				})
				t.Logf("panels=%d bullets=%d title_chars=%d body_budget=%d", panels, bullets, titleChars, bodyBudget)
			}
		}
	}
	for _, tc := range []struct{ panels, bullets, bodyChars int }{
		{3, 6, 121},
		{4, 3, 180}, {4, 4, 120}, {4, 6, 90}, {4, 8, 60},
		{5, 2, 182}, {5, 2, 162}, {5, 3, 122}, {5, 3, 102},
		{5, 4, 82}, {5, 6, 62}, {5, 8, 42}, {5, 8, 38},
	} {
		titleBudget := probeReadableBudget(t, "stylish-panels", 80, func(length int) any {
			v := patterns.StylishPanelsValues{}
			for i := 0; i < tc.panels; i++ {
				item := patterns.StylishPanelsItem{Title: "Panel"}
				for j := 0; j < tc.bullets; j++ {
					item.Body = append(item.Body, budgetProbeCopy(tc.bodyChars))
				}
				v = append(v, item)
			}
			v[0].Title = budgetProbeCopy(length)
			return &v
		})
		t.Logf("panels=%d bullets=%d body_chars=%d title_limit=%d", tc.panels, tc.bullets, tc.bodyChars, titleBudget)
	}
}

func TestStylishPanelsThresholdProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_STYLISH_PANELS_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_STYLISH_PANELS_BUDGET_PROBE=1")
	}
	for _, bullets := range []int{5, 7} {
		titleBudget := probeReadableBudget(t, "stylish-panels", 80, func(length int) any {
			v := patterns.StylishPanelsValues{}
			for i := 0; i < 5; i++ {
				item := patterns.StylishPanelsItem{Title: "Panel"}
				for j := 0; j < bullets; j++ {
					item.Body = append(item.Body, budgetProbeCopy(map[int]int{5: 62, 7: 42}[bullets]))
				}
				v = append(v, item)
			}
			v[0].Title = budgetProbeCopy(length)
			return &v
		})
		t.Logf("panels=5 bullets=%d body_target=%d title_limit=%d", bullets, map[int]int{5: 62, 7: 42}[bullets], titleBudget)
	}
}

func TestStylishPanelsSingleLongProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_STYLISH_PANELS_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_STYLISH_PANELS_BUDGET_PROBE=1")
	}
	for _, panels := range []int{3, 4, 5} {
		budget := probeReadableBudget(t, "stylish-panels", 200, func(length int) any {
			v := patterns.StylishPanelsValues{}
			for i := 0; i < panels; i++ {
				item := patterns.StylishPanelsItem{Title: budgetProbeCopy(80)}
				for j := 0; j < 8; j++ {
					item.Body = append(item.Body, "Short")
				}
				v = append(v, item)
			}
			v[0].Body[0] = budgetProbeCopy(length)
			return &v
		})
		t.Logf("panels=%d title_chars=80 bullets=8 single_long_budget=%d", panels, budget)
	}
}

// Run with JSON2PPTX_HERO_DETAIL_BUDGET_PROBE=1 to measure the hero and detail
// fields across card counts, styles, and icon presence.
func TestHeroDetailBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_HERO_DETAIL_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_HERO_DETAIL_BUDGET_PROBE=1")
	}
	for _, style := range []string{"cards", "minimal"} {
		for details := 2; details <= 4; details++ {
			for _, icon := range []bool{false, true} {
				for _, field := range []string{"hero.value", "hero.label", "hero.context", "detail.title", "detail.body"} {
					limit := map[string]int{"hero.value": 20, "hero.label": 80, "hero.context": 120, "detail.title": 60, "detail.body": 200}[field]
					budget := probeReadableBudget(t, "hero-detail", limit, func(length int) any {
						v := &patterns.HeroDetailValues{Hero: patterns.HeroDetailHero{Value: "$2.4B", Label: "Market size", Context: "Forecast"}}
						for i := 0; i < details; i++ {
							item := patterns.HeroDetailItem{Title: "Driver", Body: "Brief"}
							if icon {
								item.Icon = &patterns.IconRef{Name: "rocket"}
							}
							v.Details = append(v.Details, item)
						}
						copy := budgetProbeCopy(length)
						switch field {
						case "hero.value":
							v.Hero.Value = copy
						case "hero.label":
							v.Hero.Label = copy
						case "hero.context":
							v.Hero.Context = copy
						case "detail.title":
							v.Details[0].Title = copy
						case "detail.body":
							v.Details[0].Body = copy
						}
						return v
					}, patterns.HeroDetailOverrides{Style: style})
					t.Logf("style=%s details=%d icon=%t field=%s budget=%d", style, details, icon, field, budget)
				}
			}
		}
	}
	for _, titleChars := range []int{6, 20, 40, 60} {
		for _, maxHero := range []bool{false, true} {
			budget := probeReadableBudget(t, "hero-detail", 200, func(length int) any {
				v := &patterns.HeroDetailValues{Hero: patterns.HeroDetailHero{Value: "$2.4B", Label: "Market size", Context: "Forecast"}}
				if maxHero {
					v.Hero = patterns.HeroDetailHero{Value: budgetProbeCopy(20), Label: budgetProbeCopy(80), Context: budgetProbeCopy(120)}
				}
				for i := 0; i < 4; i++ {
					v.Details = append(v.Details, patterns.HeroDetailItem{Icon: &patterns.IconRef{Name: "rocket"}, Title: "Driver", Body: "Brief"})
				}
				v.Details[0].Title = budgetProbeCopy(titleChars)
				v.Details[0].Body = budgetProbeCopy(length)
				return v
			})
			t.Logf("details=4 icon=true title_chars=%d max_hero=%t body_budget=%d", titleChars, maxHero, budget)
		}
	}
	for _, bodyChars := range []int{90, 60, 30} {
		budget := probeReadableBudget(t, "hero-detail", 60, func(length int) any {
			v := &patterns.HeroDetailValues{Hero: patterns.HeroDetailHero{Value: "$2.4B", Label: "Market size"}}
			for i := 0; i < 4; i++ {
				v.Details = append(v.Details, patterns.HeroDetailItem{Icon: &patterns.IconRef{Name: "rocket"}, Title: "Driver", Body: "Brief"})
			}
			v.Details[0].Title = budgetProbeCopy(length)
			v.Details[0].Body = budgetProbeCopy(bodyChars)
			return v
		})
		t.Logf("details=4 icon=true body_chars=%d title_budget=%d", bodyChars, budget)
	}
	for details := 2; details <= 4; details++ {
		for _, icon := range []bool{false, true} {
			budget := probeReadableBudget(t, "hero-detail", 200, func(length int) any {
				v := &patterns.HeroDetailValues{Hero: patterns.HeroDetailHero{Value: "$2.4B", Label: "Market size"}}
				for i := 0; i < details; i++ {
					item := patterns.HeroDetailItem{Title: "Driver", Body: "Brief"}
					if icon {
						item.Icon = &patterns.IconRef{Name: "rocket"}
					}
					v.Details = append(v.Details, item)
				}
				v.Details[0].Title = budgetProbeCopy(60)
				v.Details[0].Body = budgetProbeCopy(length)
				return v
			})
			t.Logf("details=%d icon=%t title_chars=60 body_budget=%d", details, icon, budget)
		}
	}
	for _, titleChars := range []int{20, 40, 60} {
		budget := probeReadableBudget(t, "hero-detail", 200, func(length int) any {
			v := &patterns.HeroDetailValues{Hero: patterns.HeroDetailHero{Value: "$2.4B", Label: "Market size"}}
			for i := 0; i < 3; i++ {
				v.Details = append(v.Details, patterns.HeroDetailItem{Icon: &patterns.IconRef{Name: "rocket"}, Title: "Driver", Body: "Brief"})
			}
			v.Details[0].Title = budgetProbeCopy(titleChars)
			v.Details[0].Body = budgetProbeCopy(length)
			return v
		})
		t.Logf("details=3 icon=true title_chars=%d body_budget=%d", titleChars, budget)
	}
	titleBudget := probeReadableBudget(t, "hero-detail", 60, func(length int) any {
		v := &patterns.HeroDetailValues{Hero: patterns.HeroDetailHero{Value: "$2.4B", Label: "Market size"}}
		for i := 0; i < 3; i++ {
			v.Details = append(v.Details, patterns.HeroDetailItem{Icon: &patterns.IconRef{Name: "rocket"}, Title: "Driver", Body: "Brief"})
		}
		v.Details[0].Title = budgetProbeCopy(length)
		v.Details[0].Body = budgetProbeCopy(200)
		return v
	})
	t.Logf("details=3 icon=true body_chars=200 title_budget=%d", titleBudget)
	noIconBudget := probeReadableBudget(t, "hero-detail", 200, func(length int) any {
		v := &patterns.HeroDetailValues{Hero: patterns.HeroDetailHero{Value: "$2.4B", Label: "Market size"}}
		for i := 0; i < 4; i++ {
			item := patterns.HeroDetailItem{Icon: &patterns.IconRef{Name: "rocket"}, Title: "Driver", Body: "Brief"}
			if i == 0 {
				item.Icon = nil
				item.Title = budgetProbeCopy(60)
				item.Body = budgetProbeCopy(length)
			}
			v.Details = append(v.Details, item)
		}
		return v
	})
	t.Logf("details=4 target_icon=false other_icons=true title_chars=60 body_budget=%d", noIconBudget)
}

// Run with JSON2PPTX_BUDGET_PROBE=1 go test ./cmd/json2pptx
// -run TestPatternBudgetProbe -v. It measures the actual fit collector against
// every bundled template, without adding a slow combinatorial sweep to normal
// tests. A clean result (no below-floor finding) has budget >= the probe size.
func TestPatternBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_BUDGET_PROBE") != "1" {
		t.Skip("manual budget calibration probe")
	}
	for count := 1; count <= 10; count++ {
		bindingName, bindingBudget := "", 201
		for _, name := range bmcProbeCellNames() {
			budget := bmcReadableBudget(t, name, count)
			t.Logf("bmc-canvas %-21s %2d bullets: %3d chars per bullet", name, count, budget)
			if budget < bindingBudget {
				bindingName, bindingBudget = name, budget
			}
		}
		t.Logf("bmc-canvas %d bullets: binding cell %s at %d chars", count, bindingName, bindingBudget)
	}

	pat, _ := patterns.Default().Get("pull-quote")
	values, note := schemaMaximumValues(pat)
	if note != "" {
		t.Fatal(note)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, name)
		input := &PresentationInput{Template: name, Slides: []SlideInput{{
			SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: pat.Name(), Values: encoded},
		}}}
		for _, finding := range collectReadabilityFindings(input, layouts, width, height) {
			t.Logf("pull-quote %s: %s", name, finding.Message)
		}
	}
}

// JSON2PPTX_DRIVER_BUDGET_PROBE=1 measures the driver-tree copy target for
// each legal uniform tree shape. The shortest result across templates binds.
func TestDriverTreeBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_DRIVER_BUDGET_PROBE") != "1" {
		t.Skip("manual driver-tree budget calibration probe")
	}
	for branches := 2; branches <= 4; branches++ {
		for leaves := 1; leaves <= 4; leaves++ {
			counts := make([]int, branches)
			for i := range counts {
				counts[i] = leaves
			}
			for _, annotated := range []bool{false, true} {
				for _, field := range []string{"root", "branch", "leaf", "annotation"} {
					if field == "annotation" && !annotated {
						continue
					}
					budget := driverTreeReadableBudget(t, counts, annotated, field)
					t.Logf("driver-tree %d branches x %d leaves, annotated=%t, %s: %d chars", branches, leaves, annotated, field, budget)
				}
			}
		}
	}
	for _, total := range []int{5, 7, 10, 11, 13, 14, 15} {
		counts := []int{1, 1}
		if total > 8 {
			counts = []int{1, 1, 1}
		}
		if total > 12 {
			counts = []int{1, 1, 1, 1}
		}
		remaining := total - len(counts)
		for i := range counts {
			add := min(3, remaining)
			counts[i] += add
			remaining -= add
		}
		for _, annotated := range []bool{false, true} {
			for _, field := range []string{"root", "branch", "leaf", "annotation"} {
				if field == "annotation" && !annotated {
					continue
				}
				budget := driverTreeReadableBudget(t, counts, annotated, field)
				t.Logf("driver-tree %d leaves, annotated=%t, %s: %d chars", total, annotated, field, budget)
			}
		}
	}
}

func TestDriverTreeSpanProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_DRIVER_SPAN_PROBE") != "1" {
		t.Skip("manual driver-tree span calibration probe")
	}
	for total := 2; total <= 14; total++ {
		for span := 1; span <= 4; span++ {
			remaining := total - span
			if remaining < 1 || remaining > 12 {
				continue
			}
			var counts []int
			for remaining > 0 {
				n := min(4, remaining)
				counts = append(counts, n)
				remaining -= n
			}
			counts = append(counts, span)
			if len(counts) > 4 {
				continue
			}
			for _, annotated := range []bool{false, true} {
				branch := driverTreeReadableBudget(t, counts, annotated, "branch")
				if annotated {
					annotation := driverTreeReadableBudget(t, counts, true, "annotation")
					t.Logf("total=%d span=%d annotated=true branch=%d annotation=%d", total, span, branch, annotation)
				} else {
					t.Logf("total=%d span=%d annotated=false branch=%d", total, span, branch)
				}
			}
		}
	}
}

func TestComparisonBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_COMPARISON_BUDGET_PROBE") != "1" {
		t.Skip("manual comparison budget calibration probe")
	}
	for rows := 1; rows <= 10; rows++ {
		for _, headers := range []bool{false, true} {
			for _, field := range []string{"body", "header"} {
				if field == "header" && !headers {
					continue
				}
				budget := comparisonReadableBudget(t, rows, headers, field)
				t.Logf("comparison-2col rows=%d headers=%t field=%s budget=%d", rows, headers, field, budget)
			}
		}
	}
}

func TestRoadmapPhasedBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_ROADMAP_BUDGET_PROBE") != "1" {
		t.Skip("manual roadmap-phased budget calibration probe")
	}
	for phases := 2; phases <= 8; phases++ {
		for workstreams := 2; workstreams <= 6; workstreams++ {
			for _, field := range []string{"phase", "name", "item"} {
				budget := roadmapPhasedReadableBudget(t, phases, workstreams, field)
				t.Logf("roadmap-phased phases=%d workstreams=%d field=%s budget=%d", phases, workstreams, field, budget)
			}
		}
	}
}

func TestTeamBiosBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_TEAM_BIOS_BUDGET_PROBE") != "1" {
		t.Skip("manual team-bios budget calibration probe")
	}
	for members := 1; members <= 8; members++ {
		for _, field := range []string{"name", "role", "bio"} {
			budget := teamBiosReadableBudget(t, members, field)
			t.Logf("team-bios members=%d field=%s budget=%d", members, field, budget)
		}
	}
}

func TestSwimlaneBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_SWIMLANE_BUDGET_PROBE") != "1" {
		t.Skip("manual swimlane budget calibration probe")
	}
	for steps := 2; steps <= 8; steps++ {
		for lanes := 2; lanes <= 6; lanes++ {
			for _, field := range []string{"actor", "step"} {
				budget := swimlaneReadableBudget(t, steps, lanes, field)
				t.Logf("swimlane steps=%d lanes=%d field=%s budget=%d", steps, lanes, field, budget)
			}
		}
	}
}

func TestNumberedStepBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_NUMBERED_BUDGET_PROBE") != "1" {
		t.Skip("manual numbered-step budget calibration probe")
	}
	for _, style := range []string{"chevron", "stacked-box", "toc"} {
		for steps := 3; steps <= 6; steps++ {
			for _, bodies := range []bool{false, true} {
				for _, field := range []string{"label", "body"} {
					if field == "body" && !bodies {
						continue
					}
					budget := numberedStepReadableBudget(t, style, steps, bodies, field)
					t.Logf("numbered-step-strip style=%s steps=%d bodies=%t field=%s budget=%d", style, steps, bodies, field, budget)
				}
			}
		}
	}
}

func TestKPIInlineBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_KPI_INLINE_BUDGET_PROBE") != "1" {
		t.Skip("manual kpi-inline budget calibration probe")
	}
	pat, _ := patterns.Default().Get("kpi-inline")
	maximum, note := schemaMaximumValues(pat)
	if note != "" {
		t.Fatal(note)
	}
	encodedMaximum, err := json.Marshal(maximum)
	if err != nil {
		t.Fatal(err)
	}
	for _, templateName := range schemaMaximaTemplates {
		layouts, width, height := schemaMaximaLayouts(t, templateName)
		input := &PresentationInput{Template: templateName, Slides: []SlideInput{{SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "kpi-inline", Values: encodedMaximum}}}}
		for _, finding := range collectReadabilityFindings(input, layouts, width, height) {
			t.Logf("kpi-inline schema max %s: %s", templateName, finding.Message)
		}
	}
	for cells := 2; cells <= 6; cells++ {
		for _, sub := range []bool{false, true} {
			for _, field := range []string{"big", "small", "sub"} {
				if field == "sub" && !sub {
					continue
				}
				budget := kpiInlineReadableBudget(t, cells, sub, field)
				t.Logf("kpi-inline cells=%d sub=%t field=%s budget=%d", cells, sub, field, budget)
			}
		}
	}
	for cells := 5; cells <= 6; cells++ {
		for bigLen := 1; bigLen <= 8; bigLen++ {
			for subLen := 0; subLen <= 12; subLen++ {
				budget := kpiInlineCaptionFullBudget(t, cells, bigLen, subLen, true)
				t.Logf("kpi-inline dense icon cells=%d big=%d sub=%d budget=%d", cells, bigLen, subLen, budget)
			}
		}
	}
}

func kpiInlineCaptionFullBudget(t *testing.T, cells, bigLen, subLen int, icon bool) int {
	return probeReadableBudget(t, "kpi-inline", 40, func(length int) any {
		v := patterns.KPINupValues{}
		for i := 0; i < cells; i++ {
			v = append(v, patterns.KPICell{Big: "42%", Small: "Revenue"})
		}
		v[0].Big = strings.Repeat("8", bigLen)
		if subLen > 0 {
			v[0].Sub = strings.Repeat("+", subLen)
		}
		if icon {
			v[0].Icon = &patterns.IconRef{Name: "rocket"}
		}
		v[0].Small = budgetProbeCopy(length)
		return v
	})
}

func kpiInlineReadableBudget(t *testing.T, cells int, sub bool, field string) int {
	limit := map[string]int{"big": 8, "small": 40, "sub": 12}[field]
	return probeReadableBudget(t, "kpi-inline", limit, func(length int) any {
		v := patterns.KPINupValues{}
		for i := 0; i < cells; i++ {
			cell := patterns.KPICell{Big: "42%", Small: "Revenue"}
			if sub {
				cell.Sub = "+5%"
			}
			v = append(v, cell)
		}
		switch field {
		case "big":
			v[0].Big = budgetProbeCopy(length)
		case "small":
			v[0].Small = budgetProbeCopy(length)
		case "sub":
			v[0].Sub = budgetProbeCopy(length)
		}
		return v
	})
}
func numberedStepReadableBudget(t *testing.T, style string, steps int, bodies bool, field string) int {
	limit := 60
	if field == "body" {
		limit = 180
	}
	return probeReadableBudget(t, "numbered-step-strip", limit, func(length int) any {
		v := &patterns.NumberedStepStripValues{Style: style}
		for i := 0; i < steps; i++ {
			step := patterns.NumberedStepStripStep{Label: "Step"}
			if bodies {
				step.Body = "Detail"
			}
			v.Steps = append(v.Steps, step)
		}
		if field == "label" {
			v.Steps[0].Label = budgetProbeCopy(length)
		} else {
			v.Steps[0].Body = budgetProbeCopy(length)
		}
		return v
	})
}
func swimlaneReadableBudget(t *testing.T, steps, lanes int, field string) int {
	limit := 80
	if field == "actor" {
		limit = 40
	}
	return probeReadableBudget(t, "swimlane", limit, func(length int) any {
		v := &patterns.SwimlaneValues{}
		for i := 0; i < lanes; i++ {
			lane := patterns.SwimlaneLane{Actor: "Team"}
			for j := 0; j < steps; j++ {
				lane.Steps = append(lane.Steps, "Task")
			}
			v.Lanes = append(v.Lanes, lane)
		}
		if field == "actor" {
			v.Lanes[0].Actor = budgetProbeCopy(length)
		} else {
			v.Lanes[0].Steps[0] = budgetProbeCopy(length)
		}
		return v
	})
}
func teamBiosReadableBudget(t *testing.T, members int, field string) int {
	limit := map[string]int{"name": 60, "role": 80, "bio": 220}[field]
	return probeReadableBudget(t, "team-bios", limit, func(length int) any {
		v := &patterns.TeamBiosValues{}
		for i := 0; i < members; i++ {
			v.Members = append(v.Members, patterns.TeamBiosMember{Name: "Jane Doe", Role: "Lead", Bio: "Short biography"})
		}
		switch field {
		case "name":
			v.Members[0].Name = budgetProbeCopy(length)
		case "role":
			v.Members[0].Role = budgetProbeCopy(length)
		case "bio":
			v.Members[0].Bio = budgetProbeCopy(length)
		}
		return v
	})
}
func roadmapPhasedReadableBudget(t *testing.T, phases, workstreams int, field string) int {
	limit := map[string]int{"phase": 20, "name": 40, "item": 80}[field]
	return probeReadableBudget(t, "roadmap-phased", limit, func(length int) any {
		v := &patterns.RoadmapPhasedValues{}
		for i := 0; i < phases; i++ {
			v.Phases = append(v.Phases, "Q1")
		}
		for i := 0; i < workstreams; i++ {
			ws := patterns.RoadmapWorkstream{Name: "Team"}
			for j := 0; j < phases; j++ {
				ws.Items = append(ws.Items, "Task")
			}
			v.Workstreams = append(v.Workstreams, ws)
		}
		switch field {
		case "phase":
			v.Phases[0] = budgetProbeCopy(length)
		case "name":
			v.Workstreams[0].Name = budgetProbeCopy(length)
		case "item":
			v.Workstreams[0].Items[0] = budgetProbeCopy(length)
		}
		return v
	})
}
func comparisonReadableBudget(t *testing.T, rowCount int, headers bool, field string) int {
	limit := 200
	if field == "header" {
		limit = 60
	}
	return probeReadableBudget(t, "comparison-2col", limit, func(length int) any {
		v := &patterns.Comparison2colValues{}
		if headers {
			v.Headers = [2]string{"Left", "Right"}
		}
		for i := 0; i < rowCount; i++ {
			v.Rows = append(v.Rows, patterns.Comparison2colRow{Left: "Item", Right: "Other"})
		}
		if field == "header" {
			v.Headers[0] = budgetProbeCopy(length)
		} else {
			v.Rows[0].Left = budgetProbeCopy(length)
		}
		return v
	})
}
func driverTreeReadableBudget(t *testing.T, counts []int, annotated bool, field string) int {
	maxChars := map[string]int{"root": 60, "branch": 60, "leaf": 120, "annotation": 140}[field]
	return probeReadableBudget(t, "driver-tree", maxChars, func(length int) any {
		v := &patterns.DriverTreeValues{Root: patterns.DriverTreeNode{Label: "Root"}}
		for _, leaves := range counts {
			branch := patterns.DriverTreeBranch{Label: "Branch"}
			for j := 0; j < leaves; j++ {
				branch.Leaves = append(branch.Leaves, "Item")
			}
			if annotated {
				branch.Annotation = "Note"
			}
			v.Branches = append(v.Branches, branch)
		}
		switch field {
		case "root":
			v.Root.Label = budgetProbeCopy(length)
		case "branch":
			v.Branches[len(v.Branches)-1].Label = budgetProbeCopy(length)
		case "leaf":
			v.Branches[0].Leaves[0] = budgetProbeCopy(length)
		case "annotation":
			v.Branches[len(v.Branches)-1].Annotation = budgetProbeCopy(length)
		}
		return v
	})
}
func bmcProbeCellNames() []string {
	return []string{"key_partners", "key_activities", "key_resources", "value_propositions", "customer_relations", "channels", "customer_segments", "cost_structure", "revenue_streams"}
}

func bmcReadableBudget(t *testing.T, cellName string, bulletCount int) int {
	return probeReadableBudget(t, "bmc-canvas", 200, func(length int) any {
		values := bmcProbeValues()
		cell := bmcProbeCell(values, cellName)
		cell.Bullets = make([]string, bulletCount)
		for i := range cell.Bullets {
			cell.Bullets[i] = budgetProbeCopy(length)
		}
		return values
	})
}
func bmcProbeValues() *patterns.BMCCanvasValues {
	cell := func(header string) patterns.BMCCell {
		return patterns.BMCCell{Header: header, Bullets: []string{"base"}}
	}
	return &patterns.BMCCanvasValues{
		KeyPartners: cell("Key Partners"), KeyActivities: cell("Key Activities"),
		KeyResources: cell("Key Resources"), ValuePropositions: cell("Value Propositions"),
		CustomerRelations: cell("Customer Relations"), Channels: cell("Channels"),
		CustomerSegments: cell("Customer Segments"), CostStructure: cell("Cost Structure"),
		RevenueStreams: cell("Revenue Streams"),
	}
}

func bmcProbeCell(values *patterns.BMCCanvasValues, name string) *patterns.BMCCell {
	switch name {
	case "key_partners":
		return &values.KeyPartners
	case "key_activities":
		return &values.KeyActivities
	case "key_resources":
		return &values.KeyResources
	case "value_propositions":
		return &values.ValuePropositions
	case "customer_relations":
		return &values.CustomerRelations
	case "channels":
		return &values.Channels
	case "customer_segments":
		return &values.CustomerSegments
	case "cost_structure":
		return &values.CostStructure
	case "revenue_streams":
		return &values.RevenueStreams
	default:
		return nil
	}
}
