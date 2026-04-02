# Visual Inspection System

The visual inspection system validates generated PPTX quality using LibreOffice/ImageMagick conversion and LLM vision analysis.

## Scope

This specification covers the visual inspection pipeline for the automated development loop. It does NOT cover:
- Unit test infrastructure (already exists)
- LLM provider implementations (spec 13)

## Purpose

The automated development loop must "see" its output to identify and fix visual defects. Without visual feedback, bugs like text overflow, misaligned content, and broken charts go undetected.

## Pipeline Overview

```
Generate PPTX → Convert to PDF → Convert to JPG → LLM Vision Analysis → Report
```

## Components

### 1. PPTX to JPG Conversion (cmd/pptx2jpg)

Uses LibreOffice and ImageMagick:

```bash
# Convert PPTX to PDF
libreoffice --headless --convert-to pdf --outdir /tmp presentation.pptx

# Convert PDF pages to JPG
convert -density 150 /tmp/presentation.pdf /tmp/slide-%d.jpg
```

**Dependencies:**
- LibreOffice: `brew install --cask libreoffice`
- ImageMagick: `brew install imagemagick`

### 2. Visual Inspector (cmd/visualinspect)

Analyzes slide images using LLM vision.

**CLI Usage:**
```bash
./cmd/visualinspect \
  -slides dir/                    # Directory containing slide JPG files (required)
  -input source.json              # Source JSON input for content-aware inspection (optional)
  -template template.pptx         # Template for font category detection (optional)
  -output report.json             # Output JSON report file (default: inspection_report.json)
  -threshold 0.7                  # Pass threshold 0.0-1.0 (default: 0.7)
  -config llm_config.yaml         # LLM config file path (optional)
  -skip 0                         # Number of template slides to skip (optional)
```

**Output Types (cmd/visualinspect):**

```go
type InspectionResult struct {
    SlideNumber        int                 `json:"slide_number"`
    Score              float64             `json:"score"`    // 0.0 to 1.0
    Category           string              `json:"category"` // excellent, good, acceptable, poor
    Issues             []Issue             `json:"issues"`
    Suggestions        []string            `json:"suggestions"`
    ContentDiscrepancy *ContentDiscrepancy `json:"content_discrepancy,omitempty"`
}

type Issue struct {
    Type        string `json:"type"`     // text, layout, content
    Severity    string `json:"severity"` // critical, major, minor
    Description string `json:"description"`
    Location    string `json:"location,omitempty"`
}

type ContentDiscrepancy struct {
    ExpectedItems   []string `json:"expected_items,omitempty"`
    VisibleItems    []string `json:"visible_items,omitempty"`
    MissingItems    []string `json:"missing_items,omitempty"`
    TruncatedItems  []string `json:"truncated_items,omitempty"`
    MatchPercentage float64  `json:"match_percentage"`
}

type InspectionReport struct {
    Timestamp     string             `json:"timestamp"`
    PPTXFile      string             `json:"pptx_file"`
    TotalSlides   int                `json:"total_slides"`
    OverallScore  float64            `json:"overall_score"`
    OverallStatus string             `json:"overall_status"` // pass, needs_improvement, fail
    Slides        []InspectionResult `json:"slides"`
    Summary       Summary            `json:"summary"`
}

type Summary struct {
    CriticalIssues int      `json:"critical_issues"`
    MajorIssues    int      `json:"major_issues"`
    MinorIssues    int      `json:"minor_issues"`
    CommonIssues   []string `json:"common_issues"`
    TopPriorities  []string `json:"top_priorities"`
}
```

**Core Library Types (internal/layout):**

```go
type QualityAssessment struct {
    OverallScore         float64                   `json:"overall_score"`
    Category             QualityCategory           `json:"category"`
    TextIssues           []QualityIssue            `json:"text_issues,omitempty"`
    LayoutIssues         []QualityIssue            `json:"layout_issues,omitempty"`
    ContentIssues        []QualityIssue            `json:"content_issues,omitempty"`
    ContentDiscrepancies *ContentDiscrepancyReport `json:"content_discrepancies,omitempty"`
    Suggestions          []string                  `json:"suggestions,omitempty"`
    Summary              string                    `json:"summary"`
}

type QualityIssue struct {
    Severity    IssueSeverity `json:"severity"`     // critical, major, minor
    Description string        `json:"description"`
    Location    string        `json:"location,omitempty"`
}

type InspectionRequest struct {
    ImageData         []byte       // PNG/JPG image data
    SlideIndex        int          // Zero-based slide index
    SlideTitle        string       // Slide title for context
    LayoutName        string       // Layout name used
    ExpectedContent   string       // Expected content for validation
    ExpectsGraphic    bool         // Whether graphics/charts expected (populated from input parsing)
    TitleFontCategory FontCategory // Expected title font category (serif, sans-serif, etc.)
    BodyFontCategory  FontCategory // Expected body font category (serif, sans-serif, etc.)
}

type MultiSlideAssessment struct {
    Assessments     []*QualityAssessment `json:"assessments"`
    AverageScore    float64              `json:"average_score"`
    OverallCategory QualityCategory      `json:"overall_category"`
    TotalSlides     int                  `json:"total_slides"`
    CriticalIssues  int                  `json:"critical_issues"`
    MajorIssues     int                  `json:"major_issues"`
    MinorIssues     int                  `json:"minor_issues"`
    PassesThreshold bool                 `json:"passes_threshold"` // score >= 0.7 && no critical
}
```

