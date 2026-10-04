package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// Preview reports a pattern or compose slide's fit findings where validate
// reports them: the collector is handed the slide as authored, not the
// expansion the plan resolver wrote into it. Handed the expansion, it took
// the slide for a raw shape_grid and pointed at /slides/N/shape_grid/…, which
// a pattern slide does not have (go-slide-creator-8jp05).
func TestPreviewFitFindingsMatchValidate(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testutil.RepoRoot(), "examples", "patterns-smoke.json"))
	if err != nil {
		t.Fatal(err)
	}
	parse := func() *PresentationInput {
		var input PresentationInput
		if err := json.Unmarshal(data, &input); err != nil {
			t.Fatal(err)
		}
		input.Template = "midnight-blue"
		return &input
	}
	templatePath := filepath.Join(testutil.TemplatesDir(), "midnight-blue.pptx")
	tctx, err := loadPreviewTemplate(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tctx.reader.Close() }()

	keys := func(input *PresentationInput, preview bool) []string {
		resolveCanonicalLayoutIDs(input.Slides, tctx.layouts)
		var out []string
		if preview {
			plan := resolvePreviewSlides(input, tctx)
			if len(plan.Errors) > 0 {
				t.Fatalf("preview errors: %v", plan.Errors)
			}
			for _, f := range computePreviewFitFindings(input, &plan, tctx, true) {
				out = append(out, f.Code+" @ "+f.Path)
			}
		} else {
			layouts, theme, width, height := fitReportGeometry("midnight-blue", testutil.TemplatesDir())
			for _, f := range dedupFitFindings(collectFitFindings(input, layouts, width, height, theme)) {
				out = append(out, f.Code+" @ "+f.Path)
			}
		}
		sort.Strings(out)
		return out
	}
	previewInput := parse()
	validate, preview := keys(parse(), false), keys(previewInput, true)
	if strings.Join(validate, "\n") != strings.Join(preview, "\n") {
		t.Errorf("preview and validate report different findings\npreview:  %q\nvalidate: %q", preview, validate)
	}
	patternFindings := 0
	for _, key := range preview {
		path := key[strings.Index(key, " @ ")+3:]
		si := slidepath.SlideIndex(path)
		if si < 0 || si >= len(previewInput.Slides) || previewInput.Slides[si].Pattern == nil {
			continue
		}
		patternFindings++
		if strings.HasPrefix(path, slidepath.ShapeGrid(si)) {
			t.Errorf("finding on pattern slide %d points at a shape_grid the deck does not have: %s", si+1, key)
		}
	}
	if patternFindings == 0 {
		t.Fatal("the example no longer raises a finding on a pattern slide; the test proves nothing")
	}
}
