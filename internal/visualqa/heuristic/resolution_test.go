package heuristic

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/internal/visualqa"
)

func TestLayoutBalanceStableAcrossResolution(t *testing.T) {
	for _, balanced := range []bool{false, true} {
		for _, scale := range []int{1, 2, 4, 8} {
			img := image.NewRGBA(image.Rect(0, 0, 400*scale, 225*scale))
			for y := 0; y < 225*scale; y++ {
				for x := 0; x < 400*scale; x++ {
					img.Set(x, y, color.White)
				}
			}
			end := 108
			if balanced {
				end = 190
			}
			for y := 48 * scale; y < end*scale; y++ {
				for x := 25 * scale; x < 375*scale; x++ {
					if (y/scale)%7 == 0 {
						img.Set(x, y, color.Black)
					}
				}
			}
			var b bytes.Buffer
			if err := png.Encode(&b, img); err != nil {
				t.Fatal(err)
			}
			result := Inspect(b.Bytes(), visualqa.SlideInfo{Type: "table"})
			found := false
			for _, f := range result.Findings {
				found = found || f.Category == "layout_balance"
			}
			if found == balanced {
				t.Errorf("balanced=%v scale=%d: findings %+v", balanced, scale, result.Findings)
			}
		}
	}
}

// A lazy image verifies the work budget even for pathological aspect ratios.
type sampledRules struct {
	width, height, scale, samples int
	vertical                      bool
}

func (r *sampledRules) ColorModel() color.Model { return color.RGBAModel }
func (r *sampledRules) Bounds() image.Rectangle { return image.Rect(0, 0, r.width, r.height) }
func (r *sampledRules) At(x, y int) color.Color {
	r.samples++
	coordinate := y
	if r.vertical {
		coordinate = x
	}
	if (coordinate/r.scale)%7 == 0 {
		return color.Black
	}
	return color.White
}
func TestThinRuleSamplingAcrossAspectAndResolution(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		for _, size := range [][2]int{{400, 225}, {225, 400}} {
			for _, scale := range []int{1, 2, 4, 8} {
				img := &sampledRules{width: size[0] * scale, height: size[1] * scale, scale: scale, vertical: vertical}
				ratio := nonBackgroundRatio(img, color8{248, 248, 248}, 0, 0, img.width, img.height)
				if math.Abs(ratio-1.0/7) > 0.02 {
					t.Errorf("size=%v scale=%d vertical=%v: ink %.3f", size, scale, vertical, ratio)
				}
				if img.samples > 10000 {
					t.Fatalf("sample budget exceeded: %d", img.samples)
				}
			}
		}
	}
	for _, size := range [][2]int{{1000000, 1}, {1, 1000000}} {
		img := &sampledRules{width: size[0], height: size[1], scale: 1}
		nonBackgroundRatio(img, color8{248, 248, 248}, 0, 0, img.width, img.height)
		if img.samples > 10000 {
			t.Fatalf("extreme aspect sample budget exceeded: %d", img.samples)
		}
	}
}
