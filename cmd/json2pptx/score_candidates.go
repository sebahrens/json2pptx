package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/rhythm"
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
	// Score is the ranking score, clamped to [0, 100]. It is
	// slide_score - rhythm_penalty, except that invalid pattern inputs score 0
	// and fit-refused candidates start from the refusal ceiling (go-slide-creator-sbqm).
	Score int `json:"score"`
	// SlideScore is the score from fit findings alone for valid candidates
	// (100 - sum of severity weights), or 0 for invalid pattern inputs. It is
	// the number score_deck reports for the same valid slide, kept
	// comparable on purpose; Score is the ranking number.
	SlideScore int `json:"slide_score"`
	// RhythmPenalty is the penalty subtracted for pattern repetition / occupancy issues.
	RhythmPenalty int `json:"rhythm_penalty"`
	// Axes break the score into the things it actually measures, so two
	// candidates with the same total can still be told apart — and so an agent
	// can see WHICH dimension a candidate lost on.
	Axes CandidateAxes `json:"axes"`
	// Blocking is the number of refuse-action findings on this candidate. Any
	// at all means the engine would refuse to render it.
	Blocking int `json:"blocking_findings"`
	// Findings are the deterministic findings scoped to the target slide for this candidate.
	Findings []deterministic.ScoreFinding `json:"findings"`
	// Notes are human-readable rhythm explanations (empty when no penalty applied).
	Notes []string `json:"notes,omitempty"`
	// ParseError is set when the candidate JSON failed to decode; score will be 0 and
	// the candidate ranks last.
	ParseError string `json:"parse_error,omitempty"`
}

// CandidateAxes splits a candidate's score into its dimensions. Each is
// 100 minus the severity weights of the findings belonging to that dimension,
// so they are read the same way as the total.
type CandidateAxes struct {
	// Fit is geometry: overflow, density, occupancy, contrast, chart render.
	Fit int `json:"fit"`
	// Content is what the slide says: placeholder copy, emptiness, missing
	// titles, over-long headlines and bodies, dropped content.
	Content int `json:"content"`
	// Rhythm is how the candidate sits in the deck around it.
	Rhythm int `json:"rhythm"`
}

// contentAxisCodes are the finding codes that judge what a slide SAYS rather
// than how it fits. Splitting them out is what lets an agent see that a
// candidate lost on emptiness rather than on overflow (go-slide-creator-sbqm).
var contentAxisCodes = map[string]bool{
	patterns.ErrCodeWeakContent:            true,
	patterns.ErrCodeSlideNearlyEmpty:       true,
	patterns.ErrCodeMissingTitle:           true,
	patterns.ErrCodeDuplicateTitle:         true,
	patterns.ErrCodeHeadlineTooLong:        true,
	patterns.ErrCodeBodyTooLong:            true,
	patterns.ErrCodeContentDropped:         true,
	patterns.ErrCodePatternContentMismatch: true,
	patterns.ErrCodeTakeawayMissing:        true,
}

// candidateRefusalCeiling is where a candidate's ranking score starts once any
// finding would refuse the render.
//
// score_candidates answers "which of these should I use", and a slide the
// engine will not render is not an answer. Ranked from 100 like everything
// else, a near-empty candidate scored 70 and an unreadable one 55 — a spread an
// agent reads as "all three are fine, take the first" (go-slide-creator-sbqm).
const candidateRefusalCeiling = 50

// CandidateScoresResult is the top-level response for score_candidates.
type CandidateScoresResult struct {
	SlideIndex int              `json:"slide_index"`
	Candidates []CandidateScore `json:"candidates"`
	ModeUsed   string           `json:"mode_used"`
	// Tie is set when the top candidates score identically. Rank 1 is then the
	// first one you passed, not a verdict — an agent reading the ranking alone
	// would take a chart it was never told apart from the others
	// (go-slide-creator-sbqm).
	Tie string `json:"tie,omitempty"`
}

