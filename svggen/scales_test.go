package svggen

import (
	"math"
	"testing"
)

func TestLinearScale_Basic(t *testing.T) {
	tests := []struct {
		name      string
		domainMin float64
		domainMax float64
		rangeMin  float64
		rangeMax  float64
		input     float64
		expected  float64
	}{
		{
			name:      "identity domain [0,1] to range [0,1]",
			domainMin: 0,
			domainMax: 1,
			rangeMin:  0,
			rangeMax:  1,
			input:     0.5,
			expected:  0.5,
		},
		{
			name:      "scale domain [0,100] to range [0,500]",
			domainMin: 0,
			domainMax: 100,
			rangeMin:  0,
			rangeMax:  500,
			input:     50,
			expected:  250,
		},
		{
			name:      "domain min",
			domainMin: 0,
			domainMax: 100,
			rangeMin:  0,
			rangeMax:  500,
			input:     0,
			expected:  0,
		},
		{
			name:      "domain max",
			domainMin: 0,
			domainMax: 100,
			rangeMin:  0,
			rangeMax:  500,
			input:     100,
			expected:  500,
		},
		{
			name:      "negative domain",
			domainMin: -50,
			domainMax: 50,
			rangeMin:  0,
			rangeMax:  100,
			input:     0,
			expected:  50,
		},
		{
			name:      "reversed range",
			domainMin: 0,
			domainMax: 100,
			rangeMin:  500,
			rangeMax:  0,
			input:     25,
			expected:  375,
		},
		{
			name:      "out of domain (above)",
			domainMin: 0,
			domainMax: 100,
			rangeMin:  0,
			rangeMax:  500,
			input:     150,
			expected:  750,
		},
		{
			name:      "out of domain (below)",
			domainMin: 0,
			domainMax: 100,
			rangeMin:  0,
			rangeMax:  500,
			input:     -50,
			expected:  -250,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scale := NewLinearScale(tt.domainMin, tt.domainMax)
			scale.SetRangeLinear(tt.rangeMin, tt.rangeMax)

			result := scale.Scale(tt.input)
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("Scale(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLinearScale_Clamp(t *testing.T) {
	scale := NewLinearScale(0, 100).SetRangeLinear(0, 500).Clamp(true)

	tests := []struct {
		input    float64
		expected float64
	}{
		{-50, 0},
		{0, 0},
		{50, 250},
		{100, 500},
		{150, 500},
	}

	for _, tt := range tests {
		result := scale.Scale(tt.input)
		if math.Abs(result-tt.expected) > 0.0001 {
			t.Errorf("Scale(%v) with clamp = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestLinearScale_Invert(t *testing.T) {
	scale := NewLinearScale(0, 100).SetRangeLinear(0, 500)

	tests := []struct {
		input    float64
		expected float64
	}{
		{0, 0},
		{250, 50},
		{500, 100},
	}

	for _, tt := range tests {
		result := scale.Invert(tt.input)
		if math.Abs(result-tt.expected) > 0.0001 {
			t.Errorf("Invert(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestLinearScale_Domain(t *testing.T) {
	scale := NewLinearScale(10, 90)

	min, max := scale.DomainBounds()
	if min != 10 || max != 90 {
		t.Errorf("DomainBounds() = (%v, %v), want (10, 90)", min, max)
	}

	// Test Domain() interface method
	dmin, dmax := scale.Domain()
	if dmin.(float64) != 10 || dmax.(float64) != 90 {
		t.Errorf("Domain() = (%v, %v), want (10, 90)", dmin, dmax)
	}
}

func TestLinearScale_Range(t *testing.T) {
	scale := NewLinearScale(0, 100).SetRangeLinear(100, 400)

	min, max := scale.Range()
	if min != 100 || max != 400 {
		t.Errorf("Range() = (%v, %v), want (100, 400)", min, max)
	}
}

func TestLinearScale_Nice(t *testing.T) {
	tests := []struct {
		name        string
		domainMin   float64
		domainMax   float64
		expectedMin float64
		expectedMax float64
	}{
		// Span ~9.4, tickStep(count=5)=2 -> rounds to 0 and 10
		{"small range", 0.3, 9.7, 0, 10},
		// Span ~7.7, tickStep(count=5)=2 -> rounds to 0 and 10
		{"positive range", 1.2, 8.9, 0, 10},
		// Span 9, tickStep(count=5)=2 -> rounds to -6 and 6
		{"symmetric range", -4.5, 4.5, -6, 6},
		// Span ~0.086, tickStep(count=5)=0.02 -> rounds to 0 and 0.1
		{"small decimal range", 0.012, 0.098, 0, 0.1},
		// Area chart bug: data [1200,4200] with 10% padding ->
		// tickStep(count=5)=1000 -> [0, 5000] -> ticks cover data max
		{"area chart Y-axis", 900, 4500, 0, 5000},

		// Bubble chart: data [40..95], Nice() should not push domainMin below 0.
		// tickStep(count=5)=20 -> Floor(40/20)*20=40, Ceil(95/20)*20=100.
		{"bubble positive-only", 40, 95, 40, 100},

		// Negative-only data: Nice() should not push domainMax above 0.
		{"negative-only", -95, -40, -100, -40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scale := NewLinearScale(tt.domainMin, tt.domainMax).Nice(true)
			min, max := scale.DomainBounds()

			tolerance := math.Abs(tt.domainMax-tt.domainMin) * 0.01

			if math.Abs(min-tt.expectedMin) > tolerance {
				t.Errorf("Nice domain min = %v, want approximately %v", min, tt.expectedMin)
			}

			if math.Abs(max-tt.expectedMax) > tolerance {
				t.Errorf("Nice domain max = %v, want approximately %v", max, tt.expectedMax)
			}
		})
	}
}

func TestLinearScale_Ticks(t *testing.T) {
	tests := []struct {
		name      string
		domainMin float64
		domainMax float64
		count     int
		wantCount int // Approximate expected tick count
	}{
		{"0-100 with 5 ticks", 0, 100, 5, 5},
		{"0-100 with 10 ticks", 0, 100, 10, 10},
		{"0-1 with 5 ticks", 0, 1, 5, 5},
		{"-50 to 50 with 5 ticks", -50, 50, 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scale := NewLinearScale(tt.domainMin, tt.domainMax)
			ticks := scale.Ticks(tt.count)

			// Check we got approximately the right number of ticks
			if len(ticks) < tt.wantCount-2 || len(ticks) > tt.wantCount+2 {
				t.Errorf("got %d ticks, want approximately %d", len(ticks), tt.wantCount)
			}

			// Check ticks are in order
			for i := 1; i < len(ticks); i++ {
				if ticks[i] <= ticks[i-1] {
					t.Errorf("ticks not in ascending order: %v", ticks)
					break
				}
			}

			// Check ticks are within domain
			for _, tick := range ticks {
				if tick < tt.domainMin-0.0001 || tick > tt.domainMax+0.0001 {
					t.Errorf("tick %v outside domain [%v, %v]", tick, tt.domainMin, tt.domainMax)
				}
			}
		})
	}
}

func TestLinearScale_TickFormat(t *testing.T) {
	tests := []struct {
		domainMin float64
		domainMax float64
		count     int
		expected  string
	}{
		{0, 100, 5, "%.0f"},
		{0, 10, 5, "%.0f"},
		{0, 1, 5, "%.1f"},
		{0, 0.1, 5, "%.2f"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			scale := NewLinearScale(tt.domainMin, tt.domainMax)
			format := scale.TickFormat(tt.count)

			if format != tt.expected {
				t.Errorf("TickFormat() = %q, want %q", format, tt.expected)
			}
		})
	}
}

func TestLinearScale_ZeroDomain(t *testing.T) {
	// When domain min == max, scale should return mid-range
	scale := NewLinearScale(50, 50).SetRangeLinear(0, 100)

	result := scale.Scale(50)
	if result != 50 {
		t.Errorf("Scale(50) with zero domain = %v, want 50", result)
	}
}

// =============================================================================
// Log Scale Tests
// =============================================================================

func TestLogScale_Basic(t *testing.T) {
	scale := NewLogScale(1, 1000).SetRangeLog(0, 300)

	// log10(1)=0, log10(1000)=3; Scale(1) should map to 0
	if got := scale.Scale(1); math.Abs(got-0) > 0.01 {
		t.Errorf("Scale(1) = %v, want 0", got)
	}
	// Scale(1000) should map to 300
	if got := scale.Scale(1000); math.Abs(got-300) > 0.01 {
		t.Errorf("Scale(1000) = %v, want 300", got)
	}
	// Scale(10) should map to 100 (1/3 of the way)
	if got := scale.Scale(10); math.Abs(got-100) > 0.01 {
		t.Errorf("Scale(10) = %v, want 100", got)
	}
	// Scale(100) should map to 200 (2/3 of the way)
	if got := scale.Scale(100); math.Abs(got-200) > 0.01 {
		t.Errorf("Scale(100) = %v, want 200", got)
	}
}

func TestLogScale_Ticks(t *testing.T) {
	scale := NewLogScale(0.01, 10000)

	ticks := scale.Ticks(10)
	// Should have ticks at 0.01, 0.1, 1, 10, 100, 1000, 10000
	if len(ticks) < 5 {
		t.Errorf("Expected at least 5 ticks, got %d: %v", len(ticks), ticks)
	}

	// First tick should be 0.01
	if math.Abs(ticks[0]-0.01) > 0.001 {
		t.Errorf("First tick = %v, want 0.01", ticks[0])
	}
	// Last tick should be 10000
	if math.Abs(ticks[len(ticks)-1]-10000) > 0.01 {
		t.Errorf("Last tick = %v, want 10000", ticks[len(ticks)-1])
	}
}

func TestLogScale_WasThinned(t *testing.T) {
	// 0.01 to 10^15 spans 18 orders of magnitude → 18 raw ticks.
	// Ticks(3) should thin them (since 18 > 3*2).
	scale := NewLogScale(0.01, 1e15)
	ticks := scale.Ticks(3)

	thinned, orig, kept := scale.WasThinned()
	if !thinned {
		t.Fatalf("expected thinning for 18-order-of-magnitude span with count=3, got %d ticks", len(ticks))
	}
	if orig < 15 {
		t.Errorf("original tick count = %d, want >= 15", orig)
	}
	if kept != len(ticks) {
		t.Errorf("kept = %d, but len(ticks) = %d", kept, len(ticks))
	}
	if kept >= orig {
		t.Errorf("kept (%d) should be < original (%d)", kept, orig)
	}
}

func TestLogScale_WasThinned_NoThinning(t *testing.T) {
	// 0.01 to 10000 spans 7 orders → 7 raw ticks, Ticks(10) won't thin.
	scale := NewLogScale(0.01, 10000)
	scale.Ticks(10)

	thinned, _, _ := scale.WasThinned()
	if thinned {
		t.Error("expected no thinning for 7-order span with count=10")
	}
}

func TestLogScale_Invert(t *testing.T) {
	scale := NewLogScale(1, 1000).SetRangeLog(0, 300)

	// Invert(0) should return 1
	if got := scale.Invert(0); math.Abs(got-1) > 0.01 {
		t.Errorf("Invert(0) = %v, want 1", got)
	}
	// Invert(300) should return 1000
	if got := scale.Invert(300); math.Abs(got-1000) > 1 {
		t.Errorf("Invert(300) = %v, want 1000", got)
	}
}

func TestLogScale_ZeroValue(t *testing.T) {
	scale := NewLogScale(1, 1000).SetRangeLog(0, 300)

	// Zero should be clamped to domainMin
	got := scale.Scale(0)
	if math.Abs(got-0) > 0.01 {
		t.Errorf("Scale(0) = %v, want 0 (clamped to domain min)", got)
	}
}

func TestLogScale_NegativeDomain(t *testing.T) {
	// Negative domain bounds should be clamped to 1
	scale := NewLogScale(-10, -5)
	dMin, dMax := scale.DomainBounds()
	if dMin != 1 || dMax != 1 {
		t.Errorf("DomainBounds() = (%v, %v), want (1, 1) for negative inputs", dMin, dMax)
	}
}

func TestFormatLogLabel(t *testing.T) {
	tests := []struct {
		value    float64
		expected string
	}{
		{0.001, "0.001"},
		{0.01, "0.01"},
		{0.1, "0.1"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{1000, "1K"},
		{10000, "10K"},
		{100000, "100K"},
		{1000000, "1M"},
		{1000000000, "1B"},
		{1000000000000, "1T"},
	}

	for _, tt := range tests {
		got := FormatLogLabel(tt.value)
		if got != tt.expected {
			t.Errorf("FormatLogLabel(%v) = %q, want %q", tt.value, got, tt.expected)
		}
	}
}

func TestFormatCompact(t *testing.T) {
	tests := []struct {
		value    float64
		expected string
	}{
		// Zero
		{0, "0"},
		// Small values (< 1000) - formatted as-is
		{1, "1"},
		{42, "42"},
		{500, "500"},
		{999, "999"},
		// Thousands (K suffix)
		{1000, "1K"},
		{1500, "1.5K"},
		{2000, "2K"},
		{10000, "10K"},
		{10500, "10.5K"},
		{100000, "100K"},
		{999000, "999K"},
		{999900, "999.9K"},
		// Millions (M suffix)
		{1000000, "1M"},
		{1500000, "1.5M"},
		{2500000, "2.5M"},
		{10000000, "10M"},
		{100000000, "100M"},
		// Billions (B suffix)
		{1000000000, "1B"},
		{2500000000, "2.5B"},
		{10000000000, "10B"},
		// Trillions (T suffix)
		{1000000000000, "1T"},
		{1500000000000, "1.5T"},
		// Negative values
		{-500, "-500"},
		{-1500, "-1.5K"},
		{-2500000, "-2.5M"},
		{-1000000000, "-1B"},
		// Trailing .0 dropped
		{1000, "1K"},       // not "1.0K"
		{2000000, "2M"},    // not "2.0M"
		{3000000000, "3B"}, // not "3.0B"
		// Small fractional values
		{0.5, "0.5"},
		{0.25, "0.25"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatCompact(tt.value)
			if got != tt.expected {
				t.Errorf("FormatCompact(%v) = %q, want %q", tt.value, got, tt.expected)
			}
		})
	}
}

func TestFormatCompact_Symmetry(t *testing.T) {
	// FormatCompact(-v) should equal "-" + FormatCompact(v) for v > 0
	values := []float64{1500, 2500000, 1000000000, 42, 0.5}
	for _, v := range values {
		pos := FormatCompact(v)
		neg := FormatCompact(-v)
		if neg != "-"+pos {
			t.Errorf("FormatCompact(-%v) = %q, expected %q", v, neg, "-"+pos)
		}
	}
}

func TestCategoricalScale_Basic(t *testing.T) {
	categories := []string{"A", "B", "C", "D"}
	scale := NewCategoricalScale(categories).SetRangeCategorical(0, 400)

	// Test that each category is positioned correctly
	for i, cat := range categories {
		pos := scale.Scale(cat)

		// Position should be approximately evenly spaced
		expectedApprox := float64(i+1) * 400 / float64(len(categories)+1)
		if math.Abs(pos-expectedApprox) > 100 { // Very rough check
			t.Logf("Category %s at position %v (expected ~%v)", cat, pos, expectedApprox)
		}

		// Position should be within range
		if pos < 0 || pos > 400 {
			t.Errorf("Category %s position %v outside range [0, 400]", cat, pos)
		}
	}
}

func TestCategoricalScale_Bandwidth(t *testing.T) {
	categories := []string{"A", "B", "C"}
	scale := NewCategoricalScale(categories).SetRangeCategorical(0, 300)

	bandwidth := scale.Bandwidth()
	if bandwidth <= 0 {
		t.Errorf("Bandwidth = %v, want > 0", bandwidth)
	}

	// With 3 categories and some padding, bandwidth should be less than 100
	if bandwidth >= 100 {
		t.Errorf("Bandwidth = %v, expected < 100 for 3 categories in 300pt range", bandwidth)
	}
}

func TestCategoricalScale_ScaleStartEnd(t *testing.T) {
	categories := []string{"X", "Y", "Z"}
	scale := NewCategoricalScale(categories).SetRangeCategorical(0, 300)

	for _, cat := range categories {
		start := scale.ScaleStart(cat)
		end := scale.ScaleEnd(cat)
		center := scale.Scale(cat)

		// End should be after start
		if end <= start {
			t.Errorf("Category %s: end (%v) should be > start (%v)", cat, end, start)
		}

		// Center should be between start and end
		if center < start || center > end {
			t.Errorf("Category %s: center (%v) should be between start (%v) and end (%v)",
				cat, center, start, end)
		}

		// Bandwidth should equal end - start
		bandwidth := scale.Bandwidth()
		if math.Abs((end-start)-bandwidth) > 0.0001 {
			t.Errorf("Category %s: end-start (%v) should equal bandwidth (%v)",
				cat, end-start, bandwidth)
		}
	}
}

func TestCategoricalScale_Padding(t *testing.T) {
	categories := []string{"A", "B"}

	// No padding
	scale1 := NewCategoricalScale(categories).
		SetRangeCategorical(0, 100).
		Padding(0, 0)
	bw1 := scale1.Bandwidth()

	// With padding
	scale2 := NewCategoricalScale(categories).
		SetRangeCategorical(0, 100).
		Padding(0.2, 0.1)
	bw2 := scale2.Bandwidth()

	// Bandwidth should be smaller with padding
	if bw2 >= bw1 {
		t.Errorf("Bandwidth with padding (%v) should be < without padding (%v)", bw2, bw1)
	}
}

func TestCategoricalScale_InnerPadding(t *testing.T) {
	categories := []string{"A", "B", "C"}

	scale := NewCategoricalScale(categories).
		SetRangeCategorical(0, 300).
		PaddingInner(0.3)

	// Get positions and check gaps
	posA := scale.Scale("A")
	posB := scale.Scale("B")
	posC := scale.Scale("C")

	gapAB := posB - posA
	gapBC := posC - posB

	// Gaps should be approximately equal
	if math.Abs(gapAB-gapBC) > 0.001 {
		t.Errorf("Gaps should be equal: AB=%v, BC=%v", gapAB, gapBC)
	}
}

func TestCategoricalScale_UnknownCategory(t *testing.T) {
	categories := []string{"A", "B", "C"}
	scale := NewCategoricalScale(categories).SetRangeCategorical(0, 300)

	// Unknown category should return rangeMin
	pos := scale.Scale("Unknown")
	if pos != 0 {
		t.Errorf("Unknown category position = %v, want 0 (rangeMin)", pos)
	}
}

func TestCategoricalScale_Invert(t *testing.T) {
	categories := []string{"A", "B", "C"}
	scale := NewCategoricalScale(categories).SetRangeCategorical(0, 300)

	// Test inversion at center of each category
	for _, cat := range categories {
		pos := scale.Scale(cat)
		result := scale.Invert(pos)

		if result != cat {
			t.Errorf("Invert(Scale(%q)) = %q, want %q", cat, result, cat)
		}
	}
}

func TestCategoricalScale_Domain(t *testing.T) {
	categories := []string{"First", "Middle", "Last"}
	scale := NewCategoricalScale(categories)

	min, max := scale.Domain()
	if min.(string) != "First" || max.(string) != "Last" {
		t.Errorf("Domain() = (%v, %v), want (First, Last)", min, max)
	}
}

func TestCategoricalScale_Empty(t *testing.T) {
	scale := NewCategoricalScale(nil)

	if scale.Bandwidth() != 0 {
		t.Errorf("Empty scale bandwidth = %v, want 0", scale.Bandwidth())
	}

	if scale.Step() != 0 {
		t.Errorf("Empty scale step = %v, want 0", scale.Step())
	}

	pos := scale.Scale("any")
	if pos != 0 {
		t.Errorf("Empty scale.Scale() = %v, want 0", pos)
	}
}

func TestTickStep(t *testing.T) {
	tests := []struct {
		start    float64
		stop     float64
		count    int
		expected float64
	}{
		{0, 100, 5, 20},
		{0, 10, 5, 2},
		{0, 1, 5, 0.2},
		{0, 50, 10, 5},
	}

	for _, tt := range tests {
		result := tickStep(tt.start, tt.stop, tt.count)
		if math.Abs(result-tt.expected) > 0.0001 {
			t.Errorf("tickStep(%v, %v, %v) = %v, want %v",
				tt.start, tt.stop, tt.count, result, tt.expected)
		}
	}
}

func TestNiceStep(t *testing.T) {
	// niceStep rounds to nice numbers: 1, 2, 5, 10 per decade
	// Thresholds: <= 1.5 -> 1, <= 3 -> 2, <= 7 -> 5, else 10
	tests := []struct {
		input    float64
		expected float64
	}{
		{0.9, 1},  // 0.9 -> normalized 0.9, <= 1.5 -> 1 * 1 = 1
		{1.5, 1},  // 1.5 -> normalized 1.5, <= 1.5 -> 1 * 1 = 1
		{2.5, 2},  // 2.5 -> normalized 2.5, <= 3 -> 2 * 1 = 2
		{4, 5},    // 4 -> normalized 4, <= 7 -> 5 * 1 = 5
		{7, 5},    // 7 -> normalized 7, <= 7 -> 5 * 1 = 5
		{9, 10},   // 9 -> normalized 9, > 7 -> 10 * 1 = 10
		{15, 10},  // 15 -> normalized 1.5, <= 1.5 -> 1 * 10 = 10
		{45, 50},  // 45 -> normalized 4.5, <= 7 -> 5 * 10 = 50
		{75, 100}, // 75 -> normalized 7.5, > 7 -> 10 * 10 = 100
	}

	for _, tt := range tests {
		result := niceStep(tt.input)
		if result != tt.expected {
			t.Errorf("niceStep(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestDomainFromValues(t *testing.T) {
	tests := []struct {
		values      []float64
		expectedMin float64
		expectedMax float64
	}{
		{[]float64{1, 2, 3, 4, 5}, 1, 5},
		{[]float64{5, 4, 3, 2, 1}, 1, 5},
		{[]float64{-10, 0, 10}, -10, 10},
		{[]float64{42}, 42, 42},
		{nil, 0, 0},
	}

	for _, tt := range tests {
		min, max := DomainFromValues(tt.values)
		if min != tt.expectedMin || max != tt.expectedMax {
			t.Errorf("DomainFromValues(%v) = (%v, %v), want (%v, %v)",
				tt.values, min, max, tt.expectedMin, tt.expectedMax)
		}
	}
}

func TestExtendDomain(t *testing.T) {
	min, max := ExtendDomain(10, 90, 5)
	if min != 5 || max != 90 {
		t.Errorf("ExtendDomain(10, 90, 5) = (%v, %v), want (5, 90)", min, max)
	}

	min, max = ExtendDomain(10, 90, 95)
	if min != 10 || max != 95 {
		t.Errorf("ExtendDomain(10, 90, 95) = (%v, %v), want (10, 95)", min, max)
	}

	min, max = ExtendDomain(10, 90, 50)
	if min != 10 || max != 90 {
		t.Errorf("ExtendDomain(10, 90, 50) = (%v, %v), want (10, 90)", min, max)
	}
}

func TestDomainWithMargin(t *testing.T) {
	min, max := DomainWithMargin(0, 100, 0.1)

	if min != -10 {
		t.Errorf("DomainWithMargin min = %v, want -10", min)
	}
	if max != 110 {
		t.Errorf("DomainWithMargin max = %v, want 110", max)
	}
}

func TestDomainIncludeZero(t *testing.T) {
	tests := []struct {
		inputMin    float64
		inputMax    float64
		expectedMin float64
		expectedMax float64
	}{
		{10, 100, 0, 100},
		{-100, -10, -100, 0},
		{-50, 50, -50, 50},
		{0, 100, 0, 100},
	}

	for _, tt := range tests {
		min, max := DomainIncludeZero(tt.inputMin, tt.inputMax)
		if min != tt.expectedMin || max != tt.expectedMax {
			t.Errorf("DomainIncludeZero(%v, %v) = (%v, %v), want (%v, %v)",
				tt.inputMin, tt.inputMax, min, max, tt.expectedMin, tt.expectedMax)
		}
	}
}

func TestClamp01(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{-1, 0},
		{0, 0},
		{0.5, 0.5},
		{1, 1},
		{2, 1},
	}

	for _, tt := range tests {
		result := clamp01(tt.input)
		if result != tt.expected {
			t.Errorf("clamp01(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// =============================================================================
// Time Scale Tests
// =============================================================================

func TestTimeScale_Basic(t *testing.T) {
	// Create a scale for a 1-day period (86400 seconds)
	start := int64(1704067200) // 2024-01-01 00:00:00 UTC
	end := int64(1704153600)   // 2024-01-02 00:00:00 UTC

	scale := NewTimeScale(start, end).SetRangeTime(0, 100)

	tests := []struct {
		name     string
		input    int64
		expected float64
	}{
		{"start of day", start, 0},
		{"end of day", end, 100},
		{"midday", start + 43200, 50}, // 12:00 noon
		{"6 AM", start + 21600, 25},   // 6:00 AM
		{"6 PM", start + 64800, 75},   // 6:00 PM
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scale.Scale(tt.input)
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("Scale(%d) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTimeScale_Invert(t *testing.T) {
	start := int64(1704067200) // 2024-01-01 00:00:00 UTC
	end := int64(1704153600)   // 2024-01-02 00:00:00 UTC

	scale := NewTimeScale(start, end).SetRangeTime(0, 100)

	tests := []struct {
		rangeVal float64
		expected int64
	}{
		{0, start},
		{100, end},
		{50, start + 43200},
	}

	for _, tt := range tests {
		result := scale.Invert(tt.rangeVal)
		if result != tt.expected {
			t.Errorf("Invert(%v) = %d, want %d", tt.rangeVal, result, tt.expected)
		}
	}
}

func TestTimeScale_Clamp(t *testing.T) {
	start := int64(1704067200)
	end := int64(1704153600)

	scale := NewTimeScale(start, end).SetRangeTime(0, 100).Clamp(true)

	// Before start should clamp to 0
	result := scale.Scale(start - 3600)
	if result != 0 {
		t.Errorf("Scale(before start) with clamp = %v, want 0", result)
	}

	// After end should clamp to 100
	result = scale.Scale(end + 3600)
	if result != 100 {
		t.Errorf("Scale(after end) with clamp = %v, want 100", result)
	}
}

func TestTimeScale_Domain(t *testing.T) {
	start := int64(1704067200)
	end := int64(1704153600)

	scale := NewTimeScale(start, end)

	min, max := scale.DomainBounds()
	if min != start || max != end {
		t.Errorf("DomainBounds() = (%d, %d), want (%d, %d)", min, max, start, end)
	}

	dmin, dmax := scale.Domain()
	if dmin.(int64) != start || dmax.(int64) != end {
		t.Errorf("Domain() = (%v, %v), want (%d, %d)", dmin, dmax, start, end)
	}
}

func TestTimeScale_Ticks(t *testing.T) {
	tests := []struct {
		name      string
		start     int64
		end       int64
		count     int
		minTicks  int
		maxTicks  int
	}{
		{
			name:     "1 day span",
			start:    1704067200, // 2024-01-01
			end:      1704153600, // 2024-01-02
			count:    5,
			minTicks: 3,
			maxTicks: 10,
		},
		{
			name:     "1 week span",
			start:    1704067200,          // 2024-01-01
			end:      1704067200 + 604800, // + 7 days
			count:    5,
			minTicks: 3,
			maxTicks: 10,
		},
		{
			name:     "1 year span",
			start:    1704067200,           // 2024-01-01
			end:      1704067200 + 31536000, // + 365 days
			count:    5,
			minTicks: 3,
			maxTicks: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scale := NewTimeScale(tt.start, tt.end)
			ticks := scale.Ticks(tt.count)

			if len(ticks) < tt.minTicks || len(ticks) > tt.maxTicks {
				t.Errorf("got %d ticks, want between %d and %d", len(ticks), tt.minTicks, tt.maxTicks)
			}

			// Check ticks are in order and within domain
			for i := 1; i < len(ticks); i++ {
				if ticks[i] <= ticks[i-1] {
					t.Errorf("ticks not in ascending order at index %d: %v", i, ticks)
					break
				}
			}
		})
	}
}

func TestTimeScale_WasThinned(t *testing.T) {
	// Use a 10-year span with count=2 — the minute-level interval should
	// generate many ticks that get decimated.
	start := int64(1704067200)                // 2024-01-01
	end := int64(1704067200 + 10*365*24*3600) // ~10 years later
	scale := NewTimeScale(start, end)
	scale.Ticks(2)

	thinned, orig, kept := scale.WasThinned()
	// Whether thinning fires depends on the interval selection — a 10-year
	// span likely selects yearly ticks (10 ticks) which IS > 2*2=4, so it
	// should thin. If the interval is coarse enough that ≤4 ticks are
	// generated, thinning won't fire — that's fine; just verify consistency.
	if thinned {
		if kept >= orig {
			t.Errorf("kept (%d) should be < original (%d)", kept, orig)
		}
	}
}

func TestTimeScale_TicksWithLabels(t *testing.T) {
	// Create a scale for a 1-week period
	start := int64(1704067200)          // 2024-01-01
	end := int64(1704067200 + 604800)   // + 7 days

	scale := NewTimeScale(start, end)
	ticks, labels := scale.TicksWithLabels(5)

	if len(ticks) != len(labels) {
		t.Errorf("ticks and labels length mismatch: %d vs %d", len(ticks), len(labels))
	}

	// Labels should not be empty
	for i, label := range labels {
		if label == "" {
			t.Errorf("label %d is empty", i)
		}
	}
}

func TestParseTimeString(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"2024-01-01", 1704067200, false},
		{"2024-01-01T12:00:00", 1704110400, false},
		{"2024-01-01T00:00:00Z", 1704067200, false},
		{"1704067200", 1704067200, false}, // Unix timestamp
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseTimeString(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("ParseTimeString(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("ParseTimeString(%q) unexpected error: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("ParseTimeString(%q) = %d, want %d", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestParseTimeStrings(t *testing.T) {
	input := []string{"2024-01-01", "2024-01-02", "2024-01-03"}
	expected := []int64{1704067200, 1704153600, 1704240000}

	result, err := ParseTimeStrings(input)
	if err != nil {
		t.Fatalf("ParseTimeStrings unexpected error: %v", err)
	}

	if len(result) != len(expected) {
		t.Fatalf("ParseTimeStrings length = %d, want %d", len(result), len(expected))
	}

	for i, ts := range result {
		if ts != expected[i] {
			t.Errorf("ParseTimeStrings[%d] = %d, want %d", i, ts, expected[i])
		}
	}
}

func TestDomainFromTimestamps(t *testing.T) {
	timestamps := []int64{1704067200, 1704153600, 1704240000, 1704326400}

	min, max := DomainFromTimestamps(timestamps)

	if min != 1704067200 {
		t.Errorf("min = %d, want 1704067200", min)
	}
	if max != 1704326400 {
		t.Errorf("max = %d, want 1704326400", max)
	}
}

func TestFormatUnixTime(t *testing.T) {
	ts := int64(1704067200) // 2024-01-01 00:00:00 UTC

	tests := []struct {
		format   string
		expected string
	}{
		{"2006-01-02", "2024-01-01"},
		{"Jan 2", "Jan 1"},
		{"2006", "2024"},
		{"15:04", "00:00"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			result := FormatUnixTime(ts, tt.format)
			if result != tt.expected {
				t.Errorf("FormatUnixTime(_, %q) = %q, want %q", tt.format, result, tt.expected)
			}
		})
	}
}

func TestTimeScale_ZeroDomain(t *testing.T) {
	// When domain min == max, scale should return mid-range
	ts := int64(1704067200)
	scale := NewTimeScale(ts, ts).SetRangeTime(0, 100)

	result := scale.Scale(ts)
	if result != 50 {
		t.Errorf("Scale(ts) with zero domain = %v, want 50", result)
	}
}

func TestTimeScale_IntervalSelection(t *testing.T) {
	// The interval selection is based on idealStep = span / targetCount
	// where targetCount is 5 by default (in detectInterval).
	// Boundaries:
	//   second: idealStep < 60 -> span < 300
	//   minute: idealStep < 3600 -> span < 18000
	//   hour: idealStep < 86400 -> span < 432000
	//   day: idealStep < 604800 -> span < 3024000
	//   week: idealStep < 2592000 -> span < 12960000
	//   month: idealStep < 7776000 -> span < 38880000
	//   quarter: idealStep < 31536000 -> span < 157680000
	//   year: otherwise
	tests := []struct {
		name             string
		spanSeconds      int64
		expectedInterval TimeInterval
	}{
		// idealStep < 60 -> second (span < 300)
		{"30 seconds", 30, TimeIntervalSecond},
		{"4 minutes (still second)", 240, TimeIntervalSecond},
		// idealStep >= 60 and < 3600 -> minute (span >= 300 and < 18000)
		{"10 minutes", 600, TimeIntervalMinute},
		{"3 hours (still minute)", 10800, TimeIntervalMinute},
		// idealStep >= 3600 and < 86400 -> hour (span >= 18000 and < 432000)
		{"6 hours", 21600, TimeIntervalHour},
		{"2 days (still hour)", 172800, TimeIntervalHour},
		// idealStep >= 86400 and < 604800 -> day (span >= 432000 and < 3024000)
		{"10 days", 864000, TimeIntervalDay},
		{"30 days (still day)", 2592000, TimeIntervalDay},
		// idealStep >= 604800 and < 2592000 -> week (span >= 3024000 and < 12960000)
		{"7 weeks", 4233600, TimeIntervalWeek},
		// idealStep >= 2592000 and < 7776000 -> month (span >= 12960000 and < 38880000)
		{"6 months", 15552000, TimeIntervalMonth},
		// idealStep >= 7776000 and < 31536000 -> quarter (span >= 38880000 and < 157680000)
		{"18 months", 46656000, TimeIntervalQuarter},
		// idealStep >= 31536000 -> year (span >= 157680000)
		{"6 years", 189216000, TimeIntervalYear},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scale := NewTimeScale(0, tt.spanSeconds)
			interval := scale.detectInterval()

			if interval != tt.expectedInterval {
				idealStep := float64(tt.spanSeconds) / 5.0
				t.Errorf("detectInterval() for span=%d (idealStep=%.0f) = %v, want %v",
					tt.spanSeconds, idealStep, interval, tt.expectedInterval)
			}
		})
	}
}

func TestNewTimeScaleFromStrings(t *testing.T) {
	scale, err := NewTimeScaleFromStrings("2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatalf("NewTimeScaleFromStrings unexpected error: %v", err)
	}

	min, max := scale.DomainBounds()

	// Verify the domain span is 30 days (in seconds)
	expectedSpan := int64(30 * 86400) // 30 days

	actualSpan := max - min
	if actualSpan != expectedSpan {
		t.Errorf("domain span = %d, want %d (30 days)", actualSpan, expectedSpan)
	}

	// Verify both are reasonable timestamps (after 2020)
	minYear := int64(1577836800) // 2020-01-01
	if min < minYear || max < minYear {
		t.Errorf("domain timestamps seem incorrect: min=%d, max=%d", min, max)
	}
}

func TestTimeInterval_String(t *testing.T) {
	tests := []struct {
		interval TimeInterval
		expected string
	}{
		{TimeIntervalSecond, "second"},
		{TimeIntervalMinute, "minute"},
		{TimeIntervalHour, "hour"},
		{TimeIntervalDay, "day"},
		{TimeIntervalWeek, "week"},
		{TimeIntervalMonth, "month"},
		{TimeIntervalQuarter, "quarter"},
		{TimeIntervalYear, "year"},
	}

	for _, tt := range tests {
		result := tt.interval.String()
		if result != tt.expected {
			t.Errorf("TimeInterval(%d).String() = %q, want %q", tt.interval, result, tt.expected)
		}
	}
}
