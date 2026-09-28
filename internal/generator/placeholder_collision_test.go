package generator

import (
	"errors"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Two text blocks on one placeholder used to overwrite each other silently;
// the first now wins and the loser is reported (go-slide-creator-rioxd).
func TestPopulateTextInSlide_DuplicateTextBlockEmitsContentDropped(t *testing.T) {
	slide := &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: oneContentShapes()}}}
	ctx := newSinglePassContext("", nil, nil, false, nil)
	content := []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "T"},
		{PlaceholderID: "body", Type: ContentBullets, Value: []string{"left one"}},
		{PlaceholderID: "body", Type: ContentBullets, Value: []string{"right one"}},
	}
	warnings := ctx.populateTextInSlide(slide, content, "One Content", 0, "")

	dropped := 0
	for _, f := range ctx.fitFindings {
		if f.Code == "CONTENT_DROPPED" {
			dropped++
			if f.Path != "/slides/0/content/2" {
				t.Errorf("finding path = %q, want /slides/0/content/2", f.Path)
			}
			// Content loss is a refusal with concrete remedies, never a
			// silent WARN (go-slide-creator-k3lyz).
			if f.Action != "refuse" || f.Severity() != "error" || !patterns.IsHardContentDrop(f) {
				t.Errorf("drop must be a refuse-class hard drop, got action=%q severity=%q", f.Action, f.Severity())
			}
			if opts, _ := f.Fix.Params["options"].([]string); len(opts) == 0 || opts[0] != "split_slide" {
				t.Errorf("fix.params.options = %v, want split_slide first", f.Fix.Params["options"])
			}
		}
	}
	if dropped != 1 {
		t.Fatalf("CONTENT_DROPPED findings = %d, want 1 (warnings: %v)", dropped, warnings)
	}
	if got := getShapeText(&slide.CommonSlideData.ShapeTree.Shapes[1]); !strings.Contains(got, "left one") {
		t.Errorf("first block must stay in the placeholder, got %q", got)
	}
}

func oneContentShapes() []shapeXML {
	return []shapeXML{
		makeShape("title", "title", nil, 0, 0, 10000, 1000),
		makeShape("body", "body", nil, 0, 1200, 10000, 5000),
	}
}

func TestCheckPlaceholderCollisions_FallbackOntoClaimedShapeErrors(t *testing.T) {
	content := []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "T"},
		{PlaceholderID: "body_2", Type: ContentBullets, Value: []string{"a"}},
		{PlaceholderID: "body", Type: ContentDiagram},
	}
	err := checkPlaceholderCollisions(oneContentShapes(), content, "One Content", 3)
	var ce *PlaceholderCollisionError
	if !errors.As(err, &ce) {
		t.Fatalf("expected PlaceholderCollisionError, got %v", err)
	}
	if ce.FirstID != "body_2" || ce.SecondID != "body" || ce.ResolvedName != "body" || ce.SlideIndex != 3 {
		t.Errorf("unexpected collision detail: %+v", ce)
	}
	if ce.Tier == TierExact {
		t.Errorf("reported tier must be the fallback tier, got %s", ce.Tier)
	}
	if ce.Error() == "" {
		t.Error("empty error message")
	}
}

func TestCheckPlaceholderCollisions_AllowedCases(t *testing.T) {
	twoCol := []shapeXML{
		makeShape("title", "title", nil, 0, 0, 10000, 1000),
		makeShape("body", "body", nil, 0, 1200, 5000, 5000),
		makeShape("body_2", "body", nil, 5000, 1200, 5000, 5000),
	}
	cases := map[string]struct {
		shapes  []shapeXML
		content []ContentItem
	}{
		"distinct placeholders": {twoCol, []ContentItem{
			{PlaceholderID: "body", Type: ContentDiagram},
			{PlaceholderID: "body_2", Type: ContentBullets},
		}},
		"same id text above diagram": {oneContentShapes(), []ContentItem{
			{PlaceholderID: "body", Type: ContentText},
			{PlaceholderID: "body", Type: ContentDiagram},
		}},
		"single item": {oneContentShapes(), []ContentItem{{PlaceholderID: "body_2", Type: ContentBullets}}},
		"unresolvable id": {oneContentShapes(), []ContentItem{
			{PlaceholderID: "body", Type: ContentText},
			{PlaceholderID: "nonexistent_zz", Type: ContentText},
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := checkPlaceholderCollisions(tc.shapes, tc.content, "L", 0); err != nil {
				t.Errorf("unexpected collision error: %v", err)
			}
		})
	}
}
