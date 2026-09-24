package template

import (
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestClosingLayoutsCenterTitleSubtitleLikeTitleSlide(t *testing.T) {
	for _, templateName := range []string{"midnight-blue", "forest-green"} {
		t.Run(templateName, func(t *testing.T) {
			reader, err := OpenTemplate(filepath.Join("..", "..", "templates", templateName+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			layouts, err := ParseLayouts(reader)
			if err != nil {
				t.Fatal(err)
			}
			var titleSlide, closing *types.LayoutMetadata
			for i := range layouts {
				switch layouts[i].ID {
				case "slideLayout1":
					titleSlide = &layouts[i]
				case "slideLayout5":
					closing = &layouts[i]
				}
			}
			if titleSlide == nil || closing == nil {
				t.Fatal("missing Title Slide or Closing layout")
			}
			for _, role := range []types.PlaceholderType{types.PlaceholderTitle, types.PlaceholderSubtitle} {
				var source, target *types.PlaceholderInfo
				for i := range titleSlide.Placeholders {
					if titleSlide.Placeholders[i].Type == role {
						source = &titleSlide.Placeholders[i]
					}
				}
				for i := range closing.Placeholders {
					if closing.Placeholders[i].Type == role {
						target = &closing.Placeholders[i]
					}
				}
				if source == nil || target == nil {
					t.Fatalf("missing %s placeholder", role)
				}
				if target.Bounds != source.Bounds {
					t.Errorf("Closing %s bounds %+v differ from centred Title Slide %+v", role, target.Bounds, source.Bounds)
				}
			}
		})
	}
}
