package shapegrid

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// go-slide-creator-zieyk: paragraphs[].bullet emits a real <a:buChar> with
// a hanging indent so wrapped lines align with the text, not the marker.
func TestResolveTextInput_ParagraphBullet(t *testing.T) {
	raw := json.RawMessage(`{"paragraphs":[
		{"content":"Heading","bold":true},
		{"content":"Default bullet","bullet":true},
		{"content":"Dash bullet","bullet":"–"},
		{"content":"Plain","bullet":false}
	],"align":"l"}`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Paragraphs) != 4 {
		t.Fatalf("paragraphs = %d", len(tb.Paragraphs))
	}
	for i, want := range []string{"", pptx.DefaultBulletChar, "–", ""} {
		p := tb.Paragraphs[i]
		if want == "" {
			if p.Bullet != nil || p.MarginL != 0 {
				t.Errorf("paragraph %d should not be bulleted: %+v", i, p)
			}
			continue
		}
		if p.Bullet == nil || p.Bullet.Char != want {
			t.Fatalf("paragraph %d bullet = %+v, want %q", i, p.Bullet, want)
		}
		// Default 14pt text: a 0.65em (9.1pt) hang, never above the 14pt
		// placeholder bullet margin.
		hang := int64(BulletHangPt(14) * 12700)
		if p.MarginL != hang || p.Indent != -hang || hang > pptx.BulletMarginLeft || hang <= 0 {
			t.Errorf("paragraph %d hanging indent = %d/%d, want %d/-%d", i, p.MarginL, p.Indent, hang, hang)
		}
		if strings.HasPrefix(p.Runs[0].Text, want) {
			t.Errorf("paragraph %d text carries the marker: %q", i, p.Runs[0].Text)
		}
	}
	var buf bytes.Buffer
	tb.WriteTo(&buf)
	xml := buf.String()
	if !strings.Contains(xml, `indent="-`) || !strings.Contains(xml, `<a:buChar char="–"/>`) {
		t.Errorf("missing hanging-indent bullet XML: %s", xml)
	}
}

func TestResolveTextInput_ParagraphBulletRejectsLongMarker(t *testing.T) {
	if _, err := ResolveTextInput(json.RawMessage(`{"paragraphs":[{"content":"x","bullet":"bullet"}]}`)); err == nil {
		t.Error("a multi-character marker should be rejected")
	}
}
