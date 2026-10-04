package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// One slide base in messages (go-slide-creator-pikfw, go-slide-creator-njok5):
// "slide N" is the 1-based slide number, and a 0-based value is written
// "slide index N" (or as a slides[N] / /slides/N path).

var (
	slideNumberToken = regexp.MustCompile(`(?i)\bslides? (index |indices )?(\d+)(?:\s*[-–]\s*(\d+))?((?:, \d+)*)`)
	slideLeadIn      = regexp.MustCompile(`^(?i)slide (\d+)\b`)
)

// slideIndexRuleViolations scans one message from a deck of slideCount slides.
// findingIdx is the 0-based slide the message's finding is about, or -1.
//
// A fixture with flaws on its first and its last slide makes both mistakes
// visible without knowing what each message means: a 0-based value printed as
// "slide N" reads "slide 0" for the first slide, and a 1-based value printed
// as "slide index N" reads "slide index <slideCount>" for the last.
func slideIndexRuleViolations(msg string, slideCount, findingIdx int) []string {
	var out []string
	for _, m := range slideNumberToken.FindAllStringSubmatch(msg, -1) {
		indexed := m[1] != ""
		values := []string{m[2]}
		if m[3] != "" {
			values = append(values, m[3])
		}
		for _, v := range strings.Split(m[4], ", ") {
			if v != "" {
				values = append(values, v)
			}
		}
		for _, v := range values {
			n, err := strconv.Atoi(v)
			switch {
			case err != nil:
			case indexed && n >= slideCount:
				out = append(out, fmt.Sprintf("%q: index %d is outside a %d-slide deck, so it is a slide number written as an index", m[0], n, slideCount))
			case !indexed && (n < 1 || n > slideCount):
				out = append(out, fmt.Sprintf("%q: %d is not a slide number of a %d-slide deck, so it is a 0-based index written as \"slide N\"", m[0], n, slideCount))
			}
		}
	}
	if m := slideLeadIn.FindStringSubmatch(msg); m != nil && findingIdx >= 0 {
		if n, _ := strconv.Atoi(m[1]); n != findingIdx+1 {
			out = append(out, fmt.Sprintf("the finding is at slide index %d but its message opens %q", findingIdx, m[0]))
		}
	}
	return out
}

// collectMessages walks a decoded response and returns every "message" /
// "reason" string with the slide its object addresses (path or slide_index),
// or -1.
func collectMessages(v any, out map[string]int) {
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			collectMessages(e, out)
		}
	case map[string]any:
		idx := -1
		if p, ok := t["path"].(string); ok {
			idx = slidepath.SlideIndex(p)
		}
		if si, ok := t["slide_index"].(float64); ok && idx < 0 && si >= 0 {
			idx = int(si)
		}
		for k, e := range t {
			if s, ok := e.(string); ok && (k == "message" || k == "reason" || k == "summary") {
				out[s] = idx
				continue
			}
			collectMessages(e, out)
		}
	}
}

func TestSlideIndexRuleScanner(t *testing.T) {
	for _, tc := range []struct {
		msg   string
		idx   int
		wrong bool
	}{
		{"slide 1: title is a label", 0, false},
		{"slide 6, content 2: unknown type", 5, false},
		{"slides 2-5 are all bullets slides", 1, false},
		{"5 slides are bullets only (slides 2, 3, 4, 5, 6)", -1, false},
		{"duplicated slide index 0 to index 5", -1, false},
		{"pattern repeats 4 consecutive slides (index 1–4)", -1, false},
		{"slide 0 argues from data", 0, true},
		{"consider inserting a different pattern at slide 0", -1, true},
		{"5 slides are bullets only (slides 1, 2, 3, 4, 0)", -1, true},
		{"slides 0–2 use different patterns", -1, true},
		{"slide index 6 is generated", -1, true},
		{"slide 2: title is a label", 0, true},
		{"slide 7 closes on \"Thank you\"", 5, true},
	} {
		if got := slideIndexRuleViolations(tc.msg, 6, tc.idx); (len(got) > 0) != tc.wrong {
			t.Errorf("%q (finding at index %d): violations = %v, want wrong=%v", tc.msg, tc.idx, got, tc.wrong)
		}
	}
}

// TestAgentVisibleMessagesFollowTheSlideIndexRule runs the fixture deck, whose
// flaws sit on the first and the last slide, through the surfaces that word
// their own slide references — `generate -dry-run` (dry_run.go), validate_input
// with the fit report, and analyze_deck_rhythm — and scans every message.
func TestAgentVisibleMessagesFollowTheSlideIndexRule(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "slide_index_rule_deck.json"))
	if err != nil {
		t.Fatal(err)
	}
	var deck map[string]any
	if err := json.Unmarshal(raw, &deck); err != nil {
		t.Fatal(err)
	}
	slideCount := len(deck["slides"].([]any))

	// The same deck with a blocking flaw on the first and on the last slide,
	// for the messages validation words before any fit check runs.
	broken := strings.Replace(string(raw), `"placeholder_id": "title", "type": "text", "text_value": "Revenue"`, `"type": "text", "text_value": "Revenue"`, 1)
	broken = strings.Replace(broken, `"type": "bullets", "bullets_value": ["Questions"]`, `"type": "no_such_type"`, 1)
	if broken == string(raw) || strings.Count(broken, "no_such_type") != 1 {
		t.Fatal("the fixture no longer carries the two lines the broken variant edits")
	}
	brokenPath := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(brokenPath, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}

	mc := testMCPConfig(t)
	surfaces := map[string]string{}
	dryRun, _, _ := cliRun(t, nil, "generate", "-json", brokenPath, "-templates-dir", testTemplatesDir, "-dry-run")
	surfaces["generate -dry-run"] = dryRun
	surfaces["validate_input"] = textContent(mustCall(t, mc.handleValidate, map[string]any{"presentation": deck, "fit_report": true}))
	surfaces["analyze_deck_rhythm"] = textContent(mustCall(t, mc.handleAnalyzeDeckRhythm, map[string]any{"presentation": deck}))

	for name, body := range surfaces {
		var decoded any
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Fatalf("%s: response is not JSON: %v\n%s", name, err, body)
		}
		messages := map[string]int{}
		collectMessages(decoded, messages)
		first, last := false, false
		for msg, idx := range messages {
			first = first || idx == 0 || strings.Contains(msg, "slide 1")
			last = last || idx == slideCount-1 || strings.Contains(msg, fmt.Sprintf("slide %d", slideCount))
			for _, v := range slideIndexRuleViolations(msg, slideCount, idx) {
				t.Errorf("%s: %s\n  message: %s", name, v, msg)
			}
		}
		// The scan only proves something while the fixture still draws
		// messages about its first and its last slide.
		if !first || !last {
			t.Errorf("%s: %d message(s), none about the first slide (%v) or the last (%v); the fixture no longer exercises the rule", name, len(messages), first, last)
		}
	}
}
