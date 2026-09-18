package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-e5tm: the one inline pattern example an agent sees first
// (in generate_presentation's `presentation` parameter description) used to be
// hand-written and wrong — values: {items:[{label,value}]}, a shape no pattern
// accepts. It must now be derived from the registry and survive the same
// validator generate runs.
func TestPatternExampleSnippet_IsSchemaTrue(t *testing.T) {
	snippet := patternExampleSnippet(exampleSnippetPattern)

	var envelope struct {
		Pattern *PatternInput `json:"pattern"`
	}
	if err := json.Unmarshal([]byte(snippet), &envelope); err != nil {
		t.Fatalf("snippet is not valid JSON: %v\nsnippet: %s", err, snippet)
	}
	if envelope.Pattern == nil {
		t.Fatalf("snippet has no pattern object: %s", snippet)
	}
	if envelope.Pattern.Name != exampleSnippetPattern {
		t.Errorf("pattern name = %q, want %q", envelope.Pattern.Name, exampleSnippetPattern)
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  validationDefaultSlideWidthEMU,
		SlideHeight: validationDefaultSlideHeightEMU,
	}
	if _, _, err := expandPattern(envelope.Pattern, ctx, patterns.Default()); err != nil {
		t.Fatalf("generate would reject the documented example: %v\nsnippet: %s", err, snippet)
	}
}

// An unknown or exemplar-less pattern must degrade to a still-parseable hint
// rather than panicking or emitting a broken example.
func TestPatternExampleSnippet_UnknownPatternFallback(t *testing.T) {
	got := patternExampleSnippet("definitely-not-a-pattern")
	if !strings.Contains(got, "show_pattern") {
		t.Errorf("fallback should point at show_pattern, got: %s", got)
	}
	if strings.Contains(got, `"items"`) {
		t.Errorf("fallback must not invent a values shape, got: %s", got)
	}
}

// Every COMPLETE JSON example embedded in the generate_presentation and
// validate_input parameter descriptions must parse, and any "pattern" block
// inside one must pass the real pattern validator. This is the guard that
// stopped a hand-written example from drifting away from the schema.
//
// Only literal examples are checked. The descriptions also carry schema
// sketches that are deliberately not valid JSON — `{"data":{...}}` shows the
// field exists without pinning its contents, `{color,alpha}` names a shape's
// keys — so any candidate containing the documented "..." ellipsis or a
// bare-word key list is skipped.
func TestToolDescriptionJSONExamplesAreValid(t *testing.T) {
	descriptions := map[string]string{
		"generate_presentation": toolPresentationDescription(t, mcpGenerateTool()),
		"validate_input":        toolPresentationDescription(t, mcpValidateTool()),
	}

	for tool, desc := range descriptions {
		t.Run(tool, func(t *testing.T) {
			examples := literalJSONExamples(desc)
			if len(examples) == 0 {
				t.Fatalf("no complete JSON examples found in the %s presentation description", tool)
			}
			for i, ex := range examples {
				var parsed map[string]any
				if err := json.Unmarshal([]byte(ex), &parsed); err != nil {
					// Fragments like `{"pattern":{...}}` are whole objects; a
					// parse failure is a real defect in the documentation.
					t.Errorf("example %d does not parse: %v\n%s", i, err, ex)
					continue
				}
				raw, ok := parsed["pattern"]
				if !ok {
					continue
				}
				encoded, err := json.Marshal(raw)
				if err != nil {
					t.Errorf("example %d: cannot re-encode pattern: %v", i, err)
					continue
				}
				var pi PatternInput
				if err := json.Unmarshal(encoded, &pi); err != nil {
					t.Errorf("example %d: pattern block is not a PatternInput: %v\n%s", i, err, ex)
					continue
				}
				ctx := patterns.ExpandContext{
					SlideWidth:  validationDefaultSlideWidthEMU,
					SlideHeight: validationDefaultSlideHeightEMU,
				}
				if _, _, err := expandPattern(&pi, ctx, patterns.Default()); err != nil {
					t.Errorf("example %d in %s would be REFUSED by generate: %v\n%s", i, tool, err, ex)
				}
			}
		})
	}
}

// toolPresentationDescription digs the `presentation` parameter description out
// of a tool definition.
func toolPresentationDescription(t *testing.T, tool interface{ GetName() string }) string {
	t.Helper()
	encoded, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal tool %s: %v", tool.GetName(), err)
	}
	var shape struct {
		InputSchema struct {
			Properties struct {
				Presentation struct {
					Description string `json:"description"`
				} `json:"presentation"`
			} `json:"properties"`
		} `json:"inputSchema"`
	}
	if err := json.Unmarshal(encoded, &shape); err != nil {
		t.Fatalf("unmarshal tool %s: %v", tool.GetName(), err)
	}
	desc := shape.InputSchema.Properties.Presentation.Description
	if desc == "" {
		t.Fatalf("tool %s has no presentation parameter description", tool.GetName())
	}
	return desc
}

// literalJSONExamples returns the balanced top-level objects in a description
// that are meant to be literal, copy-pasteable JSON. Schema sketches — those
// carrying the "..." ellipsis or a bare-word key list like {color,alpha} — are
// intentional prose, not examples, and are excluded.
func literalJSONExamples(text string) []string {
	var out []string
	for _, candidate := range extractJSONObjects(text) {
		if strings.Contains(candidate, "...") {
			continue
		}
		if !strings.Contains(candidate, `"`) {
			continue // bare-word key list, e.g. {color,alpha}
		}
		out = append(out, candidate)
	}
	return out
}

// extractJSONObjects pulls every balanced top-level {...} run out of a prose
// description, skipping braces inside JSON strings.
func extractJSONObjects(text string) []string {
	var out []string
	depth, start := 0, 0
	inString, escaped := false, false

	for i, r := range text {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString:
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// brace inside a string literal is not structure
		case r == '{':
			if depth == 0 {
				start = i
			}
			depth++
		case r == '}':
			if depth > 0 {
				depth--
				if depth == 0 {
					out = append(out, text[start:i+1])
				}
			}
		}
	}
	return out
}
