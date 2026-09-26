package examine

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Explicit legal text must reserve space without being interpreted as a
// generic subtitle/header. This is a promotion check for the new role.
func TestExplicitDisclosureReservesSafeZone(t *testing.T) {
	const width, height int64 = 12192000, 6858000
	legal := PlaceholderReport{
		Role:   string(types.PlaceholderRoleDisclosure),
		Bounds: BoundsReport{XEMU: 353658, YEMU: 5000000, WEMU: 6629400, HEMU: 1000000},
	}
	zone := computeZone([]PlaceholderReport{legal}, width, height)
	if zone.BottomEMU > legal.Bounds.YEMU {
		t.Fatalf("safe content bottom %d overlaps required legal disclosure starting at %d", zone.BottomEMU, legal.Bounds.YEMU)
	}
}

func TestExplicitDisclosureReservesUpperBand(t *testing.T) {
	const width, height int64 = 12192000, 6858000
	legal := PlaceholderReport{
		Role:   string(types.PlaceholderRoleDisclosure),
		Bounds: BoundsReport{XEMU: 353658, YEMU: 100000, WEMU: 6629400, HEMU: 700000},
	}
	zone := computeZone([]PlaceholderReport{legal}, width, height)
	if zone.TopEMU < legal.Bounds.YEMU+legal.Bounds.HEMU {
		t.Fatalf("safe content top %d overlaps upper legal text", zone.TopEMU)
	}
	if zone.BottomEMU != height*95/100 {
		t.Fatalf("upper disclosure changed unrelated bottom: %d", zone.BottomEMU)
	}
}
