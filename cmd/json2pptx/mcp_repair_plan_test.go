package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

func proposeMC(t *testing.T) *mcpConfig {
	t.Helper()
	return repairMC(t)
}

// TestProposeRepairs_FitFindingProducesRankedDirective verifies that a single
// fit finding with an attached fix is translated into a directive routed to
// the correct slide, with the fix kind preserved and a tool_call generated.
func TestProposeRepairs_FitFindingProducesRankedDirective(t *testing.T) {
	mc := proposeMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "A reasonably long title that should be shortened by the repair tool",
		},
	)

	findings := []any{
		map[string]any{
			"path":    "/slides/0/content/title",
			"code":    "TEXT_OVERFLOW",
			"message": "title overflows allowed extent",
			"action":  "shrink_or_split",
			"fix": map[string]any{
				"kind": "shorten_title",
				"params": map[string]any{
					"max_length": float64(30),
				},
			},
		},
	}

	result, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"findings":     findings,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var out proposeRepairsOutput
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(out.Slides) != 1 {
		t.Fatalf("expected 1 slide entry, got %d", len(out.Slides))
	}
	slide := out.Slides[0]
	if slide.SlideIndex != 0 {
		t.Errorf("expected slide_index=0, got %d", slide.SlideIndex)
	}
	if slide.FindingCount != 1 {
		t.Errorf("expected finding_count=1, got %d", slide.FindingCount)
	}
	if len(slide.Directives) != 1 {
		t.Fatalf("expected 1 directive, got %d", len(slide.Directives))
	}

	d := slide.Directives[0]
	if d.Kind != "shorten_title" {
		t.Errorf("expected kind=shorten_title, got %q", d.Kind)
	}
	if d.Rank != 0 {
		t.Errorf("expected rank=0, got %d", d.Rank)
	}
	if d.Source.Type != "fit" {
		t.Errorf("expected source.type=fit, got %q", d.Source.Type)
	}
	if d.Source.Code != "TEXT_OVERFLOW" {
		t.Errorf("expected source.code=TEXT_OVERFLOW, got %q", d.Source.Code)
	}
	if d.Source.Action != "shrink_or_split" {
		t.Errorf("expected source.action=shrink_or_split, got %q", d.Source.Action)
	}
	if d.Source.Severity != "warning" {
		t.Errorf("expected severity=warning, got %q", d.Source.Severity)
	}
	if d.ToolCall == nil || d.ToolCall.Tool != "repair_slide" {
		t.Fatalf("expected tool_call to repair_slide, got %#v", d.ToolCall)
	}

	// Verify the tool_call carries a directly-applyable fix payload.
	fixes, _ := d.ToolCall.ArgsTemplate["fixes"].([]any)
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix in tool_call.fixes, got %d", len(fixes))
	}
	fixObj, _ := fixes[0].(map[string]any)
	if fixObj["kind"] != "shorten_title" {
		t.Errorf("expected tool_call fix kind=shorten_title, got %v", fixObj["kind"])
	}

	if out.Summary.TotalFindings != 1 || out.Summary.MappedFindings != 1 || out.Summary.UnmappedFindings != 0 {
		t.Errorf("unexpected summary: %+v", out.Summary)
	}
	if slide.BatchToolCall == nil {
		t.Fatal("expected batch_tool_call to be populated")
	}
	if v, ok := slide.BatchToolCall.ArgsTemplate["slide_index"].(float64); !ok || int(v) != 0 {
		t.Errorf("expected batch slide_index=0, got %v (%T)", slide.BatchToolCall.ArgsTemplate["slide_index"], slide.BatchToolCall.ArgsTemplate["slide_index"])
	}
}

