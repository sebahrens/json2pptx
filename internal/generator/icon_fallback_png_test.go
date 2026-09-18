package generator

import (
	"bytes"
	"image/png"
	"testing"
)

// TestIconFallbackPNG verifies shape_grid icons get a real rasterized PNG
// fallback (for viewers that ignore asvg:svgBlip) instead of the 1x1 stub,
// and that unparseable input degrades to the stub rather than failing.
func TestIconFallbackPNG(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#1F3864" stroke-width="2"><path d="M3 17l6 -6l4 4l8 -8"/></svg>`)
	out := iconFallbackPNG(svg)
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode fallback png: %v", err)
	}
	if img.Bounds().Dx() != iconFallbackPNGSizePx {
		t.Errorf("fallback width = %d, want %d", img.Bounds().Dx(), iconFallbackPNGSizePx)
	}
	opaque := false
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y && !opaque; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
				opaque = true
				break
			}
		}
	}
	if !opaque {
		t.Error("fallback png is fully transparent; icon stroke was not rasterized")
	}

	if got := iconFallbackPNG(nil); !bytes.Equal(got, transparentPNG1x1) {
		t.Error("empty svg should fall back to the 1x1 stub")
	}
	if got := iconFallbackPNG([]byte("not svg")); !bytes.Equal(got, transparentPNG1x1) {
		t.Error("unparseable svg should fall back to the 1x1 stub")
	}
}
