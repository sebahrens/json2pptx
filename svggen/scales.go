package svggen

import (
	"fmt"
	"math"
	"time"
)

// Scale is the interface for all scale types.
// Scales map data values to output coordinates.
type Scale interface {
	// Domain returns the input domain bounds.
	Domain() (min, max any)

	// Range returns the output range bounds in pixels/points.
	Range() (min, max float64)

	// SetRange sets the output range.
	SetRange(min, max float64) Scale
}

// =============================================================================
// Linear Scale
// =============================================================================

// LinearScale maps a continuous numeric domain to a continuous output range.
// It performs a linear interpolation between domain and range.
type LinearScale struct {
	domainMin, domainMax float64
	rangeMin, rangeMax   float64

	// Clamp determines whether to clamp output values to the range.
	clamp bool

	// Nice rounds the domain to nice round numbers.
	nice bool
}

// NewLinearScale creates a new linear scale with the given domain.
func NewLinearScale(domainMin, domainMax float64) *LinearScale {
	return &LinearScale{
		domainMin: domainMin,
		domainMax: domainMax,
		rangeMin:  0,
		rangeMax:  1,
		clamp:     false,
		nice:      false,
	}
}

// Domain returns the input domain bounds.
func (s *LinearScale) Domain() (min, max any) {
	return s.domainMin, s.domainMax
}

// DomainBounds returns the numeric domain bounds.
func (s *LinearScale) DomainBounds() (min, max float64) {
	return s.domainMin, s.domainMax
}

// SetDomain sets the input domain.
func (s *LinearScale) SetDomain(min, max float64) *LinearScale {
	s.domainMin = min
	s.domainMax = max
	if s.nice {
		s.applyNice()
	}
	return s
}

// Range returns the output range bounds.
func (s *LinearScale) Range() (min, max float64) {
	return s.rangeMin, s.rangeMax
}

// SetRange sets the output range.
func (s *LinearScale) SetRange(min, max float64) Scale {
	s.rangeMin = min
	s.rangeMax = max
	return s
}

// SetRangeLinear sets the output range and returns the concrete type.
func (s *LinearScale) SetRangeLinear(min, max float64) *LinearScale {
	s.rangeMin = min
	s.rangeMax = max
	return s
}

// Clamp enables or disables output clamping.
func (s *LinearScale) Clamp(clamp bool) *LinearScale {
	s.clamp = clamp
	return s
}

// Nice rounds the domain to nice round numbers.
func (s *LinearScale) Nice(nice bool) *LinearScale {
	s.nice = nice
	if nice {
		s.applyNice()
	}
	return s
}

// defaultNiceTickCount matches the default TickCount (5) used by Axis and
// grid drawing.  Using the same count ensures that applyNice rounds the
// domain to boundaries that align with the tick positions actually shown on
// the chart.  A mismatch (e.g. 10 here vs 5 in Ticks) caused the highest
// visible tick to fall below the data maximum, making data appear to clip.
const defaultNiceTickCount = 5

// applyNice rounds the domain to nice values using tick-aligned steps.
// Uses tickStep (span / count) rather than niceStep(span) to avoid
// over-rounding when the span lands just above a nice boundary
// (e.g., span=5115 → niceStep=5000 → ceil rounds to 10000).
func (s *LinearScale) applyNice() {
	if s.domainMin == s.domainMax {
		return
	}

	origMin := s.domainMin
	origMax := s.domainMax

	step := tickStep(s.domainMin, s.domainMax, defaultNiceTickCount)
	if step == 0 {
		return
	}

	s.domainMin = math.Floor(s.domainMin/step) * step
	s.domainMax = math.Ceil(s.domainMax/step) * step

	// Don't let Nice() push domainMin below 0 when all data is non-negative,
	// or push domainMax above 0 when all data is non-positive.
	// This prevents phantom ticks from appearing outside the data's natural range.
	if origMin >= 0 && s.domainMin < 0 {
		s.domainMin = 0
	}
	if origMax <= 0 && s.domainMax > 0 {
		s.domainMax = 0
	}
}

// Scale maps a domain value to a range value.
func (s *LinearScale) Scale(value float64) float64 {
	// Avoid division by zero
	if s.domainMax == s.domainMin {
		return (s.rangeMin + s.rangeMax) / 2
	}

	// Normalize value to [0, 1]
	t := (value - s.domainMin) / (s.domainMax - s.domainMin)

	// Apply clamping if enabled
	if s.clamp {
		t = clamp01(t)
	}

	// Interpolate to range
	return s.rangeMin + t*(s.rangeMax-s.rangeMin)
}

