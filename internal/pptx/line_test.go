package pptx

import (
	"bytes"
	"testing"
)

func TestLine_NoLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	NoLine().WriteTo(&buf)
	got := buf.String()
	expected := `<a:ln><a:noFill/></a:ln>`
	if got != expected {
		t.Errorf("NoLine:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestLine_SolidLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	SolidLine(12700, "000000").WriteTo(&buf)
	got := buf.String()
	expected := `<a:ln w="12700"><a:solidFill><a:srgbClr val="000000"/></a:solidFill></a:ln>`
	if got != expected {
		t.Errorf("SolidLine:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestLine_SolidLinePoints(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	SolidLinePoints(1.0, "FF0000").WriteTo(&buf)
	got := buf.String()
	expected := `<a:ln w="12700"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:ln>`
	if got != expected {
		t.Errorf("SolidLinePoints:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestLine_WithDashAndCap(t *testing.T) {
	t.Parallel()
	l := Line{
		Width: 25400,
		Fill:  SolidFill("0070C0"),
		Dash:  "dash",
		Cap:   "rnd",
		Join:  "round",
	}
	var buf bytes.Buffer
	l.WriteTo(&buf)
	got := buf.String()
	expected := `<a:ln w="25400" cap="rnd"><a:solidFill><a:srgbClr val="0070C0"/></a:solidFill><a:prstDash val="dash"/><a:round/></a:ln>`
	if got != expected {
		t.Errorf("Line with dash/cap:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestLine_BevelJoin(t *testing.T) {
	t.Parallel()
	l := Line{Width: 12700, Fill: SolidFill("333333"), Join: "bevel"}
	var buf bytes.Buffer
	l.WriteTo(&buf)
	got := buf.String()
	if got != `<a:ln w="12700"><a:solidFill><a:srgbClr val="333333"/></a:solidFill><a:bevel/></a:ln>` {
		t.Errorf("unexpected: %s", got)
	}
}
