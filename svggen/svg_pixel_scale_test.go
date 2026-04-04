package svggen

import (
	"math"
	"strings"
	"testing"
)

// approxEqual checks if two floats are within a tolerance.
func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// --- scalePathData ---

func TestScalePathData_SimpleMoveAndLine(t *testing.T) {
	input := "M10 20L30 40"
	result := scalePathData(input)

	// M10 → 10*3.7795... ≈ 37.7953, M20 → 20*3.7795... ≈ 75.5906
	// L30 → ≈113.3858, L40 → ≈151.1811
	expectSubstrings := []string{"M", "L"}
	for _, sub := range expectSubstrings {
		if !strings.Contains(result, sub) {
			t.Errorf("expected result to contain %q, got %q", sub, result)
		}
	}

	// Parse out the numbers and verify scaling
	// The result should be approximately "M37.7953 75.5906L113.3858 151.1811"
	// We verify by checking the formatted values
	exp10 := formatScaledNum(10 * mmToPxFactor)
	exp20 := formatScaledNum(20 * mmToPxFactor)
	exp30 := formatScaledNum(30 * mmToPxFactor)
	exp40 := formatScaledNum(40 * mmToPxFactor)

	expected := "M" + exp10 + " " + exp20 + "L" + exp30 + " " + exp40
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_HVCommands(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"H command", "H50"},
		{"V command", "V60"},
		{"H and V", "H50V60"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := scalePathData(tc.input)
			switch tc.input {
			case "H50":
				expected := "H" + formatScaledNum(50*mmToPxFactor)
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			case "V60":
				expected := "V" + formatScaledNum(60*mmToPxFactor)
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			case "H50V60":
				expected := "H" + formatScaledNum(50*mmToPxFactor) + "V" + formatScaledNum(60*mmToPxFactor)
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			}
		})
	}
}

