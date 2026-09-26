package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestStrictFitBlocksPredictedParagraphLossWithoutChangingSource(t *testing.T) {
	layouts := []types.LayoutMetadata{{ID: "slideLayout2", CanonicalType: types.CanonicalLayoutType("content"), Placeholders: []types.PlaceholderInfo{{ID: "body", Type: types.PlaceholderBody, FontSize: 1200, FontFamily: "Calibri", Bounds: types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 1200000}}}}}
	for _, mode := range []string{"strict", "warn", "off"} {
		t.Run(mode, func(t *testing.T) {
			input := &PresentationInput{Slides: []SlideInput{denseBulletsSlide("slideLayout2", 14)}}
			before, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			findings, err := evaluateStrictFit(input, mode, layouts, 12192000, 6858000, nil)
			if (err != nil) != (mode == "strict") {
				t.Fatalf("mode=%s err=%v", mode, err)
			}
			finding := firstFindingCode(findings, patterns.ErrCodeTextTrimmed)
			if finding == nil || finding.Action != "refuse" {
				t.Fatalf("missing text-loss refusal: %+v", findings)
			}
			after, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("fit evaluation changed authored source")
			}
		})
	}
}

func TestNativeParagraphLossStrictGenerateRefusesBeforeWriting(t *testing.T) {
	output := t.TempDir()
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: output, cache: template.NewMemoryCache(24 * time.Hour)}
	bullets := make([]string, 10)
	for i := range bullets {
		bullets[i] = fmt.Sprintf("P%02d %s", i, strings.Repeat("Required service owner confirms reporting deadlines and unresolved handoffs. ", 12))
	}
	deck := map[string]any{"template": "abstract", "slides": []any{map[string]any{"layout_id": "content", "content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Required source evidence"}, map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": bullets}}}}}
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": deck, "strict_fit": "strict"}))
	if err != nil {
		t.Fatal(err)
	}
	requireStructuredError(t, result, patterns.ErrCodeTextTrimmed)
	files, err := os.ReadDir(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("strict source-loss refusal wrote output: %v", files)
	}
	result, err = mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": deck, "strict_fit": "warn"}))
	if err != nil || result.IsError {
		t.Fatalf("warning compatibility changed: err=%v result=%v", err, result)
	}
	var generated JSONOutput
	if err := json.Unmarshal([]byte(textContent(result)), &generated); err != nil {
		t.Fatal(err)
	}
	if !generated.Success {
		t.Fatal("warning mode did not produce its compatibility draft")
	}
	if finding := firstFindingCode(generated.FitFindings, patterns.ErrCodeTextTrimmed); finding == nil || finding.Action != "refuse" {
		t.Fatalf("warning draft hides paragraph loss: %+v", generated.FitFindings)
	}
}
