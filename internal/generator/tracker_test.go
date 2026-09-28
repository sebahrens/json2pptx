package generator

import "testing"

// TestPlaceTitleTrackerTypography pins chrome.tracker's spec
// (go-slide-creator-r3gsw): 9pt caps, +8% letter-spacing, accent1, in its own
// band 6pt above the title.
func TestPlaceTitleTrackerTypography(t *testing.T) {
	slide := &slideXML{}
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{eyebrowTestTitle(1800000)}
	if err := (&singlePassContext{}).placeTitleTracker(slide, "Market context", "title"); err != nil {
		t.Fatal(err)
	}
	shapes := slide.CommonSlideData.ShapeTree.Shapes
	if len(shapes) != 2 {
		t.Fatalf("got %d shapes, want title plus tracker", len(shapes))
	}
	title, tracker := shapes[0], shapes[1]
	if tracker.NonVisualProperties.ConnectionNonVisual.Name != "Section Tracker" {
		t.Errorf("tracker shape name = %q", tracker.NonVisualProperties.ConnectionNonVisual.Name)
	}
	run := tracker.TextBody.Paragraphs[0].Runs[0]
	rp := run.RunProperties
	if run.Text != "Market context" || rp.FontSize != "900" || rp.Caps != "all" || rp.Spacing != "72" || rp.Bold != "" {
		t.Errorf("tracker run = %q %+v, want 9pt caps spc=72 regular", run.Text, rp)
	}
	if want := `<a:solidFill><a:schemeClr val="accent1"/></a:solidFill><a:latin typeface="+mn-lt"/>`; rp.Inner != want {
		t.Errorf("tracker ink = %s, want accent1 body font", rp.Inner)
	}
	gap := title.ShapeProperties.Transform.Offset.Y - (tracker.ShapeProperties.Transform.Offset.Y + tracker.ShapeProperties.Transform.Extent.CY)
	if gap != eyebrowGapEMU {
		t.Errorf("gap above title = %d EMU, want 6pt (%d)", gap, eyebrowGapEMU)
	}
	if title.ShapeProperties.Transform.Offset.Y+title.ShapeProperties.Transform.Extent.CY != 200000+1800000 {
		t.Error("the tracker band moved the title's bottom edge")
	}
}

// A title too short to give up the band keeps its height and copy: the
// tracker is optional chrome and must not fold into the title.
func TestPlaceTitleTrackerSkipsShortTitle(t *testing.T) {
	slide := &slideXML{}
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{eyebrowTestTitle(150000)}
	if err := (&singlePassContext{}).placeTitleTracker(slide, "Market context", "title"); err != nil {
		t.Fatal(err)
	}
	shapes := slide.CommonSlideData.ShapeTree.Shapes
	if len(shapes) != 1 || len(shapes[0].TextBody.Paragraphs) != 1 {
		t.Fatalf("short title changed: %d shapes, %d paragraphs", len(shapes), len(shapes[0].TextBody.Paragraphs))
	}
}
