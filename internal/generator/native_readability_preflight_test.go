package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// smallDiagramBounds are the shortest content areas the shipped templates give
// a diagram placeholder (abstract, modern): the ones the native renderers used
// to shrink to 2-6pt (go-slide-creator-zbo58).
var smallDiagramBounds = map[string]types.BoundingBox{
	"abstract": {Width: 8724423, Height: 3407051},
	"modern":   {Width: 10803850, Height: 3474720},
}

func exampleDiagramSpec(t *testing.T, name string) *types.DiagramSpec {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "diagrams", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var deck struct {
		Slides []struct {
			Content []struct {
				DiagramValue *types.DiagramSpec `json:"diagram_value"`
			} `json:"content"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(raw, &deck); err != nil {
		t.Fatal(err)
	}
	for _, s := range deck.Slides {
		for _, c := range s.Content {
			if c.DiagramValue != nil {
				return c.DiagramValue
			}
		}
	}
	t.Fatalf("%s has no diagram", name)
	return nil
}

func TestNativeDiagramExamplesStayReadableOnSmallContentAreas(t *testing.T) {
	for _, name := range []string{"business_model_canvas", "nine_box_talent", "porters_five_forces", "swot", "pyramid"} {
		spec := exampleDiagramSpec(t, name)
		for template, bounds := range smallDiagramBounds {
			t.Run(name+"/"+template, func(t *testing.T) {
				if nativeDiagramGroupXML(spec, bounds, "Arial", nil) == "" {
					t.Fatal("no native group rendered")
				}
				if got := NativeDiagramReadabilityPreflight(spec, bounds, "Arial", nil, tokens.ViewingModePresentation, "/slides/1/content/1/diagram_value"); len(got) != 0 {
					t.Fatalf("shipped example would be refused: %+v", got)
				}
			})
		}
	}
}

func TestNativeDiagramReadabilityPreflightReportsRefusal(t *testing.T) {
	// Four two-line bullets in every stacked BMC cell cannot be set above the
	// 7pt floor on abstract's 268pt-tall placeholder.
	items := []any{"Cloud providers (AWS, GCP)", "Data warehouse partners", "Dedicated CSM for enterprise", "Website & product-led growth"}
	data := map[string]any{}
	for _, key := range bmcSectionOrder {
		data[string(key)] = items
	}
	spec := &types.DiagramSpec{Type: "business_model_canvas", Data: data}
	path := "/slides/3/content/1/diagram_value"
	got := NativeDiagramReadabilityPreflight(spec, smallDiagramBounds["abstract"], "Arial", nil, tokens.ViewingModePresentation, path)
	if len(got) != 1 {
		t.Fatalf("findings = %+v, want one refusal", got)
	}
	f := got[0]
	if f.Action != "refuse" || f.Severity() != "error" || f.Path != path || f.Pattern != "business_model_canvas" || !strings.Contains(f.Message, "generate refuses") {
		t.Fatalf("refusal = %+v", f)
	}
}
