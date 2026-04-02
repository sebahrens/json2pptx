package pptx

import (
	"bytes"
	"testing"
)

func TestWriteTransform_Simple(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	bounds := RectEmu{X: 914400, Y: 914400, CX: 4572000, CY: 2743200}
	WriteTransform(&buf, bounds, 0, false, false)
	got := buf.String()
	expected := `<a:xfrm><a:off x="914400" y="914400"/><a:ext cx="4572000" cy="2743200"/></a:xfrm>`
	if got != expected {
		t.Errorf("WriteTransform simple:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestWriteTransform_WithRotation(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	bounds := RectEmu{X: 0, Y: 0, CX: 1828800, CY: 1371600}
	WriteTransform(&buf, bounds, 5400000, false, false) // 90 degrees
	got := buf.String()
	expected := `<a:xfrm rot="5400000"><a:off x="0" y="0"/><a:ext cx="1828800" cy="1371600"/></a:xfrm>`
	if got != expected {
		t.Errorf("WriteTransform rotation:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestWriteTransform_WithFlips(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	bounds := RectEmu{X: 100000, Y: 200000, CX: 300000, CY: 400000}
	WriteTransform(&buf, bounds, 0, true, true)
	got := buf.String()
	expected := `<a:xfrm flipH="1" flipV="1"><a:off x="100000" y="200000"/><a:ext cx="300000" cy="400000"/></a:xfrm>`
	if got != expected {
		t.Errorf("WriteTransform flips:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestWriteTransform_AllOptions(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	bounds := RectFromInches(1, 2, 5, 3)
	WriteTransform(&buf, bounds, 2700000, true, false) // 45 degrees, flipH
	got := buf.String()
	if got == "" {
		t.Fatal("empty output")
	}
	// Check key attributes present
	checks := []string{`rot="2700000"`, `flipH="1"`, `<a:off`, `<a:ext`}
	for _, c := range checks {
		if !bytes.Contains([]byte(got), []byte(c)) {
			t.Errorf("missing %q in: %s", c, got)
		}
	}
	// flipV should NOT be present
	if bytes.Contains([]byte(got), []byte(`flipV`)) {
		t.Errorf("unexpected flipV in: %s", got)
	}
}

func TestRectFromPoints(t *testing.T) {
	t.Parallel()
	r := RectFromPoints(72, 72, 360, 216) // 1", 1", 5", 3" in points
	// 72 points * 12700 EMU/pt = 914400 EMU
	if r.X != 914400 {
		t.Errorf("X = %d, want 914400", r.X)
	}
	if r.Y != 914400 {
		t.Errorf("Y = %d, want 914400", r.Y)
	}
	// 360 points * 12700 = 4572000
	if r.CX != 4572000 {
		t.Errorf("CX = %d, want 4572000", r.CX)
	}
	// 216 points * 12700 = 2743200
	if r.CY != 2743200 {
		t.Errorf("CY = %d, want 2743200", r.CY)
	}
}
