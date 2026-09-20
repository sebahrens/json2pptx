package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TestPFStepHeightScalesWithTheSpace pins the fix for go-slide-creator-rkq0:
// a process step was always the 0.5" minimum, so a five-step flow drew a thin
// strip of boxes in the middle of a 4.75" body placeholder.
func TestPFStepHeightScalesWithTheSpace(t *testing.T) {
	const inch = int64(914400)
	tests := []struct {
		name   string
		height int64
		want   int64
	}{
		{"a body placeholder gets a readable box", 4*inch + inch*3/4, pfMinStepHeight * pfMaxStepHeightFactor},
		{"a short band keeps the minimum", inch, pfMinStepHeight},
		{"a half-height zone lands between", 2 * inch, 2 * inch / pfStepHeightDivisor},
		{"no bounds falls back to the minimum", 0, pfMinStepHeight},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pfStepHeight(types.BoundingBox{Width: 10 * inch, Height: tt.height})
			if got != tt.want {
				t.Errorf("pfStepHeight(h=%d) = %d, want %d", tt.height, got, tt.want)
			}
		})
	}

	// The cap holds however tall the placeholder is: a flow is a strip.
	if got := pfStepHeight(types.BoundingBox{Width: 10 * inch, Height: 20 * inch}); got != pfMinStepHeight*pfMaxStepHeightFactor {
		t.Errorf("pfStepHeight on a very tall zone = %d, want the %d cap", got, pfMinStepHeight*pfMaxStepHeightFactor)
	}
}
