package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestSanitizeOutputFilename_PathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Path traversal attacks
		{
			name:     "relative path traversal",
			input:    "../../etc/evil.pptx",
			expected: "evil.pptx",
		},
		{
			name:     "absolute path",
			input:    "/tmp/secret.pptx",
			expected: "secret.pptx",
		},
		{
			name:     "deep traversal",
			input:    "../../../../../../../tmp/pwned.pptx",
			expected: "pwned.pptx",
		},
		{
			name:     "traversal without extension",
			input:    "../../evil",
			expected: "evil.pptx",
		},
		{
			name:     "directory only traversal",
			input:    "../../",
			expected: "output.pptx",
		},
		{
			name:     "dot-dot only",
			input:    "..",
			expected: "output.pptx",
		},
		{
			name:     "hidden file traversal",
			input:    "../.secret.pptx",
			expected: ".secret.pptx",
		},

		// Edge cases
		{
			name:     "empty string defaults to output.pptx",
			input:    "",
			expected: "output.pptx",
		},
		{
			name:     "single dot defaults to output.pptx",
			input:    ".",
			expected: "output.pptx",
		},

		// Valid filenames (should pass through)
		{
			name:     "simple filename with extension",
			input:    "presentation.pptx",
			expected: "presentation.pptx",
		},
		{
			name:     "simple filename without extension",
			input:    "myfile",
			expected: "myfile.pptx",
		},
		{
			name:     "filename with spaces",
			input:    "my presentation.pptx",
			expected: "my presentation.pptx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeOutputFilename(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeOutputFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestSanitizeOutputFilename_NeverEscapesOutputDir verifies that no matter what
// the user supplies, the resulting path stays within the output directory.
func TestSanitizeOutputFilename_NeverEscapesOutputDir(t *testing.T) {
	outputDir := "/safe/output/dir"

	malicious := []string{
		"../../etc/passwd",
		"../../../tmp/evil.pptx",
		"/absolute/path/to/file.pptx",
		"subdir/../../../escape.pptx",
		"../sibling.pptx",
		"",
		".",
		"..",
		"../../",
	}

	for _, input := range malicious {
		t.Run(input, func(t *testing.T) {
			filename := sanitizeOutputFilename(input)
			fullPath := filepath.Join(outputDir, filename)

			// The resulting path must start with the output directory
			if !strings.HasPrefix(fullPath, outputDir+string(filepath.Separator)) {
				t.Errorf("path escaped output dir: sanitizeOutputFilename(%q) produced %q, full path %q", input, filename, fullPath)
			}

			// The filename must not contain any path separator
			if strings.ContainsRune(filename, filepath.Separator) {
				t.Errorf("filename contains path separator: sanitizeOutputFilename(%q) = %q", input, filename)
			}

			// The filename must end with .pptx
			if !strings.HasSuffix(filename, ".pptx") {
				t.Errorf("filename missing .pptx suffix: sanitizeOutputFilename(%q) = %q", input, filename)
			}
		})
	}
}

// --- convertJSONContent tests ---

func TestConvertJSONContent_TableWithStyle(t *testing.T) {
	tableJSON := `{"headers":["Name","Score"],"rows":[["Alice","95"],["Bob","87"]],"style":{"header_background":"accent3","borders":"horizontal"}}`
	items, err := convertJSONContent([]JSONContentItem{
		{PlaceholderID: "body", Type: "table", Value: json.RawMessage(tableJSON)},
	}, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Type != generator.ContentTable {
		t.Errorf("Type = %v, want ContentTable", items[0].Type)
	}
	spec, ok := items[0].Value.(*types.TableSpec)
	if !ok {
		t.Fatalf("Value type = %T, want *types.TableSpec", items[0].Value)
	}
	if len(spec.Headers) != 2 || spec.Headers[0] != "Name" {
		t.Errorf("Headers = %v, want [Name Score]", spec.Headers)
	}
	if len(spec.Rows) != 2 {
		t.Errorf("len(Rows) = %d, want 2", len(spec.Rows))
	}
	if spec.Style.HeaderBackground != "accent3" {
		t.Errorf("Style.HeaderBackground = %q, want accent3", spec.Style.HeaderBackground)
	}
}

func TestConvertJSONContent_BodyAndBulletsWithTrailing(t *testing.T) {
	babJSON := `{"body":"Intro paragraph","bullets":["Point 1","Point 2"],"trailing_body":"Summary"}`
	items, err := convertJSONContent([]JSONContentItem{
		{PlaceholderID: "body", Type: "body_and_bullets", Value: json.RawMessage(babJSON)},
	}, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent error: %v", err)
	}
	if items[0].Type != generator.ContentBodyAndBullets {
		t.Errorf("Type = %v, want ContentBodyAndBullets", items[0].Type)
	}
	bab, ok := items[0].Value.(generator.BodyAndBulletsContent)
	if !ok {
		t.Fatalf("Value type = %T, want generator.BodyAndBulletsContent", items[0].Value)
	}
	if bab.Body != "Intro paragraph" {
		t.Errorf("Body = %q, want 'Intro paragraph'", bab.Body)
	}
	if len(bab.Bullets) != 2 || bab.Bullets[0] != "Point 1" {
		t.Errorf("Bullets = %v, want [Point 1 Point 2]", bab.Bullets)
	}
	if bab.TrailingBody != "Summary" {
		t.Errorf("TrailingBody = %q, want Summary", bab.TrailingBody)
	}
}

func TestConvertJSONContent_BulletGroupsWithBody(t *testing.T) {
	bgJSON := `{"body":"Overview","groups":[{"header":"Phase 1","body":"Setup","bullets":["Install","Configure"]},{"header":"Phase 2","bullets":["Deploy"]}],"trailing_body":"Done"}`
	items, err := convertJSONContent([]JSONContentItem{
		{PlaceholderID: "body", Type: "bullet_groups", Value: json.RawMessage(bgJSON)},
	}, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent error: %v", err)
	}
	if items[0].Type != generator.ContentBulletGroups {
		t.Errorf("Type = %v, want ContentBulletGroups", items[0].Type)
	}
	bg, ok := items[0].Value.(generator.BulletGroupsContent)
	if !ok {
		t.Fatalf("Value type = %T, want generator.BulletGroupsContent", items[0].Value)
	}
	if bg.Body != "Overview" {
		t.Errorf("Body = %q, want Overview", bg.Body)
	}
	if len(bg.Groups) != 2 {
		t.Fatalf("len(Groups) = %d, want 2", len(bg.Groups))
	}
	if bg.Groups[0].Header != "Phase 1" {
		t.Errorf("Groups[0].Header = %q, want 'Phase 1'", bg.Groups[0].Header)
	}
	if bg.Groups[0].Body != "Setup" {
		t.Errorf("Groups[0].Body = %q, want Setup", bg.Groups[0].Body)
	}
	if len(bg.Groups[0].Bullets) != 2 {
		t.Errorf("Groups[0].Bullets = %v, want [Install Configure]", bg.Groups[0].Bullets)
	}
	if bg.TrailingBody != "Done" {
		t.Errorf("TrailingBody = %q, want Done", bg.TrailingBody)
	}
}

func TestConvertJSONSlides_MetadataFields(t *testing.T) {
	slides := []JSONSlide{
		{
			LayoutID:        "slideLayout5",
			SpeakerNotes:    "Remember to mention the deadline",
			Source:          "Q4 2025 Report",
			Transition:      "wipe",
			TransitionSpeed: "slow",
			Build:           "bullets",
			Content: []JSONContentItem{
				{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"Test Slide"`)},
			},
		},
	}

	specs, err := convertJSONSlides(slides)
	if err != nil {
		t.Fatalf("convertJSONSlides error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("len(specs) = %d, want 1", len(specs))
	}

	s := specs[0]
	if s.LayoutID != "slideLayout5" {
		t.Errorf("LayoutID = %q, want slideLayout5", s.LayoutID)
	}
	if s.SpeakerNotes != "Remember to mention the deadline" {
		t.Errorf("SpeakerNotes = %q", s.SpeakerNotes)
	}
	if s.SourceNote != "Q4 2025 Report" {
		t.Errorf("SourceNote = %q, want 'Q4 2025 Report'", s.SourceNote)
	}
	if s.Transition != "wipe" {
		t.Errorf("Transition = %q, want wipe", s.Transition)
	}
	if s.TransitionSpeed != "slow" {
		t.Errorf("TransitionSpeed = %q, want slow", s.TransitionSpeed)
	}
	if s.Build != "bullets" {
		t.Errorf("Build = %q, want bullets", s.Build)
	}
}

func TestConvertJSONContent_LegacyTextBulletsUnchanged(t *testing.T) {
	items, err := convertJSONContent([]JSONContentItem{
		{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"Hello"`)},
		{PlaceholderID: "body", Type: "bullets", Value: json.RawMessage(`["a","b","c"]`)},
	}, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}

	// Text
	if items[0].Type != generator.ContentText {
		t.Errorf("items[0].Type = %v, want ContentText", items[0].Type)
	}
	if items[0].Value.(string) != "Hello" {
		t.Errorf("items[0].Value = %v, want Hello", items[0].Value)
	}

	// Bullets
	if items[1].Type != generator.ContentBullets {
		t.Errorf("items[1].Type = %v, want ContentBullets", items[1].Type)
	}
	bs := items[1].Value.([]string)
	if len(bs) != 3 || bs[0] != "a" {
		t.Errorf("items[1].Value = %v, want [a b c]", bs)
	}
}

func TestConvertJSONContent_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		content []JSONContentItem
	}{
		{
			name:    "missing placeholder_id",
			content: []JSONContentItem{{Type: "text", Value: json.RawMessage(`"hi"`)}},
		},
		{
			name:    "missing type",
			content: []JSONContentItem{{PlaceholderID: "p1", Value: json.RawMessage(`"hi"`)}},
		},
		{
			name:    "unknown type",
			content: []JSONContentItem{{PlaceholderID: "p1", Type: "video", Value: json.RawMessage(`"hi"`)}},
		},
		{
			name:    "invalid text value",
			content: []JSONContentItem{{PlaceholderID: "p1", Type: "text", Value: json.RawMessage(`[1,2]`)}},
		},
		{
			name:    "invalid bullets value",
			content: []JSONContentItem{{PlaceholderID: "p1", Type: "bullets", Value: json.RawMessage(`"not an array"`)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertJSONContent(tt.content, 1, types.SlideTypeContent)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// --- computeQualityScore tests (using SlideInput/ContentInput) ---

func TestComputeQualityScore_TableContent(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"Good Title"`)},
				{PlaceholderID: "body", Type: "table", Value: json.RawMessage(`{"headers":["A","B"],"rows":[["1","2"]]}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score == nil {
		t.Fatal("score is nil")
	}
	if score.Score < 0.9 {
		t.Errorf("expected high score for valid table, got %f", score.Score)
	}
}

func TestComputeQualityScore_EmptyTable(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "table", Value: json.RawMessage(`{"headers":[],"rows":[]}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score == nil {
		t.Fatal("score is nil")
	}
	// Should have issues for empty table
	if len(score.SlideScores) != 1 {
		t.Fatalf("len(SlideScores) = %d, want 1", len(score.SlideScores))
	}
	if len(score.SlideScores[0].Issues) == 0 {
		t.Error("expected issues for empty table, got none")
	}
}

func TestComputeQualityScore_SubtitleLengthLimit(t *testing.T) {
	// Subtitles up to 120 chars should not be penalized
	longSubtitle := strings.Repeat("x", 100) // 100 chars — within limit
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"Short Title"`)},
				{PlaceholderID: "subtitle", Type: "text", Value: json.RawMessage(fmt.Sprintf("%q", longSubtitle))},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score == nil {
		t.Fatal("score is nil")
	}
	for _, iss := range score.SlideScores[0].Issues {
		if strings.Contains(iss, "subtitle too long") || strings.Contains(iss, "title too long") {
			t.Errorf("unexpected length penalty for 100-char subtitle: %s", iss)
		}
	}

	// Subtitles over 120 chars should be penalized as subtitle, not title
	veryLongSubtitle := strings.Repeat("y", 130) // 130 chars — over limit
	slides2 := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "subtitle", Type: "text", Value: json.RawMessage(fmt.Sprintf("%q", veryLongSubtitle))},
			},
		},
	}
	score2 := computeQualityScore(slides2, nil)
	if score2 == nil {
		t.Fatal("score2 is nil")
	}
	foundSubtitleIssue := false
	for _, iss := range score2.SlideScores[0].Issues {
		if strings.Contains(iss, "subtitle too long") {
			foundSubtitleIssue = true
		}
		if strings.HasPrefix(iss, "title too long") {
			t.Errorf("subtitle penalized as title: %s", iss)
		}
	}
	if !foundSubtitleIssue {
		t.Error("expected 'subtitle too long' issue for 130-char subtitle, got none")
	}
}

