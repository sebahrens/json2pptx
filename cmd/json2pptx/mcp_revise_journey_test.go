package main

import (
	"bufio"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/internal/render"
)

// The e-revise agent journey (tests/quality/results/agent-journey-20261003/
// e-revise) took one eight-slide investor deck through eight revision turns on
// a single deck_id. This test replays those turns through the real MCP server,
// with the patches the journey recorded in log-calls.jsonl, and pins what each
// turn's response now tells the agent (go-slide-creator-1w3uo, 83kru, v5e9h,
// yxf1k, rq1z9, j77xe, veqn2).

const reviseJourneyLog = "../../tests/quality/results/agent-journey-20261003/e-revise/log-calls.jsonl"

// journeyCalls loads the recorded tool calls, keyed by their call number n.
func journeyCalls(t *testing.T) map[int]map[string]any {
	t.Helper()
	f, err := os.Open(reviseJourneyLog)
	if err != nil {
		t.Fatalf("open journey log: %v", err)
	}
	defer f.Close()
	out := map[int]map[string]any{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	for sc.Scan() {
		var row struct {
			N      int    `json:"n"`
			Method string `json:"method"`
			Params struct {
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			t.Fatalf("decode journey log row: %v", err)
		}
		if row.Method == "tools/call" {
			out[row.N] = row.Params.Arguments
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read journey log: %v", err)
	}
	return out
}

// journeyServer is one MCP server in the default tool profile, as the journey
// agent saw it.
type journeyServer struct {
	t *testing.T
	s *server.MCPServer
}

func newJourneyServer(t *testing.T) journeyServer {
	t.Helper()
	withToolProfile(t, activeToolProfile())
	return journeyServer{t: t, s: newJSON2PPTXMCPServer(handleTestConfig(t), toolProfileDeckSpec)}
}

// render calls render_deck_spec and returns the decoded response, the raw
// result and the size of the text the agent reads.
func (j journeyServer) render(args map[string]any) (renderDeckSpecResponse, *mcp.CallToolResult, int) {
	j.t.Helper()
	res := callToolViaServer(j.t, j.s, "render_deck_spec", args)
	var out renderDeckSpecResponse
	structuredInto(j.t, res.StructuredContent, &out)
	return out, res, len(textContent(res))
}

// renderOK is render for a turn the journey rendered successfully. Fit
// refusals depend on font metrics, which differ off macOS; the journey ran on
// macOS, so elsewhere a refusal skips rather than fails.
func (j journeyServer) renderOK(args map[string]any) (renderDeckSpecResponse, int) {
	j.t.Helper()
	out, res, size := j.render(args)
	if !out.Success {
		if runtime.GOOS != "darwin" && len(out.Diagnostics) > 0 {
			j.t.Skipf("font-dependent fit refusal off macOS: %s %+v", out.Error, out.Diagnostics)
		}
		j.t.Fatalf("render failed: %s", resultText(res))
	}
	return out, size
}

func (j journeyServer) validate(args map[string]any) deckSpecEnvelopeResponse {
	j.t.Helper()
	res := callToolViaServer(j.t, j.s, "validate_deck_spec", args)
	if res.IsError {
		j.t.Fatalf("validate_deck_spec failed: %s", resultText(res))
	}
	var out deckSpecEnvelopeResponse
	structuredInto(j.t, res.StructuredContent, &out)
	return out
}

// changeOf returns the change class reported for a slide id ("" when the
// slide is not listed).
func changeOf(changes []slideChange, id string) string {
	for _, c := range changes {
		if c.ID == id {
			return c.Change
		}
	}
	return ""
}

func countChange(changes []slideChange, class string) int {
	n := 0
	for _, c := range changes {
		if c.Change == class {
			n++
		}
	}
	return n
}

// thumbnailIndices returns the slide_indices a next_tool_call asks for, and
// whether the call is a thumbnails call at all.
func thumbnailIndices(t *testing.T, res renderDeckSpecResponse) ([]int, bool) {
	t.Helper()
	if res.NextToolCall == nil || res.NextToolCall.Tool != "render_deck_thumbnails" {
		return nil, false
	}
	raw, ok := res.NextToolCall.ArgsTemplate["slide_indices"]
	if !ok {
		return nil, true
	}
	var out []int
	structuredInto(t, raw, &out)
	return out, true
}

func TestReviseJourneyReplay(t *testing.T) {
	calls := journeyCalls(t)
	j := newJourneyServer(t)
	const maxPatchResponse = 2048

	// --- baseline: validate the authored spec, then the first render ---
	validated := j.validate(map[string]any{"spec": calls[9]["spec"]})
	if validated.DeckID == "" || !validated.Stored || validated.Revision != 1 {
		t.Fatalf("baseline validate: deck_id=%q stored=%v revision=%d", validated.DeckID, validated.Stored, validated.Revision)
	}
	deck := validated.DeckID
	patched := func(n int, extra ...string) map[string]any {
		args := map[string]any{"deck_id": deck, "patch": calls[n]["patch"]}
		for i := 0; i+1 < len(extra); i += 2 {
			args[extra[i]] = extra[i+1]
		}
		return args
	}

	first, _ := j.renderOK(patched(10))
	if first.SlideCount != 8 || len(first.Slides) != 8 {
		t.Fatalf("first render: slide_count=%d slides=%d, want 8", first.SlideCount, len(first.Slides))
	}
	// E13: nothing had been rendered, so every slide is new to the eye — not
	// just the two the patch touched.
	if len(first.ChangedSlides) != 8 {
		t.Errorf("first render changed_slides = %v, want all 8", first.ChangedSlides)
	}
	if idx, ok := thumbnailIndices(t, first); !ok || idx != nil {
		t.Errorf("first render should ask for thumbnails of the whole deck, got %+v", first.NextToolCall)
	}
	// E4: every slide has an id, an index and a slide number.
	ids := map[int]string{}
	for i, s := range first.Slides {
		if s.ID == "" || s.Index != i || s.SlideNumber != i+1 {
			t.Fatalf("slides[%d] = %+v, want an id with index %d and slide_number %d", i, s, i, i+1)
		}
		ids[i] = s.ID
	}
	riskID, askID := ids[5], ids[7]

	// --- turn 1: slide 3 becomes a chart (one-slide edit) ---
	turn1, size := j.renderOK(patched(12))
	turn1Size := size
	if !equalInts(turn1.ChangedSlides, []int{2}) || changeOf(turn1.SlideChanges, ids[2]) != slideChangeEdited {
		t.Errorf("turn 1: changed_slides=%v slide_changes=%+v, want slide 2 edited", turn1.ChangedSlides, turn1.SlideChanges)
	}
	// E10: a one-slide edit used to return 7 KB, 88% of it unchanged.
	if size >= maxPatchResponse {
		t.Errorf("turn 1: a one-slide patch render returned %d bytes, want < %d", size, maxPatchResponse)
	}
	if !turn1.Stored || turn1.Revision <= first.Revision {
		t.Errorf("turn 1: stored=%v revision=%d (was %d)", turn1.Stored, turn1.Revision, first.Revision)
	}
	verbose, _ := j.renderOK(map[string]any{"deck_id": deck, "verbose": true,
		"patch": []any{map[string]any{"op": "replace", "path": "/slides/2/takeaway", "value": "ARR of $9.4m is the base for the 2027 plan."}}})
	if verbose.Explanation == nil || len(verbose.Explanation.Slides) != 8 || len(verbose.Slides) != 8 {
		t.Errorf("verbose patch render should carry the full summaries, got explanation=%v slides=%d", verbose.Explanation != nil, len(verbose.Slides))
	}

	// --- turn 2: insert a competitor matrix after the roadmap ---
	// The journey's first attempt (n=15) was refused by the readability floor
	// and still inserted the slide (E9). A refused patched render now stores
	// nothing, and says so.
	attempt, res, refusedSize := j.render(patched(15))
	t.Logf("turn 2: the journey's first insert renders: %v (%d bytes)", attempt.Success, refusedSize)
	if !attempt.Success {
		if attempt.Stored || attempt.Revision != turn1.Revision {
			t.Errorf("turn 2: refused render reports stored=%v revision=%d; want stored=false at the unchanged revision %d: %s",
				attempt.Stored, attempt.Revision, turn1.Revision, resultText(res))
		}
		if got := j.validate(map[string]any{"deck_id": deck, "read": "history"}); len(got.Slides) != 8 {
			t.Fatalf("turn 2: a refused render left %d slides in the stored deck, want 8", len(got.Slides))
		}
	}
	matrix := deepCopyJSON(calls[15]["patch"]).([]any)
	if !attempt.Success {
		// The journey's eventual fix (n=20): drop the detail line of the row
		// that also carries the highlight badge. Nothing was stored, so the
		// corrected slide is sent whole.
		slide := matrix[0].(map[string]any)["value"].(map[string]any)
		delete(slide["options"].([]any)[0].(map[string]any), "detail")
		inserted, _ := j.renderOK(map[string]any{"deck_id": deck, "patch": matrix})
		attempt = inserted
	}
	// E6: an insert used to flag the shifted slide too.
	if !equalInts(attempt.ChangedSlides, []int{7}) {
		t.Errorf("turn 2: changed_slides = %v, want only the inserted slide [7]", attempt.ChangedSlides)
	}
	if got := changeOf(attempt.SlideChanges, askID); got != slideChangeRenumbered {
		t.Errorf("turn 2: the ask slide shifted from 7 to 8 and should be %q, got %q in %+v", slideChangeRenumbered, got, attempt.SlideChanges)
	}
	matrixID := ""
	for _, c := range attempt.SlideChanges {
		if c.Change == slideChangeInserted {
			matrixID = c.ID
		}
	}
	if matrixID == "" {
		t.Fatalf("turn 2: no inserted slide in %+v", attempt.SlideChanges)
	}

	// --- turn 3: move the risk slide before the ask, and shorten its title ---
	// n=26 as the journey sent it: one move op, rejected then, 49 bytes now.
	moveOp, err := json.Marshal(calls[26]["patch"].([]any)[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(moveOp) >= 100 {
		t.Errorf("turn 3: the move op is %d bytes, want < 100", len(moveOp))
	}
	turn3, size := j.renderOK(map[string]any{"deck_id": deck, "patch": []any{
		calls[26]["patch"].([]any)[0],
		// The risk slide is addressed by id: its index just changed.
		map[string]any{"op": "replace", "path": "/slides/" + riskID + "/title", "value": "Permitting delay is the main risk"},
	}})
	if !equalInts(turn3.ChangedSlides, []int{7}) || changeOf(turn3.SlideChanges, riskID) != slideChangeEdited {
		t.Errorf("turn 3: changed_slides=%v slide_changes=%+v, want only the risk slide (now 7) edited", turn3.ChangedSlides, turn3.SlideChanges)
	}
	if countChange(turn3.SlideChanges, slideChangeRenumbered) != 2 {
		t.Errorf("turn 3: the two slides the move shifted should be renumbered, got %+v", turn3.SlideChanges)
	}
	if size >= maxPatchResponse {
		t.Errorf("turn 3: response is %d bytes, want < %d", size, maxPatchResponse)
	}
	t.Logf("patch render responses: turn 1 %d bytes, turn 3 %d bytes (the journey measured 7.0-7.6 KB)", turn1Size, size)
	// A pure move changes nothing an agent needs to look at again.
	moved, _ := j.renderOK(map[string]any{"deck_id": deck, "dry_run": true,
		"patch": []any{map[string]any{"op": "move", "from": "/slides/" + riskID, "path": "/slides/1"}}})
	if len(moved.ChangedSlides) != 0 || changeOf(moved.SlideChanges, riskID) != slideChangeMoved || moved.NextToolCall != nil || moved.Stored {
		t.Errorf("a pure move: changed_slides=%v change=%q next=%+v stored=%v; want [] / moved / no thumbnails / not stored",
			moved.ChangedSlides, changeOf(moved.SlideChanges, riskID), moved.NextToolCall, moved.Stored)
	}

	// --- turn 4: the ARR figure is wrong, fix it everywhere ---
	found := j.validate(map[string]any{"deck_id": deck, "find": "9.4"})
	if found.Hits == nil || len(*found.Hits) < 5 {
		t.Fatalf("turn 4: find 9.4 returned %+v, want the titles, body lines and the chart value", found.Hits)
	}
	numeric := false
	for _, h := range *found.Hits {
		if h.SlideID == "" || h.Index == nil || h.Excerpt == "" {
			t.Errorf("turn 4: hit lacks its slide or excerpt: %+v", h)
		}
		numeric = numeric || strings.Contains(h.Path, "/chart/data/series/0/values/")
	}
	if !numeric {
		t.Errorf("turn 4: find missed the chart series value: %+v", *found.Hits)
	}
	replaced := j.validate(map[string]any{"deck_id": deck, "find": "9.4", "replace": "9.6"})
	if replaced.HitCount == nil || *replaced.HitCount != len(*found.Hits) || !replaced.Stored {
		t.Errorf("turn 4: replace rewrote %v hits (stored=%v), want %d", replaced.HitCount, replaced.Stored, len(*found.Hits))
	}
	if !equalInts(replaced.ChangedSlides, []int{1, 2}) {
		t.Errorf("turn 4: replace changed_slides = %v, want [1 2]", replaced.ChangedSlides)
	}
	// The derived figures (54% -> 57%, the whole series) are the journey's own
	// n=29 patch; its replace ops name whole fields, so they still apply.
	turn4, _ := j.renderOK(patched(29))
	if !equalInts(turn4.ChangedSlides, []int{1, 2}) {
		t.Errorf("turn 4: changed_slides = %v, want [1 2]", turn4.ChangedSlides)
	}
	for _, stale := range []string{"9.4", "54%"} {
		if left := j.validate(map[string]any{"deck_id": deck, "find": stale}); left.HitCount == nil || *left.HitCount != 0 {
			t.Errorf("turn 4: %q still occurs after the fix: %+v", stale, left.Hits)
		}
	}

	// --- turn 5: burn chart and roadmap share one slide ---
	turn5, _ := j.renderOK(patched(32))
	if !equalInts(turn5.ChangedSlides, []int{4}) {
		t.Errorf("turn 5: changed_slides = %v, want only the merged slide [4] (the journey got [3 4 5 6 7])", turn5.ChangedSlides)
	}
	if countChange(turn5.SlideChanges, slideChangeRemoved) != 1 || countChange(turn5.SlideChanges, slideChangeRenumbered) != 4 {
		t.Errorf("turn 5: want 1 removed and 4 renumbered, got %+v", turn5.SlideChanges)
	}

	// --- turn 6: switch the template ---
	turn6, _ := j.renderOK(patched(34))
	if len(turn6.ChangedSlides) != 8 || countChange(turn6.SlideChanges, slideChangeRestyled) != 8 {
		t.Errorf("turn 6: a template switch restyles all 8 slides, got changed=%v changes=%+v", turn6.ChangedSlides, turn6.SlideChanges)
	}
	if idx, ok := thumbnailIndices(t, turn6); !ok || idx != nil {
		t.Errorf("turn 6: want a whole-deck thumbnails call, got %+v", turn6.NextToolCall)
	}

	// --- turn 7: "undo the last change to the title slide" ---
	// There was none. One call now answers that: the title slide's only
	// changes are its creation and the deck-wide restyle.
	history := j.validate(map[string]any{"deck_id": deck, "read": "history"})
	if len(history.Revisions) != turn6.Revision || len(history.Slides) != 8 {
		t.Fatalf("turn 7: history has %d revisions and %d slides, want %d and 8", len(history.Revisions), len(history.Slides), turn6.Revision)
	}
	title := history.Slides[0]
	if title.ID != ids[0] || title.LastChanged != turn6.Revision || title.LastChange != slideChangeRestyled {
		t.Errorf("turn 7: title slide history = %+v, want last changed by the restyle in revision %d", title, turn6.Revision)
	}
	for _, rev := range history.Revisions[1 : len(history.Revisions)-1] {
		if changeOf(rev.Changes, ids[0]) != "" {
			t.Errorf("turn 7: revision %d claims to have changed the title slide: %+v", rev.Revision, rev.Changes)
		}
	}
	for _, rev := range history.Revisions {
		if rev.Revision == 0 || rev.Time == "" || rev.Changes == nil {
			t.Errorf("turn 7: revision row lacks number, time or changes: %+v", rev)
		}
	}
	// The journey's probes are still rejected as unknown arguments, cleanly.
	for _, n := range []int{36, 37} {
		args := map[string]any{}
		for k, v := range calls[n] {
			args[k] = v
		}
		args["deck_id"] = deck
		if res := callToolViaServer(t, j.s, "validate_deck_spec", args); !res.IsError || !strings.Contains(resultText(res), "UNKNOWN_PARAMETER") {
			t.Errorf("call n=%d should still be an UNKNOWN_PARAMETER error, got %s", n, resultText(res))
		}
	}

	// --- turn 8: speaker notes on every slide, plus two appendix slides ---
	turn8, size := j.renderOK(patched(40))
	if turn8.SlideCount != 10 || !equalInts(turn8.ChangedSlides, []int{8, 9}) {
		t.Errorf("turn 8: slide_count=%d changed_slides=%v, want 10 slides and only the two new ones [8 9] (the journey got all ten)",
			turn8.SlideCount, turn8.ChangedSlides)
	}
	if countChange(turn8.SlideChanges, slideChangeNotesOnly) != 8 {
		t.Errorf("turn 8: the 8 annotated slides should be notes_only, got %+v", turn8.SlideChanges)
	}
	if idx, _ := thumbnailIndices(t, turn8); !equalInts(idx, []int{8, 9}) {
		t.Errorf("turn 8: next_tool_call asks for %v, want [8 9]", idx)
	}
	t.Logf("turn 8 response: %d bytes", size)
	// E16: the eight annotated slides render exactly as before, so their
	// image identity must not change (go-slide-creator-6ffgv). The identity
	// key is what the thumbnail cache reuses a stored image by.
	beforeKeys, err := render.VisibleSlideKeys(turn6.PptxPath)
	if err != nil {
		t.Fatalf("slide keys of the turn 6 deck: %v", err)
	}
	afterKeys, err := render.VisibleSlideKeys(turn8.PptxPath)
	if err != nil {
		t.Fatalf("slide keys of the turn 8 deck: %v", err)
	}
	if len(beforeKeys) != 8 || len(afterKeys) != 10 {
		t.Fatalf("slide keys: %d before, %d after; want 8 and 10", len(beforeKeys), len(afterKeys))
	}
	for i := range beforeKeys {
		if beforeKeys[i] != afterKeys[i] {
			t.Errorf("turn 8: slide %d has a new image identity after a notes-only edit", i)
		}
	}

	// The slide ids survived every insert, removal and move.
	final := j.validate(map[string]any{"deck_id": deck, "read": "history"})
	at := map[string]int{}
	for _, s := range final.Slides {
		at[s.ID] = s.Index
	}
	if at[ids[0]] != 0 || at[riskID] != 6 || at[askID] != 7 || at[matrixID] != 5 {
		t.Errorf("final positions by id = %v; want title 0, matrix 5, risk 6, ask 7", at)
	}
	// And the stored spec reads back whole or per slide (E5).
	whole := j.validate(map[string]any{"deck_id": deck, "read": "spec"})
	var spec struct {
		Slides []map[string]any `json:"slides"`
	}
	if err := json.Unmarshal(whole.Spec, &spec); err != nil || len(spec.Slides) != 10 {
		t.Fatalf("read spec: %d slides, err %v", len(spec.Slides), err)
	}
	one := j.validate(map[string]any{"deck_id": deck, "read": riskID})
	if one.SlideRef == nil || one.SlideRef.Index != 6 || !strings.Contains(string(one.Slide), "Permitting delay is the main risk") {
		t.Errorf("read %s: ref=%+v slide=%s", riskID, one.SlideRef, one.Slide)
	}
}

// thumbnailHashes renders a deck's thumbnails at the default density and
// returns each slide's content_hash.
func thumbnailHashes(t *testing.T, mc *mcpConfig, pptxPath string) []string {
	t.Helper()
	res := mustCall(t, mc.handleRenderDeckThumbnails, map[string]any{"pptx_path": pptxPath})
	if res.IsError {
		t.Fatalf("render_deck_thumbnails failed: %s", textContent(res))
	}
	var meta renderedDeckThumbnailsResponse
	if err := json.Unmarshal([]byte(textContent(res)), &meta); err != nil {
		t.Fatalf("decode thumbnails metadata: %v", err)
	}
	out := make([]string, len(meta.Slides))
	for i, s := range meta.Slides {
		out[i] = s.ContentHash
	}
	return out
}

// go-slide-creator-6ffgv, end to end through LibreOffice: a revision that only
// adds speaker notes and a slide returns the same content_hash for every
// slide that looks the same, at the default thumbnail density.
func TestNotesOnlyRevisionKeepsThumbnailHashes(t *testing.T) {
	if testing.Short() {
		t.Skip("render integration skipped in -short mode")
	}
	if ok, _ := render.DependencyStatus(); !ok {
		t.Skip("LibreOffice/ImageMagick not installed")
	}
	// A private render cache: the stored slide images are the subject.
	t.Setenv("TMPDIR", t.TempDir())
	mc := handleTestConfig(t)

	first := renderDeckSpec(t, mc, map[string]any{"spec": revisionTestSpec})
	before := thumbnailHashes(t, mc, first.PptxPath)
	if len(before) != 4 {
		t.Fatalf("rendered %d thumbnails, want 4", len(before))
	}

	second := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID, "patch": []any{
		map[string]any{"op": "add", "path": "/slides/0/notes", "value": "Welcome."},
		map[string]any{"op": "add", "path": "/slides/costs/notes", "value": "Stress the trend."},
		map[string]any{"op": "replace", "path": "/slides/1/title", "value": "Growth held at 43%"},
		map[string]any{"op": "add", "path": "/slides/-", "value": map[string]any{"kind": "closing", "title": "Appendix"}},
	}})
	if second.PptxPath == first.PptxPath {
		t.Fatal("the revision should be a different artifact")
	}
	after := thumbnailHashes(t, mc, second.PptxPath)
	if len(after) != 5 {
		t.Fatalf("rendered %d thumbnails, want 5", len(after))
	}
	for _, i := range []int{0, 2, 3} {
		if before[i] != after[i] {
			t.Errorf("slide %d looks the same in both revisions but its content_hash changed", i)
		}
	}
	if before[1] == after[1] {
		t.Error("slide 1 was edited but kept its content_hash")
	}
	if !equalInts(second.ChangedSlides, []int{1, 4}) {
		t.Errorf("changed_slides = %v, want the edited and the new slide [1 4]", second.ChangedSlides)
	}
}
