package generator

import (
	"strings"
	"testing"

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
