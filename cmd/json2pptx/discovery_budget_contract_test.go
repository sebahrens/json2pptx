package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAllProfileAbridgesOnlyProseAndKeepsLiveDetails(t *testing.T) {
	withToolProfile(t, toolProfileAll)
	wire, tools := listToolsOverWire(t, newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileAll))
	var envelope struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(wire, &envelope); err != nil {
		t.Fatal(err)
	}
	schemas := map[string]json.RawMessage{}
	for _, entry := range envelope.Result.Tools {
		schemas[entry.Name] = entry.InputSchema
	}
	mc := refusalTestConfig(t)
	abridged := 0
	for _, tool := range tools {
		ctor, ok := toolConstructors()[tool.Name]
		if !ok {
			t.Fatalf("no constructor %s", tool.Name)
		}
		full := ctor()
		raw := full.RawInputSchema
		if len(raw) == 0 {
			var err error
			raw, err = json.Marshal(full.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
		}
		listed := schemas[tool.Name]
		var want, got any
		if json.Unmarshal(raw, &want) != nil || json.Unmarshal(listed, &got) != nil {
			t.Fatal("invalid input schema")
		}
		if !reflect.DeepEqual(got, want) {

			t.Errorf("%s input contract changed in listing", tool.Name)
		}
		if tool.Description != full.Description {
			abridged++
			if !strings.Contains(tool.Description, `get_started tool:"`+tool.Name+`"`) {
				t.Errorf("%s lacks full documentation call", tool.Name)
			}
			var response struct {
				Detail struct {
					Description string `json:"description"`
				} `json:"tool_detail"`
			}
			structuredInto(t, mustCall(t, mc.handleGetStarted, map[string]any{"tool": tool.Name}).StructuredContent, &response)
			if response.Detail.Description != full.Description {
				t.Errorf("%s live documentation was abridged", tool.Name)
			}
		}
	}
	if abridged == 0 {
		t.Fatal("no long descriptions exercised")
	}
}
