package types

import (
	"testing"
)

func TestEMUConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant EMU
		expected EMU
	}{
		{"EMUPerInch", EMUPerInch, 914400},
		{"EMUPerPoint", EMUPerPoint, 12700},
		{"EMUPerCM", EMUPerCM, 360000},
		{"EMUPerPixel", EMUPerPixel, 9525},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestFromInches(t *testing.T) {
	tests := []struct {
		name     string
		inches   float64
		expected EMU
	}{
		{"AC2: 1 inch", 1.0, 914400},
		{"AC2: 2.5 inches", 2.5, 2286000},
		{"zero inches", 0.0, 0},
		{"half inch", 0.5, 457200},
		{"negative inch", -1.0, -914400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromInches(tt.inches)
			if result != tt.expected {
				t.Errorf("FromInches(%v) = %d, want %d", tt.inches, result, tt.expected)
			}
		})
	}
}

func TestFromPoints(t *testing.T) {
	tests := []struct {
		name     string
		points   float64
		expected EMU
	}{
		{"AC3: 72 points (1 inch)", 72.0, 914400},
		{"zero points", 0.0, 0},
		{"36 points (half inch)", 36.0, 457200},
		{"144 points (2 inches)", 144.0, 1828800},
		{"negative points", -72.0, -914400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromPoints(tt.points)
			if result != tt.expected {
				t.Errorf("FromPoints(%v) = %d, want %d", tt.points, result, tt.expected)
			}
		})
	}
}

func TestFromPixels(t *testing.T) {
	tests := []struct {
		name     string
		pixels   int
		expected EMU
	}{
		{"AC4: 96 pixels (1 inch at 96 DPI)", 96, 914400},
		{"zero pixels", 0, 0},
		{"48 pixels (half inch)", 48, 457200},
		{"192 pixels (2 inches)", 192, 1828800},
		{"single pixel", 1, 9525},
		{"negative pixels", -96, -914400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromPixels(tt.pixels)
			if result != tt.expected {
				t.Errorf("FromPixels(%d) = %d, want %d", tt.pixels, result, tt.expected)
			}
		})
	}
}

func TestFromCM(t *testing.T) {
	tests := []struct {
		name     string
		cm       float64
		expected EMU
	}{
		{"1 cm", 1.0, 360000},
		{"2.54 cm (1 inch)", 2.54, 914400},
		{"zero cm", 0.0, 0},
		{"5 cm", 5.0, 1800000},
		{"negative cm", -1.0, -360000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromCM(tt.cm)
			if result != tt.expected {
				t.Errorf("FromCM(%v) = %d, want %d", tt.cm, result, tt.expected)
			}
		})
	}
}

func TestInches(t *testing.T) {
	tests := []struct {
		name     string
		emu      EMU
		expected float64
	}{
		{"AC5: 914400 EMU to 1 inch", 914400, 1.0},
		{"zero EMU", 0, 0.0},
		{"457200 EMU to 0.5 inch", 457200, 0.5},
		{"2286000 EMU to 2.5 inches", 2286000, 2.5},
		{"negative EMU", -914400, -1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.emu.Inches()
			if result != tt.expected {
				t.Errorf("EMU(%d).Inches() = %f, want %f", tt.emu, result, tt.expected)
			}
		})
	}
}

func TestPoints(t *testing.T) {
	tests := []struct {
		name     string
		emu      EMU
		expected float64
	}{
		{"AC6: 914400 EMU to 72 points", 914400, 72.0},
		{"zero EMU", 0, 0.0},
		{"457200 EMU to 36 points", 457200, 36.0},
		{"1828800 EMU to 144 points", 1828800, 144.0},
		{"negative EMU", -914400, -72.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.emu.Points()
			if result != tt.expected {
				t.Errorf("EMU(%d).Points() = %f, want %f", tt.emu, result, tt.expected)
			}
		})
	}
}

func TestPixels(t *testing.T) {
	tests := []struct {
		name     string
		emu      EMU
		expected int
	}{
		{"AC7: 914400 EMU to 96 pixels", 914400, 96},
		{"zero EMU", 0, 0},
		{"457200 EMU to 48 pixels", 457200, 48},
		{"1828800 EMU to 192 pixels", 1828800, 192},
		{"single pixel worth", 9525, 1},
		{"negative EMU", -914400, -96},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.emu.Pixels()
			if result != tt.expected {
				t.Errorf("EMU(%d).Pixels() = %d, want %d", tt.emu, result, tt.expected)
			}
		})
	}
}

func TestCM(t *testing.T) {
	tests := []struct {
		name     string
		emu      EMU
		expected float64
	}{
		{"360000 EMU to 1 cm", 360000, 1.0},
		{"zero EMU", 0, 0.0},
		{"720000 EMU to 2 cm", 720000, 2.0},
		{"1800000 EMU to 5 cm", 1800000, 5.0},
		{"negative EMU", -360000, -1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.emu.CM()
			if result != tt.expected {
				t.Errorf("EMU(%d).CM() = %f, want %f", tt.emu, result, tt.expected)
			}
		})
	}
}

func TestInt64(t *testing.T) {
	tests := []struct {
		name     string
		emu      EMU
		expected int64
	}{
		{"914400 EMU", 914400, 914400},
		{"zero EMU", 0, 0},
		{"large EMU", 10000000, 10000000},
		{"negative EMU", -914400, -914400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.emu.Int64()
			if result != tt.expected {
				t.Errorf("EMU(%d).Int64() = %d, want %d", tt.emu, result, tt.expected)
			}
		})
	}
}

func TestRoundTripConversions(t *testing.T) {
	t.Run("inches round trip", func(t *testing.T) {
		original := 2.5
		emu := FromInches(original)
		result := emu.Inches()
		if result != original {
			t.Errorf("Inches round trip: %f -> EMU -> %f", original, result)
		}
	})

	t.Run("points round trip", func(t *testing.T) {
		original := 144.0
		emu := FromPoints(original)
		result := emu.Points()
		if result != original {
			t.Errorf("Points round trip: %f -> EMU -> %f", original, result)
		}
	})

	t.Run("pixels round trip", func(t *testing.T) {
		original := 192
		emu := FromPixels(original)
		result := emu.Pixels()
		if result != original {
			t.Errorf("Pixels round trip: %d -> EMU -> %d", original, result)
		}
	})

	t.Run("cm round trip", func(t *testing.T) {
		original := 5.0
		emu := FromCM(original)
		result := emu.CM()
		if result != original {
			t.Errorf("CM round trip: %f -> EMU -> %f", original, result)
		}
	})
}

func TestEMUTypeAlias(t *testing.T) {
	// AC1: EMU type defined as int64 alias
	var e EMU = 914400
	i := int64(e)
	if i != 914400 {
		t.Errorf("EMU type alias conversion failed: got %d, want 914400", i)
	}
}
