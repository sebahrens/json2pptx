package generator

import (
	"archive/zip"
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestKeepDatesTogether(t *testing.T) {
	for in, want := range map[string]string{
		"Steering committee read-out | 1 October 2026": "Steering committee read-out | 1" + nbsp + "October" + nbsp + "2026",
		"Next steering committee: 15 November 2026":    "Next steering committee: 15" + nbsp + "November" + nbsp + "2026",
		"Board update, November 15, 2026":              "Board update, November" + nbsp + "15," + nbsp + "2026",
		"Q3 2026 results":                              "Q3" + nbsp + "2026 results",
		"No date here":                                 "No date here",
	} {
		if got := keepDatesTogether(in); got != want {
			t.Errorf("keepDatesTogether(%q) = %q, want %q", in, got, want)
		}
	}
}

var subtitleXfrmRE = regexp.MustCompile(`name="subtitle"[\s\S]*?<a:off x="(-?\d+)" y="-?\d+"\s*/?>(?:</a:off>)?\s*<a:ext cx="(\d+)"`)
var titleOffRE = regexp.MustCompile(`name="title"[\s\S]*?<a:off x="(-?\d+)" y="-?\d+"\s*/?>(?:</a:off>)?\s*<a:ext cx="(\d+)"`)

func generateTitleSlide(t *testing.T, templateName, layoutID, subtitle string) (*GenerationResult, []byte) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "cover.pptx")
	result, err := Generate(context.Background(), GenerationRequest{
		TemplatePath: "../../templates/" + templateName + ".pptx", OutputPath: out, ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{LayoutID: layoutID, Content: []ContentItem{
			{PlaceholderID: "title", Type: ContentTitleSlideTitle, Value: "Decision requested: approve acquisition due diligence"},
			{PlaceholderID: "subtitle", Type: ContentTitleSlideTitle, Value: subtitle},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	return result, zipSlideXML(t, &z.Reader, 1)
}

// TestClosingSubtitleWidenedToTitleSpan pins go-slide-creator-9bmaz on
// modern-template's Closing layout: its ~4.3in subtitle wrapped "Next steering
// committee: 15 November 2026" to an orphaned "2026". The subtitle now spans
// the title and stays on one line.
func TestClosingSubtitleWidenedToTitleSpan(t *testing.T) {
	result, slide := generateTitleSlide(t, "modern-template", "slideLayout5", "Next steering committee: 15 November 2026")
	sub, title := subtitleXfrmRE.FindSubmatch(slide), titleOffRE.FindSubmatch(slide)
	if sub == nil || title == nil {
		t.Fatalf("title / subtitle transforms not found in slide XML")
	}
	sx, _ := strconv.ParseInt(string(sub[1]), 10, 64)
	sw, _ := strconv.ParseInt(string(sub[2]), 10, 64)
	tx, _ := strconv.ParseInt(string(title[1]), 10, 64)
	tw, _ := strconv.ParseInt(string(title[2]), 10, 64)
	if sx != tx || sw != tw {
		t.Errorf("subtitle frame x=%d w=%d, want the title span x=%d w=%d", sx, sw, tx, tw)
	}
	if !bytes.Contains(slide, []byte("15"+nbsp+"November"+nbsp+"2026")) {
		t.Error("date token not joined with non-breaking spaces")
	}
	for _, f := range result.FitFindings {
		if f.Code == patterns.ErrCodeSubtitleWraps {
			t.Errorf("subtitle still reported as wrapping after widening: %s", f.Message)
		}
	}
}

// TestNarrowCoverSubtitleReportsWrap pins the advisory on abstract's Title
// Slide, whose title is as narrow as its subtitle: nothing to widen into, so
// the wrap is reported and the date stays whole.
func TestNarrowCoverSubtitleReportsWrap(t *testing.T) {
	result, slide := generateTitleSlide(t, "abstract", "slideLayout1", "Steering committee read-out | 1 October 2026")
	if !bytes.Contains(slide, []byte("1"+nbsp+"October"+nbsp+"2026")) {
		t.Error("date token not joined with non-breaking spaces")
	}
	for _, f := range result.FitFindings {
		if f.Code == patterns.ErrCodeSubtitleWraps && f.Path == "/slides/0/content/1" {
			return
		}
	}
	t.Errorf("SUBTITLE_WRAPS not reported: %+v", result.FitFindings)
}
