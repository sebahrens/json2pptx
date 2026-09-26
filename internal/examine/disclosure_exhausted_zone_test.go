package examine

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDisclosureExhaustedZoneDoesNotAdvertiseLegalTextAsFreeSpace(t *testing.T) {
	const width, height int64 = 12192000, 6858000
	phs := []PlaceholderReport{
		{Role: string(types.PlaceholderRoleTitle), Bounds: BoundsReport{YEMU: 100000, HEMU: 5200000}},
		{Role: string(types.PlaceholderRoleDisclosure), Bounds: BoundsReport{YEMU: 5000000, HEMU: 1000000}},
	}
	zone := computeZone(phs, width, height)
	if zone.BottomEMU > 5000000 || zone.BottomEMU != zone.TopEMU {
		t.Fatalf("exhausted content area must remain empty above protected legal band, got %+v", zone)
	}
	report := BuildReport(Inputs{SlideWidthEMU: width, SlideHeightEMU: height, Layouts: []types.LayoutMetadata{{
		ID: "exhausted",
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Role: types.PlaceholderRoleTitle, Bounds: types.BoundingBox{Y: 100000, Height: 5200000}},
			{ID: "legal_disclosure", Type: types.PlaceholderSubtitle, Role: types.PlaceholderRoleDisclosure, Bounds: types.BoundingBox{Y: 5000000, Height: 1000000}},
		},
	}}})
	projected := report.Layouts[0].ContentZone
	if projected.BottomEMU > 5000000 || projected.BottomEMU != projected.TopEMU {
		t.Fatalf("actual report projection restored unsafe free space: %+v", projected)
	}
}

func TestDisclosureOnlyBlankLayoutKeepsLegalBandProtectedInReport(t *testing.T) {
	const width, height int64 = 12192000, 6858000
	report := BuildReport(Inputs{SlideWidthEMU: width, SlideHeightEMU: height, Layouts: []types.LayoutMetadata{{
		ID: "blank-legal", CanonicalType: types.CanonicalLayoutBlank,
		Placeholders: []types.PlaceholderInfo{{ID: "legal_disclosure", Type: types.PlaceholderSubtitle, Role: types.PlaceholderRoleDisclosure, Bounds: types.BoundingBox{Y: 5000000, Width: 6000000, Height: 1000000}}},
	}}})
	projected := report.Layouts[0].ContentZone
	if projected.BottomEMU > 5000000 {
		t.Fatalf("blank-layout override exposes required legal band: %+v", projected)
	}
	if report.Layouts[0].Placeholders[0].Role != string(types.PlaceholderRoleDisclosure) {
		t.Fatal("API projection lost legal role")
	}
}