### 3. Inspection Runner Script (scripts/run_visual_inspection.sh)

Orchestrates the full pipeline:
1. Runs unit tests to verify code compiles
2. Generates PPTX using E2E test
3. Converts to JPG
4. Runs visual inspection
5. Outputs report

## Integration with Automated Development Loop

The `PROMPT_build.md` instructs Claude to:

1. After unit tests pass, run `./scripts/run_visual_inspection.sh`
2. Read generated slide images in `test_output/`
3. Analyze for visual issues
4. Fix code if critical issues found
5. Re-run until visual inspection passes

## Acceptance Criteria

### AC1: Conversion Pipeline
- Given a valid PPTX file
- When `./cmd/pptx2jpg -input file.pptx -output dir/`
- Then JPG files created for each slide

### AC2: Visual Inspector
- Given slide JPG images
- When `./cmd/visualinspect -slides dir/ [-input source.json] [-template template.pptx]`
- Then JSON report with scores, issues, and content discrepancy analysis

### AC3: E2E Test Generation
- Given `TestVisualE2E` test
- When run with valid template
- Then PPTX generated at `test_output/visual_e2e.pptx`

### AC4: Shell Script Integration
- Given `./scripts/run_visual_inspection.sh`
- When executed
- Then generates PPTX, converts to JPG, outputs image paths

### AC5: Development Loop Integration
- Given visual issues in generated slides
- When automated development loop runs
- Then issues documented and fixes attempted

## Quality Criteria

The visual inspector checks:

1. **Text Rendering**
   - All text visible and readable
   - No truncation or overflow
   - Proper font sizes

2. **Layout Alignment**
   - Elements properly positioned
   - Consistent margins
   - Balanced visual weight

3. **Content Completeness**
   - All input content present
   - Charts display correct data
   - Images render correctly

4. **Theme Consistency**
   - Colors match template
   - Title fonts match template title font (with FontCategory classification: serif, sans-serif, monospace, decorative)
   - Body fonts match template body font (separate classification from title font)
   - Style elements preserved

5. **Position Validation** (Critical)
   - Title appears in top portion of slide
   - Body/bullet content appears below title
   - No content cropping or overflow
   - No unintended element overlap

6. **Graphic Rendering** (when ExpectsGraphic=true)
   - No blank areas where content should appear
   - No placeholder text like "[Chart]" or "Image not found"
   - Charts fully rendered with visible data labels
   - SVG/vector graphics appear crisp

## Scoring Guide

| Score | Category | Description |
|-------|----------|-------------|
| 0.9-1.0 | Excellent | No issues, professional quality |
| 0.7-0.89 | Good | Minor issues only |
| 0.5-0.69 | Acceptable | Some major issues |
| 0.0-0.49 | Poor | Critical issues present |

## Pass/Fail Threshold

The threshold is configurable via `-threshold` flag (default: 0.7).

**Standard threshold (0.7):**
- **Pass**: Score >= threshold AND no critical issues
- **Needs Improvement**: Score >= threshold*0.8 (0.56) AND <= 1 critical issue
- **Fail**: Score < threshold*0.8 OR > 1 critical issues

**Lower threshold (for decorative font templates):**
- When threshold < 0.7, allows up to 2 critical issues for Pass
- This accommodates vision model limitations with decorative fonts

**Exit Codes:**
- `0`: Pass
- `1`: Needs Improvement
- `2`: Fail

## Testing

Run visual inspection manually:
```bash
# Generate and inspect
./scripts/run_visual_inspection.sh

# View results
ls -la test_output/*.jpg
cat test_output/inspection_report.md
```

Run E2E test:
```bash
go test -v -run TestVisualE2E ./internal/generator/...
```
