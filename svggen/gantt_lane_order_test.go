package svggen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// developmentPlanRequest is the go-slide-creator-n0adg review case: lanes
// authored Discovery -> Build -> Test, plus two milestones with no group.
func developmentPlanRequest(w, h int) *RequestEnvelope {
	task := func(id, label, start, end, group string) map[string]any {
		return map[string]any{"id": id, "label": label, "start_date": start, "end_date": end, "group": group}
	}
	return &RequestEnvelope{Type: "gantt", Title: "Development plan", Data: map[string]any{
		"tasks": []any{
			task("t1", "Requirements", "2026-01-01", "2026-01-31", "Discovery"),
			task("t2", "System design", "2026-01-15", "2026-02-28", "Discovery"),
			task("t3", "Backend development", "2026-02-15", "2026-04-30", "Build"),
			task("t4", "Frontend development", "2026-03-01", "2026-05-15", "Build"),
			task("t5", "Integration testing", "2026-04-15", "2026-05-31", "Test"),
			task("t6", "Documentation and training", "2026-05-01", "2026-06-15", "Test"),
		},
		"milestones": []any{
			map[string]any{"id": "m1", "label": "Design sign-off", "date": "2026-02-28"},
			map[string]any{"id": "m2", "label": "Go-live", "date": "2026-06-15"},
		},
	}, Output: OutputSpec{Width: w, Height: h}}
}

var ganttTextRE = regexp.MustCompile(`<text[^>]*?y="([\d.]+)"[^>]*>(?:<tspan[^>]*>)?([^<]+)<`)

type ganttText struct {
	y    float64
	text string
}

func ganttTexts(t *testing.T, svg string) []ganttText {
	t.Helper()
	var out []ganttText
	for _, m := range ganttTextRE.FindAllStringSubmatch(svg, -1) {
		y, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, ganttText{y, m[2]})
	}
	return out
}

func ganttTextY(texts []ganttText, s string) (float64, bool) {
	for _, tx := range texts {
		if tx.text == s {
			return tx.y, true
		}
	}
	return 0, false
}

// TestGantt_LanesKeepAuthoredOrderAndFullHeaders pins go-slide-creator-n0adg:
// lanes render in first-appearance order (Discovery, Build, Test — not
// alphabetical), a lane name is never ellipsised, and ungrouped milestones
// gather in a trailing Milestones lane with their dates beside the diamond,
// not dropped onto the next row.
func TestGantt_LanesKeepAuthoredOrderAndFullHeaders(t *testing.T) {
	for _, size := range [][2]int{{800, 400}, {600, 400}, {400, 300}} {
		doc, err := (&GanttDiagram{NewBaseDiagram("gantt")}).Render(developmentPlanRequest(size[0], size[1]))
		if err != nil {
			t.Fatal(err)
		}
		svg := doc.String()
		texts := ganttTexts(t, svg)
		var ys []float64
		for _, lane := range []string{"Discovery", "Build", "Test", "Milestones"} {
			y, ok := ganttTextY(texts, lane)
			if !ok {
				t.Fatalf("%v: lane header %q missing or shortened; texts=%v", size, lane, texts)
			}
			ys = append(ys, y)
		}
		for i := 1; i < len(ys); i++ {
			if ys[i] <= ys[i-1] {
				t.Errorf("%v: lanes out of authored order: Discovery/Build/Test/Milestones at y=%v", size, ys)
			}
		}
		if strings.Contains(svg, "...") || strings.Contains(svg, "…") {
			t.Errorf("%v: a label was ellipsised", size)
		}
		// Each milestone's date label shares its diamond's row.
		for _, pair := range [][2]string{{"Design sign-off", "28 Feb"}, {"Go-live", "15 Jun"}} {
			rowY, ok1 := ganttTextY(texts, pair[0])
			dateY, ok2 := ganttTextY(texts, pair[1])
			if !ok1 || !ok2 {
				t.Fatalf("%v: milestone %q or its date %q missing", size, pair[0], pair[1])
			}
			if diff := dateY - rowY; diff > 6 || diff < -6 {
				t.Errorf("%v: %q date label at y=%.1f is off its row at y=%.1f", size, pair[0], dateY, rowY)
			}
		}
	}
}
