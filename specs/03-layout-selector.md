# Layout Selector

Select optimal slide layouts for content using heuristic rules with optional LLM enhancement.

## Scope

This specification covers:
- Layout selection logic (heuristic and LLM-enhanced)
- Visual quality inspection of rendered slides

It does NOT cover:
- JSON input parsing (see `docs/INPUT_FORMAT.md`)
- Template analysis (see `02-template-analyzer.md`)
- PPTX generation (see `04-pptx-generator.md`)

## Purpose

Given slide content and available layouts, select the best layout that:
- Fits the content without overflow
- Matches the content type (title, bullets, image, chart)
- Maintains visual consistency across the presentation

## Architecture

Two-tier selection:

1. **Heuristic Selector** (always available, fast, deterministic)
2. **LLM Selector** (optional, slower, higher quality)

The system uses heuristics by default and LLM only when:
- Explicitly requested via `UseLLM=true`
- Heuristics produce low confidence (< 0.6 threshold)

Additional component:
3. **Visual Inspector** (optional, post-rendering quality check via vision LLM)

## Input

```go
type SelectionRequest struct {
    Slide   types.SlideDefinition  // From parser
    Layouts []types.LayoutMetadata // From template analyzer
    Context SelectionContext       // Presentation context
    UseLLM  bool                   // Enable LLM enhancement
}

type SelectionContext struct {
    Position     int            // Slide index (0-based)
    TotalSlides  int            // Total presentation length
    PreviousType string         // Type of previous slide layout (layout ID)
    Theme        string         // Presentation theme name
    UsedLayouts  map[string]int // Track layout usage counts for variety scoring
}
```

## Output

```go
type SelectionResult struct {
    LayoutID     string             // Selected layout ID
    Confidence   float64            // 0.0-1.0 confidence score
    Reasoning    string             // Explanation (for debugging)
    Mappings     []ContentMapping   // How content maps to placeholders
    Warnings     []string           // Potential issues
}

type ContentMapping struct {
    ContentField   string // "title", "body", "bullets", "image", "chart"
    PlaceholderID  string // Target placeholder
    Truncated      bool   // Content will be truncated
    TruncateAt     int    // Character position if truncated
}
```

## Heuristic Selection Rules

### Rule Priority (highest first)

1. **Type Match**: Layout tags must match slide type
   - `title` slide -> layout with `title-slide` tag
   - `chart` slide -> layout with `HasChartSlot=true`
   - `image` slide -> layout with `HasImageSlot=true`
   - `two-column` slide -> layout with 2+ body placeholders

2. **Capacity Match**: Content must fit
   - Bullet count <= MaxBullets
   - Total characters <= MaxChars

3. **Visual Balance**: Prefer layouts matching content density
   - Sparse content (<=2 elements) -> `VisualFocused` layouts
   - Dense content (>5 elements) -> `TextHeavy` layouts

4. **Consistency**: Avoid repeating same layout consecutively
   - Penalty (0.5 score) for using same layout as previous slide

5. **Variety**: Prefer less-used layouts across the presentation
   - Unused layouts score 1.0
   - Frequently used layouts score down to 0.4

6. **Semantic Match**: Bonus for content-to-tag matching
   - Quote detection (title/body contains "quote" or quotation marks)
   - Section header detection ("Section:", "Part N:", "Overview")
   - Agenda detection ("agenda", "outline", "table of contents")
   - Timeline detection ("timeline", "roadmap", "milestone", "process")
   - Big number detection (short content with prominent numbers/percentages)
   - Statement detection (short body text, no bullets, no image)

7. **Position Awareness**:
   - First slide (position 0) prefers `title-slide` layouts
   - Last slide prefers `closing` or `thank-you` tagged layouts when available

### Scoring Formula

```
Score = TypeMatch*0.35 + CapacityMatch*0.25 + VisualBalance*0.15 + Consistency*0.1 + Variety*0.05 + SemanticBonus*0.1
```

Each component scores 0.0-1.0. Total is clamped to [0, 1].

### Score Constants

| Constant | Value | Description |
|----------|-------|-------------|
| scorePerfectMatch | 1.0 | Perfect match for layout type |
| scoreGoodMatch | 0.9 | Good alternative match |
| scoreAcceptable | 0.8 | Acceptable fallback |
| scorePartialMatch | 0.5 | Partial compatibility |
| scorePoorMatch | 0.4 | Low compatibility |
| scoreMinimalMatch | 0.3 | Minimal compatibility |
| scoreNoMatch | 0.0 | Incompatible |

### Confidence Calculation

```
Confidence = TopScore - SecondScore + (TopScore * 0.5)
```

Confidence is clamped to [0, 1]. Confidence < 0.6 triggers LLM consultation when `UseLLM=true`.

