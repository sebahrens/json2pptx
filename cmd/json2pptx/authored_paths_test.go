package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// A deck whose slides the engine expands, and a finding about something the
// engine built, is reported where the author wrote it
// (go-slide-creator-9564b, -2v8me, -o45pn). The decks below carry one fault of
// each family; the tests pin its authored path, the rendered slide_number and
// the engine's locator, and hold the tools that take a path back to both
// forms.

// expandedSplitDeck: a cover, a split_slide of seven rows in pages of three
// (rendered slides 2-4) and two more slides (rendered slides 5 and 6). The
// faults sit on page 2 (row 4), page 3 (row 6), the envelope's base and the
// last slide.
const expandedSplitDeck = `{"template": "midnight-blue", "slides": [
  ` + verdictCover + `,
  {"type": "split_slide", "split": {"by": "table.rows", "group_size": 3, "repeat_headers": true, "title_suffix": " ({page}/{total})"},
   "base": {"layout_id": "content", "source": "Finance, FY25", "zz": 1, "content": [
     {"placeholder_id": "title", "type": "text", "text_value": "Seven vendors were compared on cost"},
     {"placeholder_id": "body", "type": "table", "table_value": {"headers": ["Vendor", "Cost"], "rows": [
       ["A", "1"], ["B", "2"], ["C", "3"], ["D", "4"],
       ["E", {"content": "5", "conditional": {"rule": "bogus", "fill": "accent2"}}],
       ["F", "6"], ["G", "7", "extra"]]}}]}},
  {"layout_id": "content", "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Revenue grew in every region"},
    {"placeholder_id": "body", "type": "bullets", "bullets_value": ["North up 12%", "South up 8%", "West up 5%", "East up 4%"]}]},
  {"layout_id": "content", "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Two options were compared"},
    {"placeholder_id": "body", "type": "table", "table_value": {"headers": ["A", "B"], "rows": [["1", "2", "3"]]}}]}]}`

// expandedStructureDeck has no slides array: cover, agenda, two dividers and
// one slide per section (rendered slides 1-6).
const expandedStructureDeck = `{"template": "midnight-blue", "structure": {"auto_agenda": true, "qq": 1,
  "cover": {"content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Quarterly review"},
    {"placeholder_id": "subtitle", "type": "text", "text_value": "Results and outlook"}]},
  "sections": [
    {"title": "Results", "yy": 1, "slides": [{"layout_id": "content", "zz": 1, "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Three options were compared"},
      {"placeholder_id": "body", "type": "table", "table_value": {"headers": ["A", "B"], "rows": [["1", "2", "3"]]}}]}]},
    {"title": "Outlook", "slides": [{"layout_id": "blank-title", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Three numbers tell the story"}],
      "pattern": {"name": "kpi-3up", "values": [
        {"big": "12%", "small": "Growth"}, {"big": "94", "small": "NPS"}, {"big": "3", "small": "Launches"}]}}]}]}}`

// expandedGridsDeck: a vertical envelope under a banner (pattern, pattern,
// diagram), and a horizontal one whose second segment is a nested envelope.
const expandedGridsDeck = `{"template": "midnight-blue", "slides": [
  ` + verdictCover + `,
  {"layout_id": "blank-title", "source": "Finance, FY25", "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Three numbers and four steps tell the story"}],
   "compose": {"direction": "vertical", "banner": {"text": "FY25 in one page"}, "segments": [
     {"pattern": {"name": "kpi-3up", "values": [
        {"big": "12%", "small": "Growth in recurring revenue, all regions"}, {"big": "94", "small": "NPS"}, {"big": "3", "small": "Launches"}]}},
     {"pattern": {"name": "process-flow", "values": {"steps": [
        {"label": "Plan the work with stakeholders"}, {"label": "Build"}, {"label": "Test"}, {"label": "Launch"}]}}},
     {"diagram": {"type": "pyramid", "data": {"levels": [{"label": "Transformation of the operating model"}, {"label": "B"}, {"label": "C"}]}}}]}},
  {"layout_id": "blank-title", "source": "Finance, FY25", "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Two envelopes sit side by side here"}],
   "compose": {"direction": "horizontal", "segments": [
     {"size_pct": 80, "pattern": {"name": "kpi-2up", "values": [
        {"big": "12%", "small": "Growth in recurring revenue, all regions"}, {"big": "94", "small": "NPS"}]}},
     {"size_pct": 20, "compose": {"direction": "vertical", "segments": [
       {"pattern": {"name": "stat-hero", "values": {"value": "42", "label": "A label that is long enough to wrap in a narrow column"}}},
       {"diagram": {"type": "pyramid", "data": {"levels": [{"label": "Transformation"}, {"label": "B"}, {"label": "C"}]}}}]}}]}}]}`

