package quality

import (
	"reflect"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestNativeDisclosureProbePreservesLegalSourceWithoutBodyEvidence(t *testing.T) {
	for _, kind := range []types.PlaceholderType{types.PlaceholderSubtitle, types.PlaceholderBody, types.PlaceholderContent} {
		legal := types.PlaceholderInfo{ID: "legal_disclosure", Type: kind, Role: types.PlaceholderRoleDisclosure, Bounds: types.BoundingBox{Width: 6000000, Height: 1000000}}
		layout := types.LayoutMetadata{ID: "legal-only", Placeholders: []types.PlaceholderInfo{legal}}
		if nativeEvidenceSlot(layout, legal) {
			t.Errorf("%s legal slot accepts generic table/chart evidence", kind)
		}
		probes := makeNativeProbes(layout, nativeReferenceImage())
		if len(probes) != 2 {
			t.Fatalf("%s legal-only layout has %d profiles, want both text profiles only", kind, len(probes))
		}
		for _, probe := range probes {
			if len(probe.Slide.Content) != 1 {
				t.Fatal("required legal source missing")
			}
			item := probe.Slide.Content[0]
			if item.Type != generator.ContentText || item.PlaceholderID != "legal_disclosure" || item.Value != "Illustrative disclosure for review" ||
				!reflect.DeepEqual(probe.ExpectedText, []string{"Illustrative disclosure for review"}) {
				t.Fatalf("legal text substituted with ordinary body/subtitle probe: %+v", probe)
			}
		}
	}
}