// Invert maps a range value back to a domain value.
func (s *LinearScale) Invert(value float64) float64 {
	// Avoid division by zero
	if s.rangeMax == s.rangeMin {
		return (s.domainMin + s.domainMax) / 2
	}

	// Normalize value to [0, 1]
	t := (value - s.rangeMin) / (s.rangeMax - s.rangeMin)

	// Apply clamping if enabled
	if s.clamp {
		t = clamp01(t)
	}

	// Interpolate to domain
	return s.domainMin + t*(s.domainMax-s.domainMin)
}

// Ticks returns an array of approximately count evenly-spaced tick values.
func (s *LinearScale) Ticks(count int) []float64 {
	if count <= 0 {
		count = 5
	}

	span := s.domainMax - s.domainMin
	if span == 0 {
		return []float64{s.domainMin}
	}

	// Calculate the step size
	step := tickStep(s.domainMin, s.domainMax, count)
	if step == 0 {
		return []float64{s.domainMin}
	}

	// Generate ticks
	start := math.Ceil(s.domainMin/step) * step
	end := math.Floor(s.domainMax/step) * step

	var ticks []float64
	for v := start; v <= end+step/2; v += step {
		ticks = append(ticks, v)
	}

	return ticks
}

// TickFormat returns a format string for tick values.
func (s *LinearScale) TickFormat(count int) string {
	span := math.Abs(s.domainMax - s.domainMin)
	step := tickStep(s.domainMin, s.domainMax, count)

	// Determine precision based on step size
	precision := int(math.Max(0, -math.Floor(math.Log10(math.Abs(step)))))

	if precision == 0 && span >= 1 {
		return "%.0f"
	}
	return fmt.Sprintf("%%.%df", precision)
}

// =============================================================================
// Categorical Scale
// =============================================================================

// CategoricalScale maps discrete categories to positions in a range.
// Categories are placed at evenly-spaced positions with optional padding.
type CategoricalScale struct {
	categories   []string
	rangeMin     float64
	rangeMax     float64
	paddingInner float64 // Padding between bands (0-1)
	paddingOuter float64 // Padding at range edges (0-1)

	// Calculated values
	bandwidth float64
	step      float64
}

// NewCategoricalScale creates a new categorical scale with the given categories.
func NewCategoricalScale(categories []string) *CategoricalScale {
	s := &CategoricalScale{
		categories:   categories,
		rangeMin:     0,
		rangeMax:     1,
		paddingInner: 0.1,
		paddingOuter: 0.05,
	}
	s.recalculate()
	return s
}

// Domain returns the categories as the domain.
func (s *CategoricalScale) Domain() (min, max any) {
	if len(s.categories) == 0 {
		return "", ""
	}
	return s.categories[0], s.categories[len(s.categories)-1]
}

// Categories returns all categories.
func (s *CategoricalScale) Categories() []string {
	return s.categories
}

// SetCategories sets the categories.
func (s *CategoricalScale) SetCategories(categories []string) *CategoricalScale {
	s.categories = categories
	s.recalculate()
	return s
}

// Range returns the output range bounds.
func (s *CategoricalScale) Range() (min, max float64) {
	return s.rangeMin, s.rangeMax
}

// SetRange sets the output range.
func (s *CategoricalScale) SetRange(min, max float64) Scale {
	s.rangeMin = min
	s.rangeMax = max
	s.recalculate()
	return s
}

// SetRangeCategorical sets the output range and returns the concrete type.
func (s *CategoricalScale) SetRangeCategorical(min, max float64) *CategoricalScale {
	s.rangeMin = min
	s.rangeMax = max
	s.recalculate()
	return s
}

// PaddingInner sets the inner padding between bands (0-1).
func (s *CategoricalScale) PaddingInner(padding float64) *CategoricalScale {
	s.paddingInner = clamp01(padding)
	s.recalculate()
	return s
}

// PaddingOuter sets the outer padding at range edges (0-1).
func (s *CategoricalScale) PaddingOuter(padding float64) *CategoricalScale {
	s.paddingOuter = clamp01(padding)
	s.recalculate()
	return s
}

// Padding sets both inner and outer padding.
func (s *CategoricalScale) Padding(inner, outer float64) *CategoricalScale {
	s.paddingInner = clamp01(inner)
	s.paddingOuter = clamp01(outer)
	s.recalculate()
	return s
}