// legacySplitDeck is a split_slide whose table is written under "value".
const legacySplitDeck = `{"template": "midnight-blue", "slides": [
  {"type": "split_slide", "split": {"by": "table.rows", "group_size": 2, "repeat_headers": true},
   "base": {"layout_id": "content", "content": [
     {"placeholder_id": "title", "type": "text", "text_value": "Five vendors were compared"},
     {"placeholder_id": "body", "type": "table", "value": {"headers": ["Vendor", "Cost"], "rows": [
       ["A", "1"], ["B", "2"], ["C", "3"], ["D", "4"], ["E", "5"]]}}]}}]}`

// validStructureDeck is expandedStructureDeck without its refusal.
var validStructureDeck = strings.NewReplacer(`[["1", "2", "3"]]`, `[["1", "2"]]`).Replace(expandedStructureDeck)

// findingRecord is one finding of a decoded answer.
type findingRecord struct {
	Code, Path, Locator string
	SlideNumber         int
	NextToolCall        map[string]any
}

func findingRecords(findings []any) []findingRecord {
	var out []findingRecord
	for _, raw := range findings {
		f, _ := raw.(map[string]any)
		if f == nil {
			continue
		}
		r := findingRecord{}
		r.Code, _ = f["code"].(string)
		r.Path, _ = f["path"].(string)
		if ev, ok := f["evidence"].(map[string]any); ok && r.Path == "" {
			r.Path, _ = ev["path"].(string)
		}
		if n, ok := f["slide_number"].(float64); ok {
			r.SlideNumber = int(n)
		}
		if debug, ok := f["debug"].(map[string]any); ok {
			r.Locator, _ = debug["locator"].(string)
		}
		r.NextToolCall, _ = f["next_tool_call"].(map[string]any)
		out = append(out, r)
	}
	return out
}

func validateFindings(t *testing.T, mc *mcpConfig, deck any, fitReport bool) []findingRecord {
	t.Helper()
	res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": deck, "fit_report": fitReport}))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(textContent(res)), &doc); err != nil {
		t.Fatalf("answer is not JSON: %v", err)
	}
	findings := envelopeFindings(t, doc)
	assertFindingPointers(t, "validate_input", deck, findings)
	return findingRecords(findings)
}

// TestExpandedDeckFindingsAreAuthored pins, for each way the engine's deck
// differs from the authored one, where a finding is reported: the authored
// pointer, the rendered slide and the engine's locator under debug.
func TestExpandedDeckFindingsAreAuthored(t *testing.T) {
	mc := testMCPConfig(t)
	for _, tc := range []struct {
		name string
		deck string
		// want is "CODE @ path #slide_number <- debug.locator", each reported
		// exactly once.
		want []string
		// never lists path prefixes no finding may have.
		never []string
	}{
		{name: "split_slide", deck: expandedSplitDeck, want: []string{
			"INPUT.unknown_key @ /slides/1/base/zz #2 <- ",
			"INPUT.INVALID_PARAMETER @ /slides/1/base/content/1/table_value/rows/4/1/conditional/rule #3 <- /slides/2/content/1/table_value/rows/1/1/conditional/rule",
			"INPUT.INVALID_PARAMETER @ /slides/1/base/content/1/table_value/rows #4 <- /slides/3/content/1/table_value/rows",
			// A slide after the envelope: authored /slides/3, rendered sixth.
			"INPUT.INVALID_PARAMETER @ /slides/3/content/1/table_value/rows #6 <- /slides/5/content/1/table_value/rows",
		}, never: []string{"/slides/4", "/slides/5", "/slides/1/content", "/slides/1/base/source"}},
		{name: "structure", deck: expandedStructureDeck, want: []string{
			"INPUT.INVALID_PARAMETER @ /structure/sections/0/slides/0/content/1/table_value/rows #4 <- /slides/3/content/1/table_value/rows",
			"INPUT.DATA_WITHOUT_SOURCE @ /structure/sections/0/slides/0/source #4 <- /slides/3/source",
			"INPUT.DATA_WITHOUT_SOURCE @ /structure/sections/1/slides/0/source #6 <- /slides/5/source",
			// Unknown keys are read from the JSON as written
			// (go-slide-creator-3znh9).
			"INPUT.unknown_key @ /structure/qq #0 <- ",
			"INPUT.unknown_key @ /structure/sections/0/yy #0 <- ",
			"INPUT.unknown_key @ /structure/sections/0/slides/0/zz #4 <- ",
		}, never: []string{"/slides"}},
		{name: "compose and pattern grids", deck: expandedGridsDeck, want: []string{
			// Row 3 of the merged grid is segment 2 (the banner is row 0).
			// Only findings that measure no text are pinned: a font decides
			// the others.
			"INPUT.MISSING_ALT_TEXT @ /slides/1/compose/segments/2/diagram #2 <- /slides/1/shape_grid/rows/3/cells/0/grid/rows/0/cells/0/diagram",
			"INPUT.MISSING_ALT_TEXT @ /slides/2/compose/segments/1/compose/segments/1/diagram #3 <- /slides/2/shape_grid/rows/0/cells/1/grid/rows/1/cells/0/grid/rows/0/cells/0/diagram",
		}, never: []string{"/slides/1/shape_grid", "/slides/2/shape_grid", "/slides/1/rendered_shapes"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := map[string]int{}
			var keys []string
			for _, f := range validateFindings(t, mc, verdictJSON(tc.deck), true) {
				key := fmt.Sprintf("%s @ %s #%d <- %s", f.Code, f.Path, f.SlideNumber, f.Locator)
				count[key]++
				keys = append(keys, key)
				for _, prefix := range tc.never {
					if f.Path == prefix || strings.HasPrefix(f.Path, prefix+"/") {
						t.Errorf("%s: the deck has nothing there", key)
					}
				}
			}
			for _, want := range tc.want {
				if count[want] != 1 {
					t.Errorf("%s reported %d times, want once", want, count[want])
				}
			}
			if t.Failed() {
				sort.Strings(keys)
				t.Logf("findings:\n  %s", strings.Join(keys, "\n  "))
			}
		})
	}
}

