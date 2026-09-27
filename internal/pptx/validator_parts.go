package pptx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// relationshipsNS is the namespace of r:id / r:embed / r:link attributes.
const relationshipsNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

// overrideRequiredPartRegex matches the PresentationML parts that need an
// explicit content-type Override: the generic "xml" Default does not give them
// their real content type.
var overrideRequiredPartRegex = regexp.MustCompile(`^ppt/(presentation\.xml|(slides|slideLayouts|slideMasters|notesSlides|notesMasters|handoutMasters)/[^/]+\.xml)$`)

// relRefPartRegex matches the parts whose r:id / r:embed / r:link references
// are resolved against their .rels.
var relRefPartRegex = regexp.MustCompile(`^ppt/(presentation\.xml|(slides|slideLayouts|slideMasters|notesSlides|notesMasters)/[^/]+\.xml)$`)

// slidePartPathRegex matches slide parts (not their rels).
var slidePartPathRegex = regexp.MustCompile(`^ppt/slides/[^/]+\.xml$`)

// ValidatePartXML parses every XML and .rels part, reporting parse failures
// (unclosed elements, bare '&', XML 1.0 illegal control characters) as
// MALFORMED_XML. For presentation, slide, layout, master and notes parts it
// also resolves every relationships-namespace attribute (r:id, r:embed,
// r:link, ...) against the part's .rels, reporting unresolved ids as
// DANGLING_REL.
func (v *Validator) ValidatePartXML() {
	for _, entry := range v.pkg.Entries() {
		if !strings.HasSuffix(entry, ".xml") && !strings.HasSuffix(entry, ".rels") {
			continue
		}
		data, err := v.pkg.ReadEntry(entry)
		if err != nil {
			v.addError(entry, ErrCodeMalformedXML, fmt.Sprintf("failed to read: %v", err))
			continue
		}
		checkRefs := relRefPartRegex.MatchString(entry)
		refs, err := scanXMLPart(data, checkRefs)
		if err != nil {
			v.addError(entry, ErrCodeMalformedXML, fmt.Sprintf("failed to parse: %v", err))
			continue
		}
		if !checkRefs || len(refs) == 0 {
			continue
		}
		var rels *Relationships
		if relsData, relsErr := v.pkg.ReadEntry(GetRelsPath(entry)); relsErr == nil {
			rels, _ = ParseRelationships(relsData)
		}
		reported := make(map[string]bool)
		for _, ref := range refs {
			if reported[ref] {
				continue
			}
			if rels != nil && rels.Get(ref) != nil {
				continue
			}
			reported[ref] = true
			v.addError(entry, ErrCodeDanglingRel,
				fmt.Sprintf("relationship id %q is referenced but not defined in %s", ref, GetRelsPath(entry)))
		}
	}
}

// scanXMLPart fully tokenizes data, returning the first parse error. When
// collectRefs is set it returns the non-empty values of relationships-namespace
// attributes in document order.
func scanXMLPart(data []byte, collectRefs bool) ([]string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	// The parts are UTF-8 in practice; do not fail on a declared label the
	// standard library has no converter for.
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }
	var refs []string
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return refs, nil
		}
		if err != nil {
			return nil, err
		}
		if !collectRefs {
			continue
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Space == relationshipsNS && attr.Value != "" {
				refs = append(refs, attr.Value)
			}
		}
	}
}

// ValidateSlideIDs checks that every <p:sldId id="..."> in presentation.xml is
// unique. Duplicate slide ids make PowerPoint show the repair prompt.
func (v *Validator) ValidateSlideIDs() {
	const presPath = "ppt/presentation.xml"
	data, err := v.pkg.ReadEntry(presPath)
	if err != nil {
		return // reported by ValidatePresentation
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }
	seen := make(map[string]int)
	var order []string
	for {
		tok, err := dec.Token()
		if err != nil {
			break // parse failures are reported by ValidatePartXML
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "sldId" {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Space == "" && attr.Name.Local == "id" {
				if seen[attr.Value] == 0 {
					order = append(order, attr.Value)
				}
				seen[attr.Value]++
			}
		}
	}
	for _, id := range order {
		if n := seen[id]; n > 1 {
			v.addError(presPath, ErrCodeDuplicateSlideID,
				fmt.Sprintf("<p:sldId id=%q> appears %d times; slide ids must be unique", id, n))
		}
	}
}

// ValidateSlideLayoutRels checks that every slide part has a slideLayout
// relationship. The link is implicit (no r:id in the slide XML), so a slide
// whose layout relationship was dropped passes the target checks but cannot
// be opened by PowerPoint without repair.
//
// A package with no slideLayout parts at all (a bare structural fixture) is
// not checked: there is no layout a slide could have lost.
func (v *Validator) ValidateSlideLayoutRels() {
	hasLayouts := false
	for _, entry := range v.pkg.Entries() {
		if strings.HasPrefix(entry, "ppt/slideLayouts/") && strings.HasSuffix(entry, ".xml") {
			hasLayouts = true
			break
		}
	}
	if !hasLayouts {
		return
	}
	for _, entry := range v.pkg.Entries() {
		if !slidePartPathRegex.MatchString(entry) {
			continue
		}
		relsData, err := v.pkg.ReadEntry(GetRelsPath(entry))
		if err != nil {
			v.addError(entry, ErrCodeMissingPart, "slide has no .rels part and therefore no slideLayout relationship")
			continue
		}
		rels, err := ParseRelationships(relsData)
		if err != nil {
			continue // reported by ValidatePartXML
		}
		if len(rels.FindByType(RelTypeSlideLayout)) == 0 {
			v.addError(entry, ErrCodeMissingPart, "slide has no slideLayout relationship")
		}
	}
}
