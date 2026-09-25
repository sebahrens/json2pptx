package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// A modern MCP client receives structuredContent for the complete result and
// this small text synopsis for hosts that still display only content blocks.
const maxMCPTextSummaryBytes = 1024

func mcpResponseText(ctx context.Context, data any) ([]byte, error) {
	if includeTextFallback(ctx) {
		return MarshalMCPResponse(ctx, data)
	}
	return compactMCPTextSummary(data)
}

func compactMCPTextSummary(data any) ([]byte, error) {
	full, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if len(full) > 0 && full[0] == '{' {
		if err := json.Unmarshal(full, &object); err != nil {
			return nil, err
		}
	}
	if object == nil {
		var array []json.RawMessage
		if json.Unmarshal(full, &array) == nil && array != nil {
			return json.Marshal(map[string]any{"summary": fmt.Sprintf("%d item(s) in structuredContent", len(array))})
		}
		return json.Marshal(map[string]any{"summary": "Result in structuredContent"})
	}

	summary := make(map[string]any)
	addSummaryScalars(summary, object)
	addSummaryFindings(summary, object)
	addSummaryWorkflow(summary, object)
	addSummaryCatalogs(summary, object)
	if len(summary) == 0 {
		addGenericSummary(summary, object)
	}
	return fitMCPTextSummary(summary, object)
}

func addSummaryScalars(summary map[string]any, object map[string]json.RawMessage) {
	for _, key := range []string{"ok", "success", "valid", "publishable", "deterministic_ready", "error", "summary", "message", "pptx_path", "path", "output_filename", "deck_id", "task"} {
		if raw, ok := object[key]; ok {
			limit := 120
			if key == "pptx_path" || key == "path" || key == "output_filename" {
				limit = 320
			}
			if value, ok := summaryScalarLimit(raw, limit); ok {
				summary[key] = value
			}
		}
	}
}

func addSummaryFindings(summary map[string]any, object map[string]json.RawMessage) {
	if raw, ok := object["next_tool_call"]; ok {
		if call := summaryNextToolCall(raw); call != nil {
			summary["next_tool_call"] = call
		}
	}
	for _, key := range []string{"findings", "diagnostics"} {
		if raw, ok := object[key]; ok {
			if count, first := summaryFindings(raw); count >= 0 {
				summary[key+"_count"] = count
				if len(first) > 0 {
					summary[key] = first
				}
			}
		}
	}
}

func addSummaryWorkflow(summary map[string]any, object map[string]json.RawMessage) {
	if raw, ok := object["sequence"]; ok {
		if tools := summaryToolSequence(raw); len(tools) > 0 {
			summary["next_tools"] = tools
		}
	}
	if raw, ok := object["fast_path"]; ok {
		var path map[string]json.RawMessage
		if json.Unmarshal(raw, &path) == nil {
			if tool, ok := summaryScalar(path["tool"]); ok {
				summary["fast_path_tool"] = tool
			}
			if tools := summaryToolSequence(path["steps"]); len(tools) > 0 {
				summary["next_tools"] = tools
			}
		}
	}
}

func addSummaryCatalogs(summary map[string]any, object map[string]json.RawMessage) {
	for _, key := range []string{"slide_kinds", "templates", "patterns"} {
		if raw, ok := object[key]; ok {
			if count, names := summaryCatalogNames(raw); count >= 0 {
				summary[key+"_count"] = count
				if len(names) > 0 {
					summary[key+"_names"] = names
				}
			}
		}
	}
}

func addGenericSummary(summary map[string]any, object map[string]json.RawMessage) {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 4 {
		keys = keys[:4]
	}
	for _, key := range keys {
		if value, ok := summaryScalar(object[key]); ok {
			summary[key] = value
		}
	}
	if len(summary) == 0 {
		summary["fields"] = keys
	}
}

func fitMCPTextSummary(summary map[string]any, object map[string]json.RawMessage) ([]byte, error) {
	encoded, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}
	// If a result contains every optional field, preserve its status and the
	// first actionable finding before dropping lower-priority detail.
	for len(encoded) > maxMCPTextSummaryBytes {
		if findings, ok := summary["findings"].([]map[string]any); ok && len(findings) > 1 {
			summary["findings"] = findings[:len(findings)-1]
		} else if findings, ok := summary["diagnostics"].([]map[string]any); ok && len(findings) > 1 {
			summary["diagnostics"] = findings[:len(findings)-1]
		} else if _, ok := summary["next_tools"]; ok {
			delete(summary, "next_tools")
		} else if _, ok := summary["message"]; ok {
			delete(summary, "message")
		} else if _, ok := summary["deck_id"]; ok {
			delete(summary, "deck_id")
		} else if names, ok := summary["slide_kinds_names"].([]string); ok && len(names) > 3 {
			summary["slide_kinds_names"] = names[:len(names)-1]
		} else if names, ok := summary["templates_names"].([]string); ok && len(names) > 3 {
			summary["templates_names"] = names[:len(names)-1]
		} else if names, ok := summary["patterns_names"].([]string); ok && len(names) > 3 {
			summary["patterns_names"] = names[:len(names)-1]
		} else {
			return minimalMCPTextSummary(object, summary)
		}
		encoded, err = json.Marshal(summary)
		if err != nil {
			return nil, err
		}
	}
	return encoded, nil
}

