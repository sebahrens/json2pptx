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

// Run with JSON2PPTX_TABLE_HIGHLIGHT_BUDGET_PROBE=1 to measure each matrix
// text field at sparse, medium and dense option/criterion counts.
func TestTableHighlightBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_TABLE_HIGHLIGHT_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_TABLE_HIGHLIGHT_BUDGET_PROBE=1")
	}
	for _, options := range []int{2, 4, 6} {
		for _, criteria := range []int{2, 4, 6} {
			for _, field := range []string{"name", "detail", "criterion", "text_score", "legend"} {
				limit := map[string]int{"name": 40, "detail": 60, "criterion": 30, "text_score": 24, "legend": 20}[field]
				budget := probeReadableBudget(t, "table-highlight", limit, func(length int) any {
					v := &patterns.TableHighlightValues{Scale: "harvey"}
					if field == "text_score" {
						v.Scale = "text"
					}
					for i := 0; i < criteria; i++ {
						v.Criteria = append(v.Criteria, patterns.TableHighlightCriterion{Label: "Quality"})
					}
					for i := 0; i < options; i++ {
						option := patterns.TableHighlightOption{Name: "Option", Detail: "Brief"}
						for j := 0; j < criteria; j++ {
							score := patterns.TableHighlightScore("3")
							if field == "text_score" {
								score = "Good"
							}
							option.Scores = append(option.Scores, score)
						}
						v.Options = append(v.Options, option)
					}
					v.LegendLabels = []string{"High", "Middle", "Low"}
					copy := budgetProbeCopy(length)
					switch field {
					case "name":
						v.Options[0].Name = copy
					case "detail":
						v.Options[0].Detail = copy
					case "criterion":
						v.Criteria[0].Label = copy
					case "text_score":
						v.Options[0].Scores[0] = patterns.TableHighlightScore(copy)
					case "legend":
						v.LegendLabels[0] = copy
					}
					return v
				})
				t.Logf("options=%d criteria=%d field=%s budget=%d", options, criteria, field, budget)
			}
		}
	}
	for _, options := range []int{4, 5, 6} {
		for _, criteria := range []int{2, 4, 6} {
			for _, tagChars := range []int{0, 24} {
				for _, field := range []string{"name", "detail"} {
					limit := map[string]int{"name": 40, "detail": 60}[field]
					budget := probeReadableBudget(t, "table-highlight", limit, func(length int) any {
						v := &patterns.TableHighlightValues{Scale: "harvey"}
						for i := 0; i < criteria; i++ {
							v.Criteria = append(v.Criteria, patterns.TableHighlightCriterion{Label: "Quality"})
						}
						for i := 0; i < options; i++ {
							option := patterns.TableHighlightOption{Name: "Option", Detail: "Brief"}
							for j := 0; j < criteria; j++ {
								option.Scores = append(option.Scores, "3")
							}
							v.Options = append(v.Options, option)
						}
						v.Options[0].Name = budgetProbeCopy(40)
						v.Options[0].Detail = budgetProbeCopy(60)
						if tagChars > 0 {
							zero := 0
							v.HighlightRow = &zero
							v.HighlightLabel = budgetProbeCopy(tagChars)
						}
						if field == "name" {
							v.Options[0].Name = budgetProbeCopy(length)
						} else {
							v.Options[0].Detail = budgetProbeCopy(length)
						}
						return v
					})
					t.Logf("options=%d criteria=%d tag_chars=%d paired_max=true field=%s budget=%d", options, criteria, tagChars, field, budget)
				}
			}
		}
	}
	for options := 2; options <= 6; options++ {
		for criteria := 2; criteria <= 6; criteria++ {
			for _, highlighted := range []bool{false, true} {
				budget := probeReadableBudget(t, "table-highlight", 40, func(length int) any {
					v := &patterns.TableHighlightValues{Scale: "harvey"}
					for i := 0; i < criteria; i++ {
						v.Criteria = append(v.Criteria, patterns.TableHighlightCriterion{Label: "Quality"})
					}
					for i := 0; i < options; i++ {
						option := patterns.TableHighlightOption{Name: budgetProbeCopy(length), Detail: budgetProbeCopy(length)}
						for j := 0; j < criteria; j++ {
							option.Scores = append(option.Scores, "3")
						}
						v.Options = append(v.Options, option)
					}
					if highlighted {
						zero := 0
						v.HighlightRow = &zero
						v.HighlightLabel = budgetProbeCopy(24)
					}
					return v
				})
				t.Logf("options=%d criteria=%d highlighted=%t paired_equal_budget=%d", options, criteria, highlighted, budget)
			}
		}
	}
}

func TestQuoteClusterBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_QUOTE_CLUSTER_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_QUOTE_CLUSTER_BUDGET_PROBE=1")
	}
	for count := 3; count <= 8; count++ {
		for _, field := range []string{"text", "name", "title", "all_text", "all_name", "all_title", "paired_text", "balanced", "single_paired_text", "single_balanced"} {
			maxChars := map[string]int{"text": 240, "name": 60, "title": 80, "all_text": 240, "all_name": 60, "all_title": 80, "paired_text": 240, "balanced": 240, "single_paired_text": 240, "single_balanced": 240}[field]
			budget := probeReadableBudget(t, "quote-cluster", maxChars, func(length int) any {
				v := &patterns.QuoteClusterValues{}
				for i := 0; i < count; i++ {
					v.Quotes = append(v.Quotes, patterns.QuoteClusterItem{Text: "A concise customer quote.", Name: "J. Lin", Title: "Director"})
				}
				copy := budgetProbeCopy(length)
				switch field {
				case "text":
					v.Quotes[0].Text = copy
				case "name":
					v.Quotes[0].Name = copy
				case "title":
					v.Quotes[0].Title = copy
				case "all_text":
					for i := range v.Quotes {
						v.Quotes[i].Text = copy
					}
				case "all_name":
					for i := range v.Quotes {
						v.Quotes[i].Name = copy
					}
				case "all_title":
					for i := range v.Quotes {
						v.Quotes[i].Title = copy
					}
				case "paired_text":
					for i := range v.Quotes {
						v.Quotes[i] = patterns.QuoteClusterItem{Text: copy, Name: budgetProbeCopy(60), Title: budgetProbeCopy(80)}
					}
				case "balanced":
					for i := range v.Quotes {
						v.Quotes[i] = patterns.QuoteClusterItem{Text: copy, Name: budgetProbeCopy(length / 4), Title: budgetProbeCopy(length / 3)}
					}
				case "single_paired_text":
					v.Quotes[0] = patterns.QuoteClusterItem{Text: copy, Name: budgetProbeCopy(60), Title: budgetProbeCopy(80)}
				case "single_balanced":
					v.Quotes[0] = patterns.QuoteClusterItem{Text: copy, Name: budgetProbeCopy(length / 4), Title: budgetProbeCopy(length / 3)}
				}
				return v
			})
			t.Logf("quotes=%d field=%s budget=%d", count, field, budget)
		}
	}
}

func TestDualOrgLadderBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_DUAL_ORG_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_DUAL_ORG_BUDGET_PROBE=1")
	}
	for count := 2; count <= 6; count++ {
		for _, field := range []string{"org", "name", "title", "paired_name", "paired_title", "balanced"} {
			maxChars := map[string]int{"org": 60, "name": 60, "title": 80, "paired_name": 60, "paired_title": 80, "balanced": 60}[field]
			budget := probeReadableBudget(t, "dual-org-ladder", maxChars, func(length int) any {
				v := &patterns.DualOrgLadderValues{OrgA: "Client", OrgB: "Partner"}
				for i := 0; i < count; i++ {
					v.Rows = append(v.Rows, patterns.DualOrgLadderRow{ANameField: "Alex Chen", ATitle: "Programme Lead", BNameField: "Bob Jones", BTitle: "Partner"})
				}
				copy := budgetProbeCopy(length)
				switch field {
				case "org":
					v.OrgA = copy
				case "name":
					v.Rows[0].ANameField = copy
				case "title":
					v.Rows[0].ATitle = copy
				case "paired_name":
					for i := range v.Rows {
						v.Rows[i].ANameField = copy
						v.Rows[i].BNameField = copy
						v.Rows[i].ATitle = budgetProbeCopy(80)
						v.Rows[i].BTitle = budgetProbeCopy(80)
					}
				case "paired_title":
					for i := range v.Rows {
						v.Rows[i].ATitle = copy
						v.Rows[i].BTitle = copy
						v.Rows[i].ANameField = budgetProbeCopy(60)
						v.Rows[i].BNameField = budgetProbeCopy(60)
					}
				case "balanced":
					for i := range v.Rows {
						v.Rows[i].ANameField = copy
						v.Rows[i].BNameField = copy
						v.Rows[i].ATitle = budgetProbeCopy(length * 4 / 3)
						v.Rows[i].BTitle = budgetProbeCopy(length * 4 / 3)
					}
				}
				return v
			})
			t.Logf("rows=%d field=%s budget=%d", count, field, budget)
		}
	}
}

func TestExecSummaryBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_EXEC_SUMMARY_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_EXEC_SUMMARY_BUDGET_PROBE=1")
	}
	for count := 3; count <= 5; count++ {
		for _, bottom := range []bool{false, true} {
			for _, field := range []string{"lead", "support", "bottom_line", "all_lead", "all_support", "paired_lead", "paired_support", "full_bottom_support", "full_support_bottom"} {
				if field == "bottom_line" && !bottom {
					continue
				}
				if (field == "full_bottom_support" || field == "full_support_bottom") && !bottom {
					continue
				}
				maxChars := map[string]int{"lead": 90, "support": 200, "bottom_line": 160, "all_lead": 90, "all_support": 200, "paired_lead": 90, "paired_support": 200, "full_bottom_support": 200, "full_support_bottom": 160}[field]
				budget := probeReadableBudget(t, "exec-summary", maxChars, func(length int) any {
					v := &patterns.ExecSummaryValues{}
					for i := 0; i < count; i++ {
						v.Points = append(v.Points, patterns.ExecSummaryPoint{Lead: "A concise conclusion", Support: "Evidence supports the conclusion."})
					}
					if bottom {
						v.BottomLine = "Act on the recommendation."
					}
					copy := budgetProbeCopy(length)
					switch field {
					case "lead":
						v.Points[0].Lead = copy
					case "support":
						v.Points[0].Support = copy
					case "bottom_line":
						v.BottomLine = copy
					case "all_lead":
						for i := range v.Points {
							v.Points[i].Lead = copy
						}
					case "all_support":
						for i := range v.Points {
							v.Points[i].Support = copy
						}
					case "paired_lead":
						for i := range v.Points {
							v.Points[i].Lead = copy
							v.Points[i].Support = budgetProbeCopy(200)
						}
					case "paired_support":
						for i := range v.Points {
							v.Points[i].Lead = budgetProbeCopy(90)
							v.Points[i].Support = copy
						}
					case "full_bottom_support":
						v.BottomLine = budgetProbeCopy(160)
						for i := range v.Points {
							v.Points[i].Support = copy
						}
					case "full_support_bottom":
						v.BottomLine = copy
						for i := range v.Points {
							v.Points[i].Support = budgetProbeCopy(200)
						}
					}
					return v
				})
				t.Logf("points=%d bottom=%t field=%s budget=%d", count, bottom, field, budget)
			}
		}
	}
}

func TestWaterfallBridgeBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_WATERFALL_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_WATERFALL_BUDGET_PROBE=1")
	}
	for columns := 3; columns <= 10; columns++ {
		for _, caption := range []bool{false, true} {
			for _, field := range []string{"label", "all_label", "unit", "caption"} {
				if field == "caption" && !caption {
					continue
				}
				limit := map[string]int{"label": 40, "all_label": 40, "unit": 8, "caption": 60}[field]
				budget := probeReadableBudget(t, "waterfall-bridge", limit, func(length int) any {
					v := &patterns.WaterfallBridgeValues{Unit: "%"}
					for i := 0; i < columns; i++ {
						column := patterns.WaterfallBridgeColumn{Label: "Driver", Value: 10, Type: "delta"}
						if i == 0 {
							column.Type = "total"
							column.Value = 100
						}
						if i == columns-1 {
							column.Type = "subtotal"
							column.Value = 0
						}
						v.Columns = append(v.Columns, column)
					}
					if caption {
						v.Caption = "Values in percent"
					}
					copy := budgetProbeCopy(length)
					switch field {
					case "label":
						v.Columns[0].Label = copy
					case "all_label":
						for i := range v.Columns {
							v.Columns[i].Label = copy
						}
					case "unit":
						v.Unit = strings.Repeat("u", length)
					case "caption":
						v.Caption = copy
					}
					return v
				})
				t.Logf("columns=%d caption=%t field=%s budget=%d", columns, caption, field, budget)
			}
		}
	}
}

func TestStatHeroBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_STAT_HERO_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_STAT_HERO_BUDGET_PROBE=1")
	}
	pat, _ := patterns.Default().Get("stat-hero")
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
		deck := &PresentationInput{Template: templateName, Slides: []SlideInput{{SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "stat-hero", Values: maximumJSON}}}}
		for _, finding := range collectReadabilityFindings(deck, layouts, width, height) {
			t.Logf("schema maximum on %s: %s", templateName, finding.Message)
		}
	}
	for _, field := range []string{"value", "unit", "label", "context", "source", "paired_value", "full_stack_label", "full_stack_context", "wide_value", "wide_unit", "wide_paired_value", "wide_full_stack_value", "wide_full_stack_unit", "wide_full_stack_label", "wide_full_stack_context", "wide_full_stack_source", "wide_balanced"} {
		limit := map[string]int{"value": 20, "unit": 10, "label": 80, "context": 120, "source": 80, "paired_value": 20, "full_stack_label": 80, "full_stack_context": 120, "wide_value": 20, "wide_unit": 10, "wide_paired_value": 20, "wide_full_stack_value": 20, "wide_full_stack_unit": 10, "wide_full_stack_label": 80, "wide_full_stack_context": 120, "wide_full_stack_source": 80, "wide_balanced": 100}[field]
		budget := probeReadableBudget(t, "stat-hero", limit, func(length int) any {
			v := &patterns.StatHeroValues{Value: "$2.4B", Label: "Market size"}
			copy := budgetProbeCopy(length)
			switch field {
			case "value":
				v.Value = copy
			case "unit":
				v.Unit = copy
			case "label":
				v.Label = copy
			case "context":
				v.Context = copy
			case "source":
				v.Source = copy
			case "paired_value":
				v.Value = copy
				v.Unit = budgetProbeCopy(10)
			case "full_stack_label":
				v.Value = budgetProbeCopy(20)
				v.Unit = budgetProbeCopy(10)
				v.Label = copy
				v.Context = budgetProbeCopy(120)
				v.Source = budgetProbeCopy(80)
			case "full_stack_context":
				v.Value = budgetProbeCopy(20)
				v.Unit = budgetProbeCopy(10)
				v.Label = budgetProbeCopy(80)
				v.Context = copy
				v.Source = budgetProbeCopy(80)
			case "wide_value":
				v.Value = strings.Repeat("W", length)
			case "wide_unit":
				v.Unit = strings.Repeat("W", length)
			case "wide_paired_value":
				v.Value = strings.Repeat("W", length)
				v.Unit = strings.Repeat("W", 10)
			case "wide_full_stack_value":
				v.Value = strings.Repeat("W", length)
				v.Unit = strings.Repeat("W", 10)
				v.Label = strings.Repeat("W", 80)
				v.Context = strings.Repeat("W", 120)
				v.Source = strings.Repeat("W", 80)
			case "wide_full_stack_unit", "wide_full_stack_label", "wide_full_stack_context", "wide_full_stack_source":
				v.Value = strings.Repeat("W", 20)
				v.Unit = strings.Repeat("W", 10)
				v.Label = strings.Repeat("W", 80)
				v.Context = strings.Repeat("W", 120)
				v.Source = strings.Repeat("W", 80)
				switch field {
				case "wide_full_stack_unit":
					v.Unit = strings.Repeat("W", length)
				case "wide_full_stack_label":
					v.Label = strings.Repeat("W", length)
				case "wide_full_stack_context":
					v.Context = strings.Repeat("W", length)
				case "wide_full_stack_source":
					v.Source = strings.Repeat("W", length)
				}
			case "wide_balanced":
				v.Value = strings.Repeat("W", 20*length/100)
				v.Unit = strings.Repeat("W", 10*length/100)
				v.Label = strings.Repeat("W", 80*length/100)
				v.Context = strings.Repeat("W", 120*length/100)
				v.Source = strings.Repeat("W", 80*length/100)
			}
			return v
		})
		t.Logf("field=%s budget=%d", field, budget)
	}
}

func TestSCQASummaryBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_SCQA_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_SCQA_BUDGET_PROBE=1")
	}
	for items := 1; items <= 4; items++ {
		for _, field := range []string{"situation", "complication", "questions", "answer", "all", "one_item"} {
			budget := probeReadableBudget(t, "scqa-summary", 240, func(length int) any {
				v := &patterns.SCQASummaryValues{Situation: patterns.SCQAText{"Current position."}, Complication: patterns.SCQAText{"New pressure."}, Questions: []string{"What changes?"}, Answer: []string{"Take action."}}
				copy := budgetProbeCopy(length)
				arr := make([]string, items)
				for i := range arr {
					arr[i] = copy
				}
				switch field {
				case "situation":
					v.Situation = patterns.SCQAText(arr)
				case "complication":
					v.Complication = patterns.SCQAText(arr)
				case "questions":
					v.Questions = arr
				case "answer":
					v.Answer = arr
				case "all":
					v.Situation = patterns.SCQAText(arr)
					v.Complication = patterns.SCQAText(arr)
					v.Questions = arr
					v.Answer = arr
				case "one_item":
					for i := range arr {
						arr[i] = "Brief"
					}
					arr[0] = copy
					v.Questions = arr
				}
				return v
			})
			t.Logf("items=%d field=%s budget=%d", items, field, budget)
		}
	}
}

func TestAgendaBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_AGENDA_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_AGENDA_BUDGET_PROBE=1")
	}
	for count := 2; count <= 10; count++ {
		for _, highlighted := range []bool{false, true} {
			for _, all := range []bool{false, true} {
				for _, wide := range []bool{false, true} {
					overrides := &patterns.AgendaOverrides{}
					if highlighted {
						overrides.Highlight = 1
					}
					budget := probeReadableBudget(t, "agenda", 100, func(length int) any {
						v := &patterns.AgendaValues{Items: make([]string, count)}
						for i := range v.Items {
							v.Items[i] = "Section"
						}
						copy := budgetProbeCopy(length)
						if wide {
							copy = strings.Repeat("W", length)
						}
						v.Items[0] = copy
						if all {
							for i := range v.Items {
								v.Items[i] = copy
							}
						}
						return v
					}, overrides)
					t.Logf("items=%d highlighted=%t all=%t wide=%t budget=%d", count, highlighted, all, wide, budget)
				}
			}
		}
	}
}

func TestValueChainBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_VALUE_CHAIN_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_VALUE_CHAIN_BUDGET_PROBE=1")
	}
	for steps := 4; steps <= 10; steps++ {
		for _, field := range []string{"description", "all_description", "wide_description", "wide_all_description"} {
			budget := probeReadableBudget(t, "value-chain", 180, func(length int) any {
				v := &patterns.ValueChainValues{Steps: make([]patterns.ValueChainStep, steps)}
				for i := range v.Steps {
					v.Steps[i] = patterns.ValueChainStep{Label: "Stage", Description: "Brief description"}
				}
				copy := budgetProbeCopy(length)
				if strings.HasPrefix(field, "wide") {
					copy = strings.Repeat("W", length)
				}
				v.Steps[0].Description = copy
				if strings.Contains(field, "all") {
					for i := range v.Steps {
						v.Steps[i].Description = copy
					}
				}
				return v
			})
			t.Logf("steps=%d field=%s budget=%d", steps, field, budget)
		}
	}
}

func TestStrategyHouseBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_STRATEGY_HOUSE_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_STRATEGY_HOUSE_BUDGET_PROBE=1")
	}
	for pillars := 3; pillars <= 5; pillars++ {
		for bullets := 1; bullets <= 5; bullets++ {
			for _, roof := range []bool{false, true} {
				for _, all := range []bool{false, true} {
					budget := probeReadableBudget(t, "strategy-house", 120, func(length int) any {
						v := &patterns.StrategyHouseValues{Objective: "Grow the business", Foundation: "Core capabilities"}
						if roof {
							v.RoofBadges = []string{"Vision", "Mission"}
						}
						for i := 0; i < pillars; i++ {
							p := patterns.StrategyHousePillar{Title: "Growth"}
							for j := 0; j < bullets; j++ {
								p.Body = append(p.Body, "Brief")
							}
							v.Pillars = append(v.Pillars, p)
						}
						v.Pillars[0].Body[0] = budgetProbeCopy(length)
						if all {
							for i := range v.Pillars {
								for j := range v.Pillars[i].Body {
									v.Pillars[i].Body[j] = budgetProbeCopy(length)
								}
							}
						}
						return v
					})
					t.Logf("pillars=%d bullets=%d roof=%t all=%t budget=%d", pillars, bullets, roof, all, budget)
				}
			}
		}
	}
	for _, pillars := range []int{3, 5} {
		for _, field := range []string{"objective", "foundation", "title", "badge"} {
			limit := map[string]int{"objective": 140, "foundation": 140, "title": 60, "badge": 24}[field]
			budget := probeReadableBudget(t, "strategy-house", limit, func(length int) any {
				v := &patterns.StrategyHouseValues{Objective: "Grow", Foundation: "Capabilities", RoofBadges: []string{"Vision", "Mission"}}
				for i := 0; i < pillars; i++ {
					v.Pillars = append(v.Pillars, patterns.StrategyHousePillar{Title: "Growth", Body: []string{"Brief"}})
				}
				copy := budgetProbeCopy(length)
				switch field {
				case "objective":
					v.Objective = copy
				case "foundation":
					v.Foundation = copy
				case "title":
					v.Pillars[0].Title = copy
				case "badge":
					v.RoofBadges[0] = copy
				}
				return v
			})
			t.Logf("pillars=%d field=%s budget=%d", pillars, field, budget)
		}
	}
}

func TestJourneyMaturityBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_JOURNEY_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_JOURNEY_BUDGET_PROBE=1")
	}
	for stages := 3; stages <= 6; stages++ {
		for _, current := range []bool{false, true} {
			for _, field := range []string{"label", "wide_label", "description", "wide_description"} {
				limit := map[string]int{"label": 40, "wide_label": 40, "description": 180, "wide_description": 180}[field]
				budget := probeReadableBudget(t, "journey-maturity-model", limit, func(length int) any {
					v := &patterns.JourneyMaturityValues{Stages: make([]patterns.JourneyMaturityStage, stages)}
					for i := range v.Stages {
						v.Stages[i] = patterns.JourneyMaturityStage{Label: "Stage", Description: "Brief description"}
					}
					if current {
						v.Stages[0].Current = true
					}
					copy := budgetProbeCopy(length)
					switch field {
					case "label":
						v.Stages[0].Label = copy
					case "wide_label":
						v.Stages[0].Label = strings.Repeat("W", length)
					case "description":
						v.Stages[0].Description = copy
					case "wide_description":
						v.Stages[0].Description = strings.Repeat("W", length)
					}
					return v
				})
				t.Logf("stages=%d current=%t field=%s budget=%d", stages, current, field, budget)
			}
		}
	}
}

func TestProcessFlowCompactBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_PROCESS_COMPACT_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_PROCESS_COMPACT_BUDGET_PROBE=1")
	}
	for steps := 3; steps <= 8; steps++ {
		for _, shape := range []string{"step", "decision", "chevron", "arrow"} {
			for _, wide := range []bool{false, true} {
				budget := probeReadableBudget(t, "process-flow-compact", 80, func(length int) any {
					v := &patterns.ProcessFlowValues{Steps: make([]patterns.ProcessFlowStep, steps)}
					for i := range v.Steps {
						v.Steps[i] = patterns.ProcessFlowStep{Label: "Stage", Type: shape}
					}
					copy := budgetProbeCopy(length)
					if wide {
						copy = strings.Repeat("W", length)
					}
					v.Steps[0].Label = copy
					return v
				})
				t.Logf("steps=%d shape=%s wide=%t budget=%d", steps, shape, wide, budget)
			}
		}
	}
}

func TestProcessFlowBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_PROCESS_FLOW_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_PROCESS_FLOW_BUDGET_PROBE=1")
	}
	for steps := 3; steps <= 8; steps++ {
		for _, shape := range []string{"step", "decision", "chevron", "arrow"} {
			for _, wide := range []bool{false, true} {
				budget := probeReadableBudget(t, "process-flow", 80, func(length int) any {
					v := &patterns.ProcessFlowValues{Steps: make([]patterns.ProcessFlowStep, steps)}
					for i := range v.Steps {
						v.Steps[i] = patterns.ProcessFlowStep{Label: "Stage", Type: shape}
					}
					copy := budgetProbeCopy(length)
					if wide {
						copy = strings.Repeat("W", length)
					}
					v.Steps[0].Label = copy
					return v
				})
				t.Logf("steps=%d shape=%s wide=%t budget=%d", steps, shape, wide, budget)
			}
		}
	}
}

func TestArchStackBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_ARCH_STACK_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_ARCH_STACK_BUDGET_PROBE=1")
	}
	for tiers := 3; tiers <= 6; tiers++ {
		for rails := 0; rails <= 3; rails++ {
			for _, field := range []string{"label", "description", "rail", "all"} {
				if field == "rail" && rails == 0 {
					continue
				}
				for _, wide := range []bool{false, true} {
					maxChars := map[string]int{"label": 60, "description": 120, "rail": 30, "all": 120}[field]
					budget := probeReadableBudget(t, "arch-stack", maxChars, func(length int) any {
						v := &patterns.ArchStackValues{Tiers: make([]patterns.ArchStackTier, tiers), SideRails: make([]string, rails)}
						for i := range v.Tiers {
							v.Tiers[i] = patterns.ArchStackTier{Label: "Layer", Description: "Brief"}
						}
						for i := range v.SideRails {
							v.SideRails[i] = "Security"
						}
						copy := budgetProbeCopy(length)
						if wide {
							copy = strings.Repeat("W", length)
						}
						switch field {
						case "label":
							v.Tiers[0].Label = copy
						case "description":
							v.Tiers[0].Description = copy
						case "rail":
							v.SideRails[0] = copy
						case "all":
							for i := range v.Tiers {
								v.Tiers[i].Label = copy[:min(len(copy), 60)]
								v.Tiers[i].Description = copy
							}
							for i := range v.SideRails {
								v.SideRails[i] = copy[:min(len(copy), 30)]
							}
						}
						return v
					})
					t.Logf("tiers=%d rails=%d field=%s wide=%t budget=%d", tiers, rails, field, wide, budget)
				}
			}
		}
	}
}

func TestPyramidBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_PYRAMID_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_PYRAMID_BUDGET_PROBE=1")
	}
	for tiers := 3; tiers <= 5; tiers++ {
		for _, position := range []string{"top", "middle", "base", "all"} {
			for _, wide := range []bool{false, true} {
				budget := probeReadableBudget(t, "pyramid", 120, func(length int) any {
					v := &patterns.PyramidValues{Tiers: make([]string, tiers)}
					for i := range v.Tiers {
						v.Tiers[i] = "Core capability"
					}
					copy := budgetProbeCopy(length)
					if wide {
						copy = strings.Repeat("W", length)
					}
					switch position {
					case "top":
						v.Tiers[0] = copy
					case "middle":
						v.Tiers[tiers/2] = copy
					case "base":
						v.Tiers[tiers-1] = copy
					case "all":
						for i := range v.Tiers {
							v.Tiers[i] = copy
						}
					}
					return v
				})
				t.Logf("tiers=%d position=%s wide=%t budget=%d", tiers, position, wide, budget)
			}
		}
	}
}

func TestChartInsightsSplitBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_CHART_INSIGHTS_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_CHART_INSIGHTS_BUDGET_PROBE=1")
	}
	pat, _ := patterns.Default().Get("chart-insights-split")
	chart := pat.(patterns.Exemplar).ExemplarValues().(*patterns.ChartInsightsSplitValues).Chart
	for bullets := 1; bullets <= 6; bullets++ {
		for _, withChart := range []bool{false, true} {
			for _, extras := range []bool{false, true} {
				for _, all := range []bool{false, true} {
					for _, wide := range []bool{false, true} {
						budget := probeReadableBudget(t, "chart-insights-split", 160, func(length int) any {
							v := &patterns.ChartInsightsSplitValues{Insights: make([]string, bullets)}
							if withChart {
								v.Chart = chart
							}
							if extras {
								v.Headline = &patterns.ChartInsightsHeadline{Value: "+75%", Label: "Revenue growth"}
								v.SoWhat = "Invest now."
							}
							for i := range v.Insights {
								v.Insights[i] = "Brief insight"
							}
							copy := budgetProbeCopy(length)
							if wide {
								copy = strings.Repeat("W", length)
							}
							v.Insights[0] = copy
							if all {
								for i := range v.Insights {
									v.Insights[i] = copy
								}
							}
							return v
						})
						t.Logf("bullets=%d chart=%t extras=%t all=%t wide=%t budget=%d", bullets, withChart, extras, all, wide, budget)
					}
				}
			}
		}
	}
	for bullets := 1; bullets <= 6; bullets++ {
		for _, wide := range []bool{false, true} {
			budget := probeReadableBudget(t, "chart-insights-split", 160, func(length int) any {
				copy := budgetProbeCopy
				if wide {
					copy = func(n int) string { return strings.Repeat("W", n) }
				}
				v := &patterns.ChartInsightsSplitValues{
					Chart: chart, InsightsTitle: copy(40), Source: copy(120),
					Headline: &patterns.ChartInsightsHeadline{Value: copy(12), Label: copy(60)},
					SoWhat:   copy(160), ChartLabel: copy(60), Unit: copy(12),
					Insights: make([]string, bullets),
				}
				for i := range v.Insights {
					v.Insights[i] = copy(length)
				}
				return v
			})
			t.Logf("bullets=%d full_stack=true wide=%t budget=%d", bullets, wide, budget)
		}
	}
	for _, field := range []string{"insights_title", "source", "source_no_chart", "source_with_extras", "source_with_headline", "source_with_so_what", "source_with_headline_so_what", "headline_value", "headline_label", "so_what", "chart_label", "unit"} {
		limit := map[string]int{"insights_title": 40, "source": 120, "source_no_chart": 120, "source_with_extras": 120, "source_with_headline": 120, "source_with_so_what": 120, "source_with_headline_so_what": 120, "headline_value": 12, "headline_label": 60, "so_what": 160, "chart_label": 60, "unit": 12}[field]
		budget := probeReadableBudget(t, "chart-insights-split", limit, func(length int) any {
			v := &patterns.ChartInsightsSplitValues{Chart: chart, Insights: []string{"Brief insight"}}
			copy := budgetProbeCopy(length)
			switch field {
			case "insights_title":
				v.InsightsTitle = copy
			case "source":
				v.Source = copy
			case "source_no_chart":
				v.Chart = nil
				v.Source = copy
			case "source_with_extras":
				v.InsightsTitle = budgetProbeCopy(40)
				v.Source = copy
				v.Headline = &patterns.ChartInsightsHeadline{Value: budgetProbeCopy(12), Label: budgetProbeCopy(60)}
				v.SoWhat = budgetProbeCopy(160)
				v.ChartLabel = budgetProbeCopy(60)
				v.Unit = budgetProbeCopy(12)
			case "source_with_headline":
				v.Source = copy
				v.Headline = &patterns.ChartInsightsHeadline{Value: "+75%", Label: "Revenue growth"}
			case "source_with_so_what":
				v.Source = copy
				v.SoWhat = "Invest now"
			case "source_with_headline_so_what":
				v.Source = copy
				v.Headline = &patterns.ChartInsightsHeadline{Value: "+75%", Label: "Revenue growth"}
				v.SoWhat = "Invest now"
			case "headline_value":
				v.Headline = &patterns.ChartInsightsHeadline{Value: copy}
			case "headline_label":
				v.Headline = &patterns.ChartInsightsHeadline{Value: "+75%", Label: copy}
			case "so_what":
				v.SoWhat = copy
			case "chart_label":
				v.ChartLabel = copy
			case "unit":
				v.Unit = copy
			}
			return v
		})
		t.Logf("field=%s isolated_budget=%d", field, budget)
	}
	for _, bullets := range []int{1, 3, 6} {
		for _, extras := range []bool{false, true} {
			budget := probeReadableBudget(t, "chart-insights-split", 120, func(length int) any {
				v := &patterns.ChartInsightsSplitValues{Chart: chart, Source: budgetProbeCopy(length), Insights: make([]string, bullets)}
				for i := range v.Insights {
					v.Insights[i] = "Brief insight"
				}
				if extras {
					v.Headline = &patterns.ChartInsightsHeadline{Value: "+75%", Label: "Revenue growth"}
					v.SoWhat = "Invest now"
				}
				return v
			})
			t.Logf("source_bullets=%d extras=%t budget=%d", bullets, extras, budget)
		}
	}
}

