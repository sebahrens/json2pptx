// Package templatefixture holds template-dependent test helpers without
// introducing an import cycle in the template package's own tests.
package templatefixture

import (
	"fmt"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
	"os"
	"path/filepath"
	"testing"
)

func OpenTemplateAnalysis(t testing.TB, name string) *types.TemplateAnalysis {
	t.Helper()
	path := filepath.Join(testutil.TemplatesDir(), name+".pptx")
	if name == "p-style" {
		if os.Getenv("JSON2PPTX_SKIP_LOCAL_TEMPLATES") == "1" {
			t.Skip("optional local templates explicitly disabled")
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Skip("optional local p-style template absent")
		}
	}
	analysis, err := ReadTemplateAnalysis(name)
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}

// ReadTemplateAnalysis also serves process-level fixture setup, before a testing.T exists.
func ReadTemplateAnalysis(name string) (*types.TemplateAnalysis, error) {
	path := filepath.Join(testutil.TemplatesDir(), name+".pptx")
	reader, err := template.OpenTemplate(path)
	if err != nil {
		return nil, fmt.Errorf("required template %s: %w", name, err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return nil, fmt.Errorf("template %s layouts: %w", name, err)
	}
	w, h := template.ParseSlideDimensions(reader)
	return &types.TemplateAnalysis{TemplatePath: path, SlideWidth: w, SlideHeight: h, Layouts: layouts, Theme: template.ParseTheme(reader)}, nil
}
