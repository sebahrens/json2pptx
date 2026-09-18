package main

import (
	"strings"
	"testing"
)

func TestDefaultTemplatesExist(t *testing.T) {
	tmpls, err := parseTemplates(defaultTemplates, "../../templates")
	if err != nil {
		t.Fatalf("default benchmark templates must exist: %v", err)
	}
	if len(tmpls) != 3 || !tmpls[2].HeldOut || tmpls[0].HeldOut {
		t.Errorf("unexpected templates: %+v", tmpls)
	}
}

func TestParseTemplatesRejectsMissingAndMalformed(t *testing.T) {
	if _, err := parseTemplates("clean-white:editorial", "../../templates"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing template must be rejected, got %v", err)
	}
	if _, err := parseTemplates("midnight-blue", "../../templates"); err == nil {
		t.Error("entry without family must be rejected")
	}
}

func TestLoadBriefsHasTwelve(t *testing.T) {
	b, err := loadBriefs("../../tests/quality/agent_briefs.json")
	if err != nil || len(b) < 12 {
		t.Fatalf("briefs: %d %v", len(b), err)
	}
}
