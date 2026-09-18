package generator

import "testing"

func TestResolveSlideChromeFrameAcrossAspectRatios(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int64
	}{
		{"four-three", 9144000, 6858000},
		{"wide", 12192000, 6858000},
		{"ultrawide", 16000000, 6000000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := ResolveSlideChromeFrame(tc.w, tc.h, true, true)
			for name, r := range map[string]struct{ X, Y, CX, CY int64 }{
				"content":  {f.Content.X, f.Content.Y, f.Content.CX, f.Content.CY},
				"takeaway": {f.Takeaway.X, f.Takeaway.Y, f.Takeaway.CX, f.Takeaway.CY},
				"source":   {f.Source.X, f.Source.Y, f.Source.CX, f.Source.CY},
			} {
				if r.X < 0 || r.Y < 0 || r.CX < 0 || r.CY < 0 || r.X+r.CX > tc.w || r.Y+r.CY > tc.h {
					t.Errorf("%s outside %dx%d: %+v", name, tc.w, tc.h, r)
				}
			}
			if f.Content.Y+f.Content.CY > f.Takeaway.Y || f.Takeaway.Y+f.Takeaway.CY > f.Source.Y {
				t.Fatalf("chrome regions overlap: %+v", f)
			}
		})
	}
}

func TestResolveSlideChromeFrameSourceAndTakeawayReservation(t *testing.T) {
	base := ResolveSlideChromeFrame(12192000, 6858000, false, false)
	source := ResolveSlideChromeFrame(12192000, 6858000, false, true)
	both := ResolveSlideChromeFrame(12192000, 6858000, true, true)
	if !(base.Content.CY > source.Content.CY && source.Content.CY > both.Content.CY) {
		t.Fatalf("reservations do not reduce content monotonically: %d %d %d", base.Content.CY, source.Content.CY, both.Content.CY)
	}
}
