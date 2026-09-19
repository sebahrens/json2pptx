package generator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// go-slide-creator-xg48. An author who writes "Source: Company filings
// FY2022-FY2026" got "Source: Source: Company filings FY2022-FY2026" on the
// slide — the band labels the line unconditionally.
func TestSourceNoteText(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"labels a bare citation", "Company filings FY2026", "Source: Company filings FY2026"},
		{"keeps an author's own label", "Source: Company filings FY2026", "Source: Company filings FY2026"},
		{"case-insensitive", "source: internal", "source: internal"},
		{"SOURCE: shouting", "SOURCE: internal", "SOURCE: internal"},
		{"trims", "  Company filings  ", "Source: Company filings"},
		{"a word starting with source is still labelled", "Sourcing data from ERP", "Source: Sourcing data from ERP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sourceNoteText(tt.in); got != tt.want {
				t.Errorf("sourceNoteText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSourceNoteShapeDoesNotStutter(t *testing.T) {
	xml := generateSourceNoteShapeInBounds("Source: Company filings FY2026", 42,
		pptx.RectEmu{X: 0, Y: 0, CX: 1000, CY: 100})
	if strings.Contains(xml, "Source: Source:") {
		t.Errorf("emitted a stuttered label:\n%s", xml)
	}
	if !strings.Contains(xml, "Source: Company filings FY2026") {
		t.Errorf("source text missing from the shape:\n%s", xml)
	}
}

// The slide-level source band and the pattern-drawn source lines are one
// convention now: same prefix, size, colour and alignment. The band used to be
// 8pt in a raw #888888 on the right while the patterns drew 9-10pt dk1 on the
// left (go-slide-creator-7eib).
func TestSourceNoteFollowsTheSharedConvention(t *testing.T) {
	xml := generateSourceNoteShapeInBounds("Helio finance data warehouse", 42,
		pptx.RectEmu{X: 0, Y: 0, CX: 5000000, CY: 300000})
	if xml == "" {
		t.Fatal("no shape generated")
	}
	if strings.Contains(xml, "888888") {
		t.Error("the source note still paints a raw hex off the template palette")
	}
	if !strings.Contains(xml, patterns.SourceNoteScheme) {
		t.Errorf("the source note does not use the shared %q scheme colour:\n%s", patterns.SourceNoteScheme, xml)
	}
	wantSize := fmt.Sprintf(`sz="%d"`, int(patterns.SourceNoteSizePt*100))
	if !strings.Contains(xml, wantSize) {
		t.Errorf("the source note is not set at the shared size (%s):\n%s", wantSize, xml)
	}
	if !strings.Contains(xml, `algn="`+patterns.SourceNoteAlign+`"`) {
		t.Errorf("the source note is not aligned the shared way (%q):\n%s", patterns.SourceNoteAlign, xml)
	}
	if !strings.Contains(xml, "Source: Helio finance data warehouse") {
		t.Errorf("the source note lost its label:\n%s", xml)
	}
}