// recalculate updates the bandwidth and step based on current settings.
func (s *CategoricalScale) recalculate() {
	n := len(s.categories)
	if n == 0 {
		s.bandwidth = 0
		s.step = 0
		return
	}

	rangeSpan := s.rangeMax - s.rangeMin
	if rangeSpan == 0 {
		s.bandwidth = 0
		s.step = 0
		return
	}

	// Calculate step and bandwidth
	// step = bandwidth + inner padding
	// Total range = outer padding + n * step + outer padding - inner padding
	// Since the last band doesn't have inner padding after it

	paddingTotal := s.paddingOuter*2 + s.paddingInner*float64(n-1)
	bandwidthFraction := 1.0 / (float64(n) + paddingTotal)

	s.bandwidth = rangeSpan * bandwidthFraction
	s.step = s.bandwidth * (1 + s.paddingInner)
}

// Scale maps a category to its center position.
func (s *CategoricalScale) Scale(category string) float64 {
	index := s.indexOf(category)
	if index < 0 {
		return s.rangeMin // Category not found
	}

	// Start position with outer padding
	start := s.rangeMin + s.paddingOuter*s.bandwidth

	// Position at center of band
	return start + float64(index)*s.step + s.bandwidth/2
}

// ScaleStart maps a category to the start of its band.
func (s *CategoricalScale) ScaleStart(category string) float64 {
	index := s.indexOf(category)
	if index < 0 {
		return s.rangeMin
	}

	start := s.rangeMin + s.paddingOuter*s.bandwidth
	return start + float64(index)*s.step
}

// ScaleEnd maps a category to the end of its band.
func (s *CategoricalScale) ScaleEnd(category string) float64 {
	return s.ScaleStart(category) + s.bandwidth
}

// Bandwidth returns the width of each band.
func (s *CategoricalScale) Bandwidth() float64 {
	return s.bandwidth
}

// Step returns the distance between the starts of adjacent bands.
func (s *CategoricalScale) Step() float64 {
	return s.step
}

// indexOf returns the index of a category, or -1 if not found.
func (s *CategoricalScale) indexOf(category string) int {
	for i, c := range s.categories {
		if c == category {
			return i
		}
	}
	return -1
}

// Invert returns the category at the given position.
func (s *CategoricalScale) Invert(value float64) string {
	if len(s.categories) == 0 {
		return ""
	}

	// Find which band the value falls into
	start := s.rangeMin + s.paddingOuter*s.bandwidth
	for i, cat := range s.categories {
		bandStart := start + float64(i)*s.step
		bandEnd := bandStart + s.bandwidth
		if value >= bandStart && value <= bandEnd {
			return cat
		}
	}

	// If not in a band, find the closest
	if value <= s.rangeMin {
		return s.categories[0]
	}
	if value >= s.rangeMax {
		return s.categories[len(s.categories)-1]
	}

	// Find closest by index
	index := int((value - start) / s.step)
	if index < 0 {
		index = 0
	}
	if index >= len(s.categories) {
		index = len(s.categories) - 1
	}
	return s.categories[index]
}

// =============================================================================
// Tick Generation Utilities
// =============================================================================

// tickStep calculates a nice step size for generating approximately count ticks.
func tickStep(start, stop float64, count int) float64 {
	if count <= 0 {
		count = 5
	}

	span := stop - start
	if span == 0 {
		return 0
	}

	rawStep := span / float64(count)
	return niceStep(rawStep)
}

// niceStep rounds a step size to a nice round number.
// Nice numbers are 1, 2, 5, 10, 20, 50, etc.
func niceStep(step float64) float64 {
	if step == 0 {
		return 0
	}

	negative := step < 0
	step = math.Abs(step)

	// Find the order of magnitude
	magnitude := math.Pow(10, math.Floor(math.Log10(step)))
	normalized := step / magnitude

	// Round to nearest nice number
	var nice float64
	switch {
	case normalized <= 1.5:
		nice = 1
	case normalized <= 3:
		nice = 2
	case normalized <= 7:
		nice = 5
	default:
		nice = 10
	}

	result := nice * magnitude
	if negative {
		return -result
	}
	return result
}

// clamp01 clamps a value to the range [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// =============================================================================
// Log Scale
// =============================================================================

// LogScale maps a continuous positive numeric domain to a continuous output
// range using a base-10 logarithmic transformation.  Values are internally
// converted to log10 space so that orders of magnitude receive equal visual
// weight.  This scale is used for bar charts where the data spans 3+ orders
// of magnitude (e.g. 0.001 to 1,000,000,000).
type LogScale struct {
	domainMin, domainMax float64 // original (non-log) domain bounds
	rangeMin, rangeMax   float64
	clamp                bool
}

