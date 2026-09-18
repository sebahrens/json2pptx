package patterns

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func TestAnchorSparseText(t *testing.T) {
	text := json.RawMessage(`{"paragraphs":[{"content":"x","size":12}],"vertical_align":"t"}`)
	var got struct {
		VerticalAlign string `json:"vertical_align"`
	}
	_ = json.Unmarshal(anchorSparseText(text, 10, 100), &got)
	if got.VerticalAlign != "ctr" {
		t.Errorf("sparse text should be centred, got %q", got.VerticalAlign)
	}
	_ = json.Unmarshal(anchorSparseText(text, 80, 100), &got)
	if got.VerticalAlign != "t" {
		t.Errorf("dense text keeps top anchor, got %q", got.VerticalAlign)
	}
	if string(anchorSparseText(json.RawMessage(`"plain"`), 1, 100)) != `"plain"` {
		t.Error("non-object text is returned unchanged")
	}
}

func TestShapeTextHeightPt(t *testing.T) {
	one := shapeTextHeightPt("", json.RawMessage(`{"paragraphs":[{"content":"Header","size":16,"bold":true}]}`), 300)
	two := shapeTextHeightPt("", json.RawMessage(`{"paragraphs":[{"content":"Header","size":16,"bold":true},{"content":"<b>Body</b> text","size":12}]}`), 300)
	if one <= 0 || two <= one {
		t.Errorf("heights must grow with paragraphs: one=%g two=%g", one, two)
	}
	if h := shapeTextHeightPt("", json.RawMessage(`"plain string"`), 300); h <= 0 {
		t.Errorf("string text must measure, got %g", h)
	}
	if h := shapeTextHeightPt("", json.RawMessage(`[1,2]`), 300); h != 0 {
		t.Errorf("unparseable text measures 0, got %g", h)
	}
}

func TestHeaderRowAndCardHeights(t *testing.T) {
	h := headerRowPt("", []string{"Short", "A somewhat longer header"}, 16, 400)
	if h < 16*1.2 || h > 50 {
		t.Errorf("one-line 16pt header band = %g, want ~1.2x line height + padding", h)
	}
	if h := headerRowPt("", nil, 16, 400); h <= 0 {
		t.Errorf("empty headers still get a band, got %g", h)
	}
	plain := contentCardHeightPt(40, 300, false)
	withIcon := contentCardHeightPt(40, 300, true)
	if withIcon <= plain {
		t.Errorf("top icon must add height: %g vs %g", withIcon, plain)
	}
	if narrow := contentCardHeightPt(40, 60, true); narrow <= plain {
		t.Errorf("narrow card with icon must add a width-derived zone, got %g", narrow)
	}
}

func TestApplyGridDefaults(t *testing.T) {
	ApplyGridDefaults(nil)
	g := &jsonschema.ShapeGridInput{}
	ApplyGridDefaults(g)
	if g.VerticalAlign != GridVerticalAlignDefault {
		t.Errorf("default vertical_align = %q", g.VerticalAlign)
	}
	g = &jsonschema.ShapeGridInput{VerticalAlign: "top"}
	ApplyGridDefaults(g)
	if g.VerticalAlign != "top" {
		t.Error("pattern-set vertical_align must be kept")
	}
}