func TestScalePathData_ArcCommand(t *testing.T) {
	// A rx ry rotation large-arc sweep x y
	// A5 10 30 0 1 20 30
	// Indices: rx(0)=5 ry(1)=10 rotation(2)=30 large-arc(3)=0 sweep(4)=1 x(5)=20 y(6)=30
	// rx, ry, x, y should be scaled; rotation, large-arc, sweep should NOT be scaled.
	input := "A5 10 30 0 1 20 30"
	result := scalePathData(input)

	expRx := formatScaledNum(5 * mmToPxFactor)
	expRy := formatScaledNum(10 * mmToPxFactor)
	expRot := "30"     // NOT scaled
	expLargeArc := "0" // NOT scaled
	expSweep := "1"    // NOT scaled
	expX := formatScaledNum(20 * mmToPxFactor)
	expY := formatScaledNum(30 * mmToPxFactor)

	expected := "A" + expRx + " " + expRy + " " + expRot + " " + expLargeArc + " " + expSweep + " " + expX + " " + expY
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_ArcConcatenatedFlags(t *testing.T) {
	// The canvas library outputs arc flags concatenated with the next coordinate
	// without separators: "A64.3 64.3 0 0140 37.9" where 01 = flags 0,1 and
	// 40 is the x coordinate. This is valid SVG but tricky to parse.
	input := "A5 10 0 0120 30"
	result := scalePathData(input)

	expRx := formatScaledNum(5 * mmToPxFactor)
	expRy := formatScaledNum(10 * mmToPxFactor)
	expX := formatScaledNum(20 * mmToPxFactor)
	expY := formatScaledNum(30 * mmToPxFactor)

	// Flags should be preserved as single characters, coordinates should be scaled
	expected := "A" + expRx + " " + expRy + " 0 0" + "1" + expX + " " + expY
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_ArcConcatenatedFlagsRealWorld(t *testing.T) {
	// Real pattern from canvas library pie chart arc output:
	// A64.28 64.28 0 01151.80 143.15
	// = rx=64.28 ry=64.28 rotation=0 large-arc=0 sweep=1 x=151.80 y=143.15
	input := "A64.28 64.28 0 01151.80 143.15"
	result := scalePathData(input)

	expRx := formatScaledNum(64.28 * mmToPxFactor)
	expRy := formatScaledNum(64.28 * mmToPxFactor)
	expX := formatScaledNum(151.80 * mmToPxFactor)
	expY := formatScaledNum(143.15 * mmToPxFactor)

	// Flags 0 and 1 should be single chars, not consumed as part of "01151.80"
	expected := "A" + expRx + " " + expRy + " 0 0" + "1" + expX + " " + expY
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_ArcBothFlagsConcatenated(t *testing.T) {
	// Both flags concatenated together: "0010" = large-arc=0 sweep=0 x=10
	input := "A5 5 0 0010 20"
	result := scalePathData(input)

	expR := formatScaledNum(5 * mmToPxFactor)
	expX := formatScaledNum(10 * mmToPxFactor)
	expY := formatScaledNum(20 * mmToPxFactor)

	expected := "A" + expR + " " + expR + " 0 0" + "0" + expX + " " + expY
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_ZCommand(t *testing.T) {
	input := "M0 0L10 10Z"
	result := scalePathData(input)

	exp0 := formatScaledNum(0 * mmToPxFactor)
	exp10 := formatScaledNum(10 * mmToPxFactor)

	expected := "M" + exp0 + " " + exp0 + "L" + exp10 + " " + exp10 + "Z"
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_ImplicitRepeatedCommands(t *testing.T) {
	// After M, extra coordinate pairs are treated as implicit L commands.
	// M0 0 10 20 30 40 → all 6 values should be scaled.
	input := "M0 0 10 20 30 40"
	result := scalePathData(input)

	exp0 := formatScaledNum(0 * mmToPxFactor)
	exp10 := formatScaledNum(10 * mmToPxFactor)
	exp20 := formatScaledNum(20 * mmToPxFactor)
	exp30 := formatScaledNum(30 * mmToPxFactor)
	exp40 := formatScaledNum(40 * mmToPxFactor)

	expected := "M" + exp0 + " " + exp0 + " " + exp10 + " " + exp20 + " " + exp30 + " " + exp40
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_CubicBezier(t *testing.T) {
	// C x1 y1 x2 y2 x y → all 6 values scaled.
	input := "C1 2 3 4 5 6"
	result := scalePathData(input)

	exp1 := formatScaledNum(1 * mmToPxFactor)
	exp2 := formatScaledNum(2 * mmToPxFactor)
	exp3 := formatScaledNum(3 * mmToPxFactor)
	exp4 := formatScaledNum(4 * mmToPxFactor)
	exp5 := formatScaledNum(5 * mmToPxFactor)
	exp6 := formatScaledNum(6 * mmToPxFactor)

	expected := "C" + exp1 + " " + exp2 + " " + exp3 + " " + exp4 + " " + exp5 + " " + exp6
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_RelativeCommands(t *testing.T) {
	// Relative commands (lowercase) should also have their params scaled.
	input := "m5 5l10 10"
	result := scalePathData(input)

	exp5 := formatScaledNum(5 * mmToPxFactor)
	exp10 := formatScaledNum(10 * mmToPxFactor)

	expected := "m" + exp5 + " " + exp5 + "l" + exp10 + " " + exp10
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_NegativeCoordinates(t *testing.T) {
	input := "M-5 -10L-20 30"
	result := scalePathData(input)

	expNeg5 := formatScaledNum(-5 * mmToPxFactor)
	expNeg10 := formatScaledNum(-10 * mmToPxFactor)
	expNeg20 := formatScaledNum(-20 * mmToPxFactor)
	exp30 := formatScaledNum(30 * mmToPxFactor)

	expected := "M" + expNeg5 + " " + expNeg10 + "L" + expNeg20 + " " + exp30
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_DecimalValues(t *testing.T) {
	input := "M1.5 2.75"
	result := scalePathData(input)

	exp15 := formatScaledNum(1.5 * mmToPxFactor)
	exp275 := formatScaledNum(2.75 * mmToPxFactor)

	expected := "M" + exp15 + " " + exp275
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_QuadraticBezier(t *testing.T) {
	// Q x1 y1 x y → all 4 values scaled.
	input := "Q10 20 30 40"
	result := scalePathData(input)

	exp10 := formatScaledNum(10 * mmToPxFactor)
	exp20 := formatScaledNum(20 * mmToPxFactor)
	exp30 := formatScaledNum(30 * mmToPxFactor)
	exp40 := formatScaledNum(40 * mmToPxFactor)

	expected := "Q" + exp10 + " " + exp20 + " " + exp30 + " " + exp40
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_SmoothCubic(t *testing.T) {
	// S x2 y2 x y → all 4 values scaled.
	input := "S10 20 30 40"
	result := scalePathData(input)

	exp10 := formatScaledNum(10 * mmToPxFactor)
	exp20 := formatScaledNum(20 * mmToPxFactor)
	exp30 := formatScaledNum(30 * mmToPxFactor)
	exp40 := formatScaledNum(40 * mmToPxFactor)

	expected := "S" + exp10 + " " + exp20 + " " + exp30 + " " + exp40
	if result != expected {
		t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", input, result, expected)
	}
}

func TestScalePathData_EmptyString(t *testing.T) {
	result := scalePathData("")
	if result != "" {
		t.Errorf("scalePathData(\"\") = %q, want \"\"", result)
	}
}

// --- scaleCoordAttributes ---

func TestScaleCoordAttributes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		checkFn  func(t *testing.T, result string)
	}{
		{
			name:  "single x attribute",
			input: `x="10"`,
			checkFn: func(t *testing.T, result string) {
				expected := `x="` + formatScaledNum(10*mmToPxFactor) + `"`
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:  "single y attribute",
			input: `y="25.4"`,
			checkFn: func(t *testing.T, result string) {
				// 25.4mm * 96/25.4 = 96px exactly
				expected := `y="` + formatScaledNum(25.4*mmToPxFactor) + `"`
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:  "multiple coord attributes in tag",
			input: `<rect x="5" y="10" width="20" height="15"/>`,
			checkFn: func(t *testing.T, result string) {
				// x and y should be scaled; width and height should NOT be matched
				expX := `x="` + formatScaledNum(5*mmToPxFactor) + `"`
				expY := `y="` + formatScaledNum(10*mmToPxFactor) + `"`
				if !strings.Contains(result, expX) {
					t.Errorf("expected result to contain %q, got %q", expX, result)
				}
				if !strings.Contains(result, expY) {
					t.Errorf("expected result to contain %q, got %q", expY, result)
				}
				// width should remain unchanged
				if !strings.Contains(result, `width="20"`) {
					t.Errorf("expected width to remain unchanged, got %q", result)
				}
				// height should remain unchanged
				if !strings.Contains(result, `height="15"`) {
					t.Errorf("expected height to remain unchanged, got %q", result)
				}
			},
		},
		{
			name:  "non-numeric value keeps original",
			input: `x="auto"`,
			checkFn: func(t *testing.T, result string) {
				// "auto" is not a number, so the match callback returns original
				if result != `x="auto"` {
					t.Errorf("expected non-numeric value to be preserved, got %q", result)
				}
			},
		},
		{
			name:  "cx and cy attributes",
			input: `<circle cx="50" cy="50"/>`,
			checkFn: func(t *testing.T, result string) {
				expCx := `cx="` + formatScaledNum(50*mmToPxFactor) + `"`
				expCy := `cy="` + formatScaledNum(50*mmToPxFactor) + `"`
				if !strings.Contains(result, expCx) {
					t.Errorf("expected cx to be scaled, got %q", result)
				}
				if !strings.Contains(result, expCy) {
					t.Errorf("expected cy to be scaled, got %q", result)
				}
			},
		},
		{
			name:  "r attribute",
			input: `<circle r="10"/>`,
			checkFn: func(t *testing.T, result string) {
				expR := `r="` + formatScaledNum(10*mmToPxFactor) + `"`
				if !strings.Contains(result, expR) {
					t.Errorf("expected r to be scaled, got %q", result)
				}
			},
		},
		{
			name:  "line x1 y1 x2 y2 attributes",
			input: `<line x1="0" y1="0" x2="100" y2="50"/>`,
			checkFn: func(t *testing.T, result string) {
				expX1 := `x1="` + formatScaledNum(0*mmToPxFactor) + `"`
				expY1 := `y1="` + formatScaledNum(0*mmToPxFactor) + `"`
				expX2 := `x2="` + formatScaledNum(100*mmToPxFactor) + `"`
				expY2 := `y2="` + formatScaledNum(50*mmToPxFactor) + `"`
				for _, exp := range []string{expX1, expY1, expX2, expY2} {
					if !strings.Contains(result, exp) {
						t.Errorf("expected result to contain %q, got %q", exp, result)
					}
				}
			},
		},
		{
			name:  "gradient fx fy fr attributes",
			input: `<radialGradient fx="25" fy="25" fr="5"/>`,
			checkFn: func(t *testing.T, result string) {
				expFx := `fx="` + formatScaledNum(25*mmToPxFactor) + `"`
				expFy := `fy="` + formatScaledNum(25*mmToPxFactor) + `"`
				expFr := `fr="` + formatScaledNum(5*mmToPxFactor) + `"`
				for _, exp := range []string{expFx, expFy, expFr} {
					if !strings.Contains(result, exp) {
						t.Errorf("expected result to contain %q, got %q", exp, result)
					}
				}
			},
		},
		{
			name:  "zero value",
			input: `x="0"`,
			checkFn: func(t *testing.T, result string) {
				expected := `x="` + formatScaledNum(0) + `"`
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := scaleCoordAttributes(tc.input)
			tc.checkFn(t, result)
		})
	}
}

// --- scaleFontSizes ---

func TestScaleFontSizes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, result string)
	}{
		{
			name:  "simple font size",
			input: `font: 5px Arial`,
			check: func(t *testing.T, result string) {
				// 5 * 3.7795... ≈ 18.8976
				expSize := formatScaledNum(5 * mmToPxFactor)
				expected := `font: ` + expSize + `px Arial`
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:  "font with bold weight",
			input: `font: bold 3.5px serif`,
			check: func(t *testing.T, result string) {
				expSize := formatScaledNum(3.5 * mmToPxFactor)
				expected := `font: bold ` + expSize + `px serif`
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:  "multiple font sizes in one string",
			input: `style="font: 5px Arial" other="font: 10px Mono"`,
			check: func(t *testing.T, result string) {
				exp5 := formatScaledNum(5 * mmToPxFactor)
				exp10 := formatScaledNum(10 * mmToPxFactor)
				if !strings.Contains(result, exp5+"px") {
					t.Errorf("expected scaled 5px value in result, got %q", result)
				}
				if !strings.Contains(result, exp10+"px") {
					t.Errorf("expected scaled 10px value in result, got %q", result)
				}
			},
		},
		{
			name:  "no match without trailing whitespace after px",
			input: `50pxl`,
			check: func(t *testing.T, result string) {
				// The regex requires a whitespace char after "px", so "50pxl" should NOT match.
				if result != `50pxl` {
					t.Errorf("expected no change for %q, got %q", `50pxl`, result)
				}
			},
		},
		{
			name:  "integer font size",
			input: `font: 12px sans-serif`,
			check: func(t *testing.T, result string) {
				expSize := formatScaledNum(12 * mmToPxFactor)
				expected := `font: ` + expSize + `px sans-serif`
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:  "no font size present",
			input: `fill: #ff0000; stroke: black`,
			check: func(t *testing.T, result string) {
				if result != `fill: #ff0000; stroke: black` {
					t.Errorf("expected no change, got %q", result)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := scaleFontSizes(tc.input)
			tc.check(t, result)
		})
	}
}

// --- scaleStrokeWidths ---

func TestScaleStrokeWidths(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "decimal stroke width",
			input:    "stroke-width:0.5",
			expected: "stroke-width:" + formatScaledNum(0.5*mmToPxFactor),
		},
		{
			name:     "integer stroke width",
			input:    "stroke-width:2",
			expected: "stroke-width:" + formatScaledNum(2*mmToPxFactor),
		},
		{
			name:     "stroke width in style context",
			input:    `style="stroke:#000;stroke-width:1;fill:none"`,
			expected: `style="stroke:#000;stroke-width:` + formatScaledNum(1*mmToPxFactor) + `;fill:none"`,
		},
		{
			name:     "no stroke width present",
			input:    `style="fill:red"`,
			expected: `style="fill:red"`,
		},
		{
			name:     "large stroke width",
			input:    "stroke-width:10",
			expected: "stroke-width:" + formatScaledNum(10*mmToPxFactor),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := scaleStrokeWidths(tc.input)
			if result != tc.expected {
				t.Errorf("scaleStrokeWidths(%q)\n  got  %q\n  want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// --- scaleTranslateTransforms ---

func TestScaleTranslateTransforms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "translate with two values",
			input:    "translate(10,20)",
			expected: "translate(" + formatScaledNum(10*mmToPxFactor) + "," + formatScaledNum(20*mmToPxFactor) + ")",
		},
		{
			name:     "translate with single value",
			input:    "translate(5)",
			expected: "translate(" + formatScaledNum(5*mmToPxFactor) + ")",
		},
		{
			name:     "translate in transform attribute",
			input:    `transform="translate(3,7)"`,
			expected: `transform="translate(` + formatScaledNum(3*mmToPxFactor) + "," + formatScaledNum(7*mmToPxFactor) + `)"`,
		},
		{
			name:     "translate with spaces around values",
			input:    "translate(10, 20)",
			expected: "translate(" + formatScaledNum(10*mmToPxFactor) + "," + formatScaledNum(20*mmToPxFactor) + ")",
		},
		{
			name:     "no translate present",
			input:    `transform="rotate(45)"`,
			expected: `transform="rotate(45)"`,
		},
		{
			name:     "translate with zero values",
			input:    "translate(0,0)",
			expected: "translate(" + formatScaledNum(0) + "," + formatScaledNum(0) + ")",
		},
		{
			name:     "translate with negative values",
			input:    "translate(-5,10)",
			expected: "translate(" + formatScaledNum(-5*mmToPxFactor) + "," + formatScaledNum(10*mmToPxFactor) + ")",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := scaleTranslateTransforms(tc.input)
			if result != tc.expected {
				t.Errorf("scaleTranslateTransforms(%q)\n  got  %q\n  want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// --- scaleMatrixTransforms ---

func TestScaleMatrixTransforms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "matrix with six values",
			input: "matrix(.882,0,0,.882,61.74,44.1)",
			expected: "matrix(" +
				formatScaledNum(.882*mmToPxFactor) + "," +
				formatScaledNum(0) + "," +
				formatScaledNum(0) + "," +
				formatScaledNum(.882*mmToPxFactor) + "," +
				formatScaledNum(61.74*mmToPxFactor) + "," +
				formatScaledNum(44.1*mmToPxFactor) + ")",
		},
		{
			name:     "no matrix present",
			input:    `transform="translate(10,20)"`,
			expected: `transform="translate(10,20)"`,
		},
		{
			name:  "matrix in transform attribute",
			input: `transform="matrix(1,0,0,1,5,10)"`,
			expected: `transform="matrix(` +
				formatScaledNum(1*mmToPxFactor) + "," +
				formatScaledNum(0) + "," +
				formatScaledNum(0) + "," +
				formatScaledNum(1*mmToPxFactor) + "," +
				formatScaledNum(5*mmToPxFactor) + "," +
				formatScaledNum(10*mmToPxFactor) + `)"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := scaleMatrixTransforms(tc.input)
			if result != tc.expected {
				t.Errorf("scaleMatrixTransforms(%q)\n  got  %q\n  want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// --- formatScaledNum ---

func TestFormatScaledNum(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{
			name:     "integer result has no decimal point",
			input:    96.0,
			expected: "96",
		},
		{
			name:     "trailing zeros trimmed",
			input:    37.8,
			expected: "37.8",
		},
		{
			name:     "zero",
			input:    0.0,
			expected: "0",
		},
		{
			name:     "negative integer",
			input:    -50.0,
			expected: "-50",
		},
		{
			name:     "negative with decimals",
			input:    -18.8976,
			expected: "-18.8976",
		},
		{
			name:     "small number with precision",
			input:    1.5,
			expected: "1.5",
		},
		{
			name:     "number that would have trailing zeros",
			input:    10.50,
			expected: "10.5",
		},
		{
			name:     "number needing 4 decimal precision",
			input:    3.7795,
			expected: "3.7795",
		},
		{
			name:     "number with more than 4 decimals rounds to 4",
			input:    3.77952755905,
			expected: "3.7795",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatScaledNum(tc.input)
			if result != tc.expected {
				t.Errorf("formatScaledNum(%v) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// --- splitSVGContentAndStyle ---

func TestSplitSVGContentAndStyle(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectContent  string
		expectSuffix   string
	}{
		{
			name:          "SVG with style tag",
			input:         `<svg><rect x="5" y="5"/><style>.cls{fill:red}</style></svg>`,
			expectContent: `<svg><rect x="5" y="5"/>`,
			expectSuffix:  `<style>.cls{fill:red}</style></svg>`,
		},
		{
			name:          "SVG with defs before style",
			input:         `<svg><rect/><defs><linearGradient/></defs><style>.cls{}</style></svg>`,
			expectContent: `<svg><rect/>`,
			expectSuffix:  `<defs><linearGradient/></defs><style>.cls{}</style></svg>`,
		},
		{
			name:          "SVG with no style or defs",
			input:         `<svg><rect x="5" y="5"/></svg>`,
			expectContent: `<svg><rect x="5" y="5"/></svg>`,
			expectSuffix:  ``,
		},
		{
			name:          "SVG with only defs",
			input:         `<svg><rect/><defs><g/></defs></svg>`,
			expectContent: `<svg><rect/>`,
			expectSuffix:  `<defs><g/></defs></svg>`,
		},
		{
			name:          "empty string",
			input:         ``,
			expectContent: ``,
			expectSuffix:  ``,
		},
		{
			name:          "style at beginning",
			input:         `<style>.x{}</style><rect/>`,
			expectContent: ``,
			expectSuffix:  `<style>.x{}</style><rect/>`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content, suffix := splitSVGContentAndStyle(tc.input)
			if content != tc.expectContent {
				t.Errorf("content:\n  got  %q\n  want %q", content, tc.expectContent)
			}
			if suffix != tc.expectSuffix {
				t.Errorf("suffix:\n  got  %q\n  want %q", suffix, tc.expectSuffix)
			}
		})
	}
}

// --- scaleSVGToPixelCoordsSafe (integration test) ---

func TestScaleSVGToPixelCoordsSafe_Integration(t *testing.T) {
	svgInput := `<svg version="1.1" width="100mm" height="50mm" viewBox="0 0 100 50" xmlns="http://www.w3.org/2000/svg">
<rect x="5" y="5" width="90" height="40"/>
<text x="50" y="25" style="font: 5px Arial">Hello</text>
</svg>`

	widthMM := 100.0
	heightMM := 50.0

	result := string(scaleSVGToPixelCoordsSafe([]byte(svgInput), widthMM, heightMM))

	widthPx := math.Round(widthMM * mmToPxFactor)
	heightPx := math.Round(heightMM * mmToPxFactor)

	// 1. Viewport should be scaled from mm to px
	expectedViewport := `width="` + formatScaledNum(widthPx) + `" height="` + formatScaledNum(heightPx) + `"`
	if !strings.Contains(result, expectedViewport) {
		t.Errorf("expected viewport %q in result, got:\n%s", expectedViewport, result)
	}

	// 2. viewBox should be updated to pixel values
	expectedViewBox := `viewBox="0 0 ` + formatScaledNum(widthPx) + ` ` + formatScaledNum(heightPx) + `"`
	if !strings.Contains(result, expectedViewBox) {
		t.Errorf("expected viewBox %q in result, got:\n%s", expectedViewBox, result)
	}

	// 3. Coordinates x="5" and y="5" on rect should be scaled
	expX5 := `x="` + formatScaledNum(5*mmToPxFactor) + `"`
	expY5 := `y="` + formatScaledNum(5*mmToPxFactor) + `"`
	if !strings.Contains(result, expX5) {
		t.Errorf("expected rect x %q in result, got:\n%s", expX5, result)
	}
	if !strings.Contains(result, expY5) {
		t.Errorf("expected rect y %q in result, got:\n%s", expY5, result)
	}

	// 4. Text x="50" and y="25" should be scaled
	expX50 := `x="` + formatScaledNum(50*mmToPxFactor) + `"`
	expY25 := `y="` + formatScaledNum(25*mmToPxFactor) + `"`
	if !strings.Contains(result, expX50) {
		t.Errorf("expected text x %q in result, got:\n%s", expX50, result)
	}
	if !strings.Contains(result, expY25) {
		t.Errorf("expected text y %q in result, got:\n%s", expY25, result)
	}

	// 5. Font size "5px" should be scaled
	expFontSize := formatScaledNum(5*mmToPxFactor) + "px"
	if !strings.Contains(result, expFontSize) {
		t.Errorf("expected font size %q in result, got:\n%s", expFontSize, result)
	}

	// 6. width="90" and height="40" on rect should NOT be scaled (not coord attrs)
	if !strings.Contains(result, `width="90"`) {
		t.Errorf("expected rect width to remain unchanged, got:\n%s", result)
	}
	if !strings.Contains(result, `height="40"`) {
		t.Errorf("expected rect height to remain unchanged, got:\n%s", result)
	}
}

func TestScaleSVGToPixelCoordsSafe_WithStyleSection(t *testing.T) {
	// Verify that <style> content is NOT scaled.
	svgInput := `<svg version="1.1" width="100mm" height="50mm" viewBox="0 0 100 50" xmlns="http://www.w3.org/2000/svg">
<rect x="10" y="10"/>
<style>
@font-face { src: url(data:font/woff2;base64,AAABBCC123px) }
.text { font-size: 12px }
</style></svg>`

	result := string(scaleSVGToPixelCoordsSafe([]byte(svgInput), 100.0, 50.0))

	// The base64 data "123px" in the style should NOT be touched (it has no trailing space,
	// so the font regex won't match it anyway, but we verify it's untouched).
	if !strings.Contains(result, "AAABBCC123px") {
		t.Errorf("expected base64 data in style to be preserved, got:\n%s", result)
	}

	// The rect x="10" should be scaled
	expX10 := `x="` + formatScaledNum(10*mmToPxFactor) + `"`
	if !strings.Contains(result, expX10) {
		t.Errorf("expected rect x to be scaled, got:\n%s", result)
	}
}

func TestScaleSVGToPixelCoordsSafe_WithDefsAndGradients(t *testing.T) {
	// Verify that gradient coordinates in <defs> are scaled.
	svgInput := `<svg version="1.1" width="50mm" height="50mm" viewBox="0 0 50 50" xmlns="http://www.w3.org/2000/svg">
<rect x="5" y="5"/>
<defs><linearGradient x1="0" y1="0" x2="50" y2="50"><stop offset="0" stop-color="#fff"/></linearGradient></defs></svg>`

	result := string(scaleSVGToPixelCoordsSafe([]byte(svgInput), 50.0, 50.0))

	// Gradient x2 and y2 should be scaled in the defs section.
	expX2 := `x2="` + formatScaledNum(50*mmToPxFactor) + `"`
	expY2 := `y2="` + formatScaledNum(50*mmToPxFactor) + `"`
	if !strings.Contains(result, expX2) {
		t.Errorf("expected gradient x2 to be scaled, got:\n%s", result)
	}
	if !strings.Contains(result, expY2) {
		t.Errorf("expected gradient y2 to be scaled, got:\n%s", result)
	}

	// stop offset="0" should NOT be scaled (it's a ratio attribute, not in the coord regex).
	if !strings.Contains(result, `offset="0"`) {
		t.Errorf("expected stop offset to remain unchanged, got:\n%s", result)
	}
}

func TestScaleSVGToPixelCoordsSafe_PathData(t *testing.T) {
	// Verify that path data d="..." is scaled in the integration path.
	svgInput := `<svg version="1.1" width="100mm" height="100mm" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
<path d="M10 20L30 40Z"/>
</svg>`

	result := string(scaleSVGToPixelCoordsSafe([]byte(svgInput), 100.0, 100.0))

	exp10 := formatScaledNum(10 * mmToPxFactor)
	exp20 := formatScaledNum(20 * mmToPxFactor)
	exp30 := formatScaledNum(30 * mmToPxFactor)
	exp40 := formatScaledNum(40 * mmToPxFactor)

	expectedPath := `d="M` + exp10 + " " + exp20 + "L" + exp30 + " " + exp40 + `Z"`
	if !strings.Contains(result, expectedPath) {
		t.Errorf("expected path data %q in result, got:\n%s", expectedPath, result)
	}
}

func TestScaleSVGToPixelCoordsSafe_StrokeWidth(t *testing.T) {
	svgInput := `<svg version="1.1" width="100mm" height="100mm" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
<line x1="0" y1="0" x2="100" y2="100" style="stroke:#000;stroke-width:0.5"/>
</svg>`

	result := string(scaleSVGToPixelCoordsSafe([]byte(svgInput), 100.0, 100.0))

	expectedStroke := "stroke-width:" + formatScaledNum(0.5*mmToPxFactor)
	if !strings.Contains(result, expectedStroke) {
		t.Errorf("expected stroke-width %q in result, got:\n%s", expectedStroke, result)
	}
}

func TestScaleSVGToPixelCoordsSafe_TranslateTransform(t *testing.T) {
	svgInput := `<svg version="1.1" width="100mm" height="100mm" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
<g transform="translate(10,20)"><rect x="0" y="0"/></g>
</svg>`

	result := string(scaleSVGToPixelCoordsSafe([]byte(svgInput), 100.0, 100.0))

	expectedTranslate := "translate(" + formatScaledNum(10*mmToPxFactor) + "," + formatScaledNum(20*mmToPxFactor) + ")"
	if !strings.Contains(result, expectedTranslate) {
		t.Errorf("expected translate %q in result, got:\n%s", expectedTranslate, result)
	}
}

// --- mmToPxFactor constant verification ---

func TestMmToPxFactor(t *testing.T) {
	expected := 96.0 / 25.4
	if !approxEqual(mmToPxFactor, expected, 1e-10) {
		t.Errorf("mmToPxFactor = %v, want %v", mmToPxFactor, expected)
	}
}

// --- scalePathData table-driven comprehensive ---

func TestScalePathData_TableDriven(t *testing.T) {
	f := func(v float64) string { return formatScaledNum(v * mmToPxFactor) }

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "M/L commands",
			input:    "M10 20L30 40",
			expected: "M" + f(10) + " " + f(20) + "L" + f(30) + " " + f(40),
		},
		{
			name:     "H command only",
			input:    "H50",
			expected: "H" + f(50),
		},
		{
			name:     "V command only",
			input:    "V60",
			expected: "V" + f(60),
		},
		{
			name:     "H then V",
			input:    "H50V60",
			expected: "H" + f(50) + "V" + f(60),
		},
		{
			name:     "arc preserves rotation and flags",
			input:    "A5 10 30 0 1 20 30",
			expected: "A" + f(5) + " " + f(10) + " 30 0 1 " + f(20) + " " + f(30),
		},
		{
			name:     "Z has no params",
			input:    "M0 0L10 10Z",
			expected: "M" + f(0) + " " + f(0) + "L" + f(10) + " " + f(10) + "Z",
		},
		{
			name:     "implicit repeated M coords",
			input:    "M0 0 10 20 30 40",
			expected: "M" + f(0) + " " + f(0) + " " + f(10) + " " + f(20) + " " + f(30) + " " + f(40),
		},
		{
			name:     "cubic bezier all scaled",
			input:    "C1 2 3 4 5 6",
			expected: "C" + f(1) + " " + f(2) + " " + f(3) + " " + f(4) + " " + f(5) + " " + f(6),
		},
		{
			name:     "smooth cubic S",
			input:    "S10 20 30 40",
			expected: "S" + f(10) + " " + f(20) + " " + f(30) + " " + f(40),
		},
		{
			name:     "quadratic bezier Q",
			input:    "Q10 20 30 40",
			expected: "Q" + f(10) + " " + f(20) + " " + f(30) + " " + f(40),
		},
		{
			name:     "smooth quadratic T",
			input:    "T10 20",
			expected: "T" + f(10) + " " + f(20),
		},
		{
			name:     "multiple commands combined",
			input:    "M0 0L50 50H75V25C10 10 20 20 30 30Z",
			expected: "M" + f(0) + " " + f(0) + "L" + f(50) + " " + f(50) + "H" + f(75) + "V" + f(25) + "C" + f(10) + " " + f(10) + " " + f(20) + " " + f(20) + " " + f(30) + " " + f(30) + "Z",
		},
		{
			name:     "comma-separated values",
			input:    "M10,20L30,40",
			expected: "M" + f(10) + "," + f(20) + "L" + f(30) + "," + f(40),
		},
		{
			name:     "negative coordinates",
			input:    "M-5 -10",
			expected: "M" + f(-5) + " " + f(-10),
		},
		{
			name:     "decimal coordinates",
			input:    "M1.5 2.75",
			expected: "M" + f(1.5) + " " + f(2.75),
		},
		{
			name:     "relative move and line",
			input:    "m5 5l10 10",
			expected: "m" + f(5) + " " + f(5) + "l" + f(10) + " " + f(10),
		},
		{
			name:     "relative arc preserves flags",
			input:    "a3 4 45 1 0 10 20",
			expected: "a" + f(3) + " " + f(4) + " 45 1 0 " + f(10) + " " + f(20),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := scalePathData(tc.input)
			if result != tc.expected {
				t.Errorf("scalePathData(%q)\n  got  %q\n  want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// --- helper function tests ---

func TestPathParamCount(t *testing.T) {
	tests := []struct {
		cmd      byte
		expected int
	}{
		{'M', 2}, {'m', 2}, {'L', 2}, {'l', 2},
		{'H', 1}, {'h', 1}, {'V', 1}, {'v', 1},
		{'C', 6}, {'c', 6},
		{'S', 4}, {'s', 4}, {'Q', 4}, {'q', 4},
		{'A', 7}, {'a', 7},
		{'T', 2}, {'t', 2},
		{'Z', 0}, {'z', 0},
	}

	for _, tc := range tests {
		t.Run(string(tc.cmd), func(t *testing.T) {
			result := pathParamCount(tc.cmd)
			if result != tc.expected {
				t.Errorf("pathParamCount(%q) = %d, want %d", tc.cmd, result, tc.expected)
			}
		})
	}
}

func TestIsPathCommand(t *testing.T) {
	commands := "MmLlHhVvCcSsQqTtAaZz"
	for _, ch := range commands {
		if !isPathCommand(byte(ch)) {
			t.Errorf("isPathCommand(%q) = false, want true", ch)
		}
	}

	nonCommands := "0123456789.+-eE ,\n\t"
	for _, ch := range nonCommands {
		if isPathCommand(byte(ch)) {
			t.Errorf("isPathCommand(%q) = true, want false", ch)
		}
	}
}

func TestShouldScaleParam_Arc(t *testing.T) {
	// A: rx(0) ry(1) rotation(2) large-arc(3) sweep(4) x(5) y(6)
	tests := []struct {
		idx      int
		expected bool
	}{
		{0, true},  // rx → scale
		{1, true},  // ry → scale
		{2, false}, // rotation → don't scale
		{3, false}, // large-arc flag → don't scale
		{4, false}, // sweep flag → don't scale
		{5, true},  // x → scale
		{6, true},  // y → scale
	}

	for _, tc := range tests {
		result := shouldScaleParam('A', tc.idx, 7)
		if result != tc.expected {
			t.Errorf("shouldScaleParam('A', %d, 7) = %v, want %v", tc.idx, result, tc.expected)
		}
	}
}

func TestShouldScaleParam_NonArc(t *testing.T) {
	// All params for non-arc commands should be scaled
	for _, cmd := range []byte{'M', 'L', 'H', 'V', 'C', 'S', 'Q', 'T'} {
		pc := pathParamCount(cmd)
		for idx := 0; idx < pc; idx++ {
			result := shouldScaleParam(cmd, idx, pc)
			if !result {
				t.Errorf("shouldScaleParam(%q, %d, %d) = false, want true", cmd, idx, pc)
			}
		}
	}
}

func TestShouldScaleParam_ZeroParams(t *testing.T) {
	// Z has 0 params, should return false
	result := shouldScaleParam('Z', 0, 0)
	if result {
		t.Errorf("shouldScaleParam('Z', 0, 0) = true, want false")
	}
}

func TestReadNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		start   int
		wantNum string
		wantEnd int
	}{
		{"positive integer", "123", 0, "123", 3},
		{"negative integer", "-42", 0, "-42", 3},
		{"decimal", "3.14", 0, "3.14", 4},
		{"negative decimal", "-0.5", 0, "-0.5", 4},
		{"exponent", "1e5", 0, "1e5", 3},
		{"negative exponent", "2.5e-3", 0, "2.5e-3", 6},
		{"number at offset", "M123 456", 1, "123", 4},
		{"positive sign", "+10", 0, "+10", 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			num, end := readNumber(tc.input, tc.start)
			if num != tc.wantNum {
				t.Errorf("readNumber(%q, %d) num = %q, want %q", tc.input, tc.start, num, tc.wantNum)
			}
			if end != tc.wantEnd {
				t.Errorf("readNumber(%q, %d) end = %d, want %d", tc.input, tc.start, end, tc.wantEnd)
			}
		})
	}
}

// --- scaleGradientCoords (no-op verification) ---

func TestScaleGradientCoords_IsNoOp(t *testing.T) {
	// scaleGradientCoords is documented as a no-op; verify it returns input unchanged.
	inputs := []string{
		`<linearGradient x1="0" y1="0" x2="100" y2="100"/>`,
		`anything here`,
		``,
	}
	for _, input := range inputs {
		result := scaleGradientCoords(input)
		if result != input {
			t.Errorf("scaleGradientCoords(%q) = %q, want same as input", input, result)
		}
	}
}

// --- replaceViewBox ---

func TestReplaceViewBox(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		widthPx  float64
		heightPx float64
		expected string
	}{
		{
			name:     "standard viewBox replacement",
			input:    `viewBox="0 0 100 50"`,
			widthPx:  378.0,
			heightPx: 189.0,
			expected: `viewBox="0 0 378 189"`,
		},
		{
			name:     "decimal viewBox values",
			input:    `viewBox="0 0 25.4 12.7"`,
			widthPx:  96.0,
			heightPx: 48.0,
			expected: `viewBox="0 0 96 48"`,
		},
		{
			name:     "no viewBox present",
			input:    `<svg width="100" height="50">`,
			widthPx:  378.0,
			heightPx: 189.0,
			expected: `<svg width="100" height="50">`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := replaceViewBox(tc.input, tc.widthPx, tc.heightPx)
			if result != tc.expected {
				t.Errorf("replaceViewBox(%q, %.0f, %.0f)\n  got  %q\n  want %q", tc.input, tc.widthPx, tc.heightPx, result, tc.expected)
			}
		})
	}
}

// --- scalePathDataInSVG (wrapper function) ---

func TestScalePathDataInSVG(t *testing.T) {
	f := func(v float64) string { return formatScaledNum(v * mmToPxFactor) }

	input := `<path d="M10 20L30 40"/>`
	result := scalePathDataInSVG(input)

	expected := `<path d="M` + f(10) + " " + f(20) + "L" + f(30) + " " + f(40) + `"/>`
	if result != expected {
		t.Errorf("scalePathDataInSVG:\n  got  %q\n  want %q", result, expected)
	}
}

func TestScalePathDataInSVG_MultiplePaths(t *testing.T) {
	f := func(v float64) string { return formatScaledNum(v * mmToPxFactor) }

	input := `<path d="M0 0"/><path d="L5 5"/>`
	result := scalePathDataInSVG(input)

	exp1 := `d="M` + f(0) + " " + f(0) + `"`
	exp2 := `d="L` + f(5) + " " + f(5) + `"`
	if !strings.Contains(result, exp1) {
		t.Errorf("expected first path data %q in result %q", exp1, result)
	}
	if !strings.Contains(result, exp2) {
		t.Errorf("expected second path data %q in result %q", exp2, result)
	}
}

func TestScalePathDataInSVG_NoDAttr(t *testing.T) {
	input := `<rect x="5" y="5"/>`
	result := scalePathDataInSVG(input)
	if result != input {
		t.Errorf("expected no change when no d attribute, got %q", result)
	}
}
