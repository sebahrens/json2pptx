package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/types"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// goldenSlideSnapshot is a serializable representation of generator.SlideSpec
// for golden file comparison. We use this instead of SlideSpec directly because
// SlideSpec.Content[].Value is any, which loses type info on JSON round-trip.
type goldenSlideSnapshot struct {
	LayoutID        string               `json:"layout_id"`
	ContentCount    int                  `json:"content_count"`
	Content         []goldenContentItem  `json:"content"`
	SpeakerNotes    string               `json:"speaker_notes,omitempty"`
	SourceNote      string               `json:"source_note,omitempty"`
	Transition      string               `json:"transition,omitempty"`
	TransitionSpeed string               `json:"transition_speed,omitempty"`
	Build           string               `json:"build,omitempty"`
}

type goldenContentItem struct {
	PlaceholderID string `json:"placeholder_id"`
	Type          string `json:"type"`
	Value         any    `json:"value"`
}

// toGoldenSnapshot converts a SlideSpec into a deterministic, serializable snapshot.
func toGoldenSnapshot(specs []generator.SlideSpec) []goldenSlideSnapshot {
	snapshots := make([]goldenSlideSnapshot, len(specs))
	for i, s := range specs {
		snap := goldenSlideSnapshot{
			LayoutID:        s.LayoutID,
			ContentCount:    len(s.Content),
			SpeakerNotes:    s.SpeakerNotes,
			SourceNote:      s.SourceNote,
			Transition:      s.Transition,
			TransitionSpeed: s.TransitionSpeed,
			Build:           s.Build,
		}
		snap.Content = make([]goldenContentItem, len(s.Content))
		for j, c := range s.Content {
			snap.Content[j] = goldenContentItem{
				PlaceholderID: c.PlaceholderID,
				Type:          string(c.Type),
				Value:         contentValueToSerializable(c),
			}
		}
		snapshots[i] = snap
	}
	return snapshots
}

// contentValueToSerializable converts a ContentItem's Value to a JSON-safe form.
func contentValueToSerializable(c generator.ContentItem) any {
	switch c.Type {
	case generator.ContentText, generator.ContentSectionTitle, generator.ContentTitleSlideTitle:
		return c.Value
	case generator.ContentBullets:
		return c.Value
	case generator.ContentBodyAndBullets:
		bab, ok := c.Value.(generator.BodyAndBulletsContent)
		if !ok {
			return nil
		}
		return map[string]any{
			"body":          bab.Body,
			"bullets":       bab.Bullets,
			"trailing_body": bab.TrailingBody,
		}
	case generator.ContentBulletGroups:
		bg, ok := c.Value.(generator.BulletGroupsContent)
		if !ok {
			return nil
		}
		groups := make([]map[string]any, len(bg.Groups))
		for i, g := range bg.Groups {
			groups[i] = map[string]any{
				"header":  g.Header,
				"body":    g.Body,
				"bullets": g.Bullets,
			}
		}
		return map[string]any{
			"body":          bg.Body,
			"groups":        groups,
			"trailing_body": bg.TrailingBody,
		}
	case generator.ContentTable:
		spec, ok := c.Value.(*types.TableSpec)
		if !ok {
			return nil
		}
		rows := make([][]map[string]any, len(spec.Rows))
		for i, row := range spec.Rows {
			cells := make([]map[string]any, len(row))
			for j, cell := range row {
				cells[j] = map[string]any{
					"content":  cell.Content,
					"col_span": cell.ColSpan,
					"row_span": cell.RowSpan,
				}
			}
			rows[i] = cells
		}
		return map[string]any{
			"headers":           spec.Headers,
			"rows":              rows,
			"style":             map[string]any{"header_background": spec.Style.HeaderBackground, "borders": spec.Style.Borders, "striped": spec.Style.Striped},
			"column_alignments": spec.ColumnAlignments,
		}
	case generator.ContentDiagram:
		// DiagramSpec is a pointer
		if ds, ok := c.Value.(*types.DiagramSpec); ok {
			return map[string]any{
				"type":  string(ds.Type),
				"title": ds.Title,
				"data":  ds.Data,
			}
		}
		return nil
	case generator.ContentImage:
		if img, ok := c.Value.(generator.ImageContent); ok {
			return map[string]any{
				"path": img.Path,
				"alt":  img.Alt,
			}
		}
		return nil
	default:
		return nil
	}
}

// runGoldenTest reads a JSON input file, converts it through the pipeline,
// and compares the output against a golden file.
func runGoldenTest(t *testing.T, inputFile string, convertFn func(t *testing.T, data []byte) []generator.SlideSpec) {
	t.Helper()

	inputPath := filepath.Join("testdata", "json", inputFile)
	goldenPath := filepath.Join("testdata", "json", inputFile[:len(inputFile)-len(".json")]+".golden.json")

	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("failed to read input file %s: %v", inputPath, err)
	}

	specs := convertFn(t, data)
	snapshots := toGoldenSnapshot(specs)

	got, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal snapshots: %v", err)
	}

	if *updateGolden {
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatalf("failed to write golden file %s: %v", goldenPath, err)
		}
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file not found: %s (run with -update-golden to create)", goldenPath)
	}

	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n\nGot:\n%s\n\nWant:\n%s", goldenPath, string(got), string(want))
	}
}

