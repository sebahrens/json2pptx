package fontcache

import (
	"testing"

	"github.com/tdewolff/canvas"
)

func TestHostIndependent(t *testing.T) {
	for name, want := range map[string]bool{
		"Calibri": true, " calibri ": true, "Lora": true, "Poppins Light": true, "Arial": true, "Liberation Sans": true,
		"Calibri Light": false, "Gill Sans": false, "Segoe UI": false, "Georgia": false, "": false,
	} {
		if got := HostIndependent(name); got != want {
			t.Errorf("HostIndependent(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestMetricClone(t *testing.T) {
	for name, want := range map[string]string{"Calibri": "Carlito", " calibri ": "Carlito", "Cambria": "", "Gill Sans": "", "Arial": ""} {
		if got := MetricClone(name); got != want {
			t.Errorf("MetricClone(%q) = %q, want %q", name, got, want)
		}
	}
}

// go-slide-creator-tl7vf: Calibri resolves to the embedded Carlito (not a
// substitution) when the real Calibri is absent — on every host, so CI jobs
// with and without LibreOffice measure Calibri templates identically.
func TestResolveCalibriToEmbeddedCarlito(t *testing.T) {
	if err := canvas.NewFontFamily("probe").LoadSystemFont("Calibri", canvas.FontRegular); err == nil {
		t.Skip("real Calibri installed; the clone is not needed")
	}
	Reset()
	defer Reset()
	ff, resolved, substituted := Resolve("Calibri", "Arial")
	if ff == nil {
		t.Fatal("Calibri did not resolve")
	}
	if resolved != "Carlito" || substituted {
		t.Errorf("Resolve(Calibri) = %q substituted=%v, want Carlito substituted=false", resolved, substituted)
	}
}

// The embedded clone loads regular and bold faces.
func TestLoadMetricCloneEmbedded(t *testing.T) {
	ff := canvas.NewFontFamily("Calibri")
	clone, ok := loadMetricClone(ff, "Calibri")
	if !ok || clone != "Carlito" {
		t.Fatalf("loadMetricClone(Calibri) = %q, %v; want Carlito, true", clone, ok)
	}
	if ff.Face(12, canvas.Black, canvas.FontBold, canvas.FontNormal) == nil {
		t.Error("bold face missing")
	}
	if _, ok := loadMetricClone(canvas.NewFontFamily("x"), "Gill Sans"); ok {
		t.Error("Gill Sans has no embedded clone")
	}
}