// TestExpandedDeckOneVerdict: the four raw surfaces refuse an expanded deck
// with the same findings at the same authored paths (each surface's paths are
// checked against the deck by verdictSurfaces).
func TestExpandedDeckOneVerdict(t *testing.T) {
	mc := testMCPConfig(t)
	for _, tc := range []struct {
		name, deck string
		want       []string
	}{
		{"split_slide", expandedSplitDeck, []string{
			"INPUT.INVALID_PARAMETER @ /slides/1/base/content/1/table_value/rows",
			"INPUT.INVALID_PARAMETER @ /slides/1/base/content/1/table_value/rows/4/1/conditional/rule",
			"INPUT.INVALID_PARAMETER @ /slides/3/content/1/table_value/rows",
		}},
		{"structure", expandedStructureDeck, []string{
			"INPUT.INVALID_PARAMETER @ /structure/sections/0/slides/0/content/1/table_value/rows",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "deck.json")
			if err := os.WriteFile(path, []byte(tc.deck), 0o600); err != nil {
				t.Fatal(err)
			}
			verdict := assertOneVerdict(t, verdictSurfaces(t, mc, path, true))
			if verdict.Valid {
				t.Fatalf("the deck is refused by no surface")
			}
			if got := strings.Join(verdict.Errors, "\n"); got != strings.Join(tc.want, "\n") {
				t.Errorf("errors:\n%s\nwant:\n%s", got, strings.Join(tc.want, "\n"))
			}
		})
	}
}

// TestExpandedDeckRenderedAndPlannedFindings: the surfaces that answer for a
// deck they accept — generate and generate_presentation with their fit
// findings, the preview plan, preflight and apply_deck_patch — address an
// expanded deck's findings the same way. The decks are the three above with
// their refusals removed.
func TestExpandedDeckRenderedAndPlannedFindings(t *testing.T) {
	mc := testMCPConfig(t)
	valid := strings.NewReplacer(
		`["G", "7", "extra"]`, `["G", "7"]`,
		`{"content": "5", "conditional": {"rule": "bogus", "fill": "accent2"}}`, `"5"`,
		`[["1", "2", "3"]]`, `[["1", "2"]]`,
	)
	for _, tc := range []struct{ name, deck string }{
		{"split_slide", valid.Replace(expandedSplitDeck)},
		{"structure", valid.Replace(expandedStructureDeck)},
		{"compose", expandedGridsDeck},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deck := verdictJSON(tc.deck)
			decode := func(surface, text string) map[string]any {
				var doc map[string]any
				if err := json.Unmarshal([]byte(text), &doc); err != nil {
					t.Fatalf("%s: answer is not JSON: %v\n%s", surface, err, text)
				}
				return doc
			}
			checked := 0
			check := func(surface string, findings []any) {
				checked += len(findings)
				assertFindingPointers(t, surface, deck, findings)
			}

			res, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": deck, "fit_report": true}))
			if err != nil {
				t.Fatal(err)
			}
			check("generate_presentation", answerFindings(t, decode("generate_presentation", textContent(res))))

			res, err = mc.handlePreviewPlan(context.Background(), makeRequest(map[string]any{"presentation": deck}))
			if err != nil {
				t.Fatal(err)
			}
			check("preview_presentation_plan", answerFindings(t, decode("preview_presentation_plan", textContent(res))))

			env := runPreflightCore([]byte(tc.deck), preflightOptions{templatesDir: mc.templatesDir})
			raw, _ := json.Marshal(env)
			check("preflight", envelopeFindings(t, map[string]any{"findings": decode("preflight", string(raw))}))

			res, err = mc.handleApplyDeckPatch(context.Background(), makeRequest(map[string]any{
				"presentation": deck, "ops": []any{map[string]any{"op": "replace_field", "path": "/template", "value": "midnight-blue"}},
			}))
			if err != nil {
				t.Fatal(err)
			}
			check("apply_deck_patch", envelopeFindings(t, decode("apply_deck_patch", textContent(res))))

			dir := t.TempDir()
			input, report := filepath.Join(dir, "deck.json"), filepath.Join(dir, "report.json")
			if err := os.WriteFile(input, []byte(tc.deck), 0o600); err != nil {
				t.Fatal(err)
			}
			_ = runJSONMode(input, report, mc.templatesDir, dir, "", false, false, "", "warn", false, "strict", "", false)
			data, err := os.ReadFile(report)
			if err != nil {
				t.Fatal(err)
			}
			check("generate", answerFindings(t, decode("generate", string(data))))

			if checked == 0 {
				t.Error("no surface reported a finding: the check ran on nothing")
			}
		})
	}
}

