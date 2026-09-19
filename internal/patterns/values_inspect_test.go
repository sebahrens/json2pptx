package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

// go-slide-creator-20jm. Every pattern input failure used to reach the agent as
// one string: Go type names for a wrong shape ("cannot unmarshal string into Go
// struct field ProcessFlowValues.steps of type patterns.ProcessFlowStep"), and
// silence for a key the decoder does not read — {"columns": [...]} on
// comparison-2col reported "rows must contain at least 1 row" and never
// mentioned columns. These tests pin the per-field findings, and just as
// importantly pin that the tolerated shapes stay silent.

func inspectValues(t *testing.T, patternName, values string) []*ValidationError {
	t.Helper()
	pat, ok := Default().Get(patternName)
	if !ok {
		t.Fatalf("pattern %q is not registered", patternName)
	}
	return InspectPatternInput(pat, json.RawMessage(values), nil, nil)
}

func TestInspectPatternInput_DroppedFieldNamesTheRename(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		values     string
		wantPath   string
		wantRename string
	}{
		{
			name:       "team-bios title is role",
			pattern:    "team-bios",
			values:     `{"members":[{"name":"Dana","title":"VP Finance"}]}`,
			wantPath:   "values.members[0].title",
			wantRename: "role",
		},
		{
			name:       "phase-roadmap label is name",
			pattern:    "phase-roadmap",
			values:     `{"phases":[{"label":"Discovery","description":"d"}]}`,
			wantPath:   "values.phases[0].label",
			wantRename: "name",
		},
		{
			name:       "phase-roadmap date is date_label",
			pattern:    "phase-roadmap",
			values:     `{"phases":[{"name":"Discovery","date":"Q1"}]}`,
			wantPath:   "values.phases[0].date",
			wantRename: "date_label",
		},
		{
			name:       "comparison-2col columns is rows",
			pattern:    "comparison-2col",
			values:     `{"columns":[{"header":"A","items":["x"]},{"header":"B","items":["y"]}]}`,
			wantPath:   "values.columns",
			wantRename: "rows",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := inspectValues(t, tt.pattern, tt.values)
			var found *ValidationError
			for _, e := range errs {
				if e.Path == tt.wantPath {
					found = e
				}
			}
			if found == nil {
				t.Fatalf("no finding at %s; got %v", tt.wantPath, messagesOf(errs))
			}
			if found.Code != ErrCodePatternUnknownField {
				t.Errorf("code = %q, want %q", found.Code, ErrCodePatternUnknownField)
			}
			if found.Fix == nil || found.Fix.Kind != "rename_field" {
				t.Fatalf("fix = %+v, want kind rename_field", found.Fix)
			}
			if got := found.Fix.Params["did_you_mean"]; got != tt.wantRename {
				t.Errorf("did_you_mean = %v, want %q", got, tt.wantRename)
			}
			if got := found.Fix.Params["to"]; got != tt.wantRename {
				t.Errorf("fix to = %v, want %q", got, tt.wantRename)
			}
			// The message has to say the content is lost — that is the reason the
			// finding blocks rather than warns.
			if !strings.Contains(found.Message, "dropped") {
				t.Errorf("message %q does not say the content is dropped", found.Message)
			}
			assertNoGoTypeNames(t, errs)
		})
	}
}

func TestInspectPatternInput_WrongShapeNamesTheExpectedShape(t *testing.T) {
	errs := inspectValues(t, "process-flow", `{"steps":["A","B","C"]}`)
	if len(errs) != 1 {
		t.Fatalf("want 1 finding, got %v", messagesOf(errs))
	}
	e := errs[0]
	if e.Path != "values.steps[0]" {
		t.Errorf("path = %q, want values.steps[0]", e.Path)
	}
	if e.Code != ErrCodeInvalidShape {
		t.Errorf("code = %q, want %q", e.Code, ErrCodeInvalidShape)
	}
	for _, want := range []string{"must be an object", "label", `the string "A"`} {
		if !strings.Contains(e.Message, want) {
			t.Errorf("message %q does not mention %q", e.Message, want)
		}
	}
	// The example is built from the caller's own content, so it can be pasted.
	if e.Fix == nil || e.Fix.Kind != "reshape_value" {
		t.Fatalf("fix = %+v, want kind reshape_value", e.Fix)
	}
	ex, ok := e.Fix.Params["example"].(map[string]any)
	if !ok || ex["label"] != "A" {
		t.Errorf("example = %v, want {\"label\":\"A\"}", e.Fix.Params["example"])
	}
	assertNoGoTypeNames(t, errs)
}

