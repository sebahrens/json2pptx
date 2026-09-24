package generator

import (
	"strings"
	"testing"
)

func TestDeriveTwoColumnGeometry(t *testing.T) {
	for _, tt := range []struct {
		name        string
		percent     int
		wantLeftCX  int64
		wantRightCX int64
	}{
		{"wide left", 65, 6256692, 3368988},
		{"wide right", 35, 3368988, 6256692},
	} {
		t.Run(tt.name, func(t *testing.T) {
			slide := &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: []shapeXML{
				makeShape("body", "body", nil, 100000, 100000, 4900000, 5000000),
				makeShape("body_2", "body", nil, 5100000, 100000, 4900000, 5000000),
			}}}}
			if err := deriveTwoColumnGeometry(slide, tt.percent); err != nil {
				t.Fatal(err)
			}
			left := slide.CommonSlideData.ShapeTree.Shapes[0].ShapeProperties.Transform
			right := slide.CommonSlideData.ShapeTree.Shapes[1].ShapeProperties.Transform
			if left.Extent.CX != tt.wantLeftCX || right.Extent.CX != tt.wantRightCX {
				t.Errorf("widths = %d/%d, want %d/%d", left.Extent.CX, right.Extent.CX, tt.wantLeftCX, tt.wantRightCX)
			}
			if right.Offset.X-left.Offset.X-left.Extent.CX != minTwoColumnGutterEMU {
				t.Errorf("gutter = %d, want %d", right.Offset.X-left.Offset.X-left.Extent.CX, minTwoColumnGutterEMU)
			}
			if right.Offset.X+right.Extent.CX != 10000000 || left.Offset.X != 100000 {
				t.Errorf("outside edges moved: left %d, right %d", left.Offset.X, right.Offset.X+right.Extent.CX)
			}
		})
	}
}

func TestDeriveTwoColumnGeometry_RejectsInvalidGeometry(t *testing.T) {
	for _, tt := range []struct {
		name    string
		shapes  []shapeXML
		percent int
		want    string
	}{
		{"invalid percent", []shapeXML{makeShape("body", "body", nil, 0, 0, 100, 100), makeShape("body_2", "body", nil, 200, 0, 100, 100)}, 100, "between 1 and 99"},
		{"missing secondary", []shapeXML{makeShape("body", "body", nil, 0, 0, 100, 100)}, 65, "body and body_2"},
		{"stacked", []shapeXML{makeShape("body", "body", nil, 0, 0, 100, 100), makeShape("body_2", "body", nil, 0, 200, 100, 100)}, 65, "side-by-side"},
		{"too narrow", []shapeXML{makeShape("body", "body", nil, 0, 0, 100, 100), makeShape("body_2", "body", nil, 200, 0, 100, 100)}, 65, "no usable width"},
		{"rounds to empty column", []shapeXML{makeShape("body", "body", nil, 0, 0, 1, 100), makeShape("body_2", "body", nil, 274321, 0, 1, 100)}, 35, "empty column"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			slide := &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: tt.shapes}}}
			if err := deriveTwoColumnGeometry(slide, tt.percent); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestDeriveTwoColumnGeometry_PreservesLargerNativeGapAndPhysicalOrder(t *testing.T) {
	slide := &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: []shapeXML{
		makeShape("body", "body", nil, 5500000, 0, 4500000, 5000000),
		makeShape("body_2", "body", nil, 0, 0, 4500000, 5000000),
	}}}}
	if err := deriveTwoColumnGeometry(slide, 65); err != nil {
		t.Fatal(err)
	}
	physicalRight := slide.CommonSlideData.ShapeTree.Shapes[0].ShapeProperties.Transform
	physicalLeft := slide.CommonSlideData.ShapeTree.Shapes[1].ShapeProperties.Transform
	if physicalLeft.Extent.CX != 5850000 || physicalRight.Extent.CX != 3150000 {
		t.Errorf("physical widths = %d/%d, want 5850000/3150000", physicalLeft.Extent.CX, physicalRight.Extent.CX)
	}
	if physicalRight.Offset.X-physicalLeft.Offset.X-physicalLeft.Extent.CX != 1000000 {
		t.Errorf("larger native gutter was not preserved")
	}
}
