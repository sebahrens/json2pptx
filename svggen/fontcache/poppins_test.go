package fontcache

import (
	"crypto/sha256"
	"fmt"
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/fonts"
	"github.com/tdewolff/canvas"
)

func TestPoppinsLightOriginalEmbeddedFaces(t *testing.T) {
	for _, tc := range []struct {
		data []byte
		sha  string
	}{
		{fonts.PoppinsLight, "650ba57fa99d12ec40c31ccfb680be656be4497fbe14164617d67e32ffe9cd46"},
		{fonts.PoppinsLightItalic, "b8f9c5be59723fadf8e5447fa1245c2c53b60a3464a24d6ece9ee3c283d8917b"},
	} {
		if got := fmt.Sprintf("%x", sha256.Sum256(tc.data)); got != tc.sha {
			t.Fatalf("original face changed: %s", got)
		}
	}
	Reset()
	ff, resolved, substituted := Resolve("Poppins Light", "Arial")
	if ff == nil || resolved != "Poppins Light" || substituted {
		t.Fatalf("native family unavailable: %q substituted=%v", resolved, substituted)
	}
	for _, tc := range []struct {
		name  string
		style canvas.FontStyle
		width float64
		bold  bool
	}{
		{"regular", canvas.FontRegular, 94.1324, false},
		{"italic", canvas.FontItalic, 94.6658, false},
		{"bold", canvas.FontBold, 94.1324, true},
		{"bold-italic", canvas.FontBold | canvas.FontItalic, 94.6658, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			face := ff.Face(18, nil, tc.style, canvas.FontNormal)
			if face.FauxItalic != 0 || (face.FauxBold != 0) != tc.bold {
				t.Fatalf("native style semantics changed: %s", tc.name)
			}
			width := canvas.NewTextLine(face, "Illustrative test data for review", canvas.Left).Bounds().W()
			if math.Abs(width-tc.width) > 0.00001 {
				t.Fatalf("native %s metric=%g want %g", tc.name, width, tc.width)
			}
		})
	}
	if again, _, _ := Resolve("Poppins Light", ""); again != ff {
		t.Fatal("native cache family changed")
	}
}

func TestPoppinsLightExplicitFallbackUsesOriginal(t *testing.T) {
	Reset()
	ff, resolved, substituted := Resolve("NonexistentPoppinsFallbackTestFont", "Poppins Light")
	if ff == nil || resolved != "Poppins Light" || !substituted {
		t.Fatalf("fallback=%q substituted=%v", resolved, substituted)
	}
	if face := ff.Face(18, nil, canvas.FontItalic, canvas.FontNormal); face.FauxItalic != 0 {
		t.Fatal("explicit fallback lost original italic")
	}
}

func TestPoppinsLightBrokenEmbeddedFaceFailsClosed(t *testing.T) {
	for _, style := range []string{"Regular", "Italic"} {
		t.Run(style, func(t *testing.T) {
			regular, italic := fonts.PoppinsLight, fonts.PoppinsLightItalic
			defer func() { fonts.PoppinsLight, fonts.PoppinsLightItalic = regular, italic; Reset() }()
			if style == "Regular" {
				fonts.PoppinsLight = []byte("invalid font")
			} else {
				fonts.PoppinsLightItalic = []byte("invalid font")
			}
			for _, request := range []struct{ name, fallback string }{{"Poppins Light", "Arial"}, {"NonexistentBrokenPoppinsFont", "Poppins Light"}} {
				Reset()
				if ff, resolved, substituted := Resolve(request.name, request.fallback); ff != nil || resolved != "" || substituted {
					t.Fatalf("corrupt native face silently substituted: %q/%v", resolved, substituted)
				}
			}
		})
	}
}