// The last-resort synopsis still preserves status and a usable recovery clue.
// It is deliberately built from a fixed small set of bounded fields, so the
// one-kilobyte cap cannot erase an error into a generic "see structured" line.
func minimalMCPTextSummary(object map[string]json.RawMessage, summary map[string]any) ([]byte, error) {
	minimal := make(map[string]any)
	for _, key := range []string{"ok", "success", "valid", "publishable"} {
		if value, ok := summary[key]; ok {
			minimal[key] = value
		}
	}
	for _, key := range []string{"error", "summary"} {
		if value, ok := summaryScalarLimit(object[key], 160); ok {
			minimal[key] = value
			break
		}
	}
	for _, key := range []string{"pptx_path", "path", "output_filename"} {
		if value, ok := summaryScalarLimit(object[key], 320); ok {
			minimal[key] = value
			break
		}
	}
	for _, key := range []string{"findings", "diagnostics"} {
		if findings, ok := summary[key].([]map[string]any); ok && len(findings) > 0 {
			first := make(map[string]any)
			for _, field := range []string{"code", "message"} {
				if value, ok := findings[0][field]; ok {
					first[field] = value
				}
			}
			minimal["first_finding"] = first
			break
		}
	}
	if call, ok := summary["next_tool_call"].(map[string]any); ok {
		minimal["next_tool_call"] = map[string]any{"tool": call["tool"]}
	}
	if len(minimal) == 0 {
		minimal["summary"] = "Result in structuredContent"
	}
	encoded, err := json.Marshal(minimal)
	if err != nil {
		return nil, err
	}
	for len(encoded) > maxMCPTextSummaryBytes {
		if first, ok := minimal["first_finding"].(map[string]any); ok {
			if _, exists := first["message"]; exists {
				delete(first, "message")
			} else {
				delete(minimal, "first_finding")
			}
		} else if _, ok := minimal["next_tool_call"]; ok {
			delete(minimal, "next_tool_call")
		} else if !halveSummaryString(minimal, "pptx_path") && !halveSummaryString(minimal, "path") && !halveSummaryString(minimal, "output_filename") && !halveSummaryString(minimal, "error") && !halveSummaryString(minimal, "summary") {
			break
		}
		encoded, err = json.Marshal(minimal)
		if err != nil {
			return nil, err
		}
	}
	return encoded, nil
}

func halveSummaryString(summary map[string]any, key string) bool {
	value, ok := summary[key].(string)
	if !ok {
		return false
	}
	if len(value) <= 16 {
		delete(summary, key)
	} else {
		summary[key] = summaryString(value, len(value)/2)
	}
	return true
}

func summaryScalar(raw json.RawMessage) (any, bool) {
	return summaryScalarLimit(raw, 120)
}

func summaryScalarLimit(raw json.RawMessage, maxBytes int) (any, bool) {
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return nil, false
	}
	switch v := value.(type) {
	case string:
		return summaryString(v, maxBytes), true
	case bool, float64:
		return v, true
	default:
		return nil, false
	}
}

func summaryString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	var truncated strings.Builder
	for _, r := range s {
		if truncated.Len()+utf8.RuneLen(r)+len("…") > maxBytes {
			break
		}
		truncated.WriteRune(r)
	}
	return truncated.String() + "…"
}

func summaryNextToolCall(raw json.RawMessage) map[string]any {
	var source map[string]json.RawMessage
	if json.Unmarshal(raw, &source) != nil {
		return nil
	}
	tool, ok := summaryScalar(source["tool"])
	if !ok {
		return nil
	}
	call := map[string]any{"tool": tool}
	if args, ok := source["args_template"]; ok && len(args) <= 220 {
		var decoded any
		if json.Unmarshal(args, &decoded) == nil {
			call["args_template"] = decoded
		}
	}
	return call
}

func summaryFindings(raw json.RawMessage) (int, []map[string]any) {
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		var envelope map[string]json.RawMessage
		if json.Unmarshal(raw, &envelope) != nil {
			return -1, nil
		}
		return summaryFindings(envelope["findings"])
	}
	first := make([]map[string]any, 0, 3)
	for _, item := range items {
		if len(first) == 3 {
			break
		}
		var source map[string]json.RawMessage
		if json.Unmarshal(item, &source) != nil {
			continue
		}
		finding := make(map[string]any)
		for _, key := range []string{"code", "message", "path", "severity", "action"} {
			if value, ok := summaryScalar(source[key]); ok {
				finding[key] = value
			}
		}
		if call := summaryNextToolCall(source["next_tool_call"]); call != nil {
			finding["next_tool_call"] = call
		}
		if len(finding) > 0 {
			first = append(first, finding)
		}
	}
	return len(items), first
}

func summaryToolSequence(raw json.RawMessage) []string {
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	tools := make([]string, 0, 4)
	for _, item := range items {
		if len(tools) == 4 {
			break
		}
		if value, ok := summaryScalar(item["tool"]); ok {
			if tool, ok := value.(string); ok {
				tools = append(tools, tool)
			}
		}
	}
	return tools
}

func summaryCatalogNames(raw json.RawMessage) (int, []string) {
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return -1, nil
	}
	names := make([]string, 0, 20)
	for _, item := range items {
		if len(names) == 20 {
			break
		}
		if value, ok := summaryScalar(item); ok {
			if name, ok := value.(string); ok {
				names = append(names, name)
				continue
			}
		}
		var entry map[string]json.RawMessage
		if json.Unmarshal(item, &entry) != nil {
			continue
		}
		for _, key := range []string{"kind", "name", "id"} {
			if value, ok := summaryScalar(entry[key]); ok {
				if name, ok := value.(string); ok {
					names = append(names, name)
					break
				}
			}
		}
	}
	return len(items), names
}
