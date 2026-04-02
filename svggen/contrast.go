package svggen

import "math"

// WCAG contrast ratio thresholds.
const (
	// WCAGAANormal is the minimum contrast ratio for normal text (< 18pt or < 14pt bold)
	// per WCAG 2.1 AA.
	WCAGAANormal = 4.5

	// WCAGAALarge is the minimum contrast ratio for large text (>= 18pt or >= 14pt bold)
	// per WCAG 2.1 AA.
	WCAGAALarge = 3.0

	// WCAGAAANormal is the minimum contrast ratio for normal text per WCAG 2.1 AAA.
	WCAGAAANormal = 7.0

	// WCAGAAALarge is the minimum contrast ratio for large text per WCAG 2.1 AAA.
	WCAGAAALarge = 4.5
)

// EnsureContrast adjusts fg so that its contrast ratio against bg meets or
// exceeds minRatio. If the pair already meets the requirement, fg is returned
// unchanged. Semi-transparent colors are composited over white before the
// contrast check (via Opaque) so the calculation reflects actual on-screen
// appearance.
//
// The algorithm first tries to push fg toward black or white (whichever
// direction yields better contrast) in small incremental steps. If the
// iterative approach cannot reach the target ratio, the function falls back
// to pure black or pure white -- whichever provides higher contrast against bg.
func EnsureContrast(fg, bg Color, minRatio float64) Color {
	// Composite semi-transparent colors over white for accurate contrast.
	fgOpaque := fg.Opaque()
	bgOpaque := bg.Opaque()

	// Already compliant -- return fg unmodified.
	if fgOpaque.ContrastWith(bgOpaque) >= minRatio {
		return fg
	}

	// Determine adjustment direction: darken toward black or lighten toward white.
	// Pick whichever extreme gives higher contrast with bg.
	black := Color{R: 0, G: 0, B: 0, A: 1.0}
	white := Color{R: 255, G: 255, B: 255, A: 1.0}

	blackContrast := black.ContrastWith(bgOpaque)
	whiteContrast := white.ContrastWith(bgOpaque)

	// If neither extreme can meet the ratio, pick the best one and return.
	if blackContrast < minRatio && whiteContrast < minRatio {
		if blackContrast >= whiteContrast {
			return black.WithAlpha(fg.A)
		}
		return white.WithAlpha(fg.A)
	}

	// Binary search for the minimum adjustment amount that meets the ratio.
	// We blend fg toward the target extreme (black or white).
	var target Color
	if blackContrast >= whiteContrast {
		target = black
	} else {
		target = white
	}

	lo := 0.0
	hi := 1.0
	result := fgOpaque

	for i := 0; i < 32; i++ { // 32 iterations gives ~1e-10 precision
		mid := (lo + hi) / 2
		candidate := lerpColors(fgOpaque, target, mid)
		ratio := candidate.ContrastWith(bgOpaque)

		if ratio >= minRatio {
			result = candidate
			hi = mid // Try less aggressive adjustment
		} else {
			lo = mid // Need more aggressive adjustment
		}
	}

	// Verify the result actually meets the ratio (defensive).
	if result.ContrastWith(bgOpaque) < minRatio {
		// Fall back to the extreme.
		if blackContrast >= whiteContrast {
			return black.WithAlpha(fg.A)
		}
		return white.WithAlpha(fg.A)
	}

	return result.WithAlpha(fg.A)
}

// EnsureWCAGAA is a convenience wrapper that enforces WCAG AA contrast for
// normal text (4.5:1 ratio).
func EnsureWCAGAA(fg, bg Color) Color {
	return EnsureContrast(fg, bg, WCAGAANormal)
}

// EnsureWCAGAALarge is a convenience wrapper that enforces WCAG AA contrast
// for large text (3:1 ratio). Large text is defined as >= 18pt or >= 14pt bold.
func EnsureWCAGAALarge(fg, bg Color) Color {
	return EnsureContrast(fg, bg, WCAGAALarge)
}

// lerpColors linearly interpolates between a and b by factor t (0..1).
// At t=0 returns a, at t=1 returns b.
func lerpColors(a, b Color, t float64) Color {
	t = math.Max(0, math.Min(1, t))
	return Color{
		R: uint8(math.Round(float64(a.R)*(1-t) + float64(b.R)*t)),
		G: uint8(math.Round(float64(a.G)*(1-t) + float64(b.G)*t)),
		B: uint8(math.Round(float64(a.B)*(1-t) + float64(b.B)*t)),
		A: 1.0,
	}
}
