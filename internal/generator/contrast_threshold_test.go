package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

// go-slide-creator-9ux4: every contrast fix targeted the WCAG AA *large text*
// ratio of 3.0:1, on the stated assumption that "presentation text is almost
// always >= 18pt or >= 14pt bold" — false for the 11pt supporting line a card
// carries. Swaps landed at exactly 3.0 and the text rendered barely visible.
func TestContrastThresholdFor(t *testing.T) {
	tests := []struct {
		name   string
		textPt float64
		bold   bool
		want   float64
	}{
		{"11pt card support line", 11, false, svggen.WCAGAANormal},
		{"12pt body", 12, false, svggen.WCAGAANormal},
		{"14pt not bold is still normal text", 14, false, svggen.WCAGAANormal},
		{"14pt bold is large text", 14, true, svggen.WCAGAALarge},
		{"17.9pt is not large", 17.9, false, svggen.WCAGAANormal},
		{"18pt is large", 18, false, svggen.WCAGAALarge},
		{"28pt KPI value is large", 28, false, svggen.WCAGAALarge},
		{"unknown size is treated as small", 0, false, svggen.WCAGAANormal},
		{"13pt bold is below the bold bar", 13, true, svggen.WCAGAANormal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contrastThresholdFor(tt.textPt, tt.bold); got != tt.want {
				t.Errorf("contrastThresholdFor(%v, %v) = %v, want %v", tt.textPt, tt.bold, got, tt.want)
			}
		})
	}
}

// The three reported swaps must now clear the normal-text ratio, not land at
// exactly 3.0.
func TestContrastReplacement_SmallTextReachesAANormal(t *testing.T) {
	cases := []struct {
		name   string
		fgHex  string
		bgHex  string
		textPt float64
	}{
		{"grey on light grey", "#CCCCCC", "#DDDDDD", 11},
		{"dark grey on dark", "#444444", "#333333", 11},
		{"sage on sage", "#9ACD9A", "#8FBC8F", 11},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fg := svggen.MustParseColor(c.fgHex)
			bg := svggen.MustParseColor(c.bgHex)
			threshold := contrastThresholdFor(c.textPt, false)

			fixed, _ := contrastReplacement(c.fgHex, fg, bg, nil, threshold, false)
			ratio := fixed.ContrastWith(bg)

			if ratio < svggen.WCAGAANormal {
				t.Errorf("%s → %s on %s: ratio %.2f, want >= %.1f for %gpt text",
					c.fgHex, fixed.Hex(), c.bgHex, ratio, svggen.WCAGAANormal, c.textPt)
			}
		})
	}
}

// Genuinely large text keeps the 3:1 target, so display type is not darkened
// unnecessarily.
func TestContrastReplacement_LargeTextKeepsLargeThreshold(t *testing.T) {
	fg := svggen.MustParseColor("#9ACD9A")
	bg := svggen.MustParseColor("#8FBC8F")

	large, _ := contrastReplacement("#9ACD9A", fg, bg, nil, contrastThresholdFor(28, false), false)
	small, _ := contrastReplacement("#9ACD9A", fg, bg, nil, contrastThresholdFor(11, false), false)

	largeRatio := large.ContrastWith(bg)
	smallRatio := small.ContrastWith(bg)

	if largeRatio < svggen.WCAGAALarge {
		t.Errorf("large text ratio %.2f is below the large-text bar", largeRatio)
	}
	if smallRatio <= largeRatio {
		t.Errorf("small text should be pushed further than large text: small %.2f, large %.2f",
			smallRatio, largeRatio)
	}
}

// smallestTextPt drives the shape-grid threshold: a body mixing a KPI value
// with an 11pt label must be fixed for the label.
func TestSmallestTextPt(t *testing.T) {
	tests := []struct {
		name     string
		fragment string
		wantPt   float64
		wantBold bool
	}{
		{
			name:     "KPI value beside a small label",
			fragment: `<a:r><a:rPr sz="2800" b="1"/><a:t>$12.4M</a:t></a:r><a:r><a:rPr sz="1100" b="0"/><a:t>ARR</a:t></a:r>`,
			wantPt:   11,
			wantBold: false,
		},
		{
			name:     "all bold",
			fragment: `<a:r><a:rPr sz="1400" b="1"/><a:t>Header</a:t></a:r>`,
			wantPt:   14,
			wantBold: true,
		},
		{
			name:     "no declared size falls back to the body default",
			fragment: `<a:r><a:rPr/><a:t>text</a:t></a:r>`,
			wantPt:   defaultBodyTextPt,
			wantBold: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt, bold := smallestTextPt(tt.fragment)
			if pt != tt.wantPt {
				t.Errorf("size = %v, want %v", pt, tt.wantPt)
			}
			if bold != tt.wantBold {
				t.Errorf("bold = %v, want %v", bold, tt.wantBold)
			}
		})
	}
}
