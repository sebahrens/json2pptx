package safeyaml

import (
	"errors"
	"strings"
	"testing"
)

func TestUnmarshal_ValidYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "simple map",
			yaml: `
type: bar
title: Test Chart
data:
  Q1: 100
  Q2: 200`,
			wantErr: false,
		},
		{
			name: "nested structure",
			yaml: `
type: timeline
data:
  events:
    - date: '2024-01'
      title: Phase 1
    - date: '2024-06'
      title: Phase 2`,
			wantErr: false,
		},
		{
			name:    "empty yaml",
			yaml:    "",
			wantErr: true, // Empty YAML returns EOF error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := UnmarshalString(tt.yaml, &result)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnmarshal_SizeLimit(t *testing.T) {
	// Create YAML larger than default limit (64KB)
	largeYAML := "data: " + strings.Repeat("x", DefaultMaxSize+100)

	var result map[string]interface{}
	err := UnmarshalString(largeYAML, &result)

	if err == nil {
		t.Fatal("expected error for oversized YAML, got nil")
	}

	if !errors.Is(err, ErrYAMLTooLarge) {
		t.Errorf("expected ErrYAMLTooLarge, got %v", err)
	}
}

func TestUnmarshal_DepthLimit(t *testing.T) {
	// Create deeply nested YAML exceeding default depth (20)
	// Use proper nested map structure: level0: {level1: {level2: ...}}
	var deepYAML strings.Builder
	for i := 0; i <= DefaultMaxDepth+5; i++ {
		deepYAML.WriteString(strings.Repeat("  ", i))
		deepYAML.WriteString("level:\n")
	}
	// Add a terminal value
	deepYAML.WriteString(strings.Repeat("  ", DefaultMaxDepth+6))
	deepYAML.WriteString("value: end\n")

	var result map[string]interface{}
	err := UnmarshalString(deepYAML.String(), &result)

	if err == nil {
		t.Fatal("expected error for deeply nested YAML, got nil")
	}

	if !errors.Is(err, ErrYAMLTooDeep) {
		t.Errorf("expected ErrYAMLTooDeep, got %v", err)
	}
}

func TestUnmarshal_AliasLimit(t *testing.T) {
	// Create YAML with excessive aliases (billion laughs pattern)
	var aliasYAML strings.Builder
	aliasYAML.WriteString("base: &base value\n")
	aliasYAML.WriteString("items:\n")
	for i := 0; i < DefaultMaxAliases+10; i++ {
		aliasYAML.WriteString("  - *base\n")
	}

	var result map[string]interface{}
	err := UnmarshalString(aliasYAML.String(), &result)

	if err == nil {
		t.Fatal("expected error for excessive aliases, got nil")
	}

	if !errors.Is(err, ErrYAMLTooManyAliases) {
		t.Errorf("expected ErrYAMLTooManyAliases, got %v", err)
	}
}