// NewLogScale creates a log scale for the given positive domain bounds.
// Both domainMin and domainMax must be > 0.  If not, the scale falls back
// to treating them as 1 to avoid log(0).
func NewLogScale(domainMin, domainMax float64) *LogScale {
	if domainMin <= 0 {
		domainMin = 1
	}
	if domainMax <= 0 {
		domainMax = 1
	}
	return &LogScale{
		domainMin: domainMin,
		domainMax: domainMax,
		rangeMin:  0,
		rangeMax:  1,
	}
}

// Domain returns the input domain bounds.
func (s *LogScale) Domain() (min, max any) {
	return s.domainMin, s.domainMax
}

// DomainBounds returns the numeric domain bounds (original, not log-transformed).
func (s *LogScale) DomainBounds() (min, max float64) {
	return s.domainMin, s.domainMax
}

// Range returns the output range bounds.
func (s *LogScale) Range() (min, max float64) {
	return s.rangeMin, s.rangeMax
}

// SetRange sets the output range.
func (s *LogScale) SetRange(min, max float64) Scale {
	s.rangeMin = min
	s.rangeMax = max
	return s
}

// SetRangeLog sets the output range and returns the concrete type.
func (s *LogScale) SetRangeLog(min, max float64) *LogScale {
	s.rangeMin = min
	s.rangeMax = max
	return s
}

// Clamp enables or disables output clamping.
func (s *LogScale) Clamp(clamp bool) *LogScale {
	s.clamp = clamp
	return s
}

// Scale maps a domain value to a range value using log10 interpolation.
// Values <= 0 are clamped to domainMin.
func (s *LogScale) Scale(value float64) float64 {
	if value <= 0 {
		value = s.domainMin
	}

	logMin := math.Log10(s.domainMin)
	logMax := math.Log10(s.domainMax)

	if logMax == logMin {
		return (s.rangeMin + s.rangeMax) / 2
	}

	t := (math.Log10(value) - logMin) / (logMax - logMin)

	if s.clamp {
		t = clamp01(t)
	}

	return s.rangeMin + t*(s.rangeMax-s.rangeMin)
}

// Invert maps a range value back to a domain value.
func (s *LogScale) Invert(value float64) float64 {
	logMin := math.Log10(s.domainMin)
	logMax := math.Log10(s.domainMax)

	if s.rangeMax == s.rangeMin {
		return math.Pow(10, (logMin+logMax)/2)
	}

	t := (value - s.rangeMin) / (s.rangeMax - s.rangeMin)
	if s.clamp {
		t = clamp01(t)
	}

	logVal := logMin + t*(logMax-logMin)
	return math.Pow(10, logVal)
}

// Ticks returns tick values at powers of 10 within the domain.
func (s *LogScale) Ticks(count int) []float64 {
	logMin := math.Floor(math.Log10(s.domainMin))
	logMax := math.Ceil(math.Log10(s.domainMax))

	var ticks []float64
	for exp := logMin; exp <= logMax; exp++ {
		v := math.Pow(10, exp)
		ticks = append(ticks, v)
	}

	// If too many ticks, thin them
	if count > 0 && len(ticks) > count*2 {
		step := len(ticks) / count
		if step < 1 {
			step = 1
		}
		var thinned []float64
		for i := 0; i < len(ticks); i += step {
			thinned = append(thinned, ticks[i])
		}
		// Always include last
		if len(thinned) > 0 && thinned[len(thinned)-1] != ticks[len(ticks)-1] {
			thinned = append(thinned, ticks[len(ticks)-1])
		}
		ticks = thinned
	}

	return ticks
}

// TickFormat returns a format function label for the given tick value.
// Uses compact notation: 0.001, 0.01, 0.1, 1, 10, 100, 1K, 10K, 100K, 1M, etc.
func (s *LogScale) TickFormat(count int) string {
	// This is a placeholder; log scale labels are generated per-tick
	// via FormatLogLabel instead of a single printf format.
	return "%g"
}

