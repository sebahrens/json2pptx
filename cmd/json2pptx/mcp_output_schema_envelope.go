package main

import (
	"encoding/json"
)

// errorEnvelopeSchema describes the diagnostics FindingEnvelope every error
// result carries as structuredContent. It is the shape produced by
// diagnostics.BuildEnvelope.
var errorEnvelopeSchema = json.RawMessage(`{
  "type": "object",
  "description": "Error envelope (isError=true): the shared diagnostics FindingEnvelope. Present instead of the success shape when the call failed.",
  "properties": {
    "schema_version": {"type": "string"},
    "tool":           {"type": "string"},
    "subcommand":     {"type": "string"},
    "ok":             {"type": "boolean", "const": false},
    "summary":        {"type": "string"},
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id":          {"type": "string"},
          "code":        {"type": "string"},
          "severity":    {"type": "string"},
          "category":    {"type": "string"},
          "message":     {"type": "string"},
          "evidence":    {"type": "object"},
          "remediation": {"type": "object"},
          "next_tool_call": {"type": "object"}
        },
        "required": ["code", "message"]
      }
    }
  },
  "required": ["ok", "findings"]
}`)

// withErrorEnvelope wraps a tool's success-shaped output schema so the schema
// also admits the error envelope.
//
// MCP 2025-06-18 requires a tool that declares an outputSchema to return
// structuredContent conforming to it, and clients are allowed to validate. Every
// json2pptx error result carries the diagnostics FindingEnvelope as
// structuredContent, which matches no success schema — so all 11 tested error
// responses were schema-invalid and a validating client rejected them instead of
// showing the (deliberately rich) diagnostics (go-slide-creator-vtqo).
//
// Rather than dropping the structured errors agents already depend on, the schema
// becomes {$defs, anyOf: [success, error_envelope]}. Any "$defs" the success
// schema declares is HOISTED to the wrapper's root so its "#/$defs/..."
// references keep resolving — an anyOf branch is not a schema root.
func withErrorEnvelope(schema json.RawMessage) json.RawMessage {
	var success map[string]json.RawMessage
	if err := json.Unmarshal(schema, &success); err != nil {
		// Not an object schema we can rewrite — leave it untouched rather than
		// shipping something malformed.
		return schema
	}

	defs, hasDefs := success["$defs"]
	if hasDefs {
		delete(success, "$defs")
	}

	successBody, err := json.Marshal(success)
	if err != nil {
		return schema
	}

	wrapper := map[string]json.RawMessage{
		"anyOf": json.RawMessage(`[` + string(successBody) + `,` + string(errorEnvelopeSchema) + `]`),
	}
	if hasDefs {
		wrapper["$defs"] = defs
	}

	out, err := json.Marshal(wrapper)
	if err != nil {
		return schema
	}
	return out
}
