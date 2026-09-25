package generator

import (
	"bytes"
	"strings"
	"testing"
)

func TestProcessFlowStatusFillsRetainAccentWithoutNeonBrightening(t *testing.T) {
	tests := []struct {
		name   string
		kind   processFlowStepType
		accent string
	}{
		{"decision", pfDecisionType, "accent3"},
		{"start", pfStartType, "accent6"},
		{"end", pfEndType, "accent6"},
		{"subprocess", pfSubprocessType, "accent2"},
		{"regular", pfStepType, "accent1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fill, line := pfColorsForStepType(tt.kind)
			var body, outline bytes.Buffer
			fill.WriteTo(&body)
			line.Fill.WriteTo(&outline)
			want := `<a:schemeClr val="` + tt.accent + `"><a:tint val="20000"/></a:schemeClr>`
			if !strings.Contains(body.String(), want) || strings.Contains(body.String(), "lumOff") {
				t.Fatalf("fill = %s, want a soft %s tint", body.String(), tt.accent)
			}
			if !strings.Contains(outline.String(), `<a:schemeClr val="`+tt.accent+`"/>`) {
				t.Fatalf("outline = %s, want full accent %s", outline.String(), tt.accent)
			}
		})
	}
}