// TestProposeRepairs_VisualFindingUsesCategoryMapping verifies that a visual
// QA finding without explicit suggested_fixes falls back to the canonical
// category→fix mapping.
func TestProposeRepairs_VisualFindingUsesCategoryMapping(t *testing.T) {
	mc := proposeMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "Title",
		},
	)

	findings := []any{
		map[string]any{
			"slide_index": float64(0),
			"slide_type":  "content",
			"severity":    "P1",
			"category":    "text_overflow",
			"description": "Body text wraps under footer",
		},
	}

	result, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"findings":     findings,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var out proposeRepairsOutput
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(out.Slides) != 1 {
		t.Fatalf("expected 1 slide entry, got %d", len(out.Slides))
	}
	// text_overflow → reduce_cell_text, split_at_row, reshape_grid (3 candidates).
	if len(out.Slides[0].Directives) != 3 {
		t.Fatalf("expected 3 directives from category mapping, got %d", len(out.Slides[0].Directives))
	}

	kinds := make([]string, 0, len(out.Slides[0].Directives))
	for _, d := range out.Slides[0].Directives {
		kinds = append(kinds, d.Kind)
		if d.Source.Type != "visual" {
			t.Errorf("expected source.type=visual, got %q", d.Source.Type)
		}
		if d.Source.Category != "text_overflow" {
			t.Errorf("expected source.category=text_overflow, got %q", d.Source.Category)
		}
		if d.Source.Severity != "P1" {
			t.Errorf("expected severity=P1, got %q", d.Source.Severity)
		}
	}
	// Order from visualqa.SuggestedFixesForCategory should be preserved.
	wantPrefix := []string{"reduce_cell_text", "split_at_row", "reshape_grid"}
	for i, w := range wantPrefix {
		if kinds[i] != w {
			t.Errorf("directive %d: expected %q, got %q (full=%v)", i, w, kinds[i], kinds)
		}
	}
}

// TestProposeRepairs_ReviewOnlyCategoryUnmapped checks that visual QA
// categories with no fix mapping land in unmapped[].
func TestProposeRepairs_ReviewOnlyCategoryUnmapped(t *testing.T) {
	mc := proposeMC(t)
	deck := minimalDeck(
		map[string]any{"placeholder_id": "title", "type": "text", "text_value": "T"},
	)
	findings := []any{
		map[string]any{
			"slide_index": float64(0),
			"slide_type":  "content",
			"severity":    "P2",
			"category":    "image_quality",
			"description": "Image is blurry",
		},
	}

	result, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"findings":     findings,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var out proposeRepairsOutput
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Slides) != 0 {
		t.Errorf("expected 0 slide entries (all unmapped), got %d", len(out.Slides))
	}
	if len(out.Unmapped) != 1 {
		t.Fatalf("expected 1 unmapped entry, got %d", len(out.Unmapped))
	}
	if out.Unmapped[0].Reason != "review_only_category" {
		t.Errorf("expected reason=review_only_category, got %q", out.Unmapped[0].Reason)
	}
	if out.Unmapped[0].Category != "image_quality" {
		t.Errorf("expected category=image_quality, got %q", out.Unmapped[0].Category)
	}
	if out.Summary.UnmappedFindings != 1 || out.Summary.MappedFindings != 0 {
		t.Errorf("unexpected summary: %+v", out.Summary)
	}
}

// TestProposeRepairs_RankingPrioritizesRefuseAction verifies that findings
// with higher action rank (refuse > shrink_or_split > review) end up first.
func TestProposeRepairs_RankingPrioritizesRefuseAction(t *testing.T) {
	mc := proposeMC(t)
	deck := minimalDeck(
		map[string]any{"placeholder_id": "title", "type": "text", "text_value": "T"},
	)

	findings := []any{
		// Lowest-severity finding first to make sure ordering doesn't just
		// preserve input order.
		map[string]any{
			"path":    "/slides/0/content/title",
			"code":    "INFO_THING",
			"action":  "info",
			"message": "info",
			"fix": map[string]any{
				"kind":   "shorten_title",
				"params": map[string]any{"max_length": float64(40)},
			},
		},
		// Highest-severity should win.
		map[string]any{
			"path":    "/slides/0/content/title",
			"code":    "REFUSE_CODE",
			"action":  "refuse",
			"message": "refuse",
			"fix": map[string]any{
				"kind":   "swap_layout",
				"params": map[string]any{"layout_id": "slideLayout1"},
			},
		},
		map[string]any{
			"path":    "/slides/0/content/title",
			"code":    "WARN_CODE",
			"action":  "shrink_or_split",
			"message": "warn",
			"fix": map[string]any{
				"kind":   "reduce_text",
				"params": map[string]any{"max_length": float64(20)},
			},
		},
	}

	result, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"findings":     findings,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var out proposeRepairsOutput
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(out.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(out.Slides))
	}
	dirs := out.Slides[0].Directives
	if len(dirs) != 3 {
		t.Fatalf("expected 3 directives, got %d", len(dirs))
	}
	if dirs[0].Source.Action != "refuse" || dirs[0].Rank != 0 {
		t.Errorf("expected first directive to be the refuse-action one, got action=%q rank=%d", dirs[0].Source.Action, dirs[0].Rank)
	}
	if dirs[1].Source.Action != "shrink_or_split" {
		t.Errorf("expected second directive to be shrink_or_split, got %q", dirs[1].Source.Action)
	}
	if dirs[2].Source.Action != "info" {
		t.Errorf("expected third directive to be info, got %q", dirs[2].Source.Action)
	}
}

