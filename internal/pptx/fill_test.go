package pptx

import (
	"bytes"
	"testing"
)

func TestFill_NoFill(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	NoFill().WriteTo(&buf)
	got := buf.String()
	expected := `<a:noFill/>`
	if got != expected {
		t.Errorf("NoFill:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestFill_SolidFill(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	SolidFill("4472C4").WriteTo(&buf)
	got := buf.String()
	expected := `<a:solidFill><a:srgbClr val="4472C4"/></a:solidFill>`
	if got != expected {
		t.Errorf("SolidFill:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestFill_SchemeFill_NoMods(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	SchemeFill("accent1").WriteTo(&buf)
	got := buf.String()
	expected := `<a:solidFill><a:schemeClr val="accent1"/></a:solidFill>`
	if got != expected {
		t.Errorf("SchemeFill no mods:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestFill_SchemeFill_WithMods(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	SchemeFill("dk1", LumMod(75000), LumOff(25000)).WriteTo(&buf)
	got := buf.String()
	expected := `<a:solidFill><a:schemeClr val="dk1"><a:lumMod val="75000"/><a:lumOff val="25000"/></a:schemeClr></a:solidFill>`
	if got != expected {
		t.Errorf("SchemeFill with mods:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestFill_SolidFill_WithAlpha(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	f := Fill{set: true, typ: fillSolid, color: "FF0000", mods: []colorMod{Alpha(50000)}}
	f.WriteTo(&buf)
	got := buf.String()
	expected := `<a:solidFill><a:srgbClr val="FF0000"><a:alpha val="50000"/></a:srgbClr></a:solidFill>`
	if got != expected {
		t.Errorf("SolidFill with alpha:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestFill_SolidFillWithAlpha(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	SolidFillWithAlpha("000000", 50000).WriteTo(&buf)
	got := buf.String()
	expected := `<a:solidFill><a:srgbClr val="000000"><a:alpha val="50000"/></a:srgbClr></a:solidFill>`
	if got != expected {
		t.Errorf("SolidFillWithAlpha:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestFill_SchemeFill_WithAlpha(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	SchemeFill("accent1", Alpha(20000)).WriteTo(&buf)
	got := buf.String()
	expected := `<a:solidFill><a:schemeClr val="accent1"><a:alpha val="20000"/></a:schemeClr></a:solidFill>`
	if got != expected {
		t.Errorf("SchemeFill with alpha:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestFill_IsZero(t *testing.T) {
	t.Parallel()
	var zero Fill
	if !zero.IsZero() {
		t.Error("zero value Fill should be IsZero")
	}
	if NoFill().IsZero() {
		t.Error("NoFill() should not be IsZero")
	}
	if SolidFill("000000").IsZero() {
		t.Error("SolidFill should not be IsZero")
	}
	if SchemeFill("accent1").IsZero() {
		t.Error("SchemeFill should not be IsZero")
	}
}

func TestFill_LinearGradient(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	LinearGradient(5400000,
		GradientStop{Position: 0, Color: SolidFill("FFFFFF")},
		GradientStop{Position: 100000, Color: SolidFill("000000")},
	).WriteTo(&buf)
	expected := `<a:gradFill><a:gsLst>` +
		`<a:gs pos="0"><a:srgbClr val="FFFFFF"/></a:gs>` +
		`<a:gs pos="100000"><a:srgbClr val="000000"/></a:gs>` +
		`</a:gsLst><a:lin ang="5400000" scaled="1"/></a:gradFill>`
	if buf.String() != expected {
		t.Errorf("LinearGradient:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestFill_LinearGradient_SchemeStops(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	LinearGradient(2700000,
		GradientStop{Position: 0, Color: SchemeFill("accent1")},
		GradientStop{Position: 50000, Color: SchemeFill("accent1", LumMod(75000))},
		GradientStop{Position: 100000, Color: SchemeFill("accent1", LumMod(50000))},
	).WriteTo(&buf)
	expected := `<a:gradFill><a:gsLst>` +
		`<a:gs pos="0"><a:schemeClr val="accent1"/></a:gs>` +
		`<a:gs pos="50000"><a:schemeClr val="accent1"><a:lumMod val="75000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="100000"><a:schemeClr val="accent1"><a:lumMod val="50000"/></a:schemeClr></a:gs>` +
		`</a:gsLst><a:lin ang="2700000" scaled="1"/></a:gradFill>`
	if buf.String() != expected {
		t.Errorf("LinearGradient scheme stops:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestFill_RadialGradient(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	RadialGradient(
		GradientStop{Position: 0, Color: SolidFill("FFFFFF")},
		GradientStop{Position: 100000, Color: SolidFill("4472C4")},
	).WriteTo(&buf)
	expected := `<a:gradFill><a:gsLst>` +
		`<a:gs pos="0"><a:srgbClr val="FFFFFF"/></a:gs>` +
		`<a:gs pos="100000"><a:srgbClr val="4472C4"/></a:gs>` +
		`</a:gsLst><a:path path="circle"><a:fillToRect l="50000" t="50000" r="50000" b="50000"/></a:path></a:gradFill>`
	if buf.String() != expected {
		t.Errorf("RadialGradient:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestFill_PatternFill(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PatternFill("pct20", "000000", "FFFFFF").WriteTo(&buf)
	expected := `<a:pattFill prst="pct20">` +
		`<a:fgClr><a:srgbClr val="000000"/></a:fgClr>` +
		`<a:bgClr><a:srgbClr val="FFFFFF"/></a:bgClr>` +
		`</a:pattFill>`
	if buf.String() != expected {
		t.Errorf("PatternFill:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestFill_PictureFill(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PictureFill("rId3").WriteTo(&buf)
	expected := `<a:blipFill><a:blip r:embed="rId3"/><a:stretch><a:fillRect/></a:stretch></a:blipFill>`
	if buf.String() != expected {
		t.Errorf("PictureFill:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestFill_NewTypes_IsZero(t *testing.T) {
	t.Parallel()
	if LinearGradient(0).IsZero() {
		t.Error("LinearGradient should not be IsZero")
	}
	if RadialGradient().IsZero() {
		t.Error("RadialGradient should not be IsZero")
	}
	if PatternFill("horz", "000000", "FFFFFF").IsZero() {
		t.Error("PatternFill should not be IsZero")
	}
	if PictureFill("rId1").IsZero() {
		t.Error("PictureFill should not be IsZero")
	}
}

func TestFill_MarshalColorXML(t *testing.T) {
	t.Parallel()

	t.Run("solid", func(t *testing.T) {
		var buf bytes.Buffer
		SolidFill("00FF00").WriteColorTo(&buf)
		expected := `<a:srgbClr val="00FF00"/>`
		if buf.String() != expected {
			t.Errorf("got: %s\nwant: %s", buf.String(), expected)
		}
	})

	t.Run("scheme with mods", func(t *testing.T) {
		var buf bytes.Buffer
		SchemeFill("accent2", Tint(60000)).WriteColorTo(&buf)
		expected := `<a:schemeClr val="accent2"><a:tint val="60000"/></a:schemeClr>`
		if buf.String() != expected {
			t.Errorf("got: %s\nwant: %s", buf.String(), expected)
		}
	})

	t.Run("noFill writes nothing", func(t *testing.T) {
		var buf bytes.Buffer
		NoFill().WriteColorTo(&buf)
		if buf.Len() != 0 {
			t.Errorf("NoFill MarshalColorXML should write nothing, got: %s", buf.String())
		}
	})
}

func TestIsSchemeColor(t *testing.T) {
	t.Parallel()

	schemes := []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6",
		"dk1", "dk2", "lt1", "lt2", "tx1", "tx2", "bg1", "bg2", "hlink", "folHlink"}
	for _, s := range schemes {
		if !IsSchemeColor(s) {
			t.Errorf("IsSchemeColor(%q) = false, want true", s)
		}
	}

	notSchemes := []string{"", "none", "FF0000", "#accent1", "primary", "red"}
	for _, s := range notSchemes {
		if IsSchemeColor(s) {
			t.Errorf("IsSchemeColor(%q) = true, want false", s)
		}
	}
}

func TestResolveColorString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"accent1", `<a:solidFill><a:schemeClr val="accent1"/></a:solidFill>`},
		{"dk1", `<a:solidFill><a:schemeClr val="dk1"/></a:solidFill>`},
		{"tx2", `<a:solidFill><a:schemeClr val="tx2"/></a:solidFill>`},
		{"FF0000", `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>`},
		{"#4472C4", `<a:solidFill><a:srgbClr val="4472C4"/></a:solidFill>`},
		{"none", `<a:noFill/>`},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			var buf bytes.Buffer
			fill := ResolveColorString(tc.input)
			fill.WriteTo(&buf)
			if buf.String() != tc.expected {
				t.Errorf("ResolveColorString(%q):\ngot:  %s\nwant: %s", tc.input, buf.String(), tc.expected)
			}
		})
	}

	t.Run("empty returns zero", func(t *testing.T) {
		fill := ResolveColorString("")
		if !fill.IsZero() {
			t.Error("ResolveColorString(\"\") should return zero Fill")
		}
	})
}

func TestPatternFill_SchemeColors(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PatternFill("pct5", "accent1", "lt1").WriteTo(&buf)
	expected := `<a:pattFill prst="pct5"><a:fgClr><a:schemeClr val="accent1"/></a:fgClr><a:bgClr><a:schemeClr val="lt1"/></a:bgClr></a:pattFill>`
	if buf.String() != expected {
		t.Errorf("PatternFill with scheme colors:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}