func TestInspectPatternInput_WrapperObjectNamesTheKeyToUnwrap(t *testing.T) {
	tests := []struct {
		pattern string
		values  string
		wantKey string
	}{
		// values IS the list; encoding/json rejects the wrapper outright.
		{"timeline-horizontal", `{"stops":[{"label":"a"},{"label":"b"},{"label":"c"}]}`, "stops"},
		// KPINupValues tolerates a lone object, so this one decoded to a single
		// empty cell and reported "exactly 3 cells, got 1" plus a swap suggestion
		// for a pattern the author never asked about.
		{"kpi-3up", `{"items":[{"label":"Revenue","value":"$1.2M"},{"label":"Growth","value":"+15%"},{"label":"Users","value":"4.3K"}]}`, "items"},
	}
	wantPhrase := map[string]string{
		"timeline-horizontal": "an array of objects {label, body?, date?, end_date?}",
		"kpi-3up":             "an array where each item is a string or an object {big, small, icon?, sub?}",
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			errs := inspectValues(t, tt.pattern, tt.values)
			if len(errs) != 1 {
				t.Fatalf("want 1 finding, got %v", messagesOf(errs))
			}
			e := errs[0]
			if e.Path != "values" {
				t.Errorf("path = %q, want values", e.Path)
			}
			if e.Code != ErrCodeInvalidShape {
				t.Errorf("code = %q, want %q", e.Code, ErrCodeInvalidShape)
			}
			if !strings.Contains(e.Message, tt.wantKey) {
				t.Errorf("message %q does not name the wrapper key %q", e.Message, tt.wantKey)
			}
			if e.Fix == nil || e.Fix.Params["unwrap_key"] != tt.wantKey {
				t.Errorf("fix = %+v, want unwrap_key %q", e.Fix, tt.wantKey)
			}
			if !strings.Contains(e.Message, wantPhrase[tt.pattern]) {
				t.Errorf("message %q does not describe the shape as %q", e.Message, wantPhrase[tt.pattern])
			}
			assertNoGoTypeNames(t, errs)
		})
	}
}