## LLM Enhancement

When enabled and heuristic confidence is below threshold (0.6), LLM provides additional layout reasoning.

### LLMLayoutSelector

```go
type LLMLayoutSelector struct {
    llmManager          llm.Manager
    metricsCollector    metrics.Collector
    confidenceThreshold float64 // Default: 0.6
}

func NewLLMLayoutSelector(manager llm.Manager) *LLMLayoutSelector
func (s *LLMLayoutSelector) SelectLayoutWithLLM(ctx context.Context, req SelectionRequest) (*SelectionResult, error)
```

### LayoutOption (for LLM candidates)

```go
type LayoutOption struct {
    LayoutID   string
    LayoutName string
    Score      float64
    Pros       []string  // Auto-generated from scores (e.g., "Perfect type match")
    Cons       []string  // Auto-generated from scores (e.g., "Content may overflow")
}
```

### LLM Prompt Structure

The LLM receives a structured prompt with:
- **System prompt**: Expert presentation designer role with selection criteria
- **User prompt**: Slide details (title, type, position), content summary, and top 3 candidates with pros/cons

### LLM Response Schema

```json
{
  "selected_layout_id": "string",  // Must be from candidates (enum validation)
  "confidence": 0.85,              // 0.0-1.0
  "reasoning": "Concise explanation",
  "adjustments": [                 // Optional suggestions
    {
      "content_field": "bullets",
      "suggestion": "Consider splitting into two slides"
    }
  ]
}
```

Required fields: `selected_layout_id`, `confidence`, `reasoning`

### LLM Response Validation

The LLM response MUST be validated:

1. **Parsed JSON**: LLM manager provides `resp.Parsed` map
2. **Layout Validation**: `selected_layout_id` must be in candidate list
3. **Confidence Bounds**: Must be 0.0-1.0
4. **Layout Existence**: Must exist in original `req.Layouts`

On validation failure:
- Log the issue
- Fall back to heuristic top choice
- Add warning to result with "(LLM fallback)" suffix

### LLM Retry Strategy

On LLM failure:
1. Retry once (2 attempts total)
2. After both failures: Fall back to heuristics with warning
3. Never block - always have fallback

### Metrics Recording

The selector records metrics for each selection:
```go
type LayoutSelectionMetric struct {
    SlideType   string
    Method      string  // "heuristic" or "llm"
    Confidence  float64
    LayoutID    string
    LLMOverride bool    // True if LLM chose different layout than heuristic
}
```

## Visual Inspector

Post-rendering quality assessment using vision LLM.

### VisualInspector

```go
type VisualInspector struct {
    llmManager llm.Manager
}

func NewVisualInspector(manager llm.Manager) *VisualInspector
func (v *VisualInspector) Inspect(ctx context.Context, req InspectionRequest) (*QualityAssessment, error)
func (v *VisualInspector) InspectSlides(ctx context.Context, requests []InspectionRequest) (*MultiSlideAssessment, error)
```

### InspectionRequest

```go
type InspectionRequest struct {
    ImageData       []byte        // PNG image data of rendered slide
    SlideIndex      int           // Zero-based slide index
    SlideTitle      string        // Slide title for context
    LayoutName      string        // Layout name used
    ExpectedContent string        // Optional: what content should be on the slide
    ExpectsGraphic  bool          // Whether slide should contain charts/diagrams
    FontCategory    FontCategory  // Expected font category (serif, sans-serif, etc.)
}
```

### QualityAssessment

```go
type QualityAssessment struct {
    OverallScore         float64                   // 0.0-1.0
    Category             QualityCategory           // excellent/good/acceptable/poor
    TextIssues           []QualityIssue            // Text readability problems
    LayoutIssues         []QualityIssue            // Visual layout problems
    ContentIssues        []QualityIssue            // Content/consistency problems
    ContentDiscrepancies *ContentDiscrepancyReport // Expected vs visible content
    Suggestions          []string                  // Improvement recommendations
    Summary              string                    // Brief overall assessment
}

type QualityCategory string // "excellent" (0.9+), "good" (0.7-0.9), "acceptable" (0.5-0.7), "poor" (<0.5)

type QualityIssue struct {
    Severity    IssueSeverity // critical, major, minor
    Description string
    Location    string        // Optional location on slide
}
```

### MultiSlideAssessment

```go
type MultiSlideAssessment struct {
    Assessments     []*QualityAssessment
    AverageScore    float64
    OverallCategory QualityCategory
    TotalSlides     int
    CriticalIssues  int
    MajorIssues     int
    MinorIssues     int
    PassesThreshold bool  // True if avg >= 0.7 and no critical issues
}
```

### Font Classification