// FormatLogLabel formats a value using compact notation suitable for
// log-scale axis labels.
func FormatLogLabel(v float64) string {
	abs := math.Abs(v)
	sign := ""
	if v < 0 {
		sign = "-"
	}
	switch {
	case abs >= 1e12:
		return fmt.Sprintf("%s%.0fT", sign, abs/1e12)
	case abs >= 1e9:
		return fmt.Sprintf("%s%.0fB", sign, abs/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("%s%.0fM", sign, abs/1e6)
	case abs >= 1e3:
		return fmt.Sprintf("%s%.0fK", sign, abs/1e3)
	case abs >= 1:
		return fmt.Sprintf("%s%.0f", sign, abs)
	case abs >= 0.1:
		return fmt.Sprintf("%s%.1f", sign, abs)
	case abs >= 0.01:
		return fmt.Sprintf("%s%.2f", sign, abs)
	case abs >= 0.001:
		return fmt.Sprintf("%s%.3f", sign, abs)
	default:
		return fmt.Sprintf("%s%g", sign, abs)
	}
}

// FormatCompact formats an arbitrary numeric value using compact notation
// with K/M/B/T suffixes.  Unlike FormatLogLabel (which only handles exact
// powers of 10), FormatCompact handles arbitrary values:
//
//	0        -> "0"
//	500      -> "500"
//	1500     -> "1.5K"
//	10000    -> "10K"
//	2500000  -> "2.5M"
//	-1500    -> "-1.5K"
//
// Trailing ".0" is always stripped (1.0K -> "1K").
func FormatCompact(v float64) string {
	abs := math.Abs(v)
	sign := ""
	if v < 0 {
		sign = "-"
	}

	switch {
	case abs >= 1e12:
		return sign + trimTrailingZero(fmt.Sprintf("%.1f", abs/1e12)) + "T"
	case abs >= 1e9:
		return sign + trimTrailingZero(fmt.Sprintf("%.1f", abs/1e9)) + "B"
	case abs >= 1e6:
		return sign + trimTrailingZero(fmt.Sprintf("%.1f", abs/1e6)) + "M"
	case abs >= 1e3:
		return sign + trimTrailingZero(fmt.Sprintf("%.1f", abs/1e3)) + "K"
	default:
		// For values < 1000, format as integer if whole, otherwise 1 decimal
		if abs == math.Trunc(abs) {
			return fmt.Sprintf("%s%.0f", sign, abs)
		}
		return fmt.Sprintf("%s%g", sign, abs)
	}
}

// trimTrailingZero removes a trailing ".0" from a formatted number string.
// "1.0" -> "1", "1.5" -> "1.5", "10.0" -> "10"
func trimTrailingZero(s string) string {
	if len(s) >= 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}

// =============================================================================
// Scale Utilities
// =============================================================================

// ExtendDomain extends a domain to include the specified value.
func ExtendDomain(domainMin, domainMax, value float64) (float64, float64) {
	if value < domainMin {
		domainMin = value
	}
	if value > domainMax {
		domainMax = value
	}
	return domainMin, domainMax
}

// DomainFromValues creates a domain from a slice of values.
func DomainFromValues(values []float64) (min, max float64) {
	if len(values) == 0 {
		return 0, 0
	}

	min = values[0]
	max = values[0]

	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	return min, max
}

// DomainWithMargin extends a domain by a percentage margin.
func DomainWithMargin(min, max, margin float64) (float64, float64) {
	span := max - min
	if span == 0 {
		// Handle zero span
		if min == 0 {
			return -1, 1
		}
		span = math.Abs(min) * 0.1
	}

	padding := span * margin
	return min - padding, max + padding
}

// DomainIncludeZero extends a domain to include zero if not already included.
func DomainIncludeZero(min, max float64) (float64, float64) {
	if min > 0 {
		min = 0
	}
	if max < 0 {
		max = 0
	}
	return min, max
}

// =============================================================================
// Time Scale
// =============================================================================

// TimeInterval represents standard time intervals for tick generation.
type TimeInterval int

const (
	TimeIntervalSecond TimeInterval = iota
	TimeIntervalMinute
	TimeIntervalHour
	TimeIntervalDay
	TimeIntervalWeek
	TimeIntervalMonth
	TimeIntervalQuarter
	TimeIntervalYear
)

// String returns a human-readable name for the interval.
func (ti TimeInterval) String() string {
	switch ti {
	case TimeIntervalSecond:
		return "second"
	case TimeIntervalMinute:
		return "minute"
	case TimeIntervalHour:
		return "hour"
	case TimeIntervalDay:
		return "day"
	case TimeIntervalWeek:
		return "week"
	case TimeIntervalMonth:
		return "month"
	case TimeIntervalQuarter:
		return "quarter"
	case TimeIntervalYear:
		return "year"
	default:
		return "unknown"
	}
}

// TimeScale maps a continuous time domain to a continuous output range.
// It provides time-aware tick generation with human-readable intervals.
type TimeScale struct {
	domainMin, domainMax int64 // Unix timestamps in seconds
	rangeMin, rangeMax   float64

	// Clamp determines whether to clamp output values to the range.
	clamp bool

	// ForceInterval overrides automatic interval detection.
	forceInterval *TimeInterval
}

// NewTimeScale creates a new time scale with the given time domain.
// Times should be provided as time.Time values or Unix timestamps.
func NewTimeScale(domainMin, domainMax int64) *TimeScale {
	return &TimeScale{
		domainMin: domainMin,
		domainMax: domainMax,
		rangeMin:  0,
		rangeMax:  1,
		clamp:     false,
	}
}

// NewTimeScaleFromStrings creates a time scale by parsing ISO8601 date strings.
// Supported formats: "2006-01-02", "2006-01-02T15:04:05", "2006-01-02T15:04:05Z07:00"
func NewTimeScaleFromStrings(minStr, maxStr string) (*TimeScale, error) {
	minTime, err := ParseTimeString(minStr)
	if err != nil {
		return nil, fmt.Errorf("parsing domain min: %w", err)
	}
	maxTime, err := ParseTimeString(maxStr)
	if err != nil {
		return nil, fmt.Errorf("parsing domain max: %w", err)
	}
	return NewTimeScale(minTime, maxTime), nil
}

// Domain returns the input domain bounds as Unix timestamps.
func (s *TimeScale) Domain() (min, max any) {
	return s.domainMin, s.domainMax
}

// DomainBounds returns the numeric domain bounds as Unix timestamps.
func (s *TimeScale) DomainBounds() (min, max int64) {
	return s.domainMin, s.domainMax
}

// SetDomain sets the input domain using Unix timestamps.
func (s *TimeScale) SetDomain(min, max int64) *TimeScale {
	s.domainMin = min
	s.domainMax = max
	return s
}

// Range returns the output range bounds.
func (s *TimeScale) Range() (min, max float64) {
	return s.rangeMin, s.rangeMax
}

// SetRange sets the output range.
func (s *TimeScale) SetRange(min, max float64) Scale {
	s.rangeMin = min
	s.rangeMax = max
	return s
}

// SetRangeTime sets the output range and returns the concrete type.
func (s *TimeScale) SetRangeTime(min, max float64) *TimeScale {
	s.rangeMin = min
	s.rangeMax = max
	return s
}

// Clamp enables or disables output clamping.
func (s *TimeScale) Clamp(clamp bool) *TimeScale {
	s.clamp = clamp
	return s
}

// ForceInterval forces a specific tick interval.
func (s *TimeScale) ForceInterval(interval TimeInterval) *TimeScale {
	s.forceInterval = &interval
	return s
}

// Scale maps a Unix timestamp to a range value.
func (s *TimeScale) Scale(timestamp int64) float64 {
	// Avoid division by zero
	if s.domainMax == s.domainMin {
		return (s.rangeMin + s.rangeMax) / 2
	}

	// Normalize value to [0, 1]
	t := float64(timestamp-s.domainMin) / float64(s.domainMax-s.domainMin)

	// Apply clamping if enabled
	if s.clamp {
		t = clamp01(t)
	}

	// Interpolate to range
	return s.rangeMin + t*(s.rangeMax-s.rangeMin)
}

// Invert maps a range value back to a Unix timestamp.
func (s *TimeScale) Invert(value float64) int64 {
	// Avoid division by zero
	if s.rangeMax == s.rangeMin {
		return (s.domainMin + s.domainMax) / 2
	}

	// Normalize value to [0, 1]
	t := (value - s.rangeMin) / (s.rangeMax - s.rangeMin)

	// Apply clamping if enabled
	if s.clamp {
		t = clamp01(t)
	}

	// Interpolate to domain
	return s.domainMin + int64(t*float64(s.domainMax-s.domainMin))
}

// Ticks returns an array of evenly-spaced tick timestamps with human-readable intervals.
func (s *TimeScale) Ticks(count int) []int64 {
	if count <= 0 {
		count = 5
	}

	span := s.domainMax - s.domainMin
	if span == 0 {
		return []int64{s.domainMin}
	}

	// Determine the best interval
	interval := s.selectInterval(span, count)

	// Generate ticks aligned to interval boundaries
	return s.generateAlignedTicks(interval, count)
}

// TicksWithLabels returns tick timestamps along with formatted labels.
func (s *TimeScale) TicksWithLabels(count int) ([]int64, []string) {
	ticks := s.Ticks(count)
	labels := make([]string, len(ticks))

	interval := s.detectInterval()
	format := s.formatForInterval(interval)

	for i, ts := range ticks {
		labels[i] = FormatUnixTime(ts, format)
	}

	return ticks, labels
}

// selectInterval chooses an appropriate time interval based on domain span.
func (s *TimeScale) selectInterval(spanSeconds int64, targetCount int) TimeInterval {
	if s.forceInterval != nil {
		return *s.forceInterval
	}

	// Calculate ideal step size in seconds
	idealStep := float64(spanSeconds) / float64(targetCount)

	// Time intervals in seconds
	const (
		second  = 1
		minute  = 60
		hour    = 3600
		day     = 86400
		week    = 604800
		month   = 2592000  // ~30 days
		quarter = 7776000  // ~90 days
		year    = 31536000 // 365 days
	)

	// Select interval based on ideal step size
	switch {
	case idealStep < minute:
		return TimeIntervalSecond
	case idealStep < hour:
		return TimeIntervalMinute
	case idealStep < day:
		return TimeIntervalHour
	case idealStep < week:
		return TimeIntervalDay
	case idealStep < month:
		return TimeIntervalWeek
	case idealStep < quarter:
		return TimeIntervalMonth
	case idealStep < year:
		return TimeIntervalQuarter
	default:
		return TimeIntervalYear
	}
}

// detectInterval determines the current interval from domain span.
func (s *TimeScale) detectInterval() TimeInterval {
	span := s.domainMax - s.domainMin
	return s.selectInterval(span, 5)
}

// generateAlignedTicks creates ticks aligned to natural boundaries.
func (s *TimeScale) generateAlignedTicks(interval TimeInterval, targetCount int) []int64 {
	var ticks []int64

	// Get step size in seconds for this interval
	stepSeconds := s.intervalStep(interval)

	// Align start to interval boundary
	start := s.alignToInterval(s.domainMin, interval)

	// Generate ticks
	for ts := start; ts <= s.domainMax; ts += stepSeconds {
		if ts >= s.domainMin {
			ticks = append(ticks, ts)
		}
	}

	// If we have too many ticks, skip some
	if len(ticks) > targetCount*2 {
		ticks = s.decimateTicks(ticks, targetCount)
	}

	return ticks
}

// alignToInterval rounds a timestamp down to the nearest interval boundary.
func (s *TimeScale) alignToInterval(ts int64, interval TimeInterval) int64 {
	switch interval {
	case TimeIntervalSecond:
		return ts
	case TimeIntervalMinute:
		return (ts / 60) * 60
	case TimeIntervalHour:
		return (ts / 3600) * 3600
	case TimeIntervalDay:
		return (ts / 86400) * 86400
	case TimeIntervalWeek:
		// Align to Monday (Unix epoch was Thursday)
		daysSinceEpoch := ts / 86400
		daysToMonday := (daysSinceEpoch + 3) % 7 // Thursday + 3 = Sunday, so we need to adjust
		return (daysSinceEpoch - daysToMonday) * 86400
	case TimeIntervalMonth:
		// Approximate: align to start of 30-day period
		return (ts / 2592000) * 2592000
	case TimeIntervalQuarter:
		// Approximate: align to start of 90-day period
		return (ts / 7776000) * 7776000
	case TimeIntervalYear:
		// Approximate: align to start of 365-day period
		return (ts / 31536000) * 31536000
	default:
		return ts
	}
}

// intervalStep returns the step size in seconds for an interval.
func (s *TimeScale) intervalStep(interval TimeInterval) int64 {
	switch interval {
	case TimeIntervalSecond:
		return 1
	case TimeIntervalMinute:
		return 60
	case TimeIntervalHour:
		return 3600
	case TimeIntervalDay:
		return 86400
	case TimeIntervalWeek:
		return 604800
	case TimeIntervalMonth:
		return 2592000 // ~30 days
	case TimeIntervalQuarter:
		return 7776000 // ~90 days
	case TimeIntervalYear:
		return 31536000 // 365 days
	default:
		return 86400 // Default to days
	}
}

// decimateTicks reduces the number of ticks by skipping some.
func (s *TimeScale) decimateTicks(ticks []int64, targetCount int) []int64 {
	if len(ticks) <= targetCount {
		return ticks
	}

	skip := len(ticks) / targetCount
	if skip < 1 {
		skip = 1
	}

	var result []int64
	for i := 0; i < len(ticks); i += skip {
		result = append(result, ticks[i])
	}

	// Always include the last tick
	if len(result) > 0 && result[len(result)-1] != ticks[len(ticks)-1] {
		result = append(result, ticks[len(ticks)-1])
	}

	return result
}

// formatForInterval returns an appropriate format string for the interval.
func (s *TimeScale) formatForInterval(interval TimeInterval) string {
	switch interval {
	case TimeIntervalSecond:
		return "15:04:05"
	case TimeIntervalMinute:
		return "15:04"
	case TimeIntervalHour:
		return "15:00"
	case TimeIntervalDay:
		return "Jan 2"
	case TimeIntervalWeek:
		return "Jan 2"
	case TimeIntervalMonth:
		return "Jan '06"
	case TimeIntervalQuarter:
		return "Q1 '06"
	case TimeIntervalYear:
		return "2006"
	default:
		return "Jan 2, 2006"
	}
}

// TickFormat returns a format string for tick values based on the domain span.
func (s *TimeScale) TickFormat() string {
	interval := s.detectInterval()
	return s.formatForInterval(interval)
}

// =============================================================================
// Time Parsing Utilities
// =============================================================================

// Common time formats supported by ParseTimeString.
var timeFormats = []string{
	"2006-01-02T15:04:05Z07:00", // ISO8601 with timezone
	"2006-01-02T15:04:05Z",      // ISO8601 UTC
	"2006-01-02T15:04:05",       // ISO8601 local
	"2006-01-02 15:04:05",       // SQL datetime
	"2006-01-02",                // Date only
	"01/02/2006",                // US date
	"02/01/2006",                // EU date
	"2006/01/02",                // Asian date
}

// ParseTimeString parses a time string in various common formats.
// Returns Unix timestamp in seconds.
func ParseTimeString(s string) (int64, error) {
	// Try standard formats first (more common for date strings)
	for _, format := range timeFormats {
		t, err := parseTime(s, format)
		if err == nil {
			return t.Unix(), nil
		}
	}

	// Then try Unix timestamp (pure numeric string)
	if ts, err := parseUnixTimestamp(s); err == nil {
		return ts, nil
	}

	return 0, fmt.Errorf("cannot parse time: %q", s)
}

// parseUnixTimestamp tries to parse a string as a Unix timestamp.
// Only accepts timestamps that look reasonable (epoch after 2000-01-01 and before 2200).
func parseUnixTimestamp(s string) (int64, error) {
	var ts int64
	n, err := fmt.Sscanf(s, "%d", &ts)
	if err != nil || n != 1 {
		return 0, fmt.Errorf("not a number")
	}

	// Make sure the entire string was consumed (no trailing characters)
	if len(fmt.Sprintf("%d", ts)) != len(s) {
		return 0, fmt.Errorf("not a pure timestamp")
	}

	// Validate it looks like a reasonable Unix timestamp:
	// - Minimum: 946684800 (2000-01-01 00:00:00 UTC)
	// - Maximum: 7258118400 (2200-01-01 00:00:00 UTC)
	// This prevents interpreting year-like numbers (e.g., 2024) as timestamps
	if ts < 946684800 || ts > 7258118400 {
		return 0, fmt.Errorf("timestamp out of range")
	}
	return ts, nil
}

// FormatUnixTime formats a Unix timestamp using the given format.
func FormatUnixTime(ts int64, format string) string {
	t := unixToTime(ts)
	return formatTime(t, format)
}

// DomainFromTimestamps creates a domain from a slice of Unix timestamps.
func DomainFromTimestamps(timestamps []int64) (min, max int64) {
	if len(timestamps) == 0 {
		return 0, 0
	}

	min = timestamps[0]
	max = timestamps[0]

	for _, ts := range timestamps[1:] {
		if ts < min {
			min = ts
		}
		if ts > max {
			max = ts
		}
	}

	return min, max
}

// ParseTimeStrings parses a slice of time strings into Unix timestamps.
func ParseTimeStrings(strings []string) ([]int64, error) {
	timestamps := make([]int64, len(strings))
	for i, s := range strings {
		ts, err := ParseTimeString(s)
		if err != nil {
			return nil, fmt.Errorf("parsing time at index %d: %w", i, err)
		}
		timestamps[i] = ts
	}
	return timestamps, nil
}

// parseTime parses a time string using the given format.
func parseTime(s, format string) (time.Time, error) {
	return time.Parse(format, s)
}

// unixToTime converts a Unix timestamp to a time.Time.
func unixToTime(ts int64) time.Time {
	return time.Unix(ts, 0).UTC()
}

// formatTime formats a time.Time using the given format.
// Handles special quarter format "Q1 '06".
func formatTime(t time.Time, format string) string {
	// Handle quarter format specially
	if format == "Q1 '06" {
		quarter := (t.Month()-1)/3 + 1
		return fmt.Sprintf("Q%d '%02d", quarter, t.Year()%100)
	}
	return t.Format(format)
}
