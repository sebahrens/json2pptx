# EMU Type Helpers

Add type-safe EMU (English Metric Units) helpers to prevent unit conversion errors.

## Scope

This specification covers ONLY the EMU type and conversion helpers. It does NOT cover:
- Changing existing code that works
- Adding new functionality

## Purpose

OOXML uses English Metric Units (914400 EMUs per inch) for all measurements. Current code has magic numbers scattered throughout:

```go
// Current (error-prone)
const emusPerPixel = 9525
imgWidthEMU := int64(img.Width) * emusPerPixel
```

A type alias with helpers prevents unit confusion.

## Implementation

Add to `internal/types/emu.go`:

```go
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

// Int64 returns the raw EMU value
func (e EMU) Int64() int64 {
    return int64(e)
}
```

## Usage Examples

```go
// Before
imgWidthEMU := int64(img.Width) * 9525

// After
imgWidthEMU := types.FromPixels(img.Width).Int64()

// Or with the type throughout
var width types.EMU = types.FromPixels(img.Width)
var height types.EMU = types.FromPixels(img.Height)
```

## Acceptance Criteria

### AC1: Type Definition
- EMU type defined as int64 alias
- Constants for common conversions defined

### AC2: FromInches
- `FromInches(1.0)` returns 914400
- `FromInches(2.5)` returns 2286000

### AC3: FromPoints
- `FromPoints(72)` returns 914400 (72 points = 1 inch)

### AC4: FromPixels
- `FromPixels(96)` returns 914400 (96 pixels at 96 DPI = 1 inch)

### AC5: Inches Method
- `EMU(914400).Inches()` returns 1.0

### AC6: Points Method
- `EMU(914400).Points()` returns 72.0

### AC7: Pixels Method
- `EMU(914400).Pixels()` returns 96

## Migration

Optionally update existing code to use EMU type:
- `internal/generator/images.go` - image scaling calculations
- `internal/template/layouts.go` - placeholder bounds

Migration is NOT required for AC completion but recommended.

## Testing

The implementation includes comprehensive tests in `internal/types/emu_test.go`:

- `TestEMUConstants` - Verifies all 4 conversion constants
- `TestFromInches` - Tests including AC2 cases (1.0→914400, 2.5→2286000)
- `TestFromPoints` - Tests including AC3 case (72→914400)
- `TestFromPixels` - Tests including AC4 case (96→914400)
- `TestInches` - Tests including AC5 case (914400→1.0)
- `TestPoints` - Tests including AC6 case (914400→72.0)
- `TestPixels` - Tests including AC7 case (914400→96)
- `TestInt64` - Tests raw value extraction
- `TestRoundTripConversions` - Verifies lossless round-trips for inches, points, pixels
- `TestEMUTypeAlias` - Verifies AC1 type alias behavior

## Implementation Status

✅ **COMPLETE** - All acceptance criteria met. Implementation matches spec exactly.
