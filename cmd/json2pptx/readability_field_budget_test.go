package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// sharedCellSlide is one shape_grid cell holding a bold 12pt eyebrow over a
// 14pt body, in a box heightPct of the slide tall: the shared cell of
// go-slide-creator-ifcng, where the eyebrow is the paragraph that drops below
// its floor but the body is what overflows the box.
func sharedCellSlide(t *testing.T, body string, heightPct float64) SlideInput {
	t.Helper()
	text, err := json.Marshal(map[string]any{
		"paragraphs": []map[string]any{
			{"content": "PILOT DESIGN", "size": 12, "bold": true},
			{"content": body, "size": 14},
		},
		"vertical_align": "t",
	})
	if err != nil {
		t.Fatal(err)
	}
	return SlideInput{LayoutID: "blank", ShapeGrid: &ShapeGridInput{
		Bounds:  &GridBoundsInput{X: 10, Y: 20, Width: 30, Height: heightPct},
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{
			Geometry: "rect", Text: text,
		}}}}},
	}}
}

const sharedCellBody = "The workbench puts unowned cases beside assigned work. The market lead assigns a resolver each morning; finance checks closure evidence at day-end and escalates any case left without an owner after a full working day."

func sharedCellFinding(t *testing.T, body string, heightPct float64) *patterns.FitFinding {
	t.Helper()
	in := &PresentationInput{ViewingMode: "present", Slides: []SlideInput{sharedCellSlide(t, body, heightPct)}}
	got := readabilityCodes(collectReadabilityFindings(in, nil, 12192000, 6858000))
	if len(got) == 0 {
		return nil
	}
	return &got[0]
}

// go-slide-creator-ifcng: the eyebrow fails, but the fix names the body and a
// budget for the body alone — one that, applied, clears the cell.
func TestSharedCellFixBudgetsTheFieldItEdits(t *testing.T) {
	f := sharedCellFinding(t, sharedCellBody, 20)
	if f == nil {
		t.Fatal("expected TEXT_BELOW_READABLE_MIN on the shared cell")
	}
	p := f.Fix.Params
	if p["paragraph_text"] != "PILOT DESIGN" {
		t.Fatalf("paragraph_text = %v, want the eyebrow that fails", p["paragraph_text"])
	}
	if p["edit_text"] != sharedCellBody {
		t.Fatalf("edit_text = %v, want the body whose length causes the shrink", p["edit_text"])
	}
	n, ok := p["field_max_chars"].(int)
	bodyLen := utf8.RuneCountInString(sharedCellBody)
	if !ok || n >= bodyLen || n < bodyLen/2 {
		t.Fatalf("field_max_chars = %v, want a real cut of the %d-char body", p["field_max_chars"], bodyLen)
	}
	if _, ok := p["max_chars"].(int); !ok {
		t.Error("the raw cell budget max_chars must stay for reduce_cell_text")
	}
	if _, ok := p["repair"]; ok {
		t.Errorf("a clearing field budget is not a composition repair: %v", p)
	}
	// Re-run the stated repair: the body cut to its budget clears the cell.
	if again := sharedCellFinding(t, cutAtWord(sharedCellBody, n), 20); again != nil {
		t.Errorf("applying field_max_chars=%d left %s", n, again.Message)
	}
}

// When no single field can absorb the overflow within half its length, the
// fix is composition-level and promises no character budget.
func TestSharedCellFixIsCompositionWhenNoFieldCutClears(t *testing.T) {
	f := sharedCellFinding(t, sharedCellBody, 14)
	if f == nil {
		t.Fatal("expected TEXT_BELOW_READABLE_MIN on the cramped cell")
	}
	p := f.Fix.Params
	if p["repair"] != "composition" {
		t.Fatalf("repair = %v, want composition", p["repair"])
	}
	if _, ok := p["field_max_chars"]; ok {
		t.Errorf("composition repair carries a field budget: %v", p)
	}
	if p["edit_text"] != sharedCellBody {
		t.Errorf("edit_text = %v, want the longest field", p["edit_text"])
	}
}

func TestSemanticFixParamsUsesFieldBudget(t *testing.T) {
	out := semanticFixParams("reduce_cell_text", map[string]any{
		"max_chars": 532, "field_max_chars": 103, "edit_text": "body text", "paragraph_text": "PILOT DESIGN",
	})
	if out["max_chars"] != 103 || out["cell_max_chars"] != 532 {
		t.Errorf("max_chars/cell_max_chars = %v/%v, want 103/532", out["max_chars"], out["cell_max_chars"])
	}
	if _, ok := out["edit_text"]; ok {
		t.Error("edit_text is an internal locator and must not reach the DeckSpec params")
	}
	comp := semanticFixParams("reduce_cell_text", map[string]any{"max_chars": 532, "repair": "composition"})
	if _, ok := comp["max_chars"]; ok || comp["cell_max_chars"] != 532 {
		t.Errorf("composition params = %v, want no max_chars and cell_max_chars 532", comp)
	}
}

