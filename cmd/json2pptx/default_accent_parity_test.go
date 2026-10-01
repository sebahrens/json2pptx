package main

import (
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/svggen"
)

// wantPrimaryFill pins each template's primary fill. Templates whose accent1
// carries white text keep accent1, so their pattern output is unchanged; the
// others default to the first slot that does (go-slide-creator-2mia4).
var wantPrimaryFill = map[string]string{
	"abstract":          "accent1", // 3:1 large-text safe, no body-safe accent
	"blue-corporate":    "dk2",     // every accent is a pastel
	"business-template": "accent3",
	"forest-green":      "accent1",
	"midnight-blue":     "accent1",
	"modern":            "accent1",
	"modern-template":   "accent1",
	"modern-yellow":     "accent1",
	"warm-coral":        "accent2",
	"p-style":           "accent1", // 3:1 large-text safe, no body-safe accent
}

// TestPatternDefaultAccentMatchesColorRolesPrimaryFill: the accent a pattern
// paints with by default is the slot list_templates reports as
// color_roles.primary_fill, and white text reads on it.
func TestPatternDefaultAccentMatchesColorRolesPrimaryFill(t *testing.T) {
	white := svggen.MustParseColor("#FFFFFF")
	for _, name := range testutil.AllTestTemplateNames() {
		t.Run(name, func(t *testing.T) {
			reader, err := template.OpenTemplate(filepath.Join(testutil.TemplatesDir(), name+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			theme := template.ParseTheme(reader)
			roles := buildColorRoles(theme.Colors)
			ctx := patterns.ExpandContext{Theme: theme}
			got := ctx.DefaultAccent()
			if got != roles.PrimaryFill {
				t.Errorf("pattern default accent %q != color_roles.primary_fill %q", got, roles.PrimaryFill)
			}
			if want, ok := wantPrimaryFill[name]; ok && got != want {
				t.Errorf("default accent = %q, want %q", got, want)
			}
			if c, err := svggen.ParseColor(findColorHex(theme.Colors, got)); err != nil || c.ContrastWith(white) < svggen.WCAGAALarge {
				t.Errorf("white text fails on the default accent %q", got)
			}
		})
	}
}
