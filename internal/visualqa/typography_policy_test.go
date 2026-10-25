package visualqa

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
)

func TestTypographyPolicyMatchesGenerationRoles(t *testing.T) {
	for _, name := range []string{"present", "read"} {
		p := TypographyPolicy(name)
		mode := tokens.ParseViewingMode(name)
		for _, tc := range []struct {
			label string
			role  tokens.TextRole
		}{{"body", tokens.TextRoleBody}, {"card body", tokens.TextRoleCardBody}, {"card titles", tokens.TextRoleCardTitle}, {"captions/dense tables", tokens.TextRoleCaption}} {
			want := fmt.Sprintf("%s %.0fpt", tc.label, float64(tokens.MinReadableHPt(mode, tc.role))/100)
			if !strings.Contains(p, want) {
				t.Errorf("%s: missing %q in %s", name, want, p)
			}
		}
		if !strings.Contains(p, fmt.Sprintf("intentionally %.0fpt", tokens.SourceLinePt)) {
			t.Errorf("source policy absent: %s", p)
		}
	}
}