func TestCutAuthoredFieldMatchesUppercasedEyebrow(t *testing.T) {
	values, _ := json.Marshal(map[string]any{"eyebrow": "Pilot design", "body": "Some body"})
	slide := SlideInput{Pattern: &PatternInput{Name: "image-text-split", Values: values}}
	cut, ok := cutAuthoredField(slide, "PILOT DESIGN", 5)
	if !ok {
		t.Fatal("eyebrow written upper-case was not found in the authored values")
	}
	var v map[string]string
	if err := json.Unmarshal(cut.Pattern.Values, &v); err != nil {
		t.Fatal(err)
	}
	if v["eyebrow"] != "Pilot" || v["body"] != "Some body" {
		t.Errorf("values = %v, want only the eyebrow cut", v)
	}
}

// The consulting benchmark's image_case (go-slide-creator-ifcng): the eyebrow
// renders below its floor, and the DeckSpec repair must not ask for a
// 532-character eyebrow. It either budgets the field it patches below that
// field's length, or says the repair is composition-level.
func TestValidateDeckSpecSharedCellRepairNeverInflatesTheEyebrow(t *testing.T) {
	const spec = `{"meta":{"template":"midnight-blue","title":"Helios","type_scale":"comfortable","source":"Illustrative Helios case; management assumptions, Oct 2026"},"slides":[{"kind":"image_case","title":"A visible unassigned queue makes stalled disputes actionable within one working day","eyebrow":"Pilot design","heading":"Turn missing ownership into a daily decision","body":"The workbench puts unowned cases beside assigned work. The market lead assigns a resolver each morning; finance checks closure evidence at day-end.","bullets":["Show owner, age and next action in one view","Escalate unowned cases after 24 hours"],"image_label":"Dispute workbench","caption":"Illustrative dashboard design; the first two cases have no assigned owner.","metrics":[{"value":"12","label":"unowned cases to assign"}],"takeaway":"Review ownership daily rather than waiting for the monthly receivables report.","source":"Illustrative Helios case; management assumptions, Oct 2026"}]}`
	mc := refusalTestConfig(t)
	res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": decodeSpecObject(t, spec)}))
	if err != nil {
		t.Fatal(err)
	}
	var env deckSpecEnvelopeResponse
	structuredInto(t, res.StructuredContent, &env)
	var found *diagnostics.Finding
	for i := range env.Findings {
		if env.Findings[i].Code == "INPUT.TEXT_BELOW_READABLE_MIN" {
			found = &env.Findings[i]
		}
	}
	if found == nil {
		t.Skipf("this template build fits the shared cell; nothing to repair: %+v", env.Findings)
	}
	if _, leaked := found.Evidence[editPathDetail]; leaked {
		t.Error("internal edit_path detail leaked into evidence")
	}
	fields := map[string]string{
		"/slides/0/eyebrow": "Pilot design", "/slides/0/heading": "Turn missing ownership into a daily decision",
		"/slides/0/body": "The workbench puts unowned cases beside assigned work. The market lead assigns a resolver each morning; finance checks closure evidence at day-end.",
	}
	// go-slide-creator-micna / -vihnl: the patch lives in next_tool_call. A
	// rewrite carries the budget of the field it patches; a patch that is
	// complete as written was tried on the spec first.
	if found.NextToolCall == nil {
		t.Fatalf("no patch offered: %+v", found)
	}
	for _, raw := range patchOf(t, found.NextToolCall) {
		op := raw.(map[string]any)
		path, _ := op["path"].(string)
		value, rewrite := op["value"].(string)
		if !rewrite || !strings.HasPrefix(value, "<") {
			if !found.PatchVerified {
				t.Errorf("a patch that is complete as written was not verified: %v", op)
			}
			continue
		}
		if path == "/slides/0/eyebrow" {
			t.Errorf("the patch rewrites the 12-char eyebrow, which cannot absorb the shared cell's overflow: %v", op)
		}
		n, ok := intFixParam(found.Remediation.Primary.Params, "max_chars")
		if src, known := fields[path]; !ok || !known || n >= utf8.RuneCountInString(src) {
			t.Errorf("max_chars %d at %s does not shorten the field it patches", n, path)
		}
		if strings.Contains(value, "532") {
			t.Errorf("hint still carries the whole-cell budget: %s", value)
		}
	}
}