// topTieNote reports whether the leading candidates are indistinguishable to
// this tool, and says what it could not see.
func topTieNote(scored []CandidateScore) string {
	if len(scored) < 2 || scored[0].Score != scored[1].Score {
		return ""
	}
	tied := make([]int, 0, len(scored))
	for _, c := range scored {
		if c.Score != scored[0].Score {
			break
		}
		tied = append(tied, c.Index)
	}
	return fmt.Sprintf(
		"candidates %v all score %d on static analysis, so rank 1 is input order, not a verdict. This tool measures text fit, content and deck rhythm; it does not judge which visual reads better — render them (render_slide_image) and compare, or ask inspect_slide_images",
		tied, scored[0].Score)
}

func mcpScoreCandidatesTool() mcp.Tool {
	return mcp.NewTool("score_candidates",
		mcp.WithDescription(`Score multiple candidate slide_json values for a single slot in a deck without rendering.

Use this to choose between alternative slides for one position in a presentation. Unlike score_deck, this tool runs only static analysis — no PPTX generation, no tempdir — and returns each candidate ranked by a deterministic score.

WHAT IT MEASURES, reported as axes so two candidates with the same total can still be told apart:
- fit: geometry — overflow, density, occupancy, contrast, chart render.
- content: what the slide says — placeholder copy, emptiness, missing or over-long titles and bodies, dropped content, a pattern that does not match its content.
- rhythm: 5 if substituting this candidate would extend a pattern run of length 2 at this position, 15 for a run of 3+.

WHAT IT CANNOT MEASURE: which visual reads better. Two legible charts of the same data score the same. When the top candidates tie, the response carries a "tie" note saying so — rank 1 is then input order, not a verdict. Render them (render_slide_image) and compare, or ask inspect_slide_images.

score = slide_score - rhythm_penalty, clamped to [0, 100]. An invalid pattern or compose input that validate_input would reject scores 0 with its blocking codes. Other candidates carrying refuse-action fit findings start from 50 rather than 100. blocking_findings counts both classes. For valid inputs, slide_score stays comparable to score_deck (100 - sum of severity weights: refuse=25, shrink_or_split=15, review=5, info=0).

Candidates are sorted best→worst by score; ties broken by input order. Findings are returned per-candidate so the caller can see why each scored as it did.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaScoreCandidates)),
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
		return argRequired(request, "score_candidates", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	// Parse the deck.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nil), nil
	}
	applyDefaults(&input)
	mc.resolveInputNamedSettings(&input)

	if len(input.Slides) == 0 {
		return argRequired(request, "score_candidates", "presentation.slides", "array", []any{map[string]any{"layout_id": "title"}}, nextCallGetInputSchema()), nil
	}

	// slide_index validation against the (possibly empty) deck.
	slideIdx, err := extractSlideIndex(request, len(input.Slides))
	if err != nil {
		return argInvalidValue("score_candidates", "INVALID_PARAMETER", "slide_index", err.Error(), "integer", 0, nil), nil
	}

	// Candidates array.
	candidatesRaw, err := extractCandidates(request)
	if err != nil {
		return argInvalidValue("score_candidates", "INVALID_PARAMETER", "candidates", err.Error(), "array", []any{"kpi-3up"}, nil), nil
	}
	if len(candidatesRaw) == 0 {
		return argRequired(request, "score_candidates", "candidates", "array", []any{"kpi-3up", "stat-hero"}, nil), nil
	}

	// Resolve template.
	templateName := input.Template
	if override, err := request.RequireString("template"); err == nil && override != "" {
		templateName = override
	}
	if templateName == "" {
		return argRequired(request, "score_candidates", "template", "string", "midnight-blue", nextCallListTemplates()), nil
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
		Tie:        topTieNote(scored),
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
	// validate_input uses this same expansion/validation path. A malformed
	// pattern cannot render, so scoring its fit geometry (which cannot expand)
	// would falsely rank it as a clean 100-point candidate.
	patternCtx := patterns.ExpandContext{SlideWidth: slideWidth, SlideHeight: slideHeight, SlideIndex: slideIdx}
	if theme != nil {
		patternCtx.Theme = *theme
	}
	if validation := slidePatternDiagnostics(&candidate, slideIdx, patternCtx, patterns.Default()); len(validation) > 0 {
		out.Blocking = len(validation)
		out.Findings = make([]deterministic.ScoreFinding, 0, len(validation))
		for _, d := range validation {
			var fix *patterns.FixSuggestion
			if d.Fix != nil {
				fix = &patterns.FixSuggestion{Kind: d.Fix.Kind, Params: d.Fix.Params}
			}
			out.Findings = append(out.Findings, deterministic.ScoreFinding{
				Code: d.Code, Severity: "error", Message: d.Message, Fix: fix,
				Class: patterns.FindingClass(d.Code),
			})
		}
		out.Axes = CandidateAxes{Fit: 100, Content: 0, Rhythm: 100}
		out.Notes = []string{"pattern validation refused this candidate; repair its blocking findings before comparing visual quality"}
		return out
	}

	// 1. Static fit findings for the whole deck, filtered to the target slide.
	findings := collectFitFindings(substituted, layouts, slideWidth, slideHeight, theme)
	slideFindings := filterFindingsForSlide(findings, slideIdx)

	// 2. Compute slide score from fit findings alone, and split the same
	//    weights across the axes so a tie can be broken on the dimension that
	//    actually differs (go-slide-creator-sbqm).
	slideScore := 100
	fitAxis, contentAxis, blocking := 100, 100, 0
	scoreFindings := make([]deterministic.ScoreFinding, 0, len(slideFindings))
	for _, f := range slideFindings {
		w := deterministic.SeverityWeight[f.Action]
		slideScore -= w
		if contentAxisCodes[f.Code] {
			contentAxis -= w
		} else {
			fitAxis -= w
		}
		if f.Action == "refuse" {
			blocking++
		}
		scoreFindings = append(scoreFindings, deterministic.ScoreFinding{
			Code:     f.Code,
			Severity: scoreFindingSeverity(f.Action),
			Message:  f.Message,
			Fix:      f.Fix,
		})
	}
	slideScore = clampScore(slideScore)

	// 3. Compute rhythm penalty from pattern run extension at slideIdx.
	penalty, notes := rhythmPenaltyAt(substituted.Slides, slideIdx)

	// 4. Ranking score. A candidate the engine would refuse starts from the
	//    refusal ceiling: it is not a choice, and ranking it a few points below
	//    a clean slide reads as "all of these are fine".
	base := 100
	if blocking > 0 {
		base = candidateRefusalCeiling
		notes = append(notes, fmt.Sprintf(
			"%d finding(s) would refuse this candidate, so it is ranked from %d rather than 100 — fix those before comparing it on polish",
			blocking, candidateRefusalCeiling))
	}
	combined := clampScore(base - (100 - slideScore) - penalty)

	out.SlideScore = slideScore
	out.RhythmPenalty = penalty
	out.Score = combined
	out.Blocking = blocking
	out.Axes = CandidateAxes{
		Fit:     clampScore(fitAxis),
		Content: clampScore(contentAxis),
		Rhythm:  clampScore(100 - penalty),
	}
	out.Findings = scoreFindings
	out.Notes = notes
	return out
}

// clampScore keeps a score inside [0, 100].
func clampScore(n int) int {
	if n < 0 {
		return 0
	}
	if n > 100 {
		return 100
	}
	return n
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

// slidePatternName uses the rhythm analyzer's canonical visual identity so
// candidate scoring cannot drift from analyze_deck_rhythm's run detection.
func slidePatternName(s SlideInput) string {
	return rhythm.FingerprintKey(toRhythmSlide(s))
}
