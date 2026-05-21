package diagnostics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// sampleDiagnostics returns a representative mix of error/warning/info
// diagnostics drawn from the real converters, for use across the envelope tests.
func sampleDiagnostics() []Diagnostic {
	return []Diagnostic{
		FromValidationError(&patterns.ValidationError{
			Pattern: "card-grid",
			Path:    "slides[2].content.cells[0].header",
			Code:    "required",
			Message: "card-grid: header is required",
			Fix:     &patterns.FixSuggestion{Kind: "provide_value", Params: map[string]any{"field": "header"}},
		}),
		FromFitFinding(patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    "slides[0].content.body",
				Code:    "placeholder_overflow",
				Message: "body overflows the placeholder",
			},
			Action:        "shrink_or_split",
			OverflowRatio: 1.4,
		}),
		{Code: "TEMPLATE_NOT_FOUND", Message: "no such template", Severity: SeverityError},
		{Code: "NO_EMOJI_POLICY", Message: "emoji not allowed", Severity: SeverityWarning},
	}
}

func TestBuildEnvelope_Shape(t *testing.T) {
	env := BuildEnvelope(EnvelopeOptions{
		Subcommand:  "validate",
		InputSHA256: ComputeInputSHA256([]byte(`{"slides":[]}`)),
		Template:    "midnight-blue",
	}, sampleDiagnostics())

	if env.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %q, want %q", env.SchemaVersion, SchemaVersion)
	}
	if env.Tool != DefaultTool {
		t.Errorf("tool = %q, want default %q", env.Tool, DefaultTool)
	}
	if env.Subcommand != "validate" {
		t.Errorf("subcommand = %q, want validate", env.Subcommand)
	}
	if env.OK {
		t.Error("ok = true, want false (envelope contains error-severity findings)")
	}
	if len(env.Findings) != 4 {
		t.Fatalf("findings length = %d, want 4", len(env.Findings))
	}
	if env.Findings == nil {
		t.Error("findings must be non-nil")
	}

	// IDs unique within the envelope.
	seen := map[string]bool{}
	for _, f := range env.Findings {
		if seen[f.ID] {
			t.Errorf("duplicate finding id %q", f.ID)
		}
		seen[f.ID] = true
	}
}

func TestFindingFromDiagnostic_Mapping(t *testing.T) {
	d := FromFitFinding(patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    "slides[3].content.body",
			Code:    "placeholder_overflow",
			Message: "overflow",
		},
		Action:        "refuse",
		OverflowRatio: 2.1,
	})

	f := FindingFromDiagnostic(d)

	if f.Code != "FIT.placeholder_overflow" {
		t.Errorf("code = %q, want FIT.placeholder_overflow", f.Code)
	}
	if f.Category != NamespaceFit {
		t.Errorf("category = %q, want FIT", f.Category)
	}
	if f.Severity != SeverityError {
		t.Errorf("severity = %q, want error (refuse action)", f.Severity)
	}
	if f.Where == nil || f.Where.Slide == nil || *f.Where.Slide != 3 {
		t.Errorf("where.slide not recovered from path: %+v", f.Where)
	}
	if f.Evidence["path"] != "slides[3].content.body" {
		t.Errorf("evidence.path = %v, want the JSON path", f.Evidence["path"])
	}
	if _, ok := f.Evidence["overflow_ratio"]; !ok {
		t.Error("evidence.overflow_ratio missing")
	}
	if f.DescribeCommand != "json2pptx describe-finding placeholder_overflow" {
		t.Errorf("describe_command = %q; must use the legacy un-prefixed code", f.DescribeCommand)
	}
}

func TestFindingFromDiagnostic_RemediationActionVocabulary(t *testing.T) {
	cases := []struct {
		kind string
		want Action
	}{
		{"provide_value", ActionReplaceValue},
		{"use_one_of", ActionReplaceValue},
		{"shrink_text", ActionShortenText},
		{"split_at_row", ActionSplitSlide},
		{"use_emoji_free_text", ActionRemoveEmoji},
		{"switch_layout", ActionSwitchLayout},
		{"regenerate_pattern", ActionRegeneratePattern},
		{"something_unknown", ActionApplyPatch},
		{"shorten_text", ActionShortenText}, // already an action verb
	}
	for _, tc := range cases {
		d := Diagnostic{Code: "required", Severity: SeverityError, Fix: &Fix{Kind: tc.kind}}
		f := FindingFromDiagnostic(d)
		if f.Remediation == nil || f.Remediation.Primary == nil {
			t.Fatalf("kind %q: no primary remediation", tc.kind)
		}
		if got := f.Remediation.Primary.Action; got != tc.want {
			t.Errorf("kind %q mapped to action %q, want %q", tc.kind, got, tc.want)
		}
		if !IsValidAction(f.Remediation.Primary.Action) {
			t.Errorf("kind %q produced non-vocabulary action %q", tc.kind, f.Remediation.Primary.Action)
		}
	}
}