```go
type FontCategory string // serif, sans-serif, decorative, monospace

func ClassifyFont(fontName string) FontCategory
```

The inspector validates font consistency by classifying fonts and checking against expected categories.

### Score Penalty Calculation

Issue penalties are applied with caps:
- Critical: -0.3 per issue (max -0.6)
- Major: -0.15 per issue (max -0.3)
- Minor: -0.05 per issue (max -0.1)

## Acceptance Criteria

### AC1: Title Slide Selection
- Given slide with type "title" and layouts including "title-slide" tag
- When selected
- Then LayoutID matches a title-slide layout

### AC2: Content Fitting
- Given slide with 5 bullets and layouts with varying MaxBullets
- When selected
- Then selected layout has MaxBullets >= 5

### AC3: Overflow Warning
- Given slide with 10 bullets and max layout capacity of 6
- When selected
- Then Warnings includes overflow message

### AC4: Consistency Penalty
- Given previous slide used layout "A" and current slide fits layouts A and B equally
- When selected
- Then prefers layout B (variety)

### AC5: First Slide Bias
- Given first slide (position 0) that could use content or title layout
- When selected
- Then prefers title layout

### AC13: Last Slide Closing Layout
- Given last slide (position == TotalSlides-1) with closing/thank-you layouts available
- When selected
- Then prefers layouts tagged with `closing` or `thank-you`

### AC6: Confidence Scoring
- Given clear best layout with large score gap
- When selected
- Then Confidence > 0.8

### AC7: Low Confidence Triggers LLM
- Given ambiguous content with close layout scores and UseLLM=true
- When selected
- Then LLM is consulted

### AC8: LLM Fallback
- Given LLM returns invalid response
- When selected
- Then falls back to heuristic choice with warning

### AC9: LLM Response Validation
- Given LLM returns layout not in candidates
- When selected
- Then rejects LLM choice, uses heuristic

### AC10: Content Mapping
- Given slide with title and bullets
- When selected
- Then Mappings includes entries for both

### AC11: Truncation Detection
- Given body content exceeding placeholder capacity
- When selected
- Then relevant ContentMapping has Truncated=true

### AC12: No Suitable Layout
- Given slide requiring chart but no chart-capable layouts
- When selected
- Then returns error (not a warning)
- Note: For multi-column requirements, synthesis should have been triggered at analysis time (see `02-template-analyzer.md` Layout Synthesis). AC12 only fires when synthesis also cannot satisfy the requirement.

### AC14: Synthetic Layout Scoring
- Given synthetic two-column layout in Layouts alongside native single-content layout
- When slide type is "two-column"
- Then synthetic layout scores higher than single-content layout
- And synthetic layout is scored identically to any native two-column layout

## Context and Logging

The layout selector supports context-aware structured logging:

```go
// ContextWithLogger returns a context with the given logger
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context

// SelectLayoutWithContext is the context-aware entry point
func SelectLayoutWithContext(ctx context.Context, req SelectionRequest) (*SelectionResult, error)

// SelectLayout is a convenience wrapper using context.Background()
func SelectLayout(req SelectionRequest) (*SelectionResult, error)
```

Log levels:
- **Debug**: Individual layout scores, candidate preparation, LLM skip reasons
- **Info**: Selection results, LLM triggers, fallbacks

## Error Handling

| Scenario | Behavior |
|----------|----------|
| No layouts provided | Error: "no layouts provided" |
| No suitable layout for content type | Error with required capability description (synthesis should have provided missing capabilities at analysis time) |
| LLM timeout/error | Fallback to heuristics with warning |
| LLM rate limit | Fallback to heuristics, log warning |
| LLM invalid response | Fallback to heuristics with "(LLM fallback)" suffix |
| All layouts overflow | Select highest-scoring, add overflow warning |
| Visual inspection failure | Return poor assessment with error summary |

## Testing Requirements

### Heuristic Selection Tests
- Single obvious layout choice
- Multiple viable layouts (test scoring formula)
- Content overflow situations
- Layout variety enforcement (UsedLayouts tracking)
- First slide position logic (title-slide preference)
- Semantic matching (quote, agenda, timeline detection)
- Two-column slide placeholder requirements
- Edge cases: empty content, no layouts, no suitable layout

### LLM Selection Tests
- LLM integration (mock responses)
- LLM failure handling and retry logic
- Invalid layout selection fallback
- Confidence threshold triggering
- Metrics recording

### Visual Inspector Tests
- Image format detection (PNG, JPEG, GIF, WebP)
- Font classification accuracy
- Issue severity counting and penalty calculation
- Multi-slide assessment aggregation
- PassesThreshold calculation (avg >= 0.7, no critical issues)
- Context cancellation handling
