package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/template"
)

// severityTestDeck is the shape that produced the report: an over-dense table
// (13 logical rows, 9 columns) that two detectors both notice, plus a
// long-headline slide and a near-empty one, so several finding families are in
// play at once.
func severityTestDeck() map[string]any {
	rows := make([]any, 0, 12)
	for i := 0; i < 12; i++ {
		rows = append(rows, []any{"Region", "Q1", "Q2", "Q3", "Q4", "FY", "Plan", "Var", "Owner"})
	}
	return map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"slide_type": "content",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text",
						"text_value": "A headline that goes on and on and says a great deal more than any single slide title has any business saying in one line"},
					map[string]any{"placeholder_id": "body", "type": "table", "table_value": map[string]any{
						"headers": []any{"Region", "Q1", "Q2", "Q3", "Q4", "FY", "Plan", "Var", "Owner"},
						"rows":    rows,
					}},
				},
			},
			map[string]any{
				"slide_type": "content",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Sparse"},
					map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": []any{"One"}},
				},
			},
		},
	}
}

// severitiesByCode reads every {code, severity} pair out of a tool response.
func severitiesByCode(t *testing.T, payload any) map[string]map[string]bool {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	out := map[string]map[string]bool{}
	var walk func(any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			code, hasCode := v["code"].(string)
			sev, hasSev := v["severity"].(string)
			if hasCode && hasSev {
				// The envelope namespaces codes ("FIT.density_exceeded") while
				// score_deck's per-slide findings carry the bare code; compare
				// the code itself.
				code = bareFindingCode(code)
				if out[code] == nil {
					out[code] = map[string]bool{}
				}
				out[code][sev] = true
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(tree)
	return out
}

// bareFindingCode strips a namespace prefix ("FIT.", "INPUT.") from a code.
func bareFindingCode(code string) string {
	if i := strings.IndexByte(code, '.'); i > 0 {
		return code[i+1:]
	}
	return code
}

func sortedSeverities(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// TestFindingSeverityIsConsistent is the go-slide-creator-7xyy acceptance test.
// One finding code must mean one severity: within a single response, and across
// the two tools an agent uses to decide what to fix. The reported deck showed
// density_exceeded arriving as four warnings AND four infos in one
// validate_input response — the same four table facts, counted twice — while
// HEADLINE_TOO_LONG was info in validate_input and warning in score_deck.
func TestFindingSeverityIsConsistent(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	ctx := context.Background()
	deck := severityTestDeck()

	validateRes, err := mc.handleValidate(ctx, makeRequest(map[string]any{
		"presentation": deck, "fit_report": true,
	}))
	if err != nil {
		t.Fatalf("validate_input: %v", err)
	}
	validate := severitiesByCode(t, validateRes.StructuredContent)
	if len(validate) == 0 {
		t.Fatal("the fixture produced no findings; it is supposed to be a dense, over-long deck")
	}

	for code, sevs := range validate {
		if len(sevs) > 1 {
			t.Errorf("validate_input reports %s at %v — one code, one severity", code, sortedSeverities(sevs))
		}
	}

	scoreRes, err := mc.handleScoreDeck(ctx, makeRequest(map[string]any{"presentation": deck}))
	if err != nil {
		t.Fatalf("score_deck: %v", err)
	}
	score := severitiesByCode(t, scoreRes.StructuredContent)

	var compared int
	for code, vs := range validate {
		ss, ok := score[code]
		if !ok {
			continue
		}
		compared++
		v, s := sortedSeverities(vs), sortedSeverities(ss)
		if len(v) != len(s) || v[0] != s[0] {
			t.Errorf("%s: validate_input says %v, score_deck says %v — an agent filtering on severity gets two work lists", code, v, s)
		}
	}
	if compared == 0 {
		t.Error("no finding code appeared in both responses; the comparison proved nothing")
	}
	t.Logf("compared %d codes shared by validate_input and score_deck", compared)
}

// TestOneFactIsReportedOnce pins the de-duplication end to end: the validator
// and the fit report both notice an over-dense table, and the deck must carry
// each fact once.
func TestOneFactIsReportedOnce(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": severityTestDeck(), "fit_report": true,
	}))
	if err != nil {
		t.Fatalf("validate_input: %v", err)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Findings struct {
			Findings []struct {
				Code     string         `json:"code"`
				Message  string         `json:"message"`
				Evidence map[string]any `json:"evidence"`
			} `json:"findings"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if len(payload.Findings.Findings) == 0 {
		t.Fatal("no findings in the envelope")
	}
	seen := map[[3]string]int{}
	for _, f := range payload.Findings.Findings {
		path, _ := f.Evidence["path"].(string)
		key := [3]string{f.Code, path, f.Message}
		seen[key]++
		if seen[key] > 1 {
			t.Errorf("the same fact is reported %d times: %s at %s — %q", seen[key], f.Code, path, f.Message)
		}
	}
}