// TestFindingFromDiagnostic_PreservesAgentRecoveryFields locks the lossless
// adaptation contract: a Diagnostic's NextToolCall and ExampleValue — the
// agent-recovery fields set by the MCP arg-error helpers and by fit findings —
// must survive into the Finding rather than being silently dropped.
func TestFindingFromDiagnostic_PreservesAgentRecoveryFields(t *testing.T) {
	next := &patterns.ToolCallSuggestion{
		Tool:         "get_input_schema",
		ArgsTemplate: map[string]any{"tool": "generate_presentation"},
	}
	example := map[string]any{"layout_id": "title", "content": []any{}}
	d := Diagnostic{
		Code:         "MISSING_PARAMETER",
		Message:      "slides is required",
		Severity:     SeverityError,
		ExpectedType: "array",
		NextToolCall: next,
		ExampleValue: example,
	}

	f := FindingFromDiagnostic(d)

	if f.NextToolCall == nil {
		t.Fatal("next_tool_call dropped during adaptation")
	}
	if f.NextToolCall.Tool != "get_input_schema" {
		t.Errorf("next_tool_call.tool = %q, want get_input_schema", f.NextToolCall.Tool)
	}
	if f.ExampleValue == nil {
		t.Fatal("example_value dropped during adaptation")
	}
	if got, ok := f.ExampleValue.(map[string]any); !ok || got["layout_id"] != "title" {
		t.Errorf("example_value not carried verbatim: %#v", f.ExampleValue)
	}
	// expected_type still rides in evidence (unchanged behavior).
	if f.Evidence["expected_type"] != "array" {
		t.Errorf("evidence.expected_type = %v, want array", f.Evidence["expected_type"])
	}

	// Round-trips on the wire with snake_case keys.
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"next_tool_call"`, `"example_value"`, `"args_template"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("marshaled finding missing key %s: %s", key, b)
		}
	}
}

func TestClassifyCode(t *testing.T) {
	cases := map[string]Namespace{
		"TEMPLATE_NOT_FOUND":    NamespaceTemplate,
		"INVALID_JSON":          NamespaceInput,
		"MISSING_PARAMETER":     NamespaceInput,
		"GENERATION_FAILED":     NamespaceRender,
		"PATTERN_ERROR":         NamespaceGrid,
		"STRICT_FIT":            NamespaceFit,
		"placeholder_overflow":  NamespaceFit,
		"accent_overload":       NamespaceFit,
		"body_too_long":         NamespaceFit,
		"chart.zero_sum_pie":    NamespaceRender,
		"NO_EMOJI_POLICY":       NamespacePolicy,
		"FIT.already_namespaced": NamespaceFit,
		"":                      NamespaceInput,
	}
	for code, want := range cases {
		if got := ClassifyCode(code); got != want {
			t.Errorf("ClassifyCode(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestDottedCode_IdempotentForNamespacedInput(t *testing.T) {
	if got := DottedCode(NamespaceFit, "FIT.placeholder_overflow"); got != "FIT.placeholder_overflow" {
		t.Errorf("DottedCode double-prefixed an already-namespaced code: %q", got)
	}
	if got := DottedCode(NamespaceInput, "required"); got != "INPUT.required" {
		t.Errorf("DottedCode(INPUT, required) = %q, want INPUT.required", got)
	}
}

func TestActionVocabulary_StableAndValid(t *testing.T) {
	want := []Action{
		"shorten_text", "replace_value", "apply_patch", "switch_layout",
		"split_slide", "move_to_placeholder", "remove_emoji", "regenerate_pattern",
	}
	got := AllActions()
	if len(got) != len(want) {
		t.Fatalf("AllActions length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllActions[%d] = %q, want %q", i, got[i], want[i])
		}
		if !IsValidAction(want[i]) {
			t.Errorf("IsValidAction(%q) = false", want[i])
		}
	}
	if IsValidAction("not_an_action") {
		t.Error("IsValidAction accepted a bogus action")
	}
}

func TestEnvelope_JSONRoundTripAndSnakeCase(t *testing.T) {
	env := BuildEnvelope(EnvelopeOptions{Subcommand: "validate"}, sampleDiagnostics())
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{
		`"schema_version"`, `"subcommand"`, `"ok"`, `"summary"`, `"findings"`,
		`"describe_command"`, `"category"`,
	} {
		if !strings.Contains(string(b), key) {
			t.Errorf("marshaled envelope missing snake_case key %s", key)
		}
	}
	var round FindingEnvelope
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round.SchemaVersion != env.SchemaVersion || len(round.Findings) != len(env.Findings) {
		t.Error("round-trip changed envelope content")
	}
}

func TestComputeInputSHA256(t *testing.T) {
	h := ComputeInputSHA256([]byte("hello"))
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(h) {
		t.Errorf("sha256 = %q, want 64 hex chars", h)
	}
	if ComputeInputSHA256([]byte("hello")) != h {
		t.Error("sha256 not deterministic")
	}
}

// TestDescribeCommandResolvesForFitCodes asserts that the legacy code embedded
// in a finding's describe_command resolves in the describe-finding registry for
// fit/validation findings — i.e. describe-finding works for the codes the
// envelope tells agents to look up. (Deliverable: describe must work for every
// emitted finding code.)
func TestDescribeCommandResolvesForFitCodes(t *testing.T) {
	codes := []string{"placeholder_overflow"}
	for _, code := range codes {
		d := Diagnostic{Code: code, Severity: SeverityWarning}
		f := FindingFromDiagnostic(d)
		legacy := strings.TrimPrefix(f.DescribeCommand, "json2pptx describe-finding ")
		if legacy != code {
			t.Fatalf("describe_command legacy code = %q, want %q", legacy, code)
		}
		if _, ok := patterns.GetFindingMeta(legacy); !ok {
			t.Errorf("describe-finding has no entry for emitted code %q", legacy)
		}
	}
}

// schema is the minimal subset of JSON Schema this test interprets to validate
// real envelope output against docs/api/finding-envelope.schema.json.
type schema struct {
	Type                 string             `json:"type"`
	Required             []string           `json:"required"`
	Properties           map[string]schema  `json:"properties"`
	AdditionalProperties *bool              `json:"additionalProperties"`
	Items                *schema            `json:"items"`
	Enum                 []string           `json:"enum"`
	Ref                  string             `json:"$ref"`
	Const                string             `json:"const"`
	Defs                 map[string]schema  `json:"$defs"`
}

// TestEnvelopeConformsToCommittedSchema marshals a real envelope and validates
// it against the committed JSON Schema's structural constraints: required keys,
// enum membership, const values, and additionalProperties=false.
func TestEnvelopeConformsToCommittedSchema(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "api", "finding-envelope.schema.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var root schema
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	// Include an arg-error-style diagnostic so the new next_tool_call /
	// example_value finding fields are validated against the committed schema
	// (the finding def is additionalProperties:false).
	ds := append(sampleDiagnostics(), Diagnostic{
		Code:         "MISSING_PARAMETER",
		Message:      "slides is required",
		Severity:     SeverityError,
		ExpectedType: "array",
		NextToolCall: &patterns.ToolCallSuggestion{
			Tool:         "get_input_schema",
			ArgsTemplate: map[string]any{"tool": "generate_presentation"},
		},
		ExampleValue: map[string]any{"layout_id": "title"},
	})
	env := BuildEnvelope(EnvelopeOptions{
		Subcommand:  "validate",
		InputSHA256: ComputeInputSHA256([]byte(`{}`)),
		Template:    "midnight-blue",
	}, ds)
	b, _ := json.Marshal(env)
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	if errs := validateAgainst(root, root.Defs, doc, "$"); len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("schema violation: %s", e)
		}
	}
}

