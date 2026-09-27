package generator

import "testing"

func TestNativeItalicOnlyStyleSurvivesExtraction(t *testing.T) {
	for _, italic := range []string{"0", "1"} {
		t.Run(italic, func(t *testing.T) {
			original := &runPropertiesXML{Italic: italic}
			_, style := extractTemplateTextStyle([]paragraphXML{{Runs: []runXML{{RunProperties: original, Text: "Native prompt"}}}})
			if style == nil || style.Italic != italic {
				t.Fatalf("native italic-only default lost: %+v", style)
			}
			bullets := extractBulletTemplateStyles([]paragraphXML{{Runs: []runXML{{RunProperties: original, Text: "Native bullet prompt"}}}})
			if len(bullets) != 1 || bullets[0].rProps == nil || bullets[0].rProps.Italic != italic {
				t.Fatalf("native italic-only bullet default lost: %+v", bullets)
			}
			runs := createFormattedRuns("Required authored source", style)
			if len(runs) != 1 || runs[0].RunProperties.Italic != italic {
				t.Fatalf("native italic lost in authored run: %+v", runs)
			}
			style.Italic = "changed"
			if original.Italic != italic {
				t.Fatal("style mutation changed native source")
			}
		})
	}
}
