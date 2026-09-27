package fontcache

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/fonts"
	"github.com/tdewolff/canvas"
)

func TestLoraOriginalEmbeddedFaces(t *testing.T) {
	for _, tc := range []struct {
		data []byte
		sha  string
	}{
		{fonts.LoraRegular, "c72a925082ba55885e7b4ead25d34a3c91fa44edda72a16a3a2983fc7fa7cef3"},
		{fonts.LoraBold, "32baf740eac1bf3f257904a6cf89e4e067ca1566b375bf22f743a26fc88b3791"},
	} {
		if got := fmt.Sprintf("%x", sha256.Sum256(tc.data)); got != tc.sha {
			t.Fatalf("original face changed: %s", got)
		}
	}
	Reset()
	ff, resolved, substituted := Resolve("Lora", "Arial")
	if ff == nil || resolved != "Lora" || substituted {
		t.Fatalf("original face unavailable: %q substituted=%v", resolved, substituted)
	}
	regular := canvas.NewTextLine(ff.Face(45, nil, canvas.FontRegular, canvas.FontNormal), "QUARTERLY OPERATING REVIEW", canvas.Left).Bounds().W()
	bold := canvas.NewTextLine(ff.Face(45, nil, canvas.FontBold, canvas.FontNormal), "QUARTERLY OPERATING REVIEW", canvas.Left).Bounds().W()
	if regular <= 0 || bold <= 0 || regular == bold {
		t.Fatalf("regular/bold original metrics not distinct: %g/%g", regular, bold)
	}
	if ff2, _, _ := Resolve("Lora", ""); ff2 != ff {
		t.Fatal("cache did not preserve original family")
	}
	t.Logf("Original regular/bold 45pt native metrics: %g / %g", regular, bold)
}

func TestLoraExplicitFallbackUsesEmbeddedOriginal(t *testing.T) {
	Reset()
	ff, resolved, substituted := Resolve("NonexistentLoraFallbackTestFont", "Lora")
	if ff == nil || resolved != "Lora" || !substituted {
		t.Fatalf("fallback = %q substituted=%v", resolved, substituted)
	}
}

func TestLoraBrokenEmbeddedFaceFailsClosed(t *testing.T) {
	for _, style := range []string{"Regular", "Bold"} {
		t.Run(style, func(t *testing.T) {
			regular, bold := fonts.LoraRegular, fonts.LoraBold
			defer func() { fonts.LoraRegular, fonts.LoraBold = regular, bold; Reset() }()
			if style == "Regular" {
				fonts.LoraRegular = []byte("invalid font")
			} else {
				fonts.LoraBold = []byte("invalid font")
			}
			for _, request := range []struct{ name, fallback string }{{"Lora", "Arial"}, {"NonexistentBrokenLoraTestFont", "Lora"}} {
				Reset()
				if ff, resolved, substituted := Resolve(request.name, request.fallback); ff != nil || resolved != "" || substituted {
					t.Fatalf("corrupt original silently replaced: %q substituted=%v", resolved, substituted)
				}
			}
		})
	}
}
