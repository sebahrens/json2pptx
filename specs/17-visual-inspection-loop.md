# Visual Inspection Tooling

Tools for visual quality assessment of generated PPTX files using LLM vision models.

## Scope

This specification covers the visual inspection **tooling**:
- PPTX to image conversion
- LLM-based visual quality assessment
- CLI tools and test scripts

It does NOT cover:
- Orchestration or loop mechanics (external concern)
- Core PPTX generation logic (see `04-pptx-generator.md`)

## Purpose

Visual inspection adds perceptual quality assessment using LLM vision models, enabling:
- Detection of text overflow, alignment, and formatting issues
- Automated feedback that mimics human review
- Quality gates for generated presentations

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Visual Inspection Pipeline                    │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │   PPTX to    │───▶│    Visual    │───▶│   Inspection │      │
│  │   JPG Tool   │    │   Inspector  │    │    Report    │      │
│  │ cmd/pptx2jpg │    │  (LLM Vision)│    │    (JSON)    │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### PPTX to JPG Conversion Tool

**File:** `cmd/pptx2jpg/main.go`

CLI tool that converts PPTX to JPG slide images:

```bash
go run ./cmd/pptx2jpg -input <pptx> -output <dir> -density 150
```

Key features:
- Uses LibreOffice (headless) for PPTX → PDF conversion
- Uses ImageMagick for PDF → JPG per-slide extraction
- Testable via dependency injection (`CommandRunner` interface)
- Output pattern: `<basename>-slide-N.jpg`

### Visual Inspector Library

**File:** `internal/layout/visual_inspector.go`

The VisualInspector provides:
- `Inspect(ctx, InspectionRequest)` - Single slide inspection
- `InspectSlides(ctx, []InspectionRequest)` - Batch inspection with aggregate stats

Features:
- Content-aware inspection (compares visible text against expected content)
- Font category validation (serif, sans-serif, decorative, monospace)
- Structured JSON output via LLM schema

```go
type InspectionRequest struct {
    ImagePath       string
    SlideNumber     int
    ExpectedContent string      // Optional: for content-aware inspection
    FontCategory    string      // Optional: serif, sans-serif, decorative, monospace
    ExpectsGraphic  bool        // Whether slide should contain chart/image
}

type QualityAssessment struct {
    Score       float64         // 0.0 - 1.0
    Category    string          // excellent, good, acceptable, needs_improvement, poor
    Issues      []QualityIssue
    Suggestions []string
}

type QualityIssue struct {
    Type        string  // text, layout, content
    Severity    string  // critical, major, minor
    Description string
    Location    string  // Optional: where on slide
}
```

### Visual Inspection CLI

**File:** `cmd/visualinspect/main.go`

Full CLI tool for running visual inspection:

```bash
go run ./cmd/visualinspect \
  -slides <dir> \
  -input <file> \
  -template <pptx> \
  -threshold 0.7
```

Features:
- Input-aware: parses source JSON for content-aware inspection
- Template-aware: extracts font category from template theme
- Configurable threshold and skip-slides options
- JSON report output with detailed issue breakdown

### E2E Test Script

**File:** `scripts/e2e_visual_test.sh`

Comprehensive test runner with:
- Template capability detection (charts, images, two-column)
- Per-template threshold overrides (e.g., 0.5 for decorative fonts)
- Multi-model testing support
- Detailed failure analysis with slide-level breakdowns

**Environment Variables:**
| Variable | Default | Description |
|----------|---------|-------------|
| `VISUAL_THRESHOLD` | 0.7 | Default pass threshold |
| `TEST_MODE` | random | `random`, `all`, or `<template.pptx>` |
| `LLM_MODELS` | - | Comma-separated model list |
| `MULTI_MODEL` | false | Test all models in sequence |

## Inspection Report Format

```json
{
  "timestamp": "2026-01-17T12:00:00Z",
  "pptx_file": "generated.pptx",
  "total_slides": 5,
  "overall_score": 0.82,
  "overall_status": "pass|needs_improvement|fail",
  "slides": [
    {
      "slide_number": 1,
      "score": 0.9,
      "category": "excellent",
      "issues": [],
      "suggestions": []
    }
  ],
  "summary": {
    "critical_issues": 0,
    "major_issues": 1,
    "minor_issues": 3,
    "common_issues": ["text_overflow (2 occurrences)"],
    "top_priorities": ["Slide 3: text_overflow - Title extends beyond placeholder"]
  }
}
```

## Pass/Fail Logic

| Condition | Result |
|-----------|--------|
| `score >= threshold` AND `critical_issues == 0` | pass |
| `score >= threshold * 0.8` AND `critical_issues <= 1` | needs_improvement |
| Otherwise | fail |

**Note:** Templates with decorative fonts use lower thresholds (e.g., 0.5) since vision models struggle with decorative font readability.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Visual inspection passed |
| 1 | Needs improvement (minor issues) |
| 2 | Failed (critical issues or low score) |

## Acceptance Criteria

### AC1: PPTX to JPG Conversion
- Given valid PPTX file
- When `cmd/pptx2jpg` runs
- Then creates one JPG per slide with pattern `<name>-slide-N.jpg`

### AC2: Visual Inspection Runs
- Given JPG slide images
- When visual inspection runs
- Then returns JSON report with scores and issues

### AC3: Pass/Fail Threshold
- Given inspection report with overall_score and threshold
- When `overall_score >= threshold` AND `critical_issues == 0`
- Then status is "pass"

### AC4: Content-Aware Inspection
- Given expected content from JSON input
- When inspection runs
- Then report includes content discrepancy detection

### AC5: Font Category Awareness
- Given template with decorative fonts
- When inspection runs
- Then threshold is adjusted and font-related issues are appropriately weighted

## Dependencies

- LibreOffice (headless mode) - PPTX to PDF conversion
- ImageMagick (`convert` command) - PDF to JPG extraction
- LLM with vision capability (configured via `llm.Manager`)

## Files

| File | Purpose |
|------|---------|
| `cmd/pptx2jpg/main.go` | PPTX to JPG conversion CLI |
| `cmd/visualinspect/main.go` | Visual inspection CLI |
| `internal/layout/visual_inspector.go` | Core visual inspection logic |
| `scripts/e2e_visual_test.sh` | E2E test script with template detection |
