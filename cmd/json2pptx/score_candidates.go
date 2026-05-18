package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// CandidateScore is the deterministic score for one candidate slide.
type CandidateScore struct {
	// Index is the 0-based position of this candidate in the input candidates array.
	Index int `json:"index"`
	// Rank is the 1-based ranking after sorting (1 = best score).
	Rank int `json:"rank"`
	// Score is the combined score (slide_score - rhythm_penalty), clamped to [0, 100].
	Score int `json:"score"`
	// SlideScore is the score from fit findings alone (100 - sum of severity weights).
	SlideScore int `json:"slide_score"`
	// RhythmPenalty is the penalty subtracted for pattern repetition / occupancy issues.
	RhythmPenalty int `json:"rhythm_penalty"`
	// Findings are the deterministic findings scoped to the target slide for this candidate.
	Findings []deterministic.ScoreFinding `json:"findings"`
	// Notes are human-readable rhythm explanations (empty when no penalty applied).
	Notes []string `json:"notes,omitempty"`
	// ParseError is set when the candidate JSON failed to decode; score will be 0 and
	// the candidate ranks last.
	ParseError string `json:"parse_error,omitempty"`
}

// CandidateScoresResult is the top-level response for score_candidates.
type CandidateScoresResult struct {
	SlideIndex int              `json:"slide_index"`
	Candidates []CandidateScore `json:"candidates"`
	ModeUsed   string           `json:"mode_used"`
}

func mcpScoreCandidatesTool() mcp.Tool {
	return mcp.NewTool("score_candidates",
		mcp.WithDescription(`Score multiple candidate slide_json values for a single slot in a deck without rendering.

Use this to choose between alternative slides (e.g., different patterns, different shape grids, different content shapes) for one position in a presentation. Unlike score_deck, this tool runs only static analysis — no PPTX generation, no tempdir — and returns each candidate ranked by a deterministic score.

Each candidate's score = slide_score - rhythm_penalty, clamped to [0, 100]:
- slide_score: 100 - sum(severity weights) of fit findings scoped to the target slide. Severity weights: refuse=25, shrink_or_split=15, review=5, info=0. Occupancy findings (pattern_underfilled, pattern_overcrowded) and overflow/contrast preflight findings are included here.
- rhythm_penalty: 5 if substituting this candidate would extend a pattern run of length 2 at this slide position, 15 if it would extend a run of length 3+. 0 otherwise.

Candidates are sorted best→worst by score; ties broken by input order. Findings are returned per-candidate so the caller can see why each scored as it did.`),
		mcp.WithRawOutputSchema(outputSchemaScoreCandidates),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description("Presentation definition. Same schema as generate_presentation."),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithNumber("slide_index",
			mcp.Required(),
			mcp.Description("0-based index of the slide slot to substitute candidates into."),
		),
		mcp.WithArray("candidates",
			mcp.Required(),
			mcp.Description("Array of candidate slide_json objects. Each entry has the same shape as a single slide in presentation.slides[] and replaces the slide at slide_index for scoring."),
		),
		mcp.WithString("template",
			mcp.Description("Template name override. If omitted, uses the template field from the presentation object."),
		),
	)
}

func (mc *mcpConfig) handleScoreCandidates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "presentation is required"), nil
	}

	// Parse the deck.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return mcpParseError("INVALID_JSON", "presentation", fmt.Sprintf("invalid JSON: %v", err)), nil
	}
	applyDefaults(&input)
	mc.resolveInputNamedSettings(&input)

	if len(input.Slides) == 0 {
		return api.MCPSimpleError("MISSING_PARAMETER", "at least one slide is required in presentation"), nil
	}

	// slide_index validation against the (possibly empty) deck.
	slideIdx, err := extractSlideIndex(request, len(input.Slides))
	if err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}

	// Candidates array.
	candidatesRaw, err := extractCandidates(request)
	if err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}
	if len(candidatesRaw) == 0 {
		return api.MCPSimpleError("MISSING_PARAMETER", "candidates array must contain at least one entry"), nil
	}

	// Resolve template.
	templateName := input.Template
	if override, err := request.RequireString("template"); err == nil && override != "" {
		templateName = override
	}
	if templateName == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "template is required (in presentation or as template parameter)"), nil
	}

	templatePath, templateCleanup, err := resolveTemplatePath(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(templateName, mc.templatesDir)), nil
	}
	defer templateCleanup()

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err)), nil
	}
	defer func() { _ = reader.Close() }()

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err)), nil
	}
	slideWidth, slideHeight := template.ParseSlideDimensions(reader)
	theme := template.ParseTheme(reader)

	// Score each candidate by substituting at slideIdx and running static analysis.
	scored := make([]CandidateScore, len(candidatesRaw))
	for i, raw := range candidatesRaw {
		scored[i] = scoreCandidate(i, slideIdx, raw, &input, layouts, slideWidth, slideHeight, &theme)
	}

	// Rank: sort by Score desc, then by Index asc for stable ties.
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Index < scored[j].Index
	})
	for i := range scored {
		scored[i].Rank = i + 1
	}

	result := &CandidateScoresResult{
		SlideIndex: slideIdx,
		Candidates: scored,
		ModeUsed:   "deterministic",
	}

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// extractCandidates reads the candidates array as raw JSON messages so each
// can be unmarshaled (or its parse failure reported) per-candidate.
func extractCandidates(request mcp.CallToolRequest) ([]json.RawMessage, error) {
	args := request.GetArguments()
	raw, ok := args["candidates"]
	if !ok {
		return nil, fmt.Errorf("candidates is required")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("candidates: %w", err)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, fmt.Errorf("candidates must be an array of slide objects: %w", err)
	}
	return arr, nil
}