func TestComputeQualityScore_BodyAndBullets(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "body_and_bullets", Value: json.RawMessage(`{"body":"intro","bullets":["a","b","c"]}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score < 0.8 {
		t.Errorf("expected good score for body_and_bullets, got %f", score.Score)
	}
}

func TestComputeQualityScore_BulletGroups(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "bullet_groups", Value: json.RawMessage(`{"groups":[{"header":"G1","bullets":["a","b"]},{"header":"G2","bullets":["c","d"]}]}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score < 0.8 {
		t.Errorf("expected good score for bullet_groups, got %f", score.Score)
	}
}

func TestComputeQualityScore_TooManyBullets(t *testing.T) {
	// Over maxBullets (8) should penalize
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "bullets", Value: json.RawMessage(`["1","2","3","4","5","6","7","8","9","10"]`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score >= 1.0 {
		t.Errorf("expected penalty for 10 bullets, got perfect score %f", score.Score)
	}
	found := false
	for _, issue := range score.SlideScores[0].Issues {
		if strings.Contains(issue, "too many bullets") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'too many bullets' issue")
	}
}

func TestComputeQualityScore_EmptySlides(t *testing.T) {
	score := computeQualityScore(nil, nil)
	if score.Score != 0.0 {
		t.Errorf("Score = %f, want 0.0 for no slides", score.Score)
	}
}

func TestComputeQualityScore_Warnings(t *testing.T) {
	slides := []SlideInput{
		{LayoutID: "l1", Content: []ContentInput{{PlaceholderID: "t", Type: "text", Value: json.RawMessage(`"hi"`)}}},
	}
	score := computeQualityScore(slides, []string{"warning1", "warning2"})
	if score.Score >= 1.0 {
		t.Errorf("expected penalty for warnings, got %f", score.Score)
	}
}

// --- chart/diagram data structure checks ---

func TestComputeQualityScore_WaterfallMissingPoints(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "chart", Value: json.RawMessage(`{"type":"waterfall","data":{}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score >= 1.0 {
		t.Errorf("expected penalty for waterfall without points, got %f", score.Score)
	}
	found := false
	for _, issue := range score.SlideScores[0].Issues {
		if strings.Contains(issue, "waterfall") && strings.Contains(issue, "points") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected waterfall 'points' issue, got %v", score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_WaterfallWithPoints(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "chart", Value: json.RawMessage(`{"type":"waterfall","data":{"points":[{"label":"Start","value":100,"type":"total"}]}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score < 0.9 {
		t.Errorf("expected high score for valid waterfall, got %f (issues: %v)", score.Score, score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_FunnelMissingStages(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "diagram", Value: json.RawMessage(`{"type":"funnel_chart","data":{}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score >= 1.0 {
		t.Errorf("expected penalty for funnel without stages, got %f", score.Score)
	}
	found := false
	for _, issue := range score.SlideScores[0].Issues {
		if strings.Contains(issue, "funnel") && strings.Contains(issue, "stages") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected funnel 'stages' issue, got %v", score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_FunnelWithStages(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "diagram", Value: json.RawMessage(`{"type":"funnel_chart","data":{"stages":[{"label":"Top","value":1000}]}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score < 0.9 {
		t.Errorf("expected high score for valid funnel, got %f (issues: %v)", score.Score, score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_GaugeMissingValue(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "diagram", Value: json.RawMessage(`{"type":"gauge_chart","data":{"min":0,"max":100}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	found := false
	for _, issue := range score.SlideScores[0].Issues {
		if strings.Contains(issue, "gauge") && strings.Contains(issue, "value") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected gauge 'value' issue, got %v", score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_GaugeMissingMinMax(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "diagram", Value: json.RawMessage(`{"type":"gauge_chart","data":{"value":75}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	found := false
	for _, issue := range score.SlideScores[0].Issues {
		if strings.Contains(issue, "gauge") && strings.Contains(issue, "min") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected gauge 'min/max' issue, got %v", score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_GaugeComplete(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "diagram", Value: json.RawMessage(`{"type":"gauge_chart","data":{"value":75,"min":0,"max":100}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score < 0.9 {
		t.Errorf("expected high score for valid gauge, got %f (issues: %v)", score.Score, score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_PortersMissingForces(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "diagram", Value: json.RawMessage(`{"type":"porters_five_forces","data":{}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	found := false
	for _, issue := range score.SlideScores[0].Issues {
		if strings.Contains(issue, "porter") && strings.Contains(issue, "forces") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected porter's 'forces' issue, got %v", score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_PortersWithForces(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "diagram", Value: json.RawMessage(`{"type":"porters_five_forces","data":{"forces":[{"type":"rivalry","intensity":0.8}]}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score < 0.9 {
		t.Errorf("expected high score for valid porter's, got %f (issues: %v)", score.Score, score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_PortersDirectKeys(t *testing.T) {
	// Porter's data with direct force keys (no "forces" array)
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "diagram", Value: json.RawMessage(`{"type":"porters_five_forces","data":{"rivalry":"High","new_entrants":"Medium"}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score < 0.9 {
		t.Errorf("expected high score for porter's with direct keys, got %f (issues: %v)", score.Score, score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_ChartAliasHandling(t *testing.T) {
	// "funnel" alias should be recognized as funnel_chart
	slides := []SlideInput{
		{
			LayoutID: "layout1",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "diagram", Value: json.RawMessage(`{"type":"funnel","data":{"stages":[{"label":"A","value":100}]}}`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score.Score < 0.9 {
		t.Errorf("expected high score for funnel alias with data, got %f (issues: %v)", score.Score, score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_SectionDividerBodyMisuse(t *testing.T) {
	// Long non-numeric body text on a section divider should trigger a warning.
	slides := []SlideInput{
		{
			SlideType: "section",
			LayoutID:  "Section Divider",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"Market Overview"`)},
				{PlaceholderID: "body", Type: "text", Value: json.RawMessage(`"This section covers our key market insights and strategic priorities"`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score == nil {
		t.Fatal("score is nil")
	}
	if len(score.SlideScores) != 1 {
		t.Fatalf("len(SlideScores) = %d, want 1", len(score.SlideScores))
	}
	found := false
	for _, issue := range score.SlideScores[0].Issues {
		if strings.Contains(issue, "section divider body misused") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected section divider body misuse warning, got issues: %v", score.SlideScores[0].Issues)
	}
}

func TestComputeQualityScore_SectionDividerShortNumeric(t *testing.T) {
	// Short numeric body text (e.g., "01") on a section divider should NOT trigger a warning.
	slides := []SlideInput{
		{
			SlideType: "section",
			LayoutID:  "Section Divider",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"Introduction"`)},
				{PlaceholderID: "body", Type: "text", Value: json.RawMessage(`"01"`)},
			},
		},
	}
	score := computeQualityScore(slides, nil)
	if score == nil {
		t.Fatal("score is nil")
	}
	if len(score.SlideScores) != 1 {
		t.Fatalf("len(SlideScores) = %d, want 1", len(score.SlideScores))
	}
	for _, issue := range score.SlideScores[0].Issues {
		if strings.Contains(issue, "section divider body misused") {
			t.Errorf("unexpected section divider body misuse warning for numeric body '01'; issues: %v", score.SlideScores[0].Issues)
		}
	}
}

// --- isLikelyTitle ---

func TestIsLikelyTitle(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"title", true},
		{"Title", true},
		{"slide_title", true},
		{"heading1", true},
		{"body", false},
		{"subtitle", false}, // subtitle is not a title
		{"chart", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := isLikelyTitle(tt.id); got != tt.want {
				t.Errorf("isLikelyTitle(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

// --- isLikelySubtitle ---

func TestIsLikelySubtitle(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"subtitle", true},
		{"Subtitle", true},
		{"slide_subtitle", true},
		{"subheading", true},
		{"title", false},
		{"body", false},
		{"heading1", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := isLikelySubtitle(tt.id); got != tt.want {
				t.Errorf("isLikelySubtitle(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

// --- inferSlideType ---

func TestInferSlideType(t *testing.T) {
	tests := []struct {
		name  string
		slide SlideInput
		want  types.SlideType
	}{
		{
			name:  "explicit slide_type overrides inference",
			slide: SlideInput{SlideType: "section"},
			want:  types.SlideTypeSection,
		},
		{
			name: "chart content infers chart type",
			slide: SlideInput{
				Content: []ContentInput{
					{Type: "text", TextValue: strPtr("Revenue Growth")},
					{Type: "chart"},
				},
			},
			want: types.SlideTypeChart,
		},
		{
			name: "diagram content infers diagram type",
			slide: SlideInput{
				Content: []ContentInput{
					{Type: "text", TextValue: strPtr("Architecture")},
					{Type: "diagram"},
				},
			},
			want: types.SlideTypeDiagram,
		},
		{
			name: "image plus text infers two-column",
			slide: SlideInput{
				Content: []ContentInput{
					{Type: "text", TextValue: strPtr("Photo Caption")},
					{Type: "image"},
				},
			},
			want: types.SlideTypeTwoColumn,
		},
		{
			name: "image only infers image type",
			slide: SlideInput{
				Content: []ContentInput{
					{Type: "image"},
				},
			},
			want: types.SlideTypeImage,
		},
		{
			name: "table infers content type",
			slide: SlideInput{
				Content: []ContentInput{
					{Type: "text", TextValue: strPtr("Data")},
					{Type: "table"},
				},
			},
			want: types.SlideTypeContent,
		},
		{
			name: "single text item infers title",
			slide: SlideInput{
				Content: []ContentInput{
					{Type: "text", TextValue: strPtr("Welcome")},
				},
			},
			want: types.SlideTypeTitle,
		},
		{
			name: "text plus bullets infers content",
			slide: SlideInput{
				Content: []ContentInput{
					{Type: "text", TextValue: strPtr("Overview")},
					{Type: "bullets", BulletsValue: &[]string{"a", "b"}},
				},
			},
			want: types.SlideTypeContent,
		},
		{
			name:  "empty slide infers title",
			slide: SlideInput{Content: []ContentInput{}},
			want:  types.SlideTypeTitle,
		},
		{
			name: "title plus subtitle infers title (closing slide)",
			slide: SlideInput{
				Content: []ContentInput{
					{Type: "text", PlaceholderID: "title", TextValue: strPtr("Thank You")},
					{Type: "text", PlaceholderID: "subtitle", TextValue: strPtr("Questions?")},
				},
			},
			want: types.SlideTypeTitle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferSlideType(tt.slide)
			if got != tt.want {
				t.Errorf("inferSlideType() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- jsonSlideToDefinition ---

func TestJsonSlideToDefinition(t *testing.T) {
	t.Run("text items map to title then body", func(t *testing.T) {
		slide := SlideInput{
			Content: []ContentInput{
				{Type: "text", TextValue: strPtr("My Title")},
				{Type: "text", TextValue: strPtr("Subtitle text")},
			},
		}
		def := jsonSlideToDefinition(slide)
		if def.Title != "My Title" {
			t.Errorf("Title = %q, want 'My Title'", def.Title)
		}
		if def.Content.Body != "Subtitle text" {
			t.Errorf("Body = %q, want 'Subtitle text'", def.Content.Body)
		}
	})

	t.Run("bullets content populates Bullets", func(t *testing.T) {
		slide := SlideInput{
			Content: []ContentInput{
				{Type: "bullets", BulletsValue: &[]string{"item1", "item2"}},
			},
		}
		def := jsonSlideToDefinition(slide)
		if len(def.Content.Bullets) != 2 {
			t.Errorf("Bullets count = %d, want 2", len(def.Content.Bullets))
		}
	})

	t.Run("body_and_bullets populates Body and Bullets", func(t *testing.T) {
		slide := SlideInput{
			Content: []ContentInput{
				{Type: "body_and_bullets", BodyAndBulletsValue: &BodyAndBulletsInput{
					Body:         "Intro",
					Bullets:      []string{"a", "b"},
					TrailingBody: "Summary",
				}},
			},
		}
		def := jsonSlideToDefinition(slide)
		if def.Content.Body != "Intro" {
			t.Errorf("Body = %q, want 'Intro'", def.Content.Body)
		}
		if len(def.Content.Bullets) != 2 {
			t.Errorf("Bullets count = %d, want 2", len(def.Content.Bullets))
		}
	})

	t.Run("table sets TableRaw signal", func(t *testing.T) {
		slide := SlideInput{
			Content: []ContentInput{
				{Type: "table", TableValue: &TableInput{Headers: []string{"A"}}},
			},
		}
		def := jsonSlideToDefinition(slide)
		if def.Content.TableRaw == "" {
			t.Error("TableRaw should be non-empty for table content")
		}
	})

	t.Run("image sets ImagePath", func(t *testing.T) {
		slide := SlideInput{
			Content: []ContentInput{
				{Type: "image", ImageValue: &ImageInput{Path: "/img/photo.png"}},
			},
		}
		def := jsonSlideToDefinition(slide)
		if def.Content.ImagePath != "/img/photo.png" {
			t.Errorf("ImagePath = %q, want '/img/photo.png'", def.Content.ImagePath)
		}
	})

	t.Run("slot markers populate Slots map for HasSlots", func(t *testing.T) {
		slide := SlideInput{
			SlideType: "two-column",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Title")},
				{PlaceholderID: "slot1", Type: "chart", ChartValue: &types.ChartSpec{Type: "bar", Title: "Revenue"}},
				{PlaceholderID: "slot2", Type: "bullets", BulletsValue: &[]string{"a", "b"}},
			},
		}
		def := jsonSlideToDefinition(slide)
		if !def.HasSlots() {
			t.Fatal("HasSlots() = false, want true for slide with slot1/slot2 content items")
		}
		if len(def.Slots) != 2 {
			t.Errorf("Slots count = %d, want 2", len(def.Slots))
		}
		if def.Slots[1] == nil || def.Slots[1].Type != types.SlotContentChart {
			t.Errorf("Slots[1].Type = %v, want SlotContentChart", def.Slots[1])
		}
		if def.Slots[2] == nil || def.Slots[2].Type != types.SlotContentBullets {
			t.Errorf("Slots[2].Type = %v, want SlotContentBullets", def.Slots[2])
		}
	})

	t.Run("non-slot content does not populate Slots", func(t *testing.T) {
		slide := SlideInput{
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Title")},
				{PlaceholderID: "body", Type: "bullets", BulletsValue: &[]string{"a"}},
			},
		}
		def := jsonSlideToDefinition(slide)
		if def.HasSlots() {
			t.Error("HasSlots() = true, want false for slide without slot markers")
		}
	})

	t.Run("two distinct content placeholders synthesize slots", func(t *testing.T) {
		// Regression for go-slide-creator-6knv: a two-column slide using
		// body + body_2 (no ::slotN:: markers) must be modeled as multi-slot so
		// layout selection requires a layout with two content placeholders
		// instead of collapsing both columns into one.
		slide := SlideInput{
			SlideType: "two-column",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Title")},
				{PlaceholderID: "body", Type: "bullets", BulletsValue: &[]string{"L1", "L2", "L3"}},
				{PlaceholderID: "body_2", Type: "bullets", BulletsValue: &[]string{"R1", "R2", "R3"}},
			},
		}
		def := jsonSlideToDefinition(slide)
		if !def.HasSlots() {
			t.Fatal("HasSlots() = false, want true for body + body_2 content")
		}
		if len(def.Slots) != 2 {
			t.Errorf("Slots count = %d, want 2", len(def.Slots))
		}
	})

	t.Run("left and right aliases synthesize slots", func(t *testing.T) {
		slide := SlideInput{
			SlideType: "comparison",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Title")},
				{PlaceholderID: "left", Type: "bullets", BulletsValue: &[]string{"a"}},
				{PlaceholderID: "right", Type: "bullets", BulletsValue: &[]string{"b"}},
			},
		}
		def := jsonSlideToDefinition(slide)
		if len(def.Slots) != 2 {
			t.Errorf("Slots count = %d, want 2 for left + right content", len(def.Slots))
		}
	})

	t.Run("title plus subtitle does not synthesize slots", func(t *testing.T) {
		// Title-area placeholders are chrome, not content columns: a title slide
		// with title + subtitle must not be treated as a two-slot slide.
		slide := SlideInput{
			SlideType: "title",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Title")},
				{PlaceholderID: "subtitle", Type: "text", TextValue: strPtr("Subtitle")},
			},
		}
		def := jsonSlideToDefinition(slide)
		if def.HasSlots() {
			t.Error("HasSlots() = true, want false for title + subtitle")
		}
	})

	t.Run("pattern slide models as full-area visual type", func(t *testing.T) {
		// Regression for go-slide-creator-ced3: a pattern slide whose content
		// array carries only a title would otherwise infer SlideTypeTitle, letting
		// the heuristic assign a Section Divider layout whose tiny content area the
		// expanded grid overflows. Model it as a full-area visual slide instead.
		slide := SlideInput{
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Our Key Metrics")},
			},
			Pattern: &PatternInput{Name: "kpi-4up"},
		}
		def := jsonSlideToDefinition(slide)
		if def.Type != types.SlideTypeDiagram {
			t.Errorf("Type = %q, want %q for pattern slide", def.Type, types.SlideTypeDiagram)
		}
	})

	t.Run("compose slide models as full-area visual type", func(t *testing.T) {
		slide := SlideInput{
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Overview")},
			},
			Compose: &ComposeInput{Direction: "vertical"},
		}
		def := jsonSlideToDefinition(slide)
		if def.Type != types.SlideTypeDiagram {
			t.Errorf("Type = %q, want %q for compose slide", def.Type, types.SlideTypeDiagram)
		}
	})

	t.Run("shape_grid slide models as full-area visual type", func(t *testing.T) {
		slide := SlideInput{
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Section Overview")},
			},
			ShapeGrid: &ShapeGridInput{},
		}
		def := jsonSlideToDefinition(slide)
		if def.Type != types.SlideTypeDiagram {
			t.Errorf("Type = %q, want %q for shape_grid slide", def.Type, types.SlideTypeDiagram)
		}
	})

	t.Run("explicit slide_type on pattern slide is respected", func(t *testing.T) {
		slide := SlideInput{
			SlideType: "section",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Part Two")},
			},
			Pattern: &PatternInput{Name: "agenda"},
		}
		def := jsonSlideToDefinition(slide)
		if def.Type != types.SlideTypeSection {
			t.Errorf("Type = %q, want %q (explicit slide_type must win)", def.Type, types.SlideTypeSection)
		}
	})
}

// --- autoMapPlaceholders ---

func TestAutoMapPlaceholders(t *testing.T) {
	contentLayout := types.LayoutMetadata{
		ID:   "slideLayout5",
		Name: "Content",
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Index: 1},
			{ID: "body", Type: types.PlaceholderBody, Index: 2},
		},
		Tags: []string{"content"},
	}

	chartLayout := types.LayoutMetadata{
		ID:   "slideLayout7",
		Name: "Chart",
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Index: 1},
			{ID: "chart", Type: types.PlaceholderChart, Index: 3},
			{ID: "body", Type: types.PlaceholderBody, Index: 2},
		},
		Tags: []string{"content"},
	}

	t.Run("text items get title then body", func(t *testing.T) {
		items := []ContentInput{
			{Type: "text", TextValue: strPtr("Title")},
			{Type: "text", TextValue: strPtr("Body text")},
		}
		mapped := autoMapPlaceholders(items, contentLayout)
		if mapped[0].PlaceholderID != "title" {
			t.Errorf("first text PlaceholderID = %q, want 'title'", mapped[0].PlaceholderID)
		}
		if mapped[1].PlaceholderID != "body" {
			t.Errorf("second text PlaceholderID = %q, want 'body'", mapped[1].PlaceholderID)
		}
	})

	t.Run("bullets get body placeholder", func(t *testing.T) {
		items := []ContentInput{
			{Type: "bullets", BulletsValue: &[]string{"a", "b"}},
		}
		mapped := autoMapPlaceholders(items, contentLayout)
		if mapped[0].PlaceholderID != "body" {
			t.Errorf("bullets PlaceholderID = %q, want 'body'", mapped[0].PlaceholderID)
		}
	})

	t.Run("chart gets chart placeholder when available", func(t *testing.T) {
		items := []ContentInput{
			{Type: "chart"},
		}
		mapped := autoMapPlaceholders(items, chartLayout)
		if mapped[0].PlaceholderID != "chart" {
			t.Errorf("chart PlaceholderID = %q, want 'chart'", mapped[0].PlaceholderID)
		}
	})

	t.Run("chart falls back to body when no chart placeholder", func(t *testing.T) {
		items := []ContentInput{
			{Type: "chart"},
		}
		mapped := autoMapPlaceholders(items, contentLayout)
		if mapped[0].PlaceholderID != "body" {
			t.Errorf("chart PlaceholderID = %q, want 'body'", mapped[0].PlaceholderID)
		}
	})

	t.Run("explicit placeholder_id is preserved", func(t *testing.T) {
		items := []ContentInput{
			{Type: "text", PlaceholderID: "custom", TextValue: strPtr("Keep this")},
		}
		mapped := autoMapPlaceholders(items, contentLayout)
		if mapped[0].PlaceholderID != "custom" {
			t.Errorf("PlaceholderID = %q, want 'custom' (should be preserved)", mapped[0].PlaceholderID)
		}
	})

	t.Run("virtual slot IDs resolve to two-column placeholders", func(t *testing.T) {
		twoColLayout := types.LayoutMetadata{
			ID:   "slideLayout10",
			Name: "Two Content",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, Index: 1},
				{ID: "left", Type: types.PlaceholderBody, Index: 2},
				{ID: "right", Type: types.PlaceholderBody, Index: 3},
			},
			Tags: []string{"two_content"},
		}
		items := []ContentInput{
			{Type: "bullets", PlaceholderID: "slot1", BulletsValue: &[]string{"a", "b"}},
			{Type: "chart", PlaceholderID: "slot2"},
		}
		mapped := autoMapPlaceholders(items, twoColLayout)
		if mapped[0].PlaceholderID != "left" {
			t.Errorf("slot1 PlaceholderID = %q, want 'left' (first content placeholder)", mapped[0].PlaceholderID)
		}
		if mapped[1].PlaceholderID != "right" {
			t.Errorf("slot2 PlaceholderID = %q, want 'right' (second content placeholder)", mapped[1].PlaceholderID)
		}
	})

	t.Run("virtual slot IDs resolve to three-column placeholders", func(t *testing.T) {
		threeColLayout := types.LayoutMetadata{
			ID:   "slideLayout12",
			Name: "Three Content",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, Index: 0},
				{ID: "body", Type: types.PlaceholderBody, Index: 1},
				{ID: "body_2", Type: types.PlaceholderBody, Index: 2},
				{ID: "body_3", Type: types.PlaceholderBody, Index: 3},
			},
			Tags: []string{"three_content"},
		}
		items := []ContentInput{
			{Type: "bullets", PlaceholderID: "slot1", BulletsValue: &[]string{"Column 1"}},
			{Type: "bullets", PlaceholderID: "slot2", BulletsValue: &[]string{"Column 2"}},
			{Type: "bullets", PlaceholderID: "slot3", BulletsValue: &[]string{"Column 3"}},
		}
		mapped := autoMapPlaceholders(items, threeColLayout)
		if mapped[0].PlaceholderID != "body" {
			t.Errorf("slot1 PlaceholderID = %q, want 'body' (first content placeholder)", mapped[0].PlaceholderID)
		}
		if mapped[1].PlaceholderID != "body_2" {
			t.Errorf("slot2 PlaceholderID = %q, want 'body_2' (second content placeholder)", mapped[1].PlaceholderID)
		}
		if mapped[2].PlaceholderID != "body_3" {
			t.Errorf("slot3 PlaceholderID = %q, want 'body_3' (third content placeholder)", mapped[2].PlaceholderID)
		}
	})
}

// --- convertPresentationSlides with auto-layout ---

func TestConvertPresentationSlides_AutoLayout(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			ID:   "slideLayout1",
			Name: "Title Slide",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, Index: 1},
				{ID: "subtitle", Type: types.PlaceholderSubtitle, Index: 2},
			},
			Tags: []string{"title-slide"},
		},
		{
			ID:   "slideLayout5",
			Name: "Content",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, Index: 1},
				{ID: "body", Type: types.PlaceholderBody, Index: 2, MaxChars: 500},
			},
			Capacity: types.CapacityEstimate{MaxBullets: 6},
			Tags:     []string{"content"},
		},
		{
			ID:   "slideLayout9",
			Name: "Blank + Title",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, Index: 1},
			},
			Tags: []string{"blank-title", "virtual-base"},
		},
	}

	t.Run("auto-selects layout when layout_id is empty", func(t *testing.T) {
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{Type: "text", TextValue: strPtr("Quarterly Results")},
					{Type: "bullets", BulletsValue: &[]string{"Revenue up 20%", "Costs down 5%"}},
				},
			},
		}
		specs, _, _, err := convertPresentationSlides(slides, layouts, 0, 0, nil, nil, "", nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(specs) != 1 {
			t.Fatalf("len(specs) = %d, want 1", len(specs))
		}
		if specs[0].LayoutID == "" {
			t.Error("LayoutID should be auto-selected, got empty")
		}
	})

	t.Run("explicit layout_id is preserved", func(t *testing.T) {
		slides := []SlideInput{
			{
				LayoutID: "slideLayout1",
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: strPtr("Hello")},
				},
			},
		}
		specs, _, _, err := convertPresentationSlides(slides, layouts, 0, 0, nil, nil, "", nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if specs[0].LayoutID != "slideLayout1" {
			t.Errorf("LayoutID = %q, want 'slideLayout1'", specs[0].LayoutID)
		}
	})

	t.Run("canonical explicit layout_id is resolved", func(t *testing.T) {
		slides := []SlideInput{
			{
				LayoutID: "blank",
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: strPtr("Hello")},
				},
			},
		}
		specs, _, _, err := convertPresentationSlides(slides, layouts, 0, 0, nil, nil, "", nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if specs[0].LayoutID != "slideLayout9" {
			t.Errorf("LayoutID = %q, want 'slideLayout9'", specs[0].LayoutID)
		}
	})

	t.Run("error when no layouts and no layout_id", func(t *testing.T) {
		slides := []SlideInput{
			{Content: []ContentInput{{Type: "text", TextValue: strPtr("Hi")}}},
		}
		_, _, _, err := convertPresentationSlides(slides, nil, 0, 0, nil, nil, "", nil, false)
		if err == nil {
			t.Error("expected error when layout_id missing and no layouts, got nil")
		}
	})

	t.Run("partial mode skips failing slides", func(t *testing.T) {
		slides := []SlideInput{
			// Slide 1: valid
			{LayoutID: "slideLayout1", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("OK")}}},
			// Slide 2: invalid — no layout_id and no layouts for auto-select
			{Content: []ContentInput{{Type: "text", TextValue: strPtr("Fail")}}},
			// Slide 3: valid
			{LayoutID: "slideLayout1", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Also OK")}}},
		}
		specs, warnings, findings, err := convertPresentationSlides(slides, nil, 0, 0, nil, nil, "", nil, true)
		if err != nil {
			t.Fatalf("partial mode should not return error, got: %v", err)
		}
		if len(specs) != 2 {
			t.Errorf("expected 2 specs (skipping failed slide), got %d", len(specs))
		}
		// Check that a warning was emitted about the skipped slide
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "skipped (partial mode)") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning about skipped slide, got warnings: %v", warnings)
		}
		// A machine-actionable CONTENT_DROPPED finding must accompany the
		// human-readable warning so the dropped slide is repairable, not silent.
		var dropped *patterns.FitFinding
		for i := range findings {
			if findings[i].Code == patterns.ErrCodeContentDropped {
				dropped = &findings[i]
				break
			}
		}
		if dropped == nil {
			t.Fatalf("expected a CONTENT_DROPPED finding for the skipped slide, got findings: %v", findings)
		}
		if dropped.Path != "/slides/1" {
			t.Errorf("CONTENT_DROPPED path = %q, want /slides/1 (the skipped slide)", dropped.Path)
		}
		if dropped.Action != "review" {
			t.Errorf("CONTENT_DROPPED action = %q, want review", dropped.Action)
		}
	})

	t.Run("partial=false aborts on failing slide", func(t *testing.T) {
		slides := []SlideInput{
			{LayoutID: "slideLayout1", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("OK")}}},
			{Content: []ContentInput{{Type: "text", TextValue: strPtr("Fail")}}},
		}
		_, _, _, err := convertPresentationSlides(slides, nil, 0, 0, nil, nil, "", nil, false)
		if err == nil {
			t.Error("expected error when partial=false and slide fails, got nil")
		}
	})
}

// TestSectionSlideContentType verifies that section divider slides produce
// ContentSectionTitle even after autoMapPlaceholders resolves "title" to a
// concrete placeholder index (regression test for blank section dividers).
func TestSectionSlideContentType(t *testing.T) {
	// Section divider layout: no title placeholder, body only (like template_2)
	sectionLayout := types.LayoutMetadata{
		ID:   "slideLayout3",
		Name: "Section Devider",
		Placeholders: []types.PlaceholderInfo{
			{ID: "body", Type: types.PlaceholderBody, Index: 13},
		},
		Tags: []string{"section-header"},
	}

	slides := []SlideInput{
		{
			SlideType: "section",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Table Coverage")},
			},
		},
	}

	specs, _, _, err := convertPresentationSlides(slides, []types.LayoutMetadata{sectionLayout}, 0, 0, nil, nil, "", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("len(specs) = %d, want 1", len(specs))
	}

	// The content item must be ContentSectionTitle, not ContentText
	found := false
	for _, item := range specs[0].Content {
		if item.Type == generator.ContentSectionTitle {
			found = true
		}
		if item.Type == generator.ContentText {
			t.Errorf("section slide text item has type ContentText, want ContentSectionTitle")
		}
	}
	if !found {
		t.Error("no ContentSectionTitle item found in section slide specs")
	}
}

// TestSectionSlideSharedPlaceholderMerge verifies that when title and body
// resolve to the same placeholder (template_2 section dividers), the text items
// are merged instead of the body overwriting the title.
func TestSectionSlideSharedPlaceholderMerge(t *testing.T) {
	// Section divider layout: no title placeholder, body only (like template_2)
	sectionLayout := types.LayoutMetadata{
		ID:   "slideLayout3",
		Name: "Section Devider",
		Placeholders: []types.PlaceholderInfo{
			{ID: "body", Type: types.PlaceholderBody, Index: 13},
		},
		Tags: []string{"section-header"},
	}

	slides := []SlideInput{
		{
			SlideType: "section",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Content Type Coverage")},
			},
		},
	}

	specs, _, _, err := convertPresentationSlides(slides, []types.LayoutMetadata{sectionLayout}, 0, 0, nil, nil, "", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("len(specs) = %d, want 1", len(specs))
	}

	// Both title ("Content Type Coverage") and auto-added body ("01") should be
	// merged into a single content item since they target the same placeholder.
	if len(specs[0].Content) != 1 {
		t.Fatalf("expected 1 merged content item, got %d: %+v", len(specs[0].Content), specs[0].Content)
	}

	text, ok := specs[0].Content[0].Value.(string)
	if !ok {
		t.Fatalf("expected string value, got %T", specs[0].Content[0].Value)
	}
	if !strings.Contains(text, "Content Type Coverage") {
		t.Errorf("merged text %q missing title", text)
	}
	if !strings.Contains(text, "01") {
		t.Errorf("merged text %q missing section number", text)
	}
}

// TestInjectSectionNumber verifies the auto-assigned section number is routed to
// the layout's decorative section_number placeholder when one exists, and falls
// back to the body/tagline slot otherwise. Regression for go-slide-creator-5mf7
// (modern-template injected "01" into the 14pt body tagline, leaving the 96pt
// Section Number frame blank).
func TestInjectSectionNumber(t *testing.T) {
	bodyOnly := types.LayoutMetadata{
		Placeholders: []types.PlaceholderInfo{
			{ID: "body", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody},
		},
	}
	withSectionNumber := types.LayoutMetadata{
		Placeholders: []types.PlaceholderInfo{
			{ID: "body", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody},
			{ID: "Section Number", Type: types.PlaceholderBody, FontSize: 9600, Role: types.PlaceholderRoleSectionNumber},
		},
	}

	t.Run("routes_to_section_number_alias_when_present", func(t *testing.T) {
		got := injectSectionNumber(nil, &withSectionNumber, "01")
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1", len(got))
		}
		if !generator.IsSectionNumberAlias(got[0].PlaceholderID) {
			t.Errorf("injected PlaceholderID = %q, want a section_number alias", got[0].PlaceholderID)
		}
		if got[0].TextValue == nil || *got[0].TextValue != "01" {
			t.Errorf("injected TextValue = %v, want \"01\"", got[0].TextValue)
		}
	})

	t.Run("falls_back_to_body_without_section_number", func(t *testing.T) {
		got := injectSectionNumber(nil, &bodyOnly, "02")
		if len(got) != 1 || got[0].PlaceholderID != "body" {
			t.Fatalf("got %+v, want a single body item", got)
		}
	})

	t.Run("noop_for_empty_number", func(t *testing.T) {
		if got := injectSectionNumber(nil, &withSectionNumber, ""); got != nil {
			t.Errorf("got %+v, want nil (no-op for non-section slide)", got)
		}
	})

	t.Run("skips_when_section_number_already_populated", func(t *testing.T) {
		existing := []ContentInput{{PlaceholderID: "section_number", Type: "text", TextValue: strPtr("99")}}
		if got := injectSectionNumber(existing, &withSectionNumber, "03"); len(got) != 1 {
			t.Errorf("len(got) = %d, want 1 (no injection over user content)", len(got))
		}
	})

	t.Run("detects_section_number_by_name_without_role", func(t *testing.T) {
		// Layouts that have not been role-classified must still be detected via
		// the "Section Number" name fallback.
		unclassified := types.LayoutMetadata{
			Placeholders: []types.PlaceholderInfo{
				{ID: "body", Type: types.PlaceholderBody},
				{ID: "Section Number", Type: types.PlaceholderBody},
			},
		}
		got := injectSectionNumber(nil, &unclassified, "05")
		if len(got) != 1 || !generator.IsSectionNumberAlias(got[0].PlaceholderID) {
			t.Errorf("got %+v, want a single section_number alias item", got)
		}
	})
}

// TestSectionNumberRoutedThroughConversion verifies the end-to-end conversion
// path: a section-divider layout exposing both a body and a section_number
// placeholder routes the auto-assigned number to the section_number alias, while
// the title text stays on the body slot.
func TestSectionNumberRoutedThroughConversion(t *testing.T) {
	sectionLayout := types.LayoutMetadata{
		ID:   "slideLayout5",
		Name: "Section Divider",
		Placeholders: []types.PlaceholderInfo{
			{ID: "body", Type: types.PlaceholderBody, Index: 1, FontSize: 1400, Role: types.PlaceholderRoleBody},
			{ID: "Section Number", Type: types.PlaceholderBody, Index: 13, FontSize: 9600, Role: types.PlaceholderRoleSectionNumber},
		},
		Tags: []string{"section-header"},
	}

	slides := []SlideInput{
		{
			SlideType: "section",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Discovery")},
			},
		},
	}

	specs, _, _, err := convertPresentationSlides(slides, []types.LayoutMetadata{sectionLayout}, 0, 0, nil, nil, "", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("len(specs) = %d, want 1", len(specs))
	}

	var foundNumber bool
	for _, item := range specs[0].Content {
		v, ok := item.Value.(string)
		if !ok || !strings.Contains(v, "01") {
			continue
		}
		foundNumber = true
		if !generator.IsSectionNumberAlias(item.PlaceholderID) {
			t.Errorf("section number '01' targeted placeholder %q, want a section_number alias", item.PlaceholderID)
		}
	}
	if !foundNumber {
		t.Errorf("auto-assigned section number '01' not found in specs: %+v", specs[0].Content)
	}
}

func TestMergeTextItemsSamePlaceholder(t *testing.T) {
	t.Run("no_merge_different_placeholders", func(t *testing.T) {
		items := []generator.ContentItem{
			{PlaceholderID: "ph1", Type: generator.ContentSectionTitle, Value: "Title"},
			{PlaceholderID: "ph2", Type: generator.ContentSectionTitle, Value: "Body"},
		}
		result := mergeTextItemsSamePlaceholder(items)
		if len(result) != 2 {
			t.Fatalf("expected 2 items, got %d", len(result))
		}
	})

	t.Run("merge_same_placeholder", func(t *testing.T) {
		items := []generator.ContentItem{
			{PlaceholderID: "ph1", Type: generator.ContentSectionTitle, Value: "Title Text"},
			{PlaceholderID: "ph1", Type: generator.ContentSectionTitle, Value: "01"},
		}
		result := mergeTextItemsSamePlaceholder(items)
		if len(result) != 1 {
			t.Fatalf("expected 1 merged item, got %d", len(result))
		}
		text := result[0].Value.(string)
		if text != "Title Text\n01" {
			t.Errorf("merged text = %q, want %q", text, "Title Text\n01")
		}
	})

	t.Run("non_text_items_preserved", func(t *testing.T) {
		items := []generator.ContentItem{
			{PlaceholderID: "ph1", Type: generator.ContentSectionTitle, Value: "Title"},
			{PlaceholderID: "ph1", Type: generator.ContentImage, Value: "img.png"},
		}
		result := mergeTextItemsSamePlaceholder(items)
		if len(result) != 2 {
			t.Fatalf("expected 2 items (text + image not merged), got %d", len(result))
		}
	})
}

// strPtr is a test helper that returns a pointer to a string.
func strPtr(s string) *string { return &s }

func boolPtr(b bool) *bool { return &b }

func TestValidateDiagramSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *types.DiagramSpec
		wantErr bool
	}{
		{
			name: "valid bar chart",
			spec: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{
					"categories": []any{"Q1", "Q2", "Q3"},
					"series": []any{
						map[string]any{"name": "Revenue", "values": []any{10.0, 20.0, 30.0}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid waterfall with points",
			spec: &types.DiagramSpec{
				Type: "waterfall",
				Data: map[string]any{
					"points": []any{
						map[string]any{"label": "Revenue", "value": 100.0, "type": "increase"},
						map[string]any{"label": "Costs", "value": -40.0, "type": "decrease"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "waterfall with flat map data (missing points)",
			spec: &types.DiagramSpec{
				Type: "waterfall",
				Data: map[string]any{
					"Revenue": 100.0,
					"Costs":   -40.0,
				},
			},
			wantErr: true,
		},
		{
			name: "funnel with flat map data (missing values)",
			spec: &types.DiagramSpec{
				Type: "funnel_chart",
				Data: map[string]any{
					"Leads":     1000.0,
					"Prospects": 500.0,
				},
			},
			wantErr: true,
		},
		{
			name: "valid funnel chart",
			spec: &types.DiagramSpec{
				Type: "funnel_chart",
				Data: map[string]any{
					"values": []any{
						map[string]any{"label": "Leads", "value": 1000.0},
						map[string]any{"label": "Prospects", "value": 500.0},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "gauge with flat map data (missing value key)",
			spec: &types.DiagramSpec{
				Type: "gauge_chart",
				Data: map[string]any{
					"Performance": 75.0,
				},
			},
			wantErr: true,
		},
		{
			name: "valid gauge chart",
			spec: &types.DiagramSpec{
				Type: "gauge_chart",
				Data: map[string]any{
					"value": 75.0,
					"min":   0.0,
					"max":   100.0,
				},
			},
			wantErr: false,
		},
		{
			name:    "nil spec returns no error",
			spec:    nil,
			wantErr: false,
		},
		{
			name: "unknown type returns no error",
			spec: &types.DiagramSpec{
				Type: "nonexistent_chart",
				Data: map[string]any{"foo": "bar"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateDiagramSpec(tt.spec, 1, 1)
			if tt.wantErr && result == "" {
				t.Error("expected validation warning, got empty string")
			}
			if !tt.wantErr && result != "" {
				t.Errorf("unexpected validation warning: %s", result)
			}
		})
	}
}

func TestValidateSlidesChartData(t *testing.T) {
	t.Run("diagram with invalid data produces warning", func(t *testing.T) {
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "diagram",
						Value:         json.RawMessage(`{"type":"waterfall","data":{"Revenue":100,"Costs":-40}}`),
					},
				},
			},
		}
		warnings := validateSlidesChartData(slides)
		if len(warnings) == 0 {
			t.Error("expected at least one warning for invalid waterfall data")
		}
	})

	t.Run("chart with valid data produces no warning", func(t *testing.T) {
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "chart",
						Value: json.RawMessage(`{
							"type":"bar",
							"data":{"Q1":10,"Q2":20,"Q3":30}
						}`),
					},
				},
			},
		}
		warnings := validateSlidesChartData(slides)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for valid bar chart, got: %v", warnings)
		}
	})

	t.Run("text content is skipped", func(t *testing.T) {
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: strPtr("Hello")},
				},
			},
		}
		warnings := validateSlidesChartData(slides)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for text content, got: %v", warnings)
		}
	})
}

func TestValidateJSONContentValue_ChartDiagramData(t *testing.T) {
	t.Run("chart with flat waterfall data auto-converts (no error)", func(t *testing.T) {
		// ChartSpec.ToDiagramSpec() runs buildChartData which converts flat maps
		// to the correct svggen points format, so this should pass validation.
		item := JSONContentItem{
			Type:  "chart",
			Value: json.RawMessage(`{"type":"waterfall","data":{"Revenue":100}}`),
		}
		result := validateJSONContentValue(item, 1, 1)
		if result != "" {
			t.Errorf("unexpected validation error (flat chart data is auto-converted): %s", result)
		}
	})

	t.Run("diagram with invalid waterfall data (flat map)", func(t *testing.T) {
		// Diagram type passes data directly to svggen without conversion,
		// so flat map data should fail waterfall validation.
		item := JSONContentItem{
			Type:  "diagram",
			Value: json.RawMessage(`{"type":"waterfall","data":{"Revenue":100,"Costs":-40}}`),
		}
		result := validateJSONContentValue(item, 1, 1)
		if result == "" {
			t.Error("expected validation error for waterfall diagram with flat map data")
		}
	})

	t.Run("diagram with invalid funnel data", func(t *testing.T) {
		item := JSONContentItem{
			Type:  "diagram",
			Value: json.RawMessage(`{"type":"funnel_chart","data":{"Leads":1000}}`),
		}
		result := validateJSONContentValue(item, 1, 1)
		if result == "" {
			t.Error("expected validation error for funnel diagram with flat map data")
		}
	})

	t.Run("chart with valid bar data", func(t *testing.T) {
		item := JSONContentItem{
			Type:  "chart",
			Value: json.RawMessage(`{"type":"bar","data":{"Q1":10,"Q2":20}}`),
		}
		result := validateJSONContentValue(item, 1, 1)
		if result != "" {
			t.Errorf("unexpected validation error for valid bar chart: %s", result)
		}
	})

	t.Run("diagram with valid waterfall data", func(t *testing.T) {
		item := JSONContentItem{
			Type: "diagram",
			Value: json.RawMessage(`{
				"type":"waterfall",
				"data":{
					"points":[
						{"label":"Revenue","value":100,"type":"increase"},
						{"label":"Costs","value":-40,"type":"decrease"}
					]
				}
			}`),
		}
		result := validateJSONContentValue(item, 1, 1)
		if result != "" {
			t.Errorf("unexpected validation error for valid waterfall: %s", result)
		}
	})
}

func TestValidateSlidesChartData_FlatMapWarnings(t *testing.T) {
	t.Run("waterfall flat map emits conversion warning", func(t *testing.T) {
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "chart",
						Value:         json.RawMessage(`{"type":"waterfall","data":{"Revenue":100,"Costs":-40}}`),
					},
				},
			},
		}
		warnings := validateSlidesChartData(slides)
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "waterfall chart received flat data") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected flat-map conversion warning for waterfall chart, got: %v", warnings)
		}
	})

	t.Run("bar flat map does not emit conversion warning", func(t *testing.T) {
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "chart",
						Value:         json.RawMessage(`{"type":"bar","data":{"Q1":10,"Q2":20}}`),
					},
				},
			},
		}
		warnings := validateSlidesChartData(slides)
		for _, w := range warnings {
			if strings.Contains(w, "received flat data") {
				t.Errorf("unexpected flat-map warning for bar chart: %s", w)
			}
		}
	})

	t.Run("waterfall with structured data has no conversion warning", func(t *testing.T) {
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "chart",
						Value: json.RawMessage(`{
							"type":"waterfall",
							"data":{
								"points":[
									{"label":"Revenue","value":100,"type":"increase"},
									{"label":"Costs","value":-40,"type":"decrease"}
								]
							}
						}`),
					},
				},
			},
		}
		warnings := validateSlidesChartData(slides)
		for _, w := range warnings {
			if strings.Contains(w, "received flat data") {
				t.Errorf("unexpected flat-map warning for structured waterfall: %s", w)
			}
		}
	})
}

func TestValidateSlidesChartDiagnostics(t *testing.T) {
	t.Run("chart with string value returns chart_value_coerced finding", func(t *testing.T) {
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "chart",
						Value:         json.RawMessage(`{"type":"bar","data":{"A":10,"B":"oops","C":30}}`),
					},
				},
			},
		}
		findings := validateSlidesChartDiagnostics(slides)
		var coerced int
		for _, f := range findings {
			if f.Code == "chart_value_coerced" {
				coerced++
				if !strings.Contains(f.Message, "oops") {
					t.Errorf("expected message to mention original value, got %q", f.Message)
				}
				if f.Action != "review" {
					t.Errorf("expected action=review, got %q", f.Action)
				}
			}
		}
		if coerced != 1 {
			t.Errorf("expected 1 chart_value_coerced finding, got %d (total: %v)", coerced, findings)
		}
	})

	t.Run("empty data returns chart_data_empty finding", func(t *testing.T) {
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "diagram",
						Value:         json.RawMessage(`{"type":"bar_chart","data":{}}`),
					},
				},
			},
		}
		findings := validateSlidesChartDiagnostics(slides)
		var empty int
		for _, f := range findings {
			if f.Code == "chart_data_empty" {
				empty++
				if f.Action != "refuse" {
					t.Errorf("expected action=refuse, got %q", f.Action)
				}
			}
		}
		if empty != 1 {
			t.Errorf("expected 1 chart_data_empty finding, got %d", empty)
		}
	})

	t.Run("valid chart produces no diagnostic findings", func(t *testing.T) {
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "chart",
						Value:         json.RawMessage(`{"type":"bar","data":{"Q1":10,"Q2":20}}`),
					},
				},
			},
		}
		findings := validateSlidesChartDiagnostics(slides)
		if len(findings) > 0 {
			t.Errorf("expected no findings for valid bar chart, got %v", findings)
		}
	})
}

func TestBuildSlideResolutions_Basic(t *testing.T) {
	inputSlides := []SlideInput{
		{LayoutID: "slideLayout1"},  // explicit
		{LayoutID: ""},              // auto-selected -> resolved to slideLayout2
		{LayoutID: "slideLayout99"}, // synthesized
	}

	specs := []generator.SlideSpec{
		{
			LayoutID: "slideLayout1",
			Content: []generator.ContentItem{
				{PlaceholderID: "title", Type: generator.ContentText},
				{PlaceholderID: "body", Type: generator.ContentText},
			},
		},
		{
			LayoutID: "slideLayout2",
			Content: []generator.ContentItem{
				{PlaceholderID: "title", Type: generator.ContentText},
			},
		},
		{
			LayoutID: "slideLayout99",
			Content: []generator.ContentItem{
				{PlaceholderID: "title", Type: generator.ContentText},
				{PlaceholderID: "body", Type: generator.ContentText},
				{PlaceholderID: "body_2", Type: generator.ContentText},
			},
		},
	}

	layouts := []types.LayoutMetadata{
		{
			ID: "slideLayout1",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title"}, {ID: "body"}, {ID: "footer"},
			},
		},
		{
			ID: "slideLayout2",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title"}, {ID: "body"},
			},
		},
		{
			ID: "slideLayout99",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title"}, {ID: "body"}, {ID: "body_2"},
			},
		},
	}

	syntheticFiles := map[string][]byte{
		"ppt/slideLayouts/slideLayout99.xml": []byte("<xml/>"),
	}

	resolutions := buildSlideResolutions(inputSlides, specs, layouts, syntheticFiles)

	if len(resolutions) != 3 {
		t.Fatalf("expected 3 resolutions, got %d", len(resolutions))
	}

	// Slide 0: explicit layout, not synthesized
	if resolutions[0].ResolvedLayoutID != "slideLayout1" {
		t.Errorf("slide 0: expected slideLayout1, got %s", resolutions[0].ResolvedLayoutID)
	}
	if resolutions[0].WasAutoSelected {
		t.Error("slide 0: should not be auto-selected")
	}
	if resolutions[0].WasSynthesized {
		t.Error("slide 0: should not be synthesized")
	}
	if resolutions[0].OccupancyPct != 66 { // 2/3 placeholders used
		t.Errorf("slide 0: expected occupancy 66, got %d", resolutions[0].OccupancyPct)
	}

	// Slide 1: auto-selected
	if !resolutions[1].WasAutoSelected {
		t.Error("slide 1: should be auto-selected")
	}
	if resolutions[1].OccupancyPct != 50 { // 1/2 placeholders used
		t.Errorf("slide 1: expected occupancy 50, got %d", resolutions[1].OccupancyPct)
	}

	// Slide 2: synthesized
	if !resolutions[2].WasSynthesized {
		t.Error("slide 2: should be synthesized")
	}
	if resolutions[2].OccupancyPct != 100 { // 3/3 placeholders used
		t.Errorf("slide 2: expected occupancy 100, got %d", resolutions[2].OccupancyPct)
	}
	if len(resolutions[2].PlaceholdersUsed) != 3 {
		t.Errorf("slide 2: expected 3 placeholders used, got %d", len(resolutions[2].PlaceholdersUsed))
	}
}

func TestBuildSlideResolutions_NilSyntheticFiles(t *testing.T) {
	inputSlides := []SlideInput{{LayoutID: "slideLayout1"}}
	specs := []generator.SlideSpec{{
		LayoutID: "slideLayout1",
		Content:  []generator.ContentItem{{PlaceholderID: "title", Type: generator.ContentText}},
	}}
	layouts := []types.LayoutMetadata{{ID: "slideLayout1", Placeholders: []types.PlaceholderInfo{{ID: "title"}}}}

	resolutions := buildSlideResolutions(inputSlides, specs, layouts, nil)
	if len(resolutions) != 1 {
		t.Fatalf("expected 1 resolution, got %d", len(resolutions))
	}
	if resolutions[0].WasSynthesized {
		t.Error("should not be synthesized with nil synthetic files")
	}
	if resolutions[0].OccupancyPct != 100 {
		t.Errorf("expected occupancy 100, got %d", resolutions[0].OccupancyPct)
	}
}

// TestRunJSONMode_RelativeAssetPathsResolved is the CLI parity test for
// go-slide-creator-tigj: relative paths in image_value, GridImageInput,
// background.image, and icons must all resolve against the JSON input
// directory. Before this change, only icons were resolved — relative
// image / background paths produced "file not found" at generate time
// whenever CWD did not match the JSON's parent directory.
//
// The test writes a deck JSON and matching asset files into a tmp dir,
// then invokes resolveLocalAssetPaths (the helper json_mode.go calls
// after loading the deck) with the input directory. All four surfaces
// must resolve to absolute paths rooted under tmpDir, with no findings.
func TestRunJSONMode_RelativeAssetPathsResolved(t *testing.T) {
	tmpDir := t.TempDir()
	// Materialize the four asset files alongside the deck JSON so each
	// surface has a real file to resolve against. The relative path
	// stored in the deck is the bare filename — resolveLocalAssetPaths
	// joins it against tmpDir.
	files := map[string]string{
		"bg.jpg":    "ignored bytes",
		"photo.png": "ignored bytes",
		"icon.svg":  "<svg/>",
		"grid.jpg":  "ignored bytes",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Build a SlideInput with every asset surface using relative paths.
	slides := []SlideInput{{
		Background: &BackgroundInput{Image: "bg.jpg"},
		Content: []ContentInput{
			{PlaceholderID: "body", Type: "image", ImageValue: &ImageInput{Path: "photo.png", Alt: "ph"}},
		},
		ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{{
				Cells: []*GridCellInput{
					{Icon: &IconInput{Path: "icon.svg"}},
					{Image: &GridImageInput{Path: "grid.jpg", Alt: "grid"}},
				},
			}},
		},
	}}

	findings := resolveLocalAssetPaths(slides, tmpDir)
	if len(findings) != 0 {
		t.Fatalf("expected no findings with all assets present in tmpDir, got %d: %+v", len(findings), findings)
	}

	// Each field must now hold an absolute path that EvalSymlinks-resolves
	// to a file in tmpDir (handles /var → /private/var on macOS).
	resolvedOf := func(name string) string {
		r, _ := filepath.EvalSymlinks(filepath.Join(tmpDir, name))
		return r
	}
	gotBg := slides[0].Background.Image
	if gotBg != resolvedOf("bg.jpg") {
		t.Errorf("background.image: got %q, want %q", gotBg, resolvedOf("bg.jpg"))
	}
	gotImg := slides[0].Content[0].ImageValue.Path
	if gotImg != resolvedOf("photo.png") {
		t.Errorf("image_value.path: got %q, want %q", gotImg, resolvedOf("photo.png"))
	}
	gotIcon := slides[0].ShapeGrid.Rows[0].Cells[0].Icon.Path
	if gotIcon != resolvedOf("icon.svg") {
		t.Errorf("icon.path: got %q, want %q", gotIcon, resolvedOf("icon.svg"))
	}
	gotGrid := slides[0].ShapeGrid.Rows[0].Cells[1].Image.Path
	if gotGrid != resolvedOf("grid.jpg") {
		t.Errorf("grid image.path: got %q, want %q", gotGrid, resolvedOf("grid.jpg"))
	}
}

// TestRunJSONMode_MissingAssetReturnsAggregatedError verifies that
// localAssetFindingsToError flattens per-asset findings into a single
// CLI error message (the legacy text-error envelope) with one line per
// broken asset. The CLI cannot emit a structured diagnostics envelope
// like MCP, so this aggregation is the only way agents see every broken
// reference in one pass.
func TestRunJSONMode_MissingAssetReturnsAggregatedError(t *testing.T) {
	tmpDir := t.TempDir()
	slides := []SlideInput{{
		Background: &BackgroundInput{Image: "missing-bg.jpg"},
		Content: []ContentInput{
			{PlaceholderID: "body", Type: "image", ImageValue: &ImageInput{Path: "missing-photo.png"}},
		},
	}}

	findings := resolveLocalAssetPaths(slides, tmpDir)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(findings), findings)
	}
	err := iconFindingsToError(findings)
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "BACKGROUND_IMAGE_PATH") {
		t.Errorf("expected error to mention BACKGROUND_IMAGE_PATH, got: %s", msg)
	}
	if !strings.Contains(msg, "IMAGE_PATH") {
		t.Errorf("expected error to mention IMAGE_PATH, got: %s", msg)
	}
	if !strings.Contains(msg, "/slides/0/background/image") {
		t.Errorf("expected error to mention background json_path, got: %s", msg)
	}
	if !strings.Contains(msg, "/slides/0/content/0/image_value/path") {
		t.Errorf("expected error to mention image_value json_path, got: %s", msg)
	}
}

// TestIconFindingsToError_WarningsAreNonBlocking verifies that diagnostics with
// non-error severity (warnings/info) do not trigger an aggregated CLI error.
// This is what makes ICON_FILL_IGNORED_ON_INLINE genuinely non-blocking — the
// CLI generate path bails only when iconFindingsToError returns non-nil.
func TestIconFindingsToError_WarningsAreNonBlocking(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg"/>`
	slides := []SlideInput{{
		ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{{
				Cells: []*GridCellInput{{
					Icon: &IconInput{SVGData: svg, Fill: "#FF0000"},
				}},
			}},
		},
	}}

	findings := resolveLocalAssetPaths(slides, t.TempDir())
	if len(findings) != 1 || findings[0].Code != "ICON_FILL_IGNORED_ON_INLINE" {
		t.Fatalf("expected 1 ICON_FILL_IGNORED_ON_INLINE finding, got %+v", findings)
	}
	if err := iconFindingsToError(findings); err != nil {
		t.Errorf("expected nil error for warning-only findings, got: %v", err)
	}
}
