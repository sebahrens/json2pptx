package visualqa

import "testing"

func TestReviewRecordRequiresCurrentAllSlidePixels(t *testing.T) {
	r := &ReviewRecord{ArtifactSHA256: "a", Revision: "r", Backend: "host", Verdict: "approved", Slides: []ReviewSlide{{Index: 0, Role: "kpi_snapshot", ImageSHA256: PixelHash([]byte("one"))}, {Index: 1, Role: "chart_insight", ImageSHA256: PixelHash([]byte("two"))}}}
	if err := r.ValidateCompletion("a", "r", 2); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateCompletion("new", "r", 2); err == nil {
		t.Fatal("expected stale artifact rejection")
	}
	r.Slides = r.Slides[:1]
	if err := r.ValidateCompletion("a", "r", 2); err == nil {
		t.Fatal("expected partial coverage rejection")
	}
}
