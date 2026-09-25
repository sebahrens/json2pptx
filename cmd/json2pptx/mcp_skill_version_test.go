package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestGenerateDeckSkillSchemaVersionMatchesServer(t *testing.T) {
	data, err := os.ReadFile("../../skills/generate-deck/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(data), "---", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[0]) != "" {
		t.Fatal("generate-deck skill has no frontmatter")
	}
	const prefix = "schema_version: "
	for _, line := range strings.Split(parts[1], "\n") {
		if strings.HasPrefix(line, prefix) {
			if got := strings.TrimSpace(strings.TrimPrefix(line, prefix)); got != SchemaVersion {
				t.Fatalf("skill schema_version = %q, server = %q", got, SchemaVersion)
			}
			return
		}
	}
	t.Fatal("generate-deck skill frontmatter has no schema_version")
}

func TestSkillVersionComparison(t *testing.T) {
	for _, tt := range []struct {
		installed, current string
		want               int
		bad                bool
	}{
		{"4.9.0", "4.10.0", -1, false},
		{"4.131.0", "4.131.0", 0, false},
		{"5.0.0", "4.131.0", 1, false},
		{"4.131", "4.131.0", 0, true},
		{"4.131.x", "4.131.0", 0, true},
	} {
		got, err := compareSkillSchemaVersions(tt.installed, tt.current)
		if (err != nil) != tt.bad || (!tt.bad && got != tt.want) {
			t.Errorf("compareSkillSchemaVersions(%q,%q) = %d, %v; want %d, bad=%v", tt.installed, tt.current, got, err, tt.want, tt.bad)
		}
	}
}

func TestGetStartedSkillVersionWarning(t *testing.T) {
	s := strictArgsTestServer(t)
	for _, tt := range []struct {
		name, version string
		warn, fail    bool
	}{
		{"stale", "4.130.0", true, false},
		{"current", SchemaVersion, false, false},
		{"newer", "5.0.0", false, false},
		{"malformed", "4.x", false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := callToolViaServer(t, s, "get_started", map[string]any{"skill_version": tt.version})
			if res.IsError != tt.fail {
				t.Fatalf("isError=%v, want %v: %s", res.IsError, tt.fail, resultText(res))
			}
			if tt.fail {
				if !strings.Contains(resultText(res), "skill_version") {
					t.Fatalf("invalid version error has no field path: %s", resultText(res))
				}
				return
			}
			var payload getStartedResponse
			encoded, err := json.Marshal(res.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.SkillSchemaVersion != SchemaVersion {
				t.Errorf("skill_schema_version = %q", payload.SkillSchemaVersion)
			}
			if (payload.SkillWarning != "") != tt.warn {
				t.Errorf("skill_warning = %q, want warning=%v", payload.SkillWarning, tt.warn)
			}
			if tt.warn && !strings.Contains(payload.SkillWarning, "run make install-skill") {
				t.Errorf("warning has no repair action: %q", payload.SkillWarning)
			}
		})
	}
}

func TestProjectedDiscoveryOutputSchemas(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  []byte
		want []string
		not  []string
	}{
		{"get_started", outputSchemaGetStarted, []string{"task", "skill_schema_version", "runtime"}, []string{"quality_workflow"}},
		{"get_capabilities", outputSchemaGetCapabilities, []string{"schema_version", "skill_schema_version", "tool_version", "sections_included"}, []string{"registry", "features", "runtime", "tool_list"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var schema struct {
				Properties map[string]any `json:"properties"`
				Required   []string       `json:"required"`
			}
			if err := json.Unmarshal(tt.raw, &schema); err != nil {
				t.Fatal(err)
			}
			required := map[string]bool{}
			for _, name := range schema.Required {
				required[name] = true
			}
			for _, name := range tt.want {
				if !required[name] || schema.Properties[name] == nil {
					t.Errorf("%s must be declared and required", name)
				}
			}
			for _, name := range tt.not {
				if required[name] {
					t.Errorf("%s is omitted by some valid projections", name)
				}
			}
		})
	}
}

func TestDiscoveryDefaultResponsesMatchProjectionSchemas(t *testing.T) {
	s := strictArgsTestServer(t)
	for _, tt := range []struct {
		tool   string
		args   map[string]any
		absent []string
	}{
		{"get_started", map[string]any{}, []string{"quality_workflow", "skill_warning"}},
		{"get_capabilities", map[string]any{"sections": []any{"runtime"}}, []string{"tool_list", "registry", "features"}},
	} {
		t.Run(tt.tool, func(t *testing.T) {
			res := callToolViaServer(t, s, tt.tool, tt.args)
			if res.IsError {
				t.Fatal(resultText(res))
			}
			wire, err := json.Marshal(res.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err := json.Unmarshal(wire, &payload); err != nil {
				t.Fatal(err)
			}
			if payload["skill_schema_version"] != SchemaVersion {
				t.Errorf("skill_schema_version = %v", payload["skill_schema_version"])
			}
			for _, name := range tt.absent {
				if _, ok := payload[name]; ok {
					t.Errorf("%s should be omitted in this projection", name)
				}
			}
			if tt.tool == "get_started" && payload["runtime"] == nil {
				t.Error("get_started omitted runtime")
			}
		})
	}
}
