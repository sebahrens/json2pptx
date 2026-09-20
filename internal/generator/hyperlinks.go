package generator

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func contentLinkMarker(index int) string {
	return fmt.Sprintf("json2pptx_content_link_%d", index)
}

func (ctx *singlePassContext) hyperlinkRelationships(slideNum int) ([]pptx.RelationshipXML, map[string]string, error) {
	spec, ok := ctx.slideContentMap[slideNum]
	if !ok {
		return nil, nil, nil
	}
	// Layout is rId1. Hyperlinks follow every preallocated media and notes ID.
	nextID := ctx.nextHyperlinkRelID(slideNum)
	ids := make(map[string]string)
	var rels []pptx.RelationshipXML
	add := func(key string, link *LinkSpec) error {
		if link == nil {
			return nil
		}
		id := fmt.Sprintf("rId%d", nextID)
		rel, err := ctx.buildLinkRelationship(slideNum, key, id, link)
		if err != nil {
			return err
		}
		nextID++
		ids[key] = id
		rels = append(rels, rel)
		return nil
	}
	if spec.SourceNote != "" {
		if err := add("source", spec.SourceLink); err != nil {
			return nil, nil, err
		}
	}
	for i, item := range spec.Content {
		if item.Link != nil && (item.Type == ContentImage || item.Type == ContentDiagram || item.Type == ContentTable) {
			return nil, nil, fmt.Errorf("slide %d hyperlink content %d: links require text content", slideNum, i+1)
		}
		if err := add(contentLinkMarker(i), item.Link); err != nil {
			return nil, nil, err
		}
	}
	for _, shape := range spec.ShapeLinks {
		if err := add(shape.Marker, &shape.Link); err != nil {
			return nil, nil, err
		}
	}
	return rels, ids, nil
}

func (ctx *singlePassContext) nextHyperlinkRelID(slideNum int) int {
	nextID := 2 + len(ctx.slideRelUpdates[slideNum]) + 2*len(ctx.nativeSVGInserts[slideNum])
	consider := func(id string) {
		var number int
		if _, err := fmt.Sscanf(id, "rId%d", &number); err == nil && number >= nextID {
			nextID = number + 1
		}
	}
	for _, media := range ctx.slideRelUpdates[slideNum] {
		consider(media.relID)
	}
	for _, svg := range ctx.nativeSVGInserts[slideNum] {
		consider(svg.pngRelID)
		consider(svg.svgRelID)
	}
	if bg, ok := ctx.slideBgMedia[slideNum]; ok && bg.relID != "" {
		consider(bg.relID)
	}
	if _, hasNotes := ctx.slideNotes[slideNum]; hasNotes {
		// Reserve the notes relationship assigned by writeNewSlideRelationships.
		nextID++
	}
	return nextID
}

func (ctx *singlePassContext) buildLinkRelationship(slideNum int, key, id string, link *LinkSpec) (pptx.RelationshipXML, error) {
	if (link.URL == "") == (link.Slide == 0) {
		return pptx.RelationshipXML{}, fmt.Errorf("slide %d hyperlink %s: set exactly one URL or slide number", slideNum, key)
	}
	if link.URL != "" {
		parsed, err := url.Parse(link.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || strings.ContainsAny(link.URL, "\r\n") {
			return pptx.RelationshipXML{}, fmt.Errorf("slide %d hyperlink %s: invalid HTTP/HTTPS URL %q", slideNum, key, link.URL)
		}
		return pptx.RelationshipXML{ID: id, Type: pptx.RelTypeHyperlink, Target: link.URL, TargetMode: "External"}, nil
	}
	totalSlides := ctx.existingSlides + len(ctx.slideSpecs)
	if ctx.excludeTemplateSlides {
		totalSlides = len(ctx.slideSpecs)
	}
	if link.Slide < 1 || link.Slide > totalSlides {
		return pptx.RelationshipXML{}, fmt.Errorf("slide %d hyperlink %s: target slide %d outside 1..%d", slideNum, key, link.Slide, totalSlides)
	}
	return pptx.RelationshipXML{ID: id, Type: pptx.RelTypeSlide, Target: fmt.Sprintf("slide%d.xml", link.Slide)}, nil
}