// The check speaks only when content is demonstrably lost. These payloads all
// work, and a checker that guessed from the schema would refuse every one of
// them: the schema does not list KPICell's aliases, and a chart's map-form data
// is keyed by series name.
func TestInspectPatternInput_SilentOnWorkingPayloads(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		values  string
	}{
		{"kpi canonical", "kpi-3up", `[{"big":"1","small":"a"},{"big":"2","small":"b"},{"big":"3","small":"c"}]`},
		{"kpi value/label aliases", "kpi-3up", `[{"value":"1","label":"a"},{"value":"2","label":"b"},{"value":"3","label":"c"}]`},
		{"kpi string shorthand", "kpi-3up", `["$4.2M | ARR","12 | Teams","98% | NPS"]`},
		{"kpi sub aliases", "kpi-3up", `[{"big":"1","small":"a","delta":"+3%"},{"big":"2","small":"b","trend":"flat"},{"big":"3","small":"c","change":"-1%"}]`},
		{"team-bios canonical", "team-bios", `{"members":[{"name":"Dana","role":"VP Finance","bio":"Twelve years in FP&A."}]}`},
		{"comparison-2col canonical", "comparison-2col", `{"rows":[{"left":"a","right":"b"},{"left":"c","right":"d"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if errs := inspectValues(t, tt.pattern, tt.values); len(errs) > 0 {
				t.Errorf("working payload reported as broken: %v", messagesOf(errs))
			}
		})
	}
}

// A free-form object — one whose schema declares no properties — carries data,
// not field names. chart-insights-split keeps its chart payload as raw JSON and
// normalizes the map form later, so it does not survive a decode round trip and
// must not be reported (go-slide-creator-yzbo would fail otherwise).
func TestInspectPatternInput_FreeFormObjectNotJudged(t *testing.T) {
	values := `{"chart":{"type":"bar","title":"Revenue ($M)",` +
		`"data":{"Q1 2025":120,"Q4 2025":190}},"insights":["Revenue grew every quarter","Q4 was strongest"]}`
	if errs := inspectValues(t, "chart-insights-split", values); len(errs) > 0 {
		t.Errorf("map-form chart data reported as dropped: %v", messagesOf(errs))
	}
}

// An unknown key with nothing close to it still has to be reported; the
// allowed list stands in for a suggestion.
func TestInspectPatternInput_NoSuggestionListsAllowedKeys(t *testing.T) {
	errs := inspectValues(t, "team-bios", `{"members":[{"name":"Dana","role":"VP","zzqq":"Frobnicate"}]}`)
	if len(errs) != 1 {
		t.Fatalf("want 1 finding, got %v", messagesOf(errs))
	}
	e := errs[0]
	if e.Fix == nil || e.Fix.Kind != "remove_field" {
		t.Fatalf("fix = %+v, want kind remove_field", e.Fix)
	}
	allowed, ok := e.Fix.Params["allowed"].([]string)
	if !ok || len(allowed) == 0 {
		t.Fatalf("allowed = %v, want the pattern's property list", e.Fix.Params["allowed"])
	}
	for _, want := range []string{"name", "role"} {
		if !containsString(allowed, want) {
			t.Errorf("allowed %v does not list %q", allowed, want)
		}
	}
}

// Findings are sorted by path so two runs over one deck agree — an agent that
// diffs responses between attempts must not see reordering as change.
func TestInspectPatternInput_DeterministicOrder(t *testing.T) {
	values := `{"phases":[{"label":"P1","date":"Q1"},{"label":"P2","date":"Q2"}]}`
	first := messagesOf(inspectValues(t, "phase-roadmap", values))
	if len(first) < 4 {
		t.Fatalf("want a finding per dropped key, got %v", first)
	}
	for i := 0; i < 5; i++ {
		again := messagesOf(inspectValues(t, "phase-roadmap", values))
		if len(again) != len(first) {
			t.Fatalf("run %d produced %d findings, first run %d", i, len(again), len(first))
		}
		for j := range first {
			if again[j] != first[j] {
				t.Fatalf("run %d finding %d = %q, first run %q", i, j, again[j], first[j])
			}
		}
	}
}

func TestInspectPatternInput_UnsupportedSectionReported(t *testing.T) {
	pat, _ := Default().Get("pull-quote")
	if pat.NewCellOverride() != nil {
		t.Skip("pull-quote now supports cell_overrides; pick another pattern")
	}
	errs := InspectPatternInput(pat, json.RawMessage(`{"quote":"q","attribution":"a"}`), nil,
		map[string]json.RawMessage{"0": json.RawMessage(`{"align":"ctr"}`)})
	if len(errs) != 1 {
		t.Fatalf("want 1 finding, got %v", messagesOf(errs))
	}
	if errs[0].Code != ErrCodeUnknownKey {
		t.Errorf("code = %q, want %q", errs[0].Code, ErrCodeUnknownKey)
	}
	if !strings.Contains(errs[0].Message, "cell_overrides[0]") {
		t.Errorf("message %q does not name the section", errs[0].Message)
	}
}

func TestSuggestFieldName_PrefersUnfilledRequiredProperty(t *testing.T) {
	pat, _ := Default().Get("team-bios")
	members := sectionSchema(pat.Schema(), "values").Property("members").ItemSchema()
	if members == nil {
		t.Fatal("team-bios values.members has no item schema")
	}
	// "name" is already filled, so the only label-ish target left is "role".
	if got := suggestFieldName("title", members, pat.Schema(), []string{"name"}); got != "role" {
		t.Errorf("suggestFieldName(title, present=[name]) = %q, want role", got)
	}
	// With nothing filled, "name" wins on the same class — it is required.
	if got := suggestFieldName("title", members, pat.Schema(), nil); got != "name" {
		t.Errorf("suggestFieldName(title, present=[]) = %q, want name", got)
	}
	// A word with no relationship to any property gets no suggestion.
	if got := suggestFieldName("zzqq", members, pat.Schema(), []string{"name", "role"}); got != "" {
		t.Errorf("suggestFieldName(zzqq) = %q, want no suggestion", got)
	}
}

func TestNormalizePatternName(t *testing.T) {
	tests := map[string]string{
		"kpi-four-up":  "kpi4up",
		"kpi-4up":      "kpi4up",
		"KPI_4_UP":     "kpi4up",
		"kpi4up":       "kpi4up",
		"matrix-2x2":   "matrix2x2",
		"exec-summary": "execsummary",
	}
	for in, want := range tests {
		if got := normalizePatternName(in); got != want {
			t.Errorf("normalizePatternName(%q) = %q, want %q", in, got, want)
		}
	}
}

// Suggest has to resolve the names agents actually write for the numbered
// families: "kpi-four-up" is 4 raw edits from "kpi-4up", so before number-word
// folding the response carried no suggestion at all.
func TestRegistrySuggest_FoldsNumberWords(t *testing.T) {
	tests := map[string]string{
		"kpi-four-up": "kpi-4up",
		"kpi-six-up":  "kpi-6up",
		"kpi-two-up":  "kpi-2up",
		"matrix2x2":   "matrix-2x2",
		"processflow": "process-flow",
	}
	for in, want := range tests {
		got, ok := Default().Suggest(in)
		if !ok || got != want {
			t.Errorf("Suggest(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}
	if got, ok := Default().Suggest("wibble-wobble-flimflam"); ok {
		t.Errorf("Suggest(nonsense) = %q, true; want no suggestion", got)
	}
}

// ---------------------------------------------------------------------------

func messagesOf(errs []*ValidationError) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Path + ": " + e.Message
	}
	return out
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// assertNoGoTypeNames is the whole point of the shape messages: an agent cannot
// act on "patterns.ProcessFlowStep".
func assertNoGoTypeNames(t *testing.T, errs []*ValidationError) {
	t.Helper()
	for _, e := range errs {
		for _, leak := range []string{"patterns.", "Go struct", "Go value", "json:"} {
			if strings.Contains(e.Message, leak) {
				t.Errorf("message leaks implementation detail %q: %s", leak, e.Message)
			}
		}
	}
}
