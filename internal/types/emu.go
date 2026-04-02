package types

// EMU represents English Metric Units (914400 per inch)
type EMU int64

const (
	EMUPerInch  EMU = 914400
	EMUPerPoint EMU = 12700
	EMUPerCM    EMU = 360000
	EMUPerPixel EMU = 9525 // at 96 DPI
)

// FromInches converts inches to EMU
func FromInches(in float64) EMU {
	return EMU(in * float64(EMUPerInch))
}

// FromPoints converts points to EMU
func FromPoints(pt float64) EMU {
	return EMU(pt * float64(EMUPerPoint))
}

// FromPixels converts pixels to EMU (at 96 DPI)
func FromPixels(px int) EMU {
	return EMU(int64(px) * int64(EMUPerPixel))
}

// FromCM converts centimeters to EMU
func FromCM(cm float64) EMU {
	return EMU(cm * float64(EMUPerCM))
}

// Inches converts EMU to inches
func (e EMU) Inches() float64 {
	return float64(e) / float64(EMUPerInch)
}

// Points converts EMU to points
func (e EMU) Points() float64 {
	return float64(e) / float64(EMUPerPoint)
}

// Pixels converts EMU to pixels (at 96 DPI)
func (e EMU) Pixels() int {
	return int(e / EMUPerPixel)
}

// CM converts EMU to centimeters
func (e EMU) CM() float64 {
	return float64(e) / float64(EMUPerCM)
}

// Int64 returns the raw EMU value
func (e EMU) Int64() int64 {
	return int64(e)
}
