package slides

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// CompileTitle compiles a title slide: a title placeholder plus an optional
// subtitle. The deck's opening chrome carries no body content.
func CompileTitle(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "title"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	if eyebrow := strField(in.Body, "eyebrow"); eyebrow != "" {
		slide.Eyebrow = eyebrow
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".eyebrow",
			SemanticPath: in.semSlide() + ".eyebrow",
		})
	}

	if subtitle := strField(in.Body, "subtitle"); subtitle != "" {
		idx := appendContent(slide, textContent("subtitle", subtitle))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".subtitle",
		})
	}

	return slide, links, nil
}

// CompileSection compiles a section-divider slide. Shipped templates reserve
// section divider body placeholders for decorative section numbers, so the
// semantic subtitle is intentionally not emitted as placeholder content: doing
// so would be remapped into the section-number slot by placeholder fallback.
func CompileSection(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "section"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	return slide, links, nil
}

// CompileClosing compiles a closing slide. With bullets/points it renders as a
// content slide (title + bullets); otherwise it renders as a title slide with
// an optional subtitle, mirroring the opening chrome. "closing" is a template
// layout name, not a slide_type hint, so the slide_type stays title/content.
func CompileClosing(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	var links []SourceLink

	bullets, sourceField := closingBullets(in.Body)
	if len(bullets) > 0 {
		slide := &deckinput.SlideInput{SlideType: "content"}
		if in.Title != "" {
			idx := appendContent(slide, textContent("title", in.Title))
			links = append(links, SourceLink{
				RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
				SemanticPath: in.semSlide() + ".title",
			})
		}
		idx := appendContent(slide, bulletsContent("body", bullets))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + "." + sourceField,
		})
		return slide, links, nil
	}

	slide := &deckinput.SlideInput{SlideType: "title"}
	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}
	if subtitle := strField(in.Body, "subtitle"); subtitle != "" {
		idx := appendContent(slide, textContent("subtitle", subtitle))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".subtitle",
		})
	}
	return slide, links, nil
}

// closingBullets returns the closing slide's bullet list and the semantic field
// it came from, preferring "bullets" then "points".
func closingBullets(body map[string]any) ([]string, string) {
	if b, ok := stringList(body, "bullets"); ok && len(b) > 0 {
		return b, "bullets"
	}
	if b, ok := stringList(body, "points"); ok && len(b) > 0 {
		return b, "points"
	}
	return nil, ""
}
