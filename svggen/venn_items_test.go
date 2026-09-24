package svggen

import (
	"strings"
	"testing"
)

// vennRequest builds a 3-circle venn with the given item counts per circle.
func vennRequest(itemsPerCircle int, width, height int) *RequestEnvelope {
	circles := make([]any, 0, 3)
	for i := 0; i < 3; i++ {
		label := string(rune('A' + i))
		items := make([]any, 0, itemsPerCircle)
		for j := 0; j < itemsPerCircle; j++ {
			items = append(items, "Capability "+label+string(rune('1'+j)))
		}
		circles = append(circles, map[string]any{"label": "Set " + label, "items": items})
	}
	return &RequestEnvelope{
		Type:   "venn",
		Data:   map[string]any{"circles": circles},
		Output: OutputSpec{Width: width, Height: height},
	}
}

// go-slide-creator-onop: items were rendered only when very short and few —
// with 7 per circle every item was dropped and only the three labels remained,
// with no fit-report finding, so the agent believed its content shipped.
func TestVenn_DroppedItemsAreReported(t *testing.T) {
	// 12 items per circle genuinely cannot fit even a full canvas.
	findings, err := DryRender(vennRequest(12, 1024, 768))
	if err != nil {
		t.Fatalf("dry render: %v", err)
	}

	var dropped *Finding
	for i := range findings {
		if findings[i].Code == FindingDiagramItemsDropped {
			dropped = &findings[i]
			break
		}
	}
	if dropped == nil {
		t.Fatalf("12 items per circle must report %s, got %+v", FindingDiagramItemsDropped, findings)
	}
	if dropped.Severity != "refuse" {
		t.Errorf("severity = %q, want refuse for lost authored content", dropped.Severity)
	}
	if dropped.Fix == nil || dropped.Fix.Kind != FixKindReduceItems {
		t.Fatalf("fix = %+v, want kind %q", dropped.Fix, FixKindReduceItems)
	}
	if n, ok := dropped.Fix.Params["dropped_count"].(int); !ok || n <= 0 {
		t.Errorf("fix.params.dropped_count = %v, want a positive count", dropped.Fix.Params["dropped_count"])
	}
	if !strings.Contains(dropped.Message, "did not fit") {
		t.Errorf("message should say the items did not fit, got: %s", dropped.Message)
	}
}

// The concrete regression: 7 items per circle used to drop EVERY item, leaving
// only the three labels. On a full canvas they must all render now, with
// nothing reported. (The item budget was 30% of the circle radius, which had
// room for barely one short line.)
func TestVenn_SevenItemsPerCircleAllRender(t *testing.T) {
	req := vennRequest(7, 1024, 768)

	findings, err := DryRender(req)
	if err != nil {
		t.Fatalf("dry render: %v", err)
	}
	for _, f := range findings {
		if f.Code == FindingDiagramItemsDropped {
			t.Errorf("7 items per circle should fit a full canvas, got: %+v", f)
		}
	}

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svg := string(doc.Content)
	for i := 1; i <= 7; i++ {
		item := "Capability A" + string(rune('0'+i))
		if !strings.Contains(svg, item) {
			t.Errorf("rendered SVG is missing item %q", item)
		}
	}
}

// A short item list on a full canvas must render completely and report nothing.
func TestVenn_ShortItemListRendersAndStaysQuiet(t *testing.T) {
	req := vennRequest(2, 1024, 768)

	findings, err := DryRender(req)
	if err != nil {
		t.Fatalf("dry render: %v", err)
	}
	for _, f := range findings {
		if f.Code == FindingDiagramItemsDropped {
			t.Errorf("2 short items per circle must fit on a full canvas, got: %+v", f)
		}
	}

	// And they must actually appear in the drawn output.
	doc, err := Render(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svg := string(doc.Content)
	for _, want := range []string{"Capability A1", "Capability A2", "Capability C2"} {
		if !strings.Contains(svg, want) {
			t.Errorf("rendered SVG is missing item %q", want)
		}
	}
}

// More than three circles has no readable planar form and must be refused with
// a message that names an alternative, not silently truncated.
func TestVenn_MoreThanThreeCirclesRefused(t *testing.T) {
	circles := make([]any, 0, 5)
	for i := 0; i < 5; i++ {
		circles = append(circles, map[string]any{"label": "Set " + string(rune('A'+i))})
	}
	_, err := DryRender(&RequestEnvelope{
		Type: "venn",
		Data: map[string]any{"circles": circles},
	})
	if err == nil {
		t.Fatal("5 circles must be refused")
	}
	if !strings.Contains(err.Error(), "at most 3") {
		t.Errorf("error should name the limit, got: %v", err)
	}
	if !strings.Contains(err.Error(), "matrix_2x2") {
		t.Errorf("error should suggest an alternative diagram, got: %v", err)
	}
}
