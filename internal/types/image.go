package types

import "math"

// ImageFocalPoint is the subject position in source-image fractions. A cover
// crop moves its window toward this point while preserving the frame and ratio.
type ImageFocalPoint struct {
	X float64 `json:"x" yaml:"x"`
	Y float64 `json:"y" yaml:"y"`
}

// Valid rejects nonfinite and out-of-image coordinates. Nil means centered.
func (p *ImageFocalPoint) Valid() bool {
	return p == nil || (!math.IsNaN(p.X) && !math.IsNaN(p.Y) && !math.IsInf(p.X, 0) && !math.IsInf(p.Y, 0) && p.X >= 0 && p.X <= 1 && p.Y >= 0 && p.Y <= 1)
}
