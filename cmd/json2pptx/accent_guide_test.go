package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestBundledTemplatesAuthorAccentUsageGuide pins go-slide-creator-u1h9b:
// every embedded template's metadata carries an authored accent_usage_guide
// line for each of accent1–accent6 (plus semantic_accents and data_palette),
// so list_templates never hands agents an empty guide.
func TestBundledTemplatesAuthorAccentUsageGuide(t *testing.T) {
	for _, name := range testutil.AllBuiltinTemplateNames() {
		t.Run(name, func(t *testing.T) {
			reader, err := template.OpenTemplate(filepath.Join(testutil.TemplatesDir(), name+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			md, err := template.ParseMetadata(reader)
			if err != nil || md == nil {
				t.Fatalf("metadata: %v", err)
			}
			for _, a := range guideAccents {
				if strings.TrimSpace(md.AccentUsageGuide[a]) == "" {
					t.Errorf("accent_usage_guide has no line for %s", a)
				}
			}
			if len(md.SemanticAccents) == 0 || len(md.DataPalette) == 0 {
				t.Errorf("semantic_accents / data_palette missing")
			}
		})
	}
}

// TestDerivedAccentUsageGuideForTemplateWithoutMetadata pins the
// bring-your-own fallback: a template with no guide gets one derived from the
// theme's measured contrast, flagged accent_usage_guide_derived.
func TestDerivedAccentUsageGuideForTemplateWithoutMetadata(t *testing.T) {
	colors := []types.ThemeColor{
		{Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk1", RGB: "#000000"}, {Name: "dk2", RGB: "#1F2A44"},
		{Name: "accent1", RGB: "#FD5108"}, {Name: "accent2", RGB: "#1F4E79"}, {Name: "accent3", RGB: "#FFAA72"},
		{Name: "accent4", RGB: "#A1A8B3"}, {Name: "accent5", RGB: "#2E7D32"}, {Name: "accent6", RGB: "#CBD1D6"},
	}
	roles := buildColorRoles(colors)
	if roles.PrimaryFill != "accent2" {
		t.Errorf("primary_fill = %q, want the first white-text-safe accent (accent2), not orange accent1", roles.PrimaryFill)
	}
	if ink := roles.InkOnAccent["accent1"]; ink.Ink == "lt1" || ink.Ratio < 4.5 {
		t.Errorf("ink_on_accent[accent1] = %+v, want a dark ink passing 4.5:1 on orange", ink)
	}
	if ink := roles.InkOnAccent["accent2"]; ink.Ink != "lt1" {
		t.Errorf("ink_on_accent[accent2] = %+v, want lt1 on a dark blue fill", ink)
	}
	guide := deriveAccentUsageGuide(colors, roles)
	for _, a := range guideAccents {
		if !strings.HasPrefix(guide[a], a+" (#") {
			t.Errorf("guide[%s] = %q", a, guide[a])
		}
	}
	if !strings.Contains(guide["accent2"], "primary fill") || !strings.Contains(guide["accent6"], "near-background") {
		t.Errorf("guide does not state primary fill / near-background roles: %v", guide)
	}

	if !slices.Contains(testutil.AllTestTemplateNames(), "p-style") {
		t.Skip("local p-style template not present")
	}
	info, err := analyzeTemplateForSkillInfoOpts(filepath.Join(testutil.TemplatesDir(), "p-style.pptx"), template.NewMemoryCache(time.Hour), "compact", skillInfoOptions{NoPreview: true})
	if err != nil {
		t.Fatal(err)
	}
	if !info.AccentUsageGuideDerived || len(info.AccentUsageGuide) != 6 {
		t.Errorf("p-style: derived=%v guide=%v, want a derived six-line guide", info.AccentUsageGuideDerived, info.AccentUsageGuide)
	}
}