func TestMatrix2x2BudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_MATRIX_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_MATRIX_BUDGET_PROBE=1")
	}
	for _, field := range []string{"header", "body", "x_axis", "y_axis", "axis_end", "paired_header", "paired_body", "all_body"} {
		limit := map[string]int{"header": 80, "body": 200, "x_axis": 60, "y_axis": 60, "axis_end": 20, "paired_header": 80, "paired_body": 200, "all_body": 200}[field]
		budget := probeReadableBudget(t, "matrix-2x2", limit, func(length int) any {
			v := &patterns.Matrix2x2Values{XAxisLabel: "Market", YAxisLabel: "Growth", TopLeft: patterns.Matrix2x2Quadrant{Header: "Stars", Body: "Brief"}, TopRight: patterns.Matrix2x2Quadrant{Header: "Emerging", Body: "Brief"}, BottomLeft: patterns.Matrix2x2Quadrant{Header: "Core", Body: "Brief"}, BottomRight: patterns.Matrix2x2Quadrant{Header: "Exit", Body: "Brief"}}
			copy := budgetProbeCopy(length)
			switch field {
			case "header":
				v.TopLeft.Header = copy
			case "body":
				v.TopLeft.Body = copy
			case "x_axis":
				v.XAxisLabel = copy
			case "y_axis":
				v.YAxisLabel = copy
			case "axis_end":
				v.XLow = copy
			case "paired_header":
				v.TopLeft.Header = copy
				v.TopLeft.Body = budgetProbeCopy(200)
			case "paired_body":
				v.TopLeft.Header = budgetProbeCopy(80)
				v.TopLeft.Body = copy
			case "all_body":
				v.TopLeft.Body = copy
				v.TopRight.Body = copy
				v.BottomLeft.Body = copy
				v.BottomRight.Body = copy
			}
			return v
		})
		t.Logf("field=%s budget=%d", field, budget)
	}
	for _, field := range []string{"header", "body", "x_axis", "y_axis", "axis_end", "paired_body", "body_word_header", "body_wide_header", "header_word_body", "header_wide_body", "all_body", "full_stack_body"} {
		limit := map[string]int{"header": 80, "body": 200, "x_axis": 60, "y_axis": 60, "axis_end": 20, "paired_body": 200, "body_word_header": 200, "body_wide_header": 200, "header_word_body": 80, "header_wide_body": 80, "all_body": 200, "full_stack_body": 200}[field]
		budget := probeReadableBudget(t, "matrix-2x2", limit, func(length int) any {
			v := &patterns.Matrix2x2Values{XAxisLabel: "Market", YAxisLabel: "Growth", TopLeft: patterns.Matrix2x2Quadrant{Header: "Stars", Body: "Brief"}, TopRight: patterns.Matrix2x2Quadrant{Header: "Emerging", Body: "Brief"}, BottomLeft: patterns.Matrix2x2Quadrant{Header: "Core", Body: "Brief"}, BottomRight: patterns.Matrix2x2Quadrant{Header: "Exit", Body: "Brief"}}
			copy := strings.Repeat("W", length)
			switch field {
			case "header":
				v.TopLeft.Header = copy
			case "body":
				v.TopLeft.Body = copy
			case "x_axis":
				v.XAxisLabel = copy
			case "y_axis":
				v.YAxisLabel = copy
			case "axis_end":
				v.XLow = copy
			case "paired_body":
				v.TopLeft.Header = strings.Repeat("W", 80)
				v.TopLeft.Body = copy
			case "body_word_header":
				v.TopLeft.Header = budgetProbeCopy(80)
				v.TopLeft.Body = copy
			case "body_wide_header":
				v.TopLeft.Header = strings.Repeat("W", 80)
				v.TopLeft.Body = copy
			case "header_word_body":
				v.TopLeft.Header = copy
				v.TopLeft.Body = budgetProbeCopy(200)
			case "header_wide_body":
				v.TopLeft.Header = copy
				v.TopLeft.Body = strings.Repeat("W", 200)
			case "all_body":
				v.TopLeft.Body = copy
				v.TopRight.Body = copy
				v.BottomLeft.Body = copy
				v.BottomRight.Body = copy
			case "full_stack_body":
				v.XAxisLabel = strings.Repeat("W", 60)
				v.YAxisLabel = strings.Repeat("W", 60)
				v.XLow = strings.Repeat("W", 20)
				v.XHigh = strings.Repeat("W", 20)
				v.YLow = strings.Repeat("W", 20)
				v.YHigh = strings.Repeat("W", 20)
				v.TopLeft.Header = strings.Repeat("W", 80)
				v.TopRight.Header = strings.Repeat("W", 80)
				v.BottomLeft.Header = strings.Repeat("W", 80)
				v.BottomRight.Header = strings.Repeat("W", 80)
				v.TopLeft.Body = copy
				v.TopRight.Body = copy
				v.BottomLeft.Body = copy
				v.BottomRight.Body = copy
			}
			return v
		})
		t.Logf("field=wide_%s budget=%d", field, budget)
	}
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
