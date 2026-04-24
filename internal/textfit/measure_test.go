package textfit

import (
	"testing"
)

func mustMeasure(t *testing.T, text, fontName string, fontPt float64, widthEMU int64, maxLines int) RunMeasurement {
	t.Helper()
	m, err := MeasureRun(text, fontName, fontPt, widthEMU, maxLines)
	if err != nil {
		t.Fatalf("MeasureRun(%q, %q, %.1f, %d, %d): unexpected error: %v", text, fontName, fontPt, widthEMU, maxLines, err)
	}
	return m
}

func TestMeasureRun_ShortTextFitsOneLine(t *testing.T) {
	m := mustMeasure(t, "Hello", "Arial", 18.0, 5*914400, 10)
	if !m.Fits {
		t.Error("short text should fit")
	}
	if m.Lines != 1 {
		t.Errorf("expected 1 line, got %d", m.Lines)
	}
	if m.OverflowChars != 0 {
		t.Errorf("expected 0 overflow chars, got %d", m.OverflowChars)
	}
	if m.RequiredEMU <= 0 {
		t.Error("RequiredEMU should be positive for non-empty text")
	}
}

func TestMeasureRun_LongTextWraps(t *testing.T) {
	// Long text in a narrow placeholder should wrap to multiple lines.
	text := "This is a significantly longer piece of text that should definitely require wrapping to multiple lines when rendered in a narrow placeholder"
	m := mustMeasure(t, text, "Arial", 18.0, 2*914400, 0) // 2 inches wide, unlimited lines
	if m.Lines <= 1 {
		t.Errorf("expected multiple lines, got %d", m.Lines)
	}
	if !m.Fits {
		t.Error("unlimited maxLines should always fit")
	}
}

func TestMeasureRun_OverflowBeyondMaxLines(t *testing.T) {
	// Long text capped to 1 line should report overflow.
	text := "This is a long sentence that certainly does not fit on a single line in a narrow two inch placeholder"
	m := mustMeasure(t, text, "Arial", 18.0, 2*914400, 1) // 2 inches, max 1 line
	if m.Fits {
		t.Error("long text capped at 1 line should not fit")
	}
	if m.OverflowChars <= 0 {
		t.Errorf("expected positive overflow chars, got %d", m.OverflowChars)
	}
	if m.Lines <= 1 {
		t.Errorf("total lines should exceed maxLines, got %d", m.Lines)
	}
}

func TestMeasureRun_MultibyteCJK(t *testing.T) {
	// Japanese text — each character is roughly square, so a narrow
	// placeholder should cause wrapping.
	text := "これは日本語のテキストです。複数行に折り返されるべきです。"
	m := mustMeasure(t, text, "Arial", 18.0, 2*914400, 0)
	// Should produce a valid measurement (may or may not wrap depending on metrics).
	if m.Lines < 1 {
		t.Errorf("expected at least 1 line, got %d", m.Lines)
	}
	if m.RequiredEMU <= 0 {
		t.Error("RequiredEMU should be positive for non-empty text")
	}
	// With maxLines=1, long CJK text should overflow.
	m2 := mustMeasure(t, text, "Arial", 18.0, 1*914400, 1) // 1 inch, max 1 line
	if m2.Lines > 1 && m2.Fits {
		t.Error("CJK text exceeding maxLines should not report Fits=true")
	}
}

func TestMeasureRun_EmptyString(t *testing.T) {
	m := mustMeasure(t, "", "Arial", 18.0, 5*914400, 10)
	if !m.Fits {
		t.Error("empty string should fit")
	}
	if m.Lines != 0 {
		t.Errorf("empty string should have 0 lines, got %d", m.Lines)
	}
	if m.OverflowChars != 0 {
		t.Errorf("empty string should have 0 overflow chars, got %d", m.OverflowChars)
	}
	if m.RequiredEMU != 0 {
		t.Errorf("empty string should have 0 RequiredEMU, got %d", m.RequiredEMU)
	}
}

func TestMeasureRun_ZeroWidth(t *testing.T) {
	m := mustMeasure(t, "Hello", "Arial", 18.0, 0, 10)
	if m.Fits {
		t.Error("zero width should not fit")
	}
}

func TestMeasureRun_ZeroFontSize(t *testing.T) {
	m := mustMeasure(t, "Hello", "Arial", 0, 5*914400, 10)
	if m.Fits {
		t.Error("zero font size should not fit")
	}
}

func TestMeasureRun_UnlimitedMaxLines(t *testing.T) {
	// maxLines=0 means unlimited — should never report overflow.
	text := "Word " // repeated
	long := ""
	for i := 0; i < 100; i++ {
		long += text
	}
	m := mustMeasure(t, long, "Arial", 18.0, 1*914400, 0)
	if !m.Fits {
		t.Error("unlimited maxLines should always report Fits=true")
	}
	if m.OverflowChars != 0 {
		t.Errorf("unlimited maxLines should have 0 overflow chars, got %d", m.OverflowChars)
	}
}

func TestMeasureRun_NegativeMaxLines(t *testing.T) {
	// Negative maxLines treated as unlimited.
	m := mustMeasure(t, "Some text here", "Arial", 18.0, 1*914400, -1)
	if !m.Fits {
		t.Error("negative maxLines should behave as unlimited")
	}
}

func TestMeasureRun_RequiredEMUGrowsWithLines(t *testing.T) {
	short := mustMeasure(t, "Hi", "Arial", 18.0, 5*914400, 0)
	long := mustMeasure(t, "This is a much longer text that will wrap to multiple lines in a narrow two inch wide placeholder area", "Arial", 18.0, 2*914400, 0)
	if long.RequiredEMU <= short.RequiredEMU {
		t.Errorf("more lines should require more height: short=%d long=%d", short.RequiredEMU, long.RequiredEMU)
	}
}

func TestMeasureRun_PureNoSideEffects(t *testing.T) {
	// Call MeasureRun multiple times with same input — results must be identical.
	text := "Deterministic measurement test"
	m1 := mustMeasure(t, text, "Arial", 18.0, 3*914400, 5)
	m2 := mustMeasure(t, text, "Arial", 18.0, 3*914400, 5)
	if m1 != m2 {
		t.Errorf("MeasureRun should be pure: first=%+v second=%+v", m1, m2)
	}
}