// convertTypedPath parses PresentationInput and runs through convertPresentationContent.
func convertTypedPath(t *testing.T, data []byte) []generator.SlideSpec {
	t.Helper()

	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatalf("failed to unmarshal PresentationInput: %v", err)
	}

	specs := make([]generator.SlideSpec, 0, len(input.Slides))
	for i, slide := range input.Slides {
		contentItems, err := convertPresentationContent(slide.Content, i+1, inferSlideType(slide))
		if err != nil {
			t.Fatalf("slide %d: convertPresentationContent error: %v", i+1, err)
		}
		specs = append(specs, generator.SlideSpec{
			LayoutID:        slide.LayoutID,
			Content:         contentItems,
			SpeakerNotes:    slide.SpeakerNotes,
			SourceNote:      slide.Source,
			Transition:      slide.Transition,
			TransitionSpeed: slide.TransitionSpeed,
			Build:           slide.Build,
			ContrastCheck:   slide.ContrastCheck,
		})
	}
	return specs
}

// convertLegacyPath parses JSONInput and runs through convertJSONContent.
func convertLegacyPath(t *testing.T, data []byte) []generator.SlideSpec {
	t.Helper()

	var input JSONInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatalf("failed to unmarshal JSONInput: %v", err)
	}

	specs, err := convertJSONSlides(input.Slides)
	if err != nil {
		t.Fatalf("convertJSONSlides error: %v", err)
	}
	return specs
}

// convertMixedPath parses PresentationInput (supports both typed and legacy Value)
// and runs through convertPresentationContent.
func convertMixedPath(t *testing.T, data []byte) []generator.SlideSpec {
	t.Helper()
	return convertTypedPath(t, data)
}

// --- Golden file tests ---

func TestGolden_AllContentTypes(t *testing.T) {
	runGoldenTest(t, "all_content_types.json", convertTypedPath)
}

func TestGolden_BackwardCompat(t *testing.T) {
	runGoldenTest(t, "backward_compat.json", convertLegacyPath)
}

func TestGolden_TableCellShorthand(t *testing.T) {
	runGoldenTest(t, "table_cell_shorthand.json", convertTypedPath)
}

func TestGolden_MixedTypedLegacy(t *testing.T) {
	runGoldenTest(t, "mixed_typed_legacy.json", convertMixedPath)
}

// --- Structural assertions ---
// These tests verify specific structural properties that golden files alone
// might not catch clearly (e.g., content type preservation, slide count).

func TestGoldenStructure_AllContentTypes(t *testing.T) {
	data, err := os.ReadFile("testdata/json/all_content_types.json")
	if err != nil {
		t.Fatal(err)
	}
	specs := convertTypedPath(t, data)

	if len(specs) != 9 {
		t.Fatalf("slide count = %d, want 9", len(specs))
	}

	expectedTypes := []struct {
		slideIdx    int
		contentIdx  int
		contentType generator.ContentType
	}{
		{0, 0, generator.ContentTitleSlideTitle}, // title+subtitle only → SlideTypeTitle
		{0, 1, generator.ContentTitleSlideTitle}, // subtitle on title-type slide
		{1, 1, generator.ContentBullets},
		{2, 1, generator.ContentBodyAndBullets},
		{3, 1, generator.ContentBulletGroups},
		{4, 1, generator.ContentDiagram}, // chart -> diagram
		{5, 1, generator.ContentDiagram}, // chart -> diagram
		{6, 1, generator.ContentDiagram}, // diagram
		{7, 1, generator.ContentTable},
		{8, 1, generator.ContentImage},
	}

	for _, exp := range expectedTypes {
		if exp.slideIdx >= len(specs) {
			t.Fatalf("slide %d out of range (total %d)", exp.slideIdx, len(specs))
		}
		slide := specs[exp.slideIdx]
		if exp.contentIdx >= len(slide.Content) {
			t.Fatalf("slide %d content %d out of range (total %d)", exp.slideIdx, exp.contentIdx, len(slide.Content))
		}
		item := slide.Content[exp.contentIdx]
		if item.Type != exp.contentType {
			t.Errorf("slide %d, content %d: type = %q, want %q", exp.slideIdx, exp.contentIdx, item.Type, exp.contentType)
		}
	}
}