// TestAuthoredPathsAddress pins the translation itself, including the forms
// that fall back to an enclosing object and are therefore not handed to a
// tool as its argument.
func TestAuthoredPathsAddress(t *testing.T) {
	decode := func(deck string) *PresentationInput {
		var input PresentationInput
		if err := json.Unmarshal([]byte(deck), &input); err != nil {
			t.Fatal(err)
		}
		if ds := applyStructureExpansion(&input); len(ds) > 0 {
			t.Fatal(ds)
		}
		return &input
	}
	patternDeck := `{"template": "midnight-blue", "slides": [` + verdictSlides["pattern"] + `, ` + verdictSlides["grid"] + `]}`
	for _, tc := range []struct {
		name, deck, path, source string
		want                     string
		slide                    int
		exact                    bool
	}{
		{"deck field", patternDeck, "/template", "", "/template", 0, true},
		{"authored grid cell", patternDeck, "/slides/1/shape_grid/rows/0/cells/1/shape/text", "", "/slides/1/shape_grid/rows/0/cells/1/shape/text", 2, true},
		{"pattern cell text", patternDeck, "/slides/0/pattern/rows/0/cells/1/shape/text/paragraphs/1", "", "/slides/0/pattern/values/1/small", 1, true},
		{"pattern cell", patternDeck, "/slides/0/pattern/rows/0/cells/1/shape/fill", "", "/slides/0/pattern/values/1", 1, false},
		{"pattern grid", patternDeck, "/slides/0/shape_grid", "", "/slides/0/pattern", 1, true},
		{"pattern row", patternDeck, "/slides/0/pattern/rows/0", "", "/slides/0/pattern", 1, false},
		{"pattern value", patternDeck, "/slides/0/pattern/values/1/small", "", "/slides/0/pattern/values/1/small", 1, true},
		{"chrome", patternDeck, "/slides/1/chrome", "", "/slides/1", 2, false},
		{"written shape, source known", patternDeck, "/slides/1/rendered_shapes/200/paragraphs/0", "/slides/1/shape_grid/rows/0/cells/0/shape/text", "/slides/1/shape_grid/rows/0/cells/0/shape/text", 2, false},
		{"written shape, source unknown", patternDeck, "/slides/1/rendered_shapes/200", "", "/slides/1", 2, false},
		{"split page title", expandedSplitDeck, "/slides/3/content/0", "", "/slides/1/base/content/0", 4, true},
		{"split page row", expandedSplitDeck, "/slides/3/content/1/table_value/rows/0/2", "", "/slides/1/base/content/1/table_value/rows/6/2", 4, true},
		{"split page headers", expandedSplitDeck, "/slides/2/content/1/table_value/headers/1", "", "/slides/1/base/content/1/table_value/headers/1", 3, true},
		{"after the split", expandedSplitDeck, "/slides/4", "", "/slides/2", 5, true},
		// A base that writes its table under the legacy "value" key.
		{"split page row, legacy value", legacySplitDeck, "/slides/1/content/1/table_value/rows/1/0", "", "/slides/0/base/content/1/value/rows/3/0", 2, true},
		{"structure cover", expandedStructureDeck, "/slides/0/content/0", "", "/structure/cover/content/0", 1, true},
		{"structure agenda", expandedStructureDeck, "/slides/1/pattern", "", "/structure/auto_agenda", 2, false},
		{"structure agenda item", expandedStructureDeck, "/slides/1/pattern/values/items/1", "", "/structure/sections/1/title", 2, true},
		{"structure divider title", expandedStructureDeck, "/slides/2/content/0", "", "/structure/sections/0/title", 3, true},
		{"structure divider", expandedStructureDeck, "/slides/4/layout_id", "", "/structure/sections/1", 5, false},
		{"structure slide", expandedStructureDeck, "/slides/5/pattern/values/2/big", "", "/structure/sections/1/slides/0/pattern/values/2/big", 6, true},
		{"compose pattern cell", expandedGridsDeck, "/slides/1/shape_grid/rows/1/cells/0/grid/rows/0/cells/1/shape/text", "", "/slides/1/compose/segments/0/pattern/values/1", 2, false},
		{"compose pattern cell text", expandedGridsDeck, "/slides/1/shape_grid/rows/1/cells/0/grid/rows/0/cells/1/shape/text/paragraphs/1", "", "/slides/1/compose/segments/0/pattern/values/1/small", 2, true},
		{"compose nested diagram", expandedGridsDeck, "/slides/2/shape_grid/rows/0/cells/1/grid/rows/1/cells/0/grid/rows/0/cells/0/diagram/data", "", "/slides/2/compose/segments/1/compose/segments/1/diagram/data", 3, true},
		{"compose banner", expandedGridsDeck, "/slides/1/shape_grid/rows/0/cells/0/shape/text", "", "/slides/1/compose/banner", 2, false},
		{"compose segment", expandedGridsDeck, "/slides/1/shape_grid/rows/2", "", "/slides/1/compose/segments/1", 2, true},
		{"compose envelope", expandedGridsDeck, "/slides/1/shape_grid", "", "/slides/1/compose", 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := decode(tc.deck)
			got := newAuthoredPaths(input).address(tc.path, tc.source)
			if got.Path != tc.want || got.SlideNumber != tc.slide || got.Exact != tc.exact {
				t.Errorf("address(%q) = %+v, want %q on slide %d (exact %t)", tc.path, got, tc.want, tc.slide, tc.exact)
			}
			var doc any
			if err := json.Unmarshal([]byte(tc.deck), &doc); err != nil {
				t.Fatal(err)
			}
			if !pointerResolves(doc, got.Path) {
				t.Errorf("%q does not resolve in the authored deck", got.Path)
			}
			// What a caller hands back names the same slide again.
			if back := newAuthoredPaths(input).engine(got.Path, tc.slide-1); tc.exact && tc.slide > 0 && back != tc.path &&
				!strings.Contains(tc.path, "/pattern/rows/") && !strings.Contains(tc.path, "/shape_grid") {
				t.Errorf("engine(%q) = %q, want %q", got.Path, back, tc.path)
			}
		})
	}
}

