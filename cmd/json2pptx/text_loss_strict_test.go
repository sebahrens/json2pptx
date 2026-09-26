package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	bullets := make([]string, 10)
	for i := range bullets {
		bullets[i] = fmt.Sprintf("P%02d %s", i, strings.Repeat("Required service owner confirms reporting deadlines and unresolved handoffs. ", 12))
	}
	deck := map[string]any{"template": "abstract", "slides": []any{map[string]any{"layout_id": "content", "content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Required source evidence"}, map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": bullets}}}}}
	before, err := json.Marshal(deck)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"", "warn", "off", "strict"} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode=%q/existing=%t", mode, existing), func(t *testing.T) {
				output := t.TempDir()
				destination := filepath.Join(output, "deck.pptx")
				if existing {
					if err := os.WriteFile(destination, []byte("existing-deck"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				mc := &mcpConfig{templatesDir: "../../templates", outputDir: output, cache: template.NewMemoryCache(24 * time.Hour)}
				params := map[string]any{"presentation": deck, "output_filename": "deck.pptx"}
				if mode != "" {
					params["strict_fit"] = mode
				}
				result, err := mc.handleGenerate(context.Background(), makeRequest(params))
				if err != nil {
					t.Fatal(err)
				}
				requireStructuredError(t, result, patterns.ErrCodeTextTrimmed)
				files, err := os.ReadDir(output)
				wantFiles := 0
				if existing {
					wantFiles = 1
					data, readErr := os.ReadFile(destination)
					if readErr != nil || string(data) != "existing-deck" {
						t.Fatalf("refusal changed destination: %q %v", data, readErr)
					}
				}
				if err != nil || len(files) != wantFiles {
					t.Fatalf("source-loss refusal leaked artifacts: %v %v", files, err)
				}
				after, err := json.Marshal(deck)
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("refusal changed authored source: %v", err)
				}
			})
		}
	}
}

func TestNativeFittingParagraphsGenerateWithoutChangingSource(t *testing.T) {
	bullets := []string{"Owner confirms reporting deadlines.", "Reviewer resolves outstanding handoffs."}
	for _, mode := range []string{"", "warn", "off", "strict"} {
		t.Run(fmt.Sprintf("mode=%q", mode), func(t *testing.T) {
			output := t.TempDir()
			mc := &mcpConfig{templatesDir: "../../templates", outputDir: output, cache: template.NewMemoryCache(24 * time.Hour)}
			deck := map[string]any{"template": "abstract", "slides": []any{map[string]any{"layout_id": "content", "content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Complete source evidence"}, map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": bullets}}}}}
			params := map[string]any{"presentation": deck, "output_filename": "complete.pptx"}
			if mode != "" {
				params["strict_fit"] = mode
			}
			result, err := mc.handleGenerate(context.Background(), makeRequest(params))
			if err != nil || result.IsError {
				t.Fatalf("fitting source refused: %v %v", err, result)
			}
			slide := readZipText(t, filepath.Join(output, "complete.pptx"), "ppt/slides/slide")
			for _, text := range append([]string{"Complete source evidence"}, bullets...) {
				if !strings.Contains(slide, text) {
					t.Fatalf("published deck omitted required source %q", text)
				}
			}
		})
	}
}