// scoreCandidate substitutes one candidate at slideIdx and computes its
// deterministic score from static analysis only.
func scoreCandidate(
	candIdx, slideIdx int,
	candidateJSON json.RawMessage,
	baseInput *PresentationInput,
	layouts []types.LayoutMetadata,
	slideWidth, slideHeight int64,
	theme *types.ThemeInfo,
) CandidateScore {
	out := CandidateScore{
		Index:      candIdx,
		Score:      0,
		SlideScore: 0,
	}

	// Parse the candidate as a SlideInput.
	var candidate SlideInput
	if err := strictUnmarshalJSON(candidateJSON, &candidate); err != nil {
		out.ParseError = fmt.Sprintf("invalid candidate JSON: %v", err)
		return out
	}

	// Build a deck copy with the candidate substituted at slideIdx.
	substituted := substituteSlide(baseInput, slideIdx, candidate)

	// 1. Static fit findings for the whole deck, filtered to the target slide.
	findings := collectFitFindings(substituted, layouts, slideWidth, slideHeight, theme)
	slideFindings := filterFindingsForSlide(findings, slideIdx)

	// 2. Compute slide score from fit findings alone.
	slideScore := 100
	scoreFindings := make([]deterministic.ScoreFinding, 0, len(slideFindings))
	for _, f := range slideFindings {
		w := deterministic.SeverityWeight[f.Action]
		slideScore -= w
		scoreFindings = append(scoreFindings, deterministic.ScoreFinding{
			Code:     f.Code,
			Severity: scoreFindingSeverity(f.Action),
			Message:  f.Message,
			Fix:      f.Fix,
		})
	}
	if slideScore < 0 {
		slideScore = 0
	}

	// 3. Compute rhythm penalty from pattern run extension at slideIdx.
	penalty, notes := rhythmPenaltyAt(substituted.Slides, slideIdx)

	// 4. Combined score.
	combined := slideScore - penalty
	if combined < 0 {
		combined = 0
	}

	out.SlideScore = slideScore
	out.RhythmPenalty = penalty
	out.Score = combined
	out.Findings = scoreFindings
	out.Notes = notes
	return out
}

// substituteSlide returns a shallow copy of input with input.Slides[slideIdx]
// replaced by candidate. The Slides slice is copied so callers can mutate it
// without aliasing the original deck.
func substituteSlide(input *PresentationInput, slideIdx int, candidate SlideInput) *PresentationInput {
	clone := *input
	clone.Slides = make([]SlideInput, len(input.Slides))
	copy(clone.Slides, input.Slides)
	clone.Slides[slideIdx] = candidate
	return &clone
}

// scoreFindingSeverity mirrors deterministic.actionToSeverity, which is
// unexported, so we keep a local copy here. Action ↔ severity mapping must
// stay in sync with internal/visualqa/deterministic/checker.go.
func scoreFindingSeverity(action string) string {
	switch action {
	case "refuse":
		return "error"
	case "shrink_or_split":
		return "warning"
	case "review":
		return "warning"
	case "info":
		return "info"
	default:
		return "info"
	}
}

// rhythmPenaltyAt computes a deterministic rhythm penalty for the slide at
// slideIdx by looking at consecutive same-pattern neighbors. Mirrors the
// pattern-run threshold used in compositionAxis (3+ flagged as warning), but
// returns a numeric penalty appropriate for per-slide candidate ranking:
//
//   - run length 1 (no repetition):       0
//   - run length 2 (one same-pattern neighbor): 5
//   - run length 3 or more:                15
//
// Note: this looks at the substituted deck, so the candidate's own pattern
// already participates in the run computation.
func rhythmPenaltyAt(slides []SlideInput, slideIdx int) (int, []string) {
	if slideIdx < 0 || slideIdx >= len(slides) {
		return 0, nil
	}

	target := slidePatternName(slides[slideIdx])

	// Count consecutive same-pattern neighbors centered on slideIdx.
	runLen := 1
	// Walk backward.
	for j := slideIdx - 1; j >= 0; j-- {
		if slidePatternName(slides[j]) != target {
			break
		}
		runLen++
	}
	// Walk forward.
	for j := slideIdx + 1; j < len(slides); j++ {
		if slidePatternName(slides[j]) != target {
			break
		}
		runLen++
	}

	switch {
	case runLen >= 3:
		return 15, []string{fmt.Sprintf("pattern %q would form a run of %d consecutive slides through index %d", target, runLen, slideIdx)}
	case runLen == 2:
		return 5, []string{fmt.Sprintf("pattern %q would form a run of 2 consecutive slides at index %d", target, slideIdx)}
	default:
		return 0, nil
	}
}

// slidePatternName returns the pattern-name fingerprint used by the rhythm
// analyzer for one slide. Mirrors the dispatch in fingerprint() so the
// candidate scorer and analyze_deck_rhythm stay in sync.
func slidePatternName(s SlideInput) string {
	switch {
	case s.Compose != nil:
		return "compose"
	case s.Pattern != nil && s.Pattern.Name != "":
		return s.Pattern.Name
	case s.ShapeGrid != nil:
		return "shape_grid"
	default:
		if s.SlideType != "" {
			return s.SlideType
		}
		return "content"
	}
}