// TestRepairToolsAcceptAuthoredPointers: repair_slide takes the path a finding
// reports and the engine's locator alike, slide_index counts rendered slides,
// and a finding's next_tool_call replays as written.
func TestRepairToolsAcceptAuthoredPointers(t *testing.T) {
	mc := repairMC(t)
	repair := func(t *testing.T, deck any, slideIdx int, fix map[string]any) repairSlideOutput {
		t.Helper()
		res, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
			"presentation": deck, "slide_index": float64(slideIdx), "fixes": []any{fix},
		}))
		if err != nil {
			t.Fatal(err)
		}
		var out repairSlideOutput
		if err := json.Unmarshal([]byte(textContent(res)), &out); err != nil {
			t.Fatalf("unmarshal: %v\n%s", err, textContent(res))
		}
		// The patched deck is what the findings address.
		var patched any
		if err := json.Unmarshal(out.PatchedDeck, &patched); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(out.Findings.Findings)
		var findings []any
		_ = json.Unmarshal(raw, &findings)
		assertFindingPointers(t, "repair_slide", patched, findings)
		return out
	}
	bullets := func(t *testing.T, out repairSlideOutput, slideIdx int) int {
		t.Helper()
		var deck PresentationInput
		if err := json.Unmarshal(out.PatchedDeck, &deck); err != nil {
			t.Fatal(err)
		}
		return len(*deck.Slides[slideIdx].Content[1].BulletsValue)
	}

	t.Run("split_slide deck", func(t *testing.T) {
		// The bullets slide is authored /slides/2 and rendered fifth.
		for _, path := range []string{"/slides/2/content/1", "/slides/4/content/1"} {
			out := repair(t, verdictJSON(expandedSplitDeck), 4, map[string]any{
				"kind": "reduce_text", "params": map[string]any{"path": path, "max_items": float64(2), "confirm_semantic_change": true},
			})
			if !out.AppliedFixes[0].Applied || bullets(t, out, 4) != 2 {
				t.Errorf("path %s: %+v", path, out.AppliedFixes[0])
			}
		}
	})

	t.Run("structure deck", func(t *testing.T) {
		// The deck is edited as the slides it renders (go-slide-creator-tfnk7):
		// the KPI slide is /structure/sections/1/slides/0 and renders sixth.
		// The answer is the expanded deck, without the block.
		for _, cellPath := range []string{
			"/structure/sections/1/slides/0/pattern/values/1/small", // what a finding reports
			"/slides/5/pattern/values/1/small",                      // the same value on the rendered slide
			"/slides/5/pattern/rows/0/cells/1",                      // the engine's locator
		} {
			out := repair(t, verdictJSON(validStructureDeck), 5, map[string]any{
				"kind": "reduce_cell_text", "params": map[string]any{"cell_path": cellPath, "max_chars": float64(2)},
			})
			var deck PresentationInput
			if err := json.Unmarshal(out.PatchedDeck, &deck); err != nil {
				t.Fatal(err)
			}
			if !out.AppliedFixes[0].Applied || deck.Structure != nil || len(deck.Slides) != 6 ||
				!strings.Contains(string(deck.Slides[5].Pattern.Values), `"small":"N…"`) {
				t.Errorf("cell_path %s: %+v\n%s", cellPath, out.AppliedFixes[0], out.PatchedDeck)
			}
		}
	})

	t.Run("pattern value", func(t *testing.T) {
		deck := `{"template": "midnight-blue", "slides": [{"layout_id": "blank-title", "content": [
		  {"placeholder_id": "title", "type": "text", "text_value": "Three numbers tell the story"}],
		  "pattern": {"name": "kpi-3up", "values": [
		    {"big": "12%", "small": "Growth in recurring revenue"}, {"big": "94", "small": "NPS"}, {"big": "3", "small": "Launches"}]}}]}`
		for _, cellPath := range []string{
			"/slides/0/pattern/values/0/small",    // what a finding reports
			"/slides/0/pattern/rows/0/cells/0",    // the engine's locator
			"/slides/0/shape_grid/rows/0/cells/0", // its older spelling
		} {
			out := repair(t, verdictJSON(deck), 0, map[string]any{
				"kind": "reduce_cell_text", "params": map[string]any{"cell_path": cellPath, "max_chars": float64(12)},
			})
			if !out.AppliedFixes[0].Applied || !strings.Contains(string(out.PatchedDeck), `"small":"Growth in r…"`) {
				t.Errorf("cell_path %s: %+v\n%s", cellPath, out.AppliedFixes[0], out.PatchedDeck)
			}
		}
	})

	t.Run("compose segment value", func(t *testing.T) {
		out := repair(t, verdictJSON(expandedGridsDeck), 1, map[string]any{
			"kind": "reduce_cell_text", "params": map[string]any{
				"cell_path": "/slides/1/compose/segments/1/pattern/values/steps/0/label", "max_chars": float64(10)},
		})
		if !out.AppliedFixes[0].Applied || !strings.Contains(string(out.PatchedDeck), `"label":"Plan the …"`) {
			t.Errorf("%+v\n%s", out.AppliedFixes[0], out.PatchedDeck)
		}
	})

	t.Run("next_tool_call replays", func(t *testing.T) {
		val := testMCPConfig(t)
		replayed := 0
		for _, deck := range []string{expandedSplitDeck, expandedGridsDeck, validStructureDeck} {
			for _, f := range validateFindings(t, val, verdictJSON(deck), true) {
				if f.NextToolCall["tool"] != "repair_slide" {
					continue
				}
				args, _ := f.NextToolCall["args_template"].(map[string]any)
				// slide_index is the rendered slide the finding names.
				if idx, _ := args["slide_index"].(float64); int(idx) != f.SlideNumber-1 {
					t.Errorf("%s at %s: next_tool_call slide_index %v, slide_number %d", f.Code, f.Path, args["slide_index"], f.SlideNumber)
				}
				call := map[string]any{"presentation": verdictJSON(deck)}
				for k, v := range args {
					call[k] = v
				}
				res, err := mc.handleRepairSlide(context.Background(), makeRequest(call))
				if err != nil {
					t.Fatal(err)
				}
				if res.IsError {
					t.Errorf("%s at %s: repair_slide refuses its own next_tool_call: %s", f.Code, f.Path, textContent(res))
				}
				replayed++
			}
		}
		if replayed == 0 {
			t.Error("no finding carried a repair_slide next_tool_call: the check ran on nothing")
		}
	})

	t.Run("propose_repairs on a structure deck", func(t *testing.T) {
		res, err := mc.handleProposeRepairs(context.Background(), makeRequest(map[string]any{
			"presentation": verdictJSON(validStructureDeck),
			"findings": []any{map[string]any{
				"code": "TITLE_TOO_LONG", "path": "/structure/sections/0/slides/0/content/0", "slide_number": float64(4), "action": "review",
				"fix": map[string]any{"kind": "shorten_title", "params": map[string]any{"max_length": float64(12)}},
			}},
		}))
		if err != nil {
			t.Fatal(err)
		}
		var out proposeRepairsOutput
		if err := json.Unmarshal([]byte(textContent(res)), &out); err != nil || res.IsError {
			t.Fatalf("propose_repairs: %v\n%s", err, textContent(res))
		}
		if len(out.Slides) != 1 || out.Slides[0].SlideIndex != 3 {
			t.Fatalf("planned on %+v, want rendered slide index 3", out.Slides)
		}
	})

	t.Run("propose_repairs", func(t *testing.T) {
		// A finding handed back with its authored path lands on the rendered
		// slide it names.
		var input PresentationInput
		if err := json.Unmarshal([]byte(expandedSplitDeck), &input); err != nil {
			t.Fatal(err)
		}
		five := 5
		for _, f := range []proposeRepairsFinding{
			{Code: "BODY_TOO_LONG", Path: "/slides/2/content/1", SlideNumber: &five, Action: "review"},
			{Code: "BODY_TOO_LONG", Path: "/slides/2/content/1", Action: "review"},
			{Code: "BODY_TOO_LONG", Path: "/slides/2/content/1", Action: "review", Debug: map[string]any{"locator": "/slides/4/content/1"}},
		} {
			f.Fix = &patterns.FixSuggestion{Kind: "reduce_text", Params: map[string]any{"max_items": float64(2), "path": "/slides/2/content/1"}}
			out := proposeRepairs(&input, []proposeRepairsFinding{f})
			if len(out.Slides) != 1 || out.Slides[0].SlideIndex != 4 {
				t.Fatalf("finding %+v planned on %+v, want rendered slide index 4", f, out.Slides)
			}
			if p, _ := out.Slides[0].Directives[0].Params["path"].(string); p != "/slides/4/content/1" {
				t.Errorf("directive path %q, want the engine's locator", p)
			}
		}
	})
}

