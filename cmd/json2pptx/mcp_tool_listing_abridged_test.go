package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// schemaSkeleton returns a schema without its descriptions: the part a
// client validates a call against.
func schemaSkeleton(v any) any {
	switch node := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(node))
		for k, child := range node {
			if k == "description" {
				if _, isText := child.(string); isText {
					continue
				}
			}
			out[k] = schemaSkeleton(child)
		}
		return out
	case []any:
		out := make([]any, len(node))
		for i, child := range node {
			out[i] = schemaSkeleton(child)
		}
		return out
	}
	return v
}

// TestAbridgedListingKeepsEveryArgument is the go-slide-creator-mvdt5 guard
// on the default profile's abridged tools/list: it may shorten prose, and
// nothing else. Every argument, type, enum, default, required list and oneOf
// of the full listing is still there (validate_deck_spec's meta outline is
// the one stated exception), every abridged wording names an argument that
// exists, and get_started tool:"<name>" returns the full text.
func TestAbridgedListingKeepsEveryArgument(t *testing.T) {
	withToolProfile(t, toolProfileDeckSpec)
	withRenderStatus(t, true, nil)
	raw, _ := listToolsOverWire(t, newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileDeckSpec))
	var wire struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	mc := refusalTestConfig(t)
	for _, tool := range wire.Result.Tools {
		listed[tool.Name] = true
		res := mustCall(t, mc.handleGetStarted, map[string]any{"tool": tool.Name})
		checkDefaultProfileResult(t, "get_started(tool:"+tool.Name+")", res)
		var body struct {
			Detail struct {
				Tool        string         `json:"tool"`
				Description string         `json:"description"`
				InputSchema map[string]any `json:"input_schema"`
				Listed      bool           `json:"listed"`
			} `json:"tool_detail"`
		}
		structuredInto(t, res.StructuredContent, &body)
		full := body.Detail
		if full.Tool != tool.Name || !full.Listed || full.Description == "" {
			t.Errorf("get_started tool:%q = %+v", tool.Name, full)
			continue
		}
		if tool.Description == "" || len(tool.Description) > len(full.Description) {
			t.Errorf("%s: the listed description (%d bytes) is not an abridgement of the full one (%d bytes)", tool.Name, len(tool.Description), len(full.Description))
		}
		entry := abridgedListings[tool.Name]
		for path := range entry.Args {
			if schemaArgument(full.InputSchema, path) == nil {
				t.Errorf("%s: abridged wording for %q, an argument the tool does not have", tool.Name, path)
			}
		}
		want := schemaSkeleton(full.InputSchema).(map[string]any)
		if tool.Name == "validate_deck_spec" {
			spec := schemaArgument(want, "spec")
			if _, ok := spec["properties"]; !ok {
				t.Error("validate_deck_spec: the full listing lost the DeckSpec outline")
			}
			delete(spec, "properties")
		}
		if got := schemaSkeleton(tool.InputSchema); !reflect.DeepEqual(got, want) {
			g, _ := json.Marshal(got)
			w, _ := json.Marshal(want)
			t.Errorf("%s: the abridged input schema differs from the full one beyond descriptions\n got: %s\nwant: %s", tool.Name, g, w)
		}
		// A top-level argument keeps a description unless the full listing
		// had none: its name alone is not a contract.
		props, _ := tool.InputSchema["properties"].(map[string]any)
		fullProps, _ := full.InputSchema["properties"].(map[string]any)
		for name, p := range props {
			_, had := fullProps[name].(map[string]any)["description"]
			if _, has := p.(map[string]any)["description"]; had && !has {
				t.Errorf("%s.%s lost its description", tool.Name, name)
			}
		}
	}
	for name := range abridgedListings {
		if !listed[name] {
			t.Errorf("abridgedListings names %q, which the default profile does not list", name)
		}
	}

	// The outline validate_deck_spec no longer lists is one call away.
	var outline struct {
		Detail struct {
			InputSchema map[string]any `json:"input_schema"`
		} `json:"tool_detail"`
	}
	structuredInto(t, mustCall(t, mc.handleGetStarted, map[string]any{"tool": "validate_deck_spec"}).StructuredContent, &outline)
	meta, _ := schemaArgument(outline.Detail.InputSchema, "spec/meta")["properties"].(map[string]any)
	for _, field := range []string{"title", "template", "chrome", "waivers"} {
		if _, ok := meta[field]; !ok {
			t.Errorf("get_started tool:validate_deck_spec does not document meta.%s", field)
		}
	}

	// A hidden tool is described and marked; an unknown one is refused with
	// the names that are listed.
	res := mustCall(t, mc.handleGetStarted, map[string]any{"tool": "generate_presentation"})
	if text := resultText(res); !strings.Contains(text, `"listed":false`) || !strings.Contains(text, "hidden_tools") {
		t.Errorf("a hidden tool's detail is not marked: %.300s", text)
	}
	bad, err := mc.handleGetStarted(t.Context(), makeRequest(map[string]any{"tool": "no_such_tool"}))
	if err != nil || !bad.IsError || !strings.Contains(resultText(bad), "render_deck_spec") {
		t.Errorf("an unknown tool name was not refused with the listed names: %v %s", err, resultText(bad))
	}

	// The other profiles list the registered tools untouched.
	withToolProfile(t, toolProfileCore)
	_, core := listToolsOverWire(t, newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileCore))
	for _, tool := range core {
		if tool.Name == "plan_deck" && tool.Description != mcpPlanDeckTool().Description {
			t.Error("the core profile lists an abridged plan_deck")
		}
	}
}