func TestGoldenStructure_BackwardCompat(t *testing.T) {
	data, err := os.ReadFile("testdata/json/backward_compat.json")
	if err != nil {
		t.Fatal(err)
	}
	specs := convertLegacyPath(t, data)

	if len(specs) != 8 {
		t.Fatalf("slide count = %d, want 8", len(specs))
	}

	// Verify all content types present in legacy format
	typesSeen := make(map[generator.ContentType]bool)
	for _, s := range specs {
		for _, c := range s.Content {
			typesSeen[c.Type] = true
		}
	}
	expectedTypes := []generator.ContentType{
		generator.ContentText,
		generator.ContentBullets,
		generator.ContentBodyAndBullets,
		generator.ContentBulletGroups,
		generator.ContentDiagram,
		generator.ContentTable,
		generator.ContentImage,
	}
	for _, et := range expectedTypes {
		if !typesSeen[et] {
			t.Errorf("expected content type %q not found in backward_compat output", et)
		}
	}
}

func TestGoldenStructure_TableCellShorthand(t *testing.T) {
	data, err := os.ReadFile("testdata/json/table_cell_shorthand.json")
	if err != nil {
		t.Fatal(err)
	}
	specs := convertTypedPath(t, data)

	if len(specs) != 3 {
		t.Fatalf("slide count = %d, want 3", len(specs))
	}

	// All table slides should produce ContentTable items
	for i, s := range specs {
		found := false
		for _, c := range s.Content {
			if c.Type == generator.ContentTable {
				found = true
				spec, ok := c.Value.(*types.TableSpec)
				if !ok {
					t.Errorf("slide %d: table Value type = %T, want *types.TableSpec", i, c.Value)
					continue
				}
				if len(spec.Headers) == 0 {
					t.Errorf("slide %d: table has no headers", i)
				}
				if len(spec.Rows) == 0 {
					t.Errorf("slide %d: table has no rows", i)
				}
				// Verify all cells have ColSpan >= 1 and RowSpan >= 1
				for ri, row := range spec.Rows {
					for ci, cell := range row {
						if cell.ColSpan < 1 {
							t.Errorf("slide %d, row %d, cell %d: ColSpan = %d, want >= 1", i, ri, ci, cell.ColSpan)
						}
						if cell.RowSpan < 1 {
							t.Errorf("slide %d, row %d, cell %d: RowSpan = %d, want >= 1", i, ri, ci, cell.RowSpan)
						}
					}
				}
			}
		}
		if !found {
			t.Errorf("slide %d: expected ContentTable item not found", i)
		}
	}
}

func TestGoldenStructure_MixedTypedLegacy(t *testing.T) {
	data, err := os.ReadFile("testdata/json/mixed_typed_legacy.json")
	if err != nil {
		t.Fatal(err)
	}
	specs := convertMixedPath(t, data)

	if len(specs) != 6 {
		t.Fatalf("slide count = %d, want 6", len(specs))
	}

	// Slide 0: typed title + legacy subtitle — title-type slide preserves template styling
	if specs[0].Content[0].Type != generator.ContentTitleSlideTitle {
		t.Errorf("slide 0, content 0: type = %q, want title_slide_title", specs[0].Content[0].Type)
	}
	if specs[0].Content[0].Value.(string) != "Typed Title" {
		t.Errorf("slide 0, content 0: value = %q, want 'Typed Title'", specs[0].Content[0].Value)
	}
	if specs[0].Content[1].Type != generator.ContentTitleSlideTitle {
		t.Errorf("slide 0, content 1: type = %q, want title_slide_title", specs[0].Content[1].Type)
	}
	if specs[0].Content[1].Value.(string) != "Legacy Subtitle" {
		t.Errorf("slide 0, content 1: value = %q, want 'Legacy Subtitle'", specs[0].Content[1].Value)
	}

	// Slide 1: legacy title + typed bullets
	if specs[1].Content[0].Value.(string) != "Legacy Title" {
		t.Errorf("slide 1, content 0: value = %q, want 'Legacy Title'", specs[1].Content[0].Value)
	}
	bs := specs[1].Content[1].Value.([]string)
	if len(bs) != 2 || bs[0] != "Typed bullet 1" {
		t.Errorf("slide 1, content 1: bullets = %v", bs)
	}

	// Slide 4: typed title + legacy chart (via Value)
	if specs[4].Content[1].Type != generator.ContentDiagram {
		t.Errorf("slide 4, content 1: type = %q, want diagram", specs[4].Content[1].Type)
	}
}

// TestGoldenStructure_MetadataPreserved verifies speaker notes, source, transition survive conversion.
func TestGoldenStructure_MetadataPreserved(t *testing.T) {
	data, err := os.ReadFile("testdata/json/all_content_types.json")
	if err != nil {
		t.Fatal(err)
	}
	specs := convertTypedPath(t, data)

	s := specs[0]
	if s.SpeakerNotes != "Welcome the audience and set expectations" {
		t.Errorf("SpeakerNotes = %q", s.SpeakerNotes)
	}
	if s.SourceNote != "FY2025 Annual Report" {
		t.Errorf("SourceNote = %q", s.SourceNote)
	}
	if s.Transition != "fade" {
		t.Errorf("Transition = %q", s.Transition)
	}
	if s.TransitionSpeed != "fast" {
		t.Errorf("TransitionSpeed = %q", s.TransitionSpeed)
	}
	if s.Build != "bullets" {
		t.Errorf("Build = %q", s.Build)
	}
}
