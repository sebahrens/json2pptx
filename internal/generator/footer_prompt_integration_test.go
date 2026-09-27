package generator

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateFooterPromptsDoNotLeakIntoGeneratedSlides(t *testing.T) {
	for _, tc := range []struct{ template, layout string }{
		{"blue-corporate", "slideLayout2"},
		{"abstract", "slideLayout3"},
		{"abstract", "slideLayout4"},
	} {
		for _, policy := range []string{"absent", "disabled", "enabled", "skipped"} {
			t.Run(tc.template+"/"+tc.layout+"/"+policy, func(t *testing.T) {
				dir := "../../templates"
				if candidate := os.Getenv("FOOTER_PROMPT_CANDIDATE_DIR"); candidate != "" {
					dir = candidate
				}
				var footer *FooterConfig
				if policy != "absent" {
					footer = &FooterConfig{Enabled: policy != "disabled", LeftText: "Required configured footer"}
				}
				output := filepath.Join(t.TempDir(), "footer.pptx")
				_, _, err := generateSinglePass(context.Background(), GenerationRequest{
					TemplatePath: filepath.Join(dir, tc.template+".pptx"), OutputPath: output,
					ExcludeTemplateSlides: true, Footer: footer,
					Slides: []SlideSpec{{LayoutID: tc.layout, SkipFooter: policy == "skipped", Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Required source title"}}}},
				})
				if err != nil {
					t.Fatal(err)
				}
				var slide struct {
					Texts []string `xml:"cSld>spTree>sp>txBody>p>r>t"`
				}
				if err := xml.Unmarshal([]byte(readZipFileString(t, output, "ppt/slides/slide1.xml")), &slide); err != nil {
					t.Fatal(err)
				}
				joined := strings.Join(slide.Texts, "\n")
				for _, prompt := range []string{"FOOTER TITLE", "PRESENTATION TITLE"} {
					if strings.Contains(joined, prompt) {
						t.Errorf("unrequested template footer prompt leaked: %q", prompt)
					}
				}
				if strings.Count(joined, "Required source title") != 1 {
					t.Error("required title must remain exactly once")
				}
				wantFooter := 0
				if policy == "enabled" {
					wantFooter = 1
				}
				if got := strings.Count(joined, "Required configured footer"); got != wantFooter {
					t.Errorf("configured footer occurs %d times; want %d", got, wantFooter)
				}
			})
		}
	}
}
