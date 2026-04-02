package pptx

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func TestShadow_WriteEffectXML(t *testing.T) {
	t.Parallel()

	s := Shadow{
		BlurRadius: 50800,  // 4pt
		Distance:   38100,  // 3pt
		Direction:  2700000, // 45°
		Color:      SolidFill("000000"),
		Alignment:  "bl",
	}
	s.Color.mods = append(s.Color.mods, Alpha(50000))

	var buf bytes.Buffer
	s.WriteEffectXML(&buf)

	got := buf.String()
	for _, want := range []string{
		`<a:outerShdw`,
		`blurRad="50800"`,
		`dist="38100"`,
		`dir="2700000"`,
		`algn="bl"`,
		`srgbClr val="000000"`,
		`alpha val="50000"`,
		`</a:outerShdw>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}

func TestGlow_WriteEffectXML(t *testing.T) {
	t.Parallel()

	g := Glow{
		Radius: 63500,
		Color:  SolidFill("FFC000"),
	}

	var buf bytes.Buffer
	g.WriteEffectXML(&buf)

	got := buf.String()
	for _, want := range []string{
		`<a:glow rad="63500"`,
		`srgbClr val="FFC000"`,
		`</a:glow>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}

func TestSoftEdge_WriteEffectXML(t *testing.T) {
	t.Parallel()

	se := SoftEdge{Radius: 25400}

	var buf bytes.Buffer
	se.WriteEffectXML(&buf)

	got := buf.String()
	want := `<a:softEdge rad="25400"/>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestOuterShadow_Convenience(t *testing.T) {
	t.Parallel()

	s := OuterShadow(4.0, 3.0, 45, "000000", 40000)

	if s.BlurRadius != 50800 {
		t.Errorf("BlurRadius = %d, want 50800", s.BlurRadius)
	}
	if s.Distance != 38100 {
		t.Errorf("Distance = %d, want 38100", s.Distance)
	}
	if s.Direction != 2700000 {
		t.Errorf("Direction = %d, want 2700000", s.Direction)
	}
	if s.Alignment != "bl" {
		t.Errorf("Alignment = %q, want %q", s.Alignment, "bl")
	}
}

func TestOuterShadow_NoAlpha(t *testing.T) {
	t.Parallel()

	s := OuterShadow(2.0, 1.0, 90, "FF0000", 0)

	var buf bytes.Buffer
	s.WriteEffectXML(&buf)

	got := buf.String()
	if strings.Contains(got, "alpha") {
		t.Error("should not have alpha modifier when alpha=0")
	}
	if !strings.Contains(got, `srgbClr val="FF0000"`) {
		t.Errorf("missing color in %s", got)
	}
}

func TestWriteEffectList_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	WriteEffectList(&buf, nil)
	if buf.Len() != 0 {
		t.Error("empty effects should produce no output")
	}
}

func TestWriteEffectList_Multiple(t *testing.T) {
	t.Parallel()

	effects := []Effect{
		Glow{Radius: 63500, Color: SolidFill("FFC000")},
		Shadow{BlurRadius: 50800, Distance: 38100, Direction: 2700000, Color: SolidFill("000000"), Alignment: "bl"},
		SoftEdge{Radius: 25400},
	}

	var buf bytes.Buffer
	WriteEffectList(&buf, effects)

	got := buf.String()
	if !strings.HasPrefix(got, `<a:effectLst>`) {
		t.Error("missing effectLst open tag")
	}
	if !strings.HasSuffix(got, `</a:effectLst>`) {
		t.Error("missing effectLst close tag")
	}
	for _, want := range []string{"<a:glow", "<a:outerShdw", "<a:softEdge"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestGenerateShape_WithEffects(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       1,
		Bounds:   RectFromInches(1, 1, 3, 2),
		Geometry: GeomRect,
		Fill:     SolidFill("4472C4"),
		Effects: []Effect{
			OuterShadow(4.0, 3.0, 45, "000000", 40000),
		},
	})
	if err != nil {
		t.Fatalf("GenerateShape failed: %v", err)
	}

	s := string(sp)

	if !strings.Contains(s, `<a:effectLst>`) {
		t.Error("missing effectLst")
	}
	if !strings.Contains(s, `<a:outerShdw`) {
		t.Error("missing outerShdw")
	}

	// Must be well-formed XML
	var v interface{}
	if err := xml.Unmarshal(sp, &v); err != nil {
		t.Errorf("invalid XML: %v\n%s", err, s)
	}
}

func TestGenerateShape_WithComposedEffects(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       2,
		Bounds:   RectFromInches(1, 1, 3, 2),
		Geometry: GeomRoundRect,
		Fill:     SolidFill("70AD47"),
		Effects: []Effect{
			OuterShadow(4.0, 3.0, 45, "000000", 40000),
			Glow{Radius: 63500, Color: SolidFill("FFC000")},
		},
	})
	if err != nil {
		t.Fatalf("GenerateShape failed: %v", err)
	}

	s := string(sp)

	if !strings.Contains(s, `<a:outerShdw`) {
		t.Error("missing shadow")
	}
	if !strings.Contains(s, `<a:glow`) {
		t.Error("missing glow")
	}

	// Must be well-formed XML
	var v interface{}
	if err := xml.Unmarshal(sp, &v); err != nil {
		t.Errorf("invalid XML: %v\n%s", err, s)
	}
}

func TestGenerateShape_NoEffects(t *testing.T) {
	t.Parallel()

	sp, err := GenerateShape(ShapeOptions{
		ID:       1,
		Bounds:   RectFromInches(0, 0, 1, 1),
		Geometry: GeomRect,
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if strings.Contains(string(sp), `effectLst`) {
		t.Error("should not have effectLst without explicit effects")
	}
}
