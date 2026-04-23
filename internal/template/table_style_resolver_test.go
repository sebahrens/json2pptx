package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestResolveTableStyleID_Empty(t *testing.T) {
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer reader.Close()

	got := reader.ResolveTableStyleID("")
	if got != types.DefaultTableStyleID {
		t.Errorf("empty → %q, want %q", got, types.DefaultTableStyleID)
	}
}

func TestResolveTableStyleID_ExplicitGUID(t *testing.T) {
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer reader.Close()

	guid := "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"
	got := reader.ResolveTableStyleID(guid)
	if got != guid {
		t.Errorf("explicit GUID → %q, want %q", got, guid)
	}
}

func TestResolveTableStyleID_UnknownGUID(t *testing.T) {
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer reader.Close()

	unknown := "{00000000-0000-0000-0000-000000000000}"
	got := reader.ResolveTableStyleID(unknown)
	if got != unknown {
		t.Errorf("unknown GUID → %q, want passthrough %q", got, unknown)
	}
}

func TestResolveTableStyleID_TemplateDefault(t *testing.T) {
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer reader.Close()

	got := reader.ResolveTableStyleID(TemplateDefaultSentinel)

	// Must resolve to some non-empty GUID
	if got == "" {
		t.Fatal("@template-default resolved to empty string")
	}
	if got == TemplateDefaultSentinel {
		t.Fatal("@template-default was not resolved")
	}
}

func TestResolveTableStyleID_TemplateDefaultAllBundled(t *testing.T) {
	templates := []string{
		"../../templates/modern-template.pptx",
		"../../templates/midnight-blue.pptx",
		"../../templates/forest-green.pptx",
		"../../templates/warm-coral.pptx",
	}

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			reader, err := OpenTemplate(tmpl)
			if err != nil {
				t.Fatalf("OpenTemplate: %v", err)
			}
			defer reader.Close()

			got := reader.ResolveTableStyleID(TemplateDefaultSentinel)
			if got == "" || got == TemplateDefaultSentinel {
				t.Errorf("@template-default → %q, want a GUID", got)
			}
		})
	}
}

func TestResolveTableStyleID_Stable(t *testing.T) {
	// Resolution must be deterministic across 100 calls.
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer reader.Close()

	first := reader.ResolveTableStyleID(TemplateDefaultSentinel)
	for i := 0; i < 100; i++ {
		got := reader.ResolveTableStyleID(TemplateDefaultSentinel)
		if got != first {
			t.Fatalf("iteration %d: got %q, want %q", i, got, first)
		}
	}
}
