package layoutpreview

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDisclosurePreviewDoesNotPublishGenericBulletsInLegalSlot(t *testing.T) {
	for _, kind := range []types.PlaceholderType{types.PlaceholderSubtitle, types.PlaceholderBody, types.PlaceholderContent} {
		layout := types.LayoutMetadata{Placeholders: []types.PlaceholderInfo{{ID: "legal_disclosure", Type: kind, Role: types.PlaceholderRoleDisclosure}}}
		items := SampleContent(layout)
		if len(items) != 1 || items[0].Type != generator.ContentText || items[0].Value != "Illustrative disclosure for review" {
			t.Errorf("%s legal preview replaced required role with ordinary content: %+v", kind, items)
		}
	}
}
