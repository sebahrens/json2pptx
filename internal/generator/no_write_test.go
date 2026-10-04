package generator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestGenerateNoWriteGivesTheRendersVerdict: a NoWrite run executes every
// phase and creates no file, and its error is the one a render of the same
// request returns — so a caller can read generation's verdict first
// (go-slide-creator-ni65k).
func TestGenerateNoWriteGivesTheRendersVerdict(t *testing.T) {
	templatePath := filepath.Join("..", "..", "templates", "midnight-blue.pptx")
	title := ContentItem{PlaceholderID: "title", Type: ContentText, Value: "Revenue grew in every region"}
	body := func(text string) ContentItem {
		return ContentItem{PlaceholderID: "body", Type: ContentBullets, Value: []string{text}}
	}
	request := func(dir string, content ...ContentItem) GenerationRequest {
		return GenerationRequest{
			TemplatePath:          templatePath,
			OutputPath:            filepath.Join(dir, "deck.pptx"),
			ExcludeTemplateSlides: true,
			Slides:                []SlideSpec{{LayoutID: "slideLayout2", Content: content}},
		}
	}
	files := func(dir string) []string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return names
	}

	t.Run("an accepted deck leaves no file", func(t *testing.T) {
		dir := t.TempDir()
		req := request(dir, title, body("North up 12%"))
		req.NoWrite, req.RefuseDroppedContent = true, true
		result, err := Generate(context.Background(), req)
		if err != nil {
			t.Fatalf("no-write run refused a deck a render writes: %v", err)
		}
		if result.SlideCount != 1 || result.OutputPath != "" {
			t.Errorf("result = %+v, want one slide and no output path", result)
		}
		if got := files(dir); len(got) != 0 {
			t.Errorf("a no-write run left files: %v", got)
		}
		// No output path is needed at all.
		req.OutputPath = ""
		if _, err := Generate(context.Background(), req); err != nil {
			t.Errorf("no-write run without an output path: %v", err)
		}
	})

	t.Run("a dropped block is the same refusal with and without the file", func(t *testing.T) {
		for _, noWrite := range []bool{true, false} {
			dir := t.TempDir()
			req := request(dir, title, body("North up 12%"), body("South up 8%"))
			req.NoWrite, req.RefuseDroppedContent = noWrite, true
			_, err := Generate(context.Background(), req)
			var refusal *ContentDropRefusal
			if !errors.As(err, &refusal) {
				t.Fatalf("NoWrite=%t: error = %v, want a *ContentDropRefusal", noWrite, err)
			}
			if len(refusal.Findings) != 1 || refusal.Findings[0].Path != "/slides/0/content/2" ||
				!patterns.IsHardContentDrop(refusal.Findings[0]) {
				t.Errorf("NoWrite=%t: findings = %+v, want one hard drop at /slides/0/content/2", noWrite, refusal.Findings)
			}
			if got := files(dir); len(got) != 0 {
				t.Errorf("NoWrite=%t: a refused deck left files: %v", noWrite, got)
			}
		}
	})

	t.Run("without the refusal the drop stays a finding on a written deck", func(t *testing.T) {
		dir := t.TempDir()
		result, err := Generate(context.Background(), request(dir, title, body("North up 12%"), body("South up 8%")))
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		dropped := false
		for _, f := range result.FitFindings {
			dropped = dropped || patterns.IsHardContentDrop(f)
		}
		if !dropped {
			t.Errorf("no CONTENT_DROPPED finding: %+v", result.FitFindings)
		}
		if _, err := os.Stat(result.OutputPath); err != nil {
			t.Errorf("the deck was not written: %v", err)
		}
	})
}