// TestProposeRepairs_NonRepairKindUnmapped verifies that fix.kind values not
// in the repair_slide vocabulary land in unmapped[] (e.g. "adopt_pattern").
func TestProposeRepairs_NonRepairKindUnmapped(t *testing.T) {
	mc := proposeMC(t)
	deck := minimalDeck(
		map[string]any{"placeholder_id": "title", "type": "text", "text_value": "T"},
	)
	findings := []any{
		map[string]any{
			"path":    "/slides/0/shape_grid",
			"code":    "PATTERN_RECOMMENDED",
			"action":  "review",
			"message": "consider adopting a named pattern",
			"fix": map[string]any{
				"kind":   "adopt_pattern",
				"params": map[string]any{"filled_slots": 3},
			},
		},
	}

	result, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"findings":     findings,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var out proposeRepairsOutput
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Slides) != 0 {
		t.Errorf("expected 0 slide entries (kind not repairable), got %d", len(out.Slides))
	}
	if len(out.Unmapped) != 1 {
		t.Fatalf("expected 1 unmapped, got %d", len(out.Unmapped))
	}
	if !strings.HasPrefix(out.Unmapped[0].Reason, "fix_kind_not_repairable:") {
		t.Errorf("expected fix_kind_not_repairable prefix, got %q", out.Unmapped[0].Reason)
	}
}

// TestProposeRepairs_GroupsBySlide verifies that findings spanning multiple
// slides each land in their own slide bucket.
func TestProposeRepairs_GroupsBySlide(t *testing.T) {
	mc := proposeMC(t)
	// Build a 2-slide deck.
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []map[string]any{
			{"layout_id": "slideLayout2", "content": []map[string]any{
				{"placeholder_id": "title", "type": "text", "text_value": "T1"},
			}},
			{"layout_id": "slideLayout2", "content": []map[string]any{
				{"placeholder_id": "title", "type": "text", "text_value": "T2"},
			}},
		},
	}
	deckBytes, _ := json.Marshal(deck)

	findings := []any{
		map[string]any{
			"path":   "/slides/1/content/title",
			"code":   "TEXT_OVERFLOW",
			"action": "shrink_or_split",
			"fix":    map[string]any{"kind": "shorten_title", "params": map[string]any{"max_length": float64(15)}},
		},
		map[string]any{
			"path":   "/slides/0/content/title",
			"code":   "TEXT_OVERFLOW",
			"action": "shrink_or_split",
			"fix":    map[string]any{"kind": "shorten_title", "params": map[string]any{"max_length": float64(15)}},
		},
	}

	result, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(string(deckBytes)),
		"findings":     findings,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var out proposeRepairsOutput
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(out.Slides) != 2 {
		t.Fatalf("expected 2 slide entries, got %d", len(out.Slides))
	}
	indices := make([]int, 0, len(out.Slides))
	for _, s := range out.Slides {
		indices = append(indices, s.SlideIndex)
	}
	sort.Ints(indices)
	if len(indices) < 2 || indices[0] != 0 || indices[1] != 1 {
		t.Errorf("expected slide indices [0,1], got %v", indices)
	}
	if out.Summary.SlidesAffected != 2 {
		t.Errorf("expected slides_affected=2, got %d", out.Summary.SlidesAffected)
	}
}