func TestUnmarshalWithLimits_CustomLimits(t *testing.T) {
	yaml := "data: " + strings.Repeat("x", 500)

	tests := []struct {
		name    string
		limits  Limits
		wantErr error
	}{
		{
			name:    "accepts within custom size limit",
			limits:  Limits{MaxSize: 1000, MaxDepth: 20, MaxAliases: 50},
			wantErr: nil,
		},
		{
			name:    "rejects over custom size limit",
			limits:  Limits{MaxSize: 100, MaxDepth: 20, MaxAliases: 50},
			wantErr: ErrYAMLTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := UnmarshalWithLimits([]byte(yaml), &result, tt.limits)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestUnmarshal_InvalidYAML(t *testing.T) {
	// Tab characters mixed with spaces cause YAML parse errors
	invalidYAML := "type: bar\n\t  invalid: indentation"

	var result map[string]interface{}
	err := UnmarshalString(invalidYAML, &result)

	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}

	// Should be a parse error, not one of our limit errors
	if errors.Is(err, ErrYAMLTooLarge) || errors.Is(err, ErrYAMLTooDeep) || errors.Is(err, ErrYAMLTooManyAliases) {
		t.Errorf("expected parse error, got limit error: %v", err)
	}
}

func TestDefaultLimits(t *testing.T) {
	limits := DefaultLimits()

	if limits.MaxSize != DefaultMaxSize {
		t.Errorf("MaxSize = %d, want %d", limits.MaxSize, DefaultMaxSize)
	}
	if limits.MaxDepth != DefaultMaxDepth {
		t.Errorf("MaxDepth = %d, want %d", limits.MaxDepth, DefaultMaxDepth)
	}
	if limits.MaxAliases != DefaultMaxAliases {
		t.Errorf("MaxAliases = %d, want %d", limits.MaxAliases, DefaultMaxAliases)
	}
}

// TestBillionLaughsProtection verifies protection against the classic YAML bomb attack.
func TestBillionLaughsProtection(t *testing.T) {
	// This is a simplified billion laughs attack pattern
	billionLaughs := `
a: &a ["lol","lol","lol","lol","lol","lol","lol","lol","lol"]
b: &b [*a,*a,*a,*a,*a,*a,*a,*a,*a]
c: &c [*b,*b,*b,*b,*b,*b,*b,*b,*b]
d: &d [*c,*c,*c,*c,*c,*c,*c,*c,*c]
e: &e [*d,*d,*d,*d,*d,*d,*d,*d,*d]
f: &f [*e,*e,*e,*e,*e,*e,*e,*e,*e]
g: &g [*f,*f,*f,*f,*f,*f,*f,*f,*f]
h: &h [*g,*g,*g,*g,*g,*g,*g,*g,*g]
i: &i [*h,*h,*h,*h,*h,*h,*h,*h,*h]
`

	var result map[string]interface{}
	err := UnmarshalString(billionLaughs, &result)

	if err == nil {
		t.Fatal("expected error for billion laughs attack, got nil")
	}

	// Should be rejected due to excessive aliases
	if !errors.Is(err, ErrYAMLTooManyAliases) {
		t.Errorf("expected ErrYAMLTooManyAliases, got %v", err)
	}
}

func TestExtractMapKeyOrder(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		field    string
		wantKeys []string
	}{
		{
			name: "preserves source order",
			yaml: `type: bar
data:
  North: 250
  South: 180
  East: 220
  West: 195`,
			field:    "data",
			wantKeys: []string{"North", "South", "East", "West"},
		},
		{
			name: "reverse alphabetical order",
			yaml: `type: pie
data:
  Zebra: 10
  Apple: 20
  Mango: 30`,
			field:    "data",
			wantKeys: []string{"Zebra", "Apple", "Mango"},
		},
		{
			name: "field not found",
			yaml: `type: bar
values:
  A: 1`,
			field:    "data",
			wantKeys: nil,
		},
		{
			name: "field is not a map",
			yaml: `type: bar
data:
  - label: A
    value: 1`,
			field:    "data",
			wantKeys: nil,
		},
		{
			name:     "invalid yaml",
			yaml:     "{{invalid",
			field:    "data",
			wantKeys: nil,
		},
		{
			name:     "empty yaml",
			yaml:     "",
			field:    "data",
			wantKeys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractMapKeyOrder(tt.yaml, tt.field)
			if tt.wantKeys == nil {
				if got != nil {
					t.Errorf("ExtractMapKeyOrder() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.wantKeys) {
				t.Fatalf("ExtractMapKeyOrder() returned %d keys, want %d: %v", len(got), len(tt.wantKeys), got)
			}
			for i, k := range tt.wantKeys {
				if got[i] != k {
					t.Errorf("key[%d] = %q, want %q", i, got[i], k)
				}
			}
		})
	}
}

// TestRealisticChartYAML ensures normal chart YAML passes validation.
func TestRealisticChartYAML(t *testing.T) {
	chartYAML := `
type: bar
title: Quarterly Revenue
width: 800
height: 600
data:
  Q1 2024: 125000
  Q2 2024: 150000
  Q3 2024: 175000
  Q4 2024: 200000
options:
  colors:
    - "#FF6384"
    - "#36A2EB"
    - "#FFCE56"
    - "#4BC0C0"
  legend:
    position: top
    display: true
  animation:
    duration: 1000
`

	var result map[string]interface{}
	err := UnmarshalString(chartYAML, &result)

	if err != nil {
		t.Errorf("realistic chart YAML should pass, got error: %v", err)
	}

	// Verify structure
	if result["type"] != "bar" {
		t.Errorf("type = %v, want bar", result["type"])
	}
	if result["title"] != "Quarterly Revenue" {
		t.Errorf("title = %v, want Quarterly Revenue", result["title"])
	}
}

// TestRealisticDiagramYAML ensures normal diagram YAML passes validation.
func TestRealisticDiagramYAML(t *testing.T) {
	diagramYAML := `
type: timeline
title: Project Milestones
data:
  events:
    - date: "2024-01-15"
      title: "Project Kickoff"
      description: "Initial team meeting and scope definition"
    - date: "2024-03-01"
      title: "Phase 1 Complete"
      description: "Core features implemented"
    - date: "2024-06-01"
      title: "Beta Release"
      description: "External testing begins"
    - date: "2024-09-01"
      title: "GA Release"
      description: "General availability"
`

	var result map[string]interface{}
	err := UnmarshalString(diagramYAML, &result)

	if err != nil {
		t.Errorf("realistic diagram YAML should pass, got error: %v", err)
	}

	// Verify structure
	if result["type"] != "timeline" {
		t.Errorf("type = %v, want timeline", result["type"])
	}
}