// validateAgainst checks doc against the given schema node, resolving local
// $refs through defs. It interprets type, required, properties,
// additionalProperties, items, enum, and const — enough to lock the envelope
// contract without a third-party validator dependency.
func validateAgainst(s schema, defs map[string]schema, doc any, path string) []string {
	if s.Ref != "" {
		name := strings.TrimPrefix(s.Ref, "#/$defs/")
		resolved, ok := defs[name]
		if !ok {
			return []string{path + ": unresolved $ref " + s.Ref}
		}
		return validateAgainst(resolved, defs, doc, path)
	}

	var errs []string
	switch s.Type {
	case "object":
		m, ok := doc.(map[string]any)
		if !ok {
			return []string{path + ": expected object"}
		}
		for _, req := range s.Required {
			if _, present := m[req]; !present {
				errs = append(errs, path+": missing required key "+req)
			}
		}
		if s.AdditionalProperties != nil && !*s.AdditionalProperties {
			for k := range m {
				if _, declared := s.Properties[k]; !declared {
					errs = append(errs, path+": additional property "+k+" not allowed")
				}
			}
		}
		for k, v := range m {
			if sub, declared := s.Properties[k]; declared {
				errs = append(errs, validateAgainst(sub, defs, v, path+"."+k)...)
			}
		}
	case "array":
		arr, ok := doc.([]any)
		if !ok {
			return []string{path + ": expected array"}
		}
		if s.Items != nil {
			for _, item := range arr {
				errs = append(errs, validateAgainst(*s.Items, defs, item, path+"[]")...)
			}
		}
	case "string":
		str, ok := doc.(string)
		if !ok {
			return []string{path + ": expected string"}
		}
		if s.Const != "" && str != s.Const {
			errs = append(errs, path+": const mismatch, got "+str)
		}
		if len(s.Enum) > 0 && !enumContains(s.Enum, str) {
			errs = append(errs, path+": value "+str+" not in enum")
		}
	}
	return errs
}

func enumContains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