// TestProposeRepairs_DoesNotMutateDeck verifies that the deck JSON passed in
// is not mutated by the propose call — it's a read-only planner.
func TestProposeRepairs_DoesNotMutateDeck(t *testing.T) {
	mc := proposeMC(t)
	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "A long title that would normally be shortened",
		},
	)
	originalDeck := deck

	findings := []any{
		map[string]any{
			"path":   "/slides/0/content/title",
			"code":   "TEXT_OVERFLOW",
			"action": "shrink_or_split",
			"fix":    map[string]any{"kind": "shorten_title", "params": map[string]any{"max_length": float64(10)}},
		},
	}

	parsed := mustParseJSON(deck)
	_, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
		"presentation": parsed,
		"findings":     findings,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The input deck JSON string should be unchanged structurally — round-trip
	// via JSON and compare.
	roundTripped, _ := json.Marshal(parsed)
	var original map[string]any
	_ = json.Unmarshal([]byte(originalDeck), &original)
	originalRound, _ := json.Marshal(original)
	if string(roundTripped) != string(originalRound) {
		t.Errorf("expected deck JSON to be unchanged.\noriginal: %s\nafter:    %s", originalRound, roundTripped)
	}
}

// TestProposeRepairs_MissingFindings rejects calls without a findings array.
func TestProposeRepairs_MissingFindings(t *testing.T) {
	mc := proposeMC(t)
	deck := minimalDeck(
		map[string]any{"placeholder_id": "title", "type": "text", "text_value": "T"},
	)
	result, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result when findings missing")
	}
}

// TestProposeRepairs_RegisteredInCatalog ensures the new tool is wired into
// mcpToolCatalog so capabilities responses stay truthful.
func TestProposeRepairs_RegisteredInCatalog(t *testing.T) {
	for _, e := range mcpToolCatalog() {
		if e.Name == "propose_repairs" {
			return
		}
	}
	t.Fatalf("propose_repairs not present in mcpToolCatalog()")
}

// TestProposeRepairs_IconFindingsHandledAsUnmapped verifies that the
// structured per-icon findings emitted by resolveIconPaths flow through
// propose_repairs without crashing. ICON_* codes don't yet map to a
// repair_slide fix kind (the broken icon needs an agent decision: bundled
// name, alternate path, URL, or inline svg_data), so they should appear in
// the `unmapped` bucket with their slide_index, code, path, and message
// preserved so the agent can act on each one individually.
func TestProposeRepairs_IconFindingsHandledAsUnmapped(t *testing.T) {
	mc := proposeMC(t)

	deck := minimalDeck(
		map[string]any{"placeholder_id": "title", "type": "text", "text_value": "T"},
	)

	findings := []any{
		map[string]any{
			"path":     "/slides/0/shape_grid/rows/0/cells/0/icon",
			"code":     "ICON_PATH",
			"message":  `icon path "missing.svg": no such file`,
			"severity": "error",
		},
		map[string]any{
			"path":     "/slides/0/shape_grid/rows/0/cells/1/icon",
			"code":     "ICON_AMBIGUOUS",
			"message":  "icon must have exactly one of 'name', 'path', 'url', or 'svg_data'",
			"severity": "error",
		},
		map[string]any{
			"path":     "/slides/0/shape_grid/rows/0/cells/2/icon",
			"code":     "ICON_MISSING",
			"message":  "icon must have one of 'name', 'path', 'url', or 'svg_data'",
			"severity": "error",
		},
	}

	result, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"findings":     findings,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var out proposeRepairsOutput
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out.Summary.TotalFindings != 3 {
		t.Errorf("expected 3 total findings, got %d", out.Summary.TotalFindings)
	}
	if out.Summary.UnmappedFindings != 3 {
		t.Errorf("expected 3 unmapped findings, got %d", out.Summary.UnmappedFindings)
	}
	if out.Summary.MappedFindings != 0 {
		t.Errorf("expected 0 mapped findings, got %d", out.Summary.MappedFindings)
	}
	if len(out.Unmapped) != 3 {
		t.Fatalf("expected 3 entries in unmapped, got %d", len(out.Unmapped))
	}

	for _, u := range out.Unmapped {
		if !strings.HasPrefix(u.Code, "ICON_") {
			t.Errorf("expected ICON_* code, got %q", u.Code)
		}
		if u.Path == "" {
			t.Errorf("expected non-empty path for code %s", u.Code)
		}
		if u.SlideIndex == nil || *u.SlideIndex != 0 {
			t.Errorf("expected slide_index=0 in unmapped entry for %s, got %v", u.Code, u.SlideIndex)
		}
		if u.Reason != "no_fix_attached" {
			t.Errorf("expected reason=no_fix_attached, got %q", u.Reason)
		}
	}
}