// TestAuthoredPathsInProse: a message, an error line and a blocking reason
// quote the place their finding names, and a locator is replaced whole.
func TestAuthoredPathsInProse(t *testing.T) {
	var input PresentationInput
	if err := json.Unmarshal([]byte(expandedSplitDeck), &input); err != nil {
		t.Fatal(err)
	}
	authored := newAuthoredPaths(&input)
	ds := authored.diagnostics([]diagnostics.Diagnostic{{
		Code: "X", Path: "/slides/4/content/1",
		Message: "slide 5 at /slides/4/content/1: too long (see /slides/4/content/10 and /slides/4/content/1/text)",
		Fix:     &diagnostics.Fix{Kind: "reduce_text", Params: map[string]any{"path": "/slides/4/content/1", "refused_path": "/slides/4/chrome", "cell_path": "/slides/4/chrome"}},
	}})
	d := ds[0]
	if want := "slide 5 at /slides/2/content/1: too long (see /slides/4/content/10 and /slides/4/content/1/text)"; d.Message != want {
		t.Errorf("message %q, want %q", d.Message, want)
	}
	// A tool argument is the authored pointer when that names the same
	// element, and stays the engine's locator when the authored address is
	// only the enclosing slide; a fact that is not an argument is authored.
	if d.Fix.Params["path"] != "/slides/2/content/1" || d.Fix.Params["cell_path"] != "/slides/4/chrome" || d.Fix.Params["refused_path"] != "/slides/2" {
		t.Errorf("fix params %v", d.Fix.Params)
	}
	fit := authored.fitFindings([]patterns.FitFinding{
		{ValidationError: patterns.ValidationError{Code: "A", Path: "/slides/4/content/1"}},
		{ValidationError: patterns.ValidationError{Code: "B", Path: "/slides/2/content/1/table_value/rows/1/0"}},
	})
	got := authored.sentences([]string{"A at /slides/4/content/1", "B at /slides/2/content/1/table_value/rows/1/0", "C at /slides/4/content/12"}, fit)
	want := []string{"A at /slides/2/content/1", "B at /slides/1/base/content/1/table_value/rows/4/0", "C at /slides/4/content/12"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("sentences %q, want %q", got, want)
	}
}

