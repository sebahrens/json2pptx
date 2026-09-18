package generator

import (
	"errors"
	"testing"
)

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