// TestProposeRepairs_ToolCallRoundTripsToRepairSlide verifies the contract bug
// fix: emitted directive tool_calls and per-slide batch_tool_calls must carry
// a `presentation` arg so an agent can submit them verbatim to repair_slide.
// Without this, repair_slide rejects every emitted call with
// MISSING_PARAMETER "presentation is required".
func TestProposeRepairs_ToolCallRoundTripsToRepairSlide(t *testing.T) {
	mc := proposeMC(t)

	deck := minimalDeck(
		map[string]any{
			"placeholder_id": "title",
			"type":           "text",
			"text_value":     "A reasonably long title that should be shortened by the repair tool",
		},
	)

	findings := []any{
		map[string]any{
			"path":    "/slides/0/content/title",
			"code":    "TEXT_OVERFLOW",
			"message": "title overflows allowed extent",
			"action":  "shrink_or_split",
			"fix": map[string]any{
				"kind":   "shorten_title",
				"params": map[string]any{"max_length": float64(30)},
			},
		},
	}

	result, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
		"findings":     findings,
	}))
	if err != nil {
		t.Fatalf("propose_repairs unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("propose_repairs tool error: %s", textContent(result))
	}

	var out proposeRepairsOutput
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatalf("unmarshal proposeRepairs: %v", err)
	}
	if len(out.Slides) != 1 || len(out.Slides[0].Directives) == 0 {
		t.Fatalf("expected at least one directive, got %+v", out.Slides)
	}

	directive := out.Slides[0].Directives[0]
	if directive.ToolCall == nil {
		t.Fatal("expected directive.ToolCall to be populated")
	}

	// Assert the per-directive tool_call carries presentation, slide_index, fixes.
	directiveArgs := directive.ToolCall.ArgsTemplate
	if _, ok := directiveArgs["presentation"].(map[string]any); !ok {
		t.Fatalf("directive.ToolCall.ArgsTemplate missing presentation object; got keys=%v", argKeys(directiveArgs))
	}
	if _, ok := directiveArgs["slide_index"]; !ok {
		t.Errorf("directive.ToolCall.ArgsTemplate missing slide_index")
	}
	if _, ok := directiveArgs["fixes"].([]any); !ok {
		t.Errorf("directive.ToolCall.ArgsTemplate missing fixes array")
	}

	// Round-trip: feed the directive's tool_call args directly into
	// handleRepairSlide and confirm it succeeds (no MISSING_PARAMETER).
	repairResult, err := mc.handleRepairSlide(context.Background(), makeRequest(directiveArgs))
	if err != nil {
		t.Fatalf("repair_slide round-trip unexpected error: %v", err)
	}
	if repairResult.IsError {
		t.Fatalf("repair_slide rejected propose_repairs tool_call: %s", textContent(repairResult))
	}
	var repairOut repairSlideOutput
	if err := json.Unmarshal([]byte(textContent(repairResult)), &repairOut); err != nil {
		t.Fatalf("unmarshal repair_slide: %v", err)
	}
	if len(repairOut.AppliedFixes) == 0 || !repairOut.AppliedFixes[0].Applied {
		t.Errorf("expected shorten_title to be applied by repair_slide, got %+v", repairOut.AppliedFixes)
	}

	// Assert the batch_tool_call also carries presentation, then round-trip it.
	batch := out.Slides[0].BatchToolCall
	if batch == nil {
		t.Fatal("expected batch_tool_call to be populated")
	}
	if _, ok := batch.ArgsTemplate["presentation"].(map[string]any); !ok {
		t.Fatalf("batch_tool_call.ArgsTemplate missing presentation object; got keys=%v", argKeys(batch.ArgsTemplate))
	}
	batchResult, err := mc.handleRepairSlide(context.Background(), makeRequest(batch.ArgsTemplate))
	if err != nil {
		t.Fatalf("repair_slide batch round-trip unexpected error: %v", err)
	}
	if batchResult.IsError {
		t.Fatalf("repair_slide rejected batch_tool_call: %s", textContent(batchResult))
	}
}

func argKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