// TestFindingEnvelopeCarriesSlideNumber: the envelope's where.slide is the
// rendered slide's 0-based index whatever the authored path says.
func TestFindingEnvelopeCarriesSlideNumber(t *testing.T) {
	f := diagnostics.FindingFromDiagnostic(diagnostics.Diagnostic{
		Code: "X", Path: "/structure/sections/0/slides/0/source", SlideNumber: 4,
		Debug: map[string]any{"locator": "/slides/3/source"},
	})
	if f.SlideNumber == nil || *f.SlideNumber != 4 || f.Where == nil || f.Where.Slide == nil || *f.Where.Slide != 3 {
		t.Errorf("slide_number / where.slide = %v / %+v, want 4 / 3", f.SlideNumber, f.Where)
	}
	if f.Debug["locator"] != "/slides/3/source" {
		t.Errorf("debug = %v", f.Debug)
	}
}

// TestPartialModeFindingsKeepAuthoredSlides: a slide --partial leaves out
// does not move the findings of the slides after it (go-slide-creator-i8nwl).
// Their paths name the slide the author wrote and slide_number the slide it
// renders as.
func TestPartialModeFindingsKeepAuthoredSlides(t *testing.T) {
	table := `{"layout_id": "content", "content": [
	  {"placeholder_id": "title", "type": "text", "text_value": "Two options were compared"},
	  {"placeholder_id": "body", "type": "table", "table_value": {"headers": ["Option", "Cost"], "rows": [["Build", "1"], ["Buy", "2"]]}}]}`
	for _, tc := range []struct {
		name, dropped string
	}{
		// Dropped before conversion: a design-mode violation.
		{"design mode", `{"layout_id": "blank-title", "content": [
		  {"placeholder_id": "title", "type": "text", "text_value": "Two steps lead to launch"}],
		  "shape_grid": {"rows": [{"cells": [{"shape": {"geometry": "rect", "fill": "#123456", "text": "Plan"}}]}]}}`},
		// Dropped by the conversion: a pattern with too few values.
		{"conversion", `{"layout_id": "blank-title", "content": [
		  {"placeholder_id": "title", "type": "text", "text_value": "Three numbers tell the story"}],
		  "pattern": {"name": "kpi-3up", "values": [{"big": "12%", "small": "Growth"}]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			input, report := filepath.Join(dir, "deck.json"), filepath.Join(dir, "report.json")
			deck := `{"template": "midnight-blue", "slides": [` + verdictCover + `, ` + tc.dropped + `, ` + table + `]}`
			if err := os.WriteFile(input, []byte(deck), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := runJSONMode(input, report, "../../templates", dir, "", false, false, "", "warn", true, "strict", "", false); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(report)
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			if n, _ := doc["slide_count"].(float64); n != 2 {
				t.Fatalf("rendered %v slides, want 2 (one dropped)", doc["slide_count"])
			}
			onTable := 0
			for _, f := range findingRecords(answerFindings(t, doc)) {
				switch {
				case strings.HasPrefix(f.Path, "/slides/2"):
					onTable++
					if f.SlideNumber != 2 {
						t.Errorf("%s at %s: slide_number %d, want 2 (the table renders second)", f.Code, f.Path, f.SlideNumber)
					}
				case strings.HasPrefix(f.Path, "/slides/1"):
					if strings.Contains(f.Path, "table_value") {
						t.Errorf("%s at %s: the table is authored slide 2", f.Code, f.Path)
					}
				}
			}
			if onTable == 0 {
				t.Error("no finding on the table slide: the check ran on nothing")
			}
		})
	}
}

// TestHumanFitReportAddressesAuthoredDeck: `validate --fit-report` in its
// human format measures a structure deck as the slides it builds
// (go-slide-creator-8nah2) and names each finding where the author wrote it,
// under the number of the slide it renders as.
func TestHumanFitReportAddressesAuthoredDeck(t *testing.T) {
	deck := strings.NewReplacer(`[["1", "2", "3"]]`, `[["1", "2"]]`).Replace(expandedStructureDeck)
	path := filepath.Join(t.TempDir(), "deck.json")
	if err := os.WriteFile(path, []byte(deck), 0o600); err != nil {
		t.Fatal(err)
	}
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	os.Args = []string{"validate", "--fit-report", "--templates-dir", "../../templates", path}
	var runErr error
	report := captureStderr(t, func() {
		_ = captureStdout(t, func() { runErr = runValidate() })
	})
	if runErr != nil {
		t.Fatalf("validate: %v\n%s", runErr, report)
	}
	for _, want := range []string{
		"  Slide 4:\n    [review] /structure/sections/0/slides/0/source",
		"  Slide 6:\n    [review] /structure/sections/1/slides/0/source",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("fit report lacks %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "] /slides/") {
		t.Errorf("fit report names /slides/N in a deck that has no slides array:\n%s", report)
	}
}
