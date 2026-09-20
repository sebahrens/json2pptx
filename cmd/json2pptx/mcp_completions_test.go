package main

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/svggen"
)

func completeRPC(t *testing.T, s interface {
	HandleMessage(context.Context, json.RawMessage) mcp.JSONRPCMessage
}, ref map[string]string, name, prefix string) mcp.Completion {
	t.Helper()
	result := promptRPC(t, s, "completion/complete", map[string]any{
		"ref": ref, "argument": map[string]string{"name": name, "value": prefix},
	})
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var completed mcp.CompleteResult
	if err := json.Unmarshal(data, &completed); err != nil {
		t.Fatal(err)
	}
	return completed.Completion
}

func TestMCPCompletionTemplates(t *testing.T) {
	s := newMCPServer(&mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()})
	initialized := promptRPC(t, s, "initialize", map[string]any{
		"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "completion-test", "version": "0"},
	})
	data, err := json.Marshal(initialized)
	if err != nil {
		t.Fatal(err)
	}
	var init struct {
		Capabilities struct {
			Completions *struct{} `json:"completions"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(data, &init); err != nil || init.Capabilities.Completions == nil {
		t.Fatalf("initialize did not advertise completions: %s (%v)", data, err)
	}
	ref := map[string]string{"type": "ref/resource", "uri": templatesResourceURI}
	completed := completeRPC(t, s, ref, "template", "")
	want := listAvailableTemplates("../../templates")
	sort.Strings(want)
	if len(want) != 9 || !reflect.DeepEqual(completed.Values, want) || completed.Total != 9 || completed.HasMore {
		t.Fatalf("template completions = %+v, want all nine installed: %v", completed, want)
	}
	filtered := completeRPC(t, s, ref, "template", "MID")
	if !reflect.DeepEqual(filtered.Values, []string{"midnight-blue"}) {
		t.Fatalf("prefix-filtered templates = %+v", filtered)
	}
	prompt := completeRPC(t, s, map[string]string{"type": "ref/prompt", "name": "deck-from-brief"}, "template", "warm")
	if !reflect.DeepEqual(prompt.Values, []string{"warm-coral"}) {
		t.Fatalf("prompt template completion = %+v", prompt)
	}
}

func TestMCPCompletionSharedEnumsAndUnknownRefs(t *testing.T) {
	s := newMCPServer(&mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()})
	patternNames := make([]string, 0)
	for _, pattern := range patterns.Default().List() {
		patternNames = append(patternNames, pattern.Name())
	}
	slideKinds := make([]string, 0)
	for _, kind := range semantic.AllSlideKinds() {
		slideKinds = append(slideKinds, string(kind))
	}
	archetypes := make([]string, 0)
	for _, archetype := range semantic.AllArchetypes() {
		archetypes = append(archetypes, string(archetype))
	}
	for _, test := range []struct {
		uri, name string
		want      []string
	}{
		{patternsResourceURI, "pattern", patternNames},
		{deckSpecResourceURI, "kind", slideKinds},
		{deckSpecResourceURI, "archetype", archetypes},
		{deckSpecResourceURI, "chart_type", svggen.Types()},
		{skillResourceURI, "fix_kind", patterns.AllFixKinds()},
		{skillResourceURI, "finding_code", patterns.AllFitFindingCodes()},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := completeRPC(t, s, map[string]string{"type": "ref/resource", "uri": test.uri}, test.name, "")
			sort.Strings(test.want)
			if !reflect.DeepEqual(got.Values, test.want) || got.Total != len(test.want) {
				t.Fatalf("completion %s/%s = %+v, want %v", test.uri, test.name, got, test.want)
			}
		})
	}
	for _, ref := range []map[string]string{
		{"type": "ref/resource", "uri": "json2pptx://missing"},
		{"type": "ref/prompt", "name": "unknown"},
	} {
		got := completeRPC(t, s, ref, "template", "")
		if len(got.Values) != 0 || got.Total != 0 {
			t.Fatalf("unknown ref produced completions: %+v", got)
		}
	}
	if got := completeRPC(t, s, map[string]string{"type": "ref/resource", "uri": templatesResourceURI}, "pattern", ""); len(got.Values) != 0 {
		t.Fatalf("unknown argument for known resource produced completions: %+v", got)
	}
}

func TestMCPCompletionCapsResultsAtOneHundred(t *testing.T) {
	names := make([]string, 101)
	for i := range names {
		names[i] = "value"
	}
	got := matchCompletion(names, "val")
	if len(got.Values) != 100 || got.Total != 101 || !got.HasMore {
		t.Fatalf("completion cap = %+v", got)
	}
}
