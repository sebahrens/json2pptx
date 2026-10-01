package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
)

// bodyAndLeadDeck is a deck whose content slide uses the documented
// body_and_lead type. generate always accepted it; validate and validate_input
// hard-rejected it as an unknown type, so an agent that validated first
// abandoned the lead/evidence hierarchy (go-slide-creator-5ld72).
const bodyAndLeadDeck = `{
  "template": "midnight-blue",
  "slides": [
    {
      "layout_id": "content",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Onboarding drives SMB churn, so fixing it protects 112% retention"},
        {"placeholder_id": "body", "type": "body_and_lead", "body_and_lead_value": {
          "lead": "Slow onboarding is the cheapest churn driver to fix.",
          "bullets": ["Time-to-first-value is 31 days for SMB", "Churned accounts used 40% fewer features"]%s
        }}
      ]
    }
  ]
}`

func TestValidateAcceptsBodyAndLead(t *testing.T) {
	deck := strings.Replace(bodyAndLeadDeck, "%s", "", 1)

	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": mustParseJSON(deck)}))
	if err != nil || result == nil {
		t.Fatalf("validate_input failed: %v %v", err, result)
	}
	text := textContent(result)
	if result.IsError || strings.Contains(text, `unknown type`) {
		t.Fatalf("validate_input rejected body_and_lead: %s", text)
	}

	path := filepath.Join(t.TempDir(), "lead.json")
	if err := os.WriteFile(path, []byte(deck), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runJSONDryRun(path, "", "../../templates", "", "", false); err != nil {
		t.Fatalf("json2pptx validate rejected body_and_lead: %v", err)
	}
}

func TestValidateBodyAndLeadUnknownKeys(t *testing.T) {
	deck := strings.Replace(bodyAndLeadDeck, "%s", `, "trailing": "x"`, 1)
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": mustParseJSON(deck)}))
	if err != nil || result == nil {
		t.Fatalf("validate_input failed: %v %v", err, result)
	}
	if text := textContent(result); !strings.Contains(text, "body_and_lead_value/trailing") {
		t.Fatalf("unknown key inside body_and_lead_value not reported: %s", text)
	}
}

func TestUnknownContentTypeMessageListsBodyAndLead(t *testing.T) {
	deck := strings.Replace(strings.Replace(bodyAndLeadDeck, "%s", "", 1), `"type": "body_and_lead"`, `"type": "body_and_leed"`, 1)
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": mustParseJSON(deck)}))
	if err != nil || result == nil {
		t.Fatalf("validate_input failed: %v %v", err, result)
	}
	if text := textContent(result); !strings.Contains(text, "body_and_lead, bullet_groups") {
		t.Fatalf("unknown-type message does not list body_and_lead: %s", text)
	}
}
