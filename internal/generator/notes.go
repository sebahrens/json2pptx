// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"encoding/xml"
	"fmt"
	"sort"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// generateNotesSlideXML creates the OOXML for a notes slide.
// The notes slide references the parent slide and contains the notes text.
// OOXML structure: <p:notes><p:cSld><p:spTree>...<p:sp><p:txBody>..notes text..</p:txBody></p:sp></p:spTree></p:cSld></p:notes>
func generateNotesSlideXML(notesText string) []byte {
	// Build paragraphs from notes text - split on newlines for multi-paragraph support
	paragraphs := buildNotesParagraphs(notesText)

	// The notes slide XML follows the OOXML notesSlide schema.
	// It contains two shapes: a slide image placeholder (type="sldImg")
	// and a notes text placeholder (type="body") with the actual notes.
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
         xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
         xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr>
        <p:cNvPr id="1" name=""/>
        <p:cNvGrpSpPr/>
        <p:nvPr/>
      </p:nvGrpSpPr>
      <p:grpSpPr>
        <a:xfrm>
          <a:off x="0" y="0"/>
          <a:ext cx="0" cy="0"/>
          <a:chOff x="0" y="0"/>
          <a:chExt cx="0" cy="0"/>
        </a:xfrm>
      </p:grpSpPr>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="2" name="Slide Image Placeholder 1"/>
          <p:cNvSpPr><a:spLocks noGrp="1" noRot="1" noChangeAspect="1"/></p:cNvSpPr>
          <p:nvPr><p:ph type="sldImg"/></p:nvPr>
        </p:nvSpPr>
        <p:spPr/>
      </p:sp>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="3" name="Notes Placeholder 2"/>
          <p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>
          <p:nvPr><p:ph type="body" idx="1"/></p:nvPr>
        </p:nvSpPr>
        <p:spPr/>
        <p:txBody>
          <a:bodyPr/>
          <a:lstStyle/>
%s
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:notes>`, paragraphs)

	return []byte(xml)
}

// buildNotesParagraphs converts notes text into OOXML paragraph elements.
// Each line becomes a separate paragraph for proper rendering.
func buildNotesParagraphs(text string) string {
	if text == "" {
		return "          <a:p><a:endParaRPr lang=\"en-US\"/></a:p>"
	}

	var result string
	lines := splitNotesLines(text)
	for _, line := range lines {
		escaped := xmlEscapeString(line)
		result += fmt.Sprintf("          <a:p><a:r><a:rPr lang=\"en-US\" dirty=\"0\"/><a:t>%s</a:t></a:r></a:p>\n", escaped)
	}
	return result
}

// splitNotesLines splits text into lines, treating empty lines as paragraph breaks.
func splitNotesLines(text string) []string {
	var lines []string
	for _, line := range splitOnNewlines(text) {
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// splitOnNewlines splits text on \n characters.
func splitOnNewlines(text string) []string {
	var result []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			result = append(result, text[start:i])
			start = i + 1
		}
	}
	result = append(result, text[start:])
	return result
}

// xmlEscapeString escapes special XML characters in a string.
func xmlEscapeString(s string) string {
	var buf []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			buf = append(buf, []byte("&amp;")...)
		case '<':
			buf = append(buf, []byte("&lt;")...)
		case '>':
			buf = append(buf, []byte("&gt;")...)
		case '"':
			buf = append(buf, []byte("&quot;")...)
		case '\'':
			buf = append(buf, []byte("&apos;")...)
		default:
			buf = append(buf, s[i])
		}
	}
	return string(buf)
}

// generateNotesSlideRels creates the relationships file for a notes slide.
// A notes slide must reference back to its parent slide.
func generateNotesSlideRels(slideNum int) ([]byte, error) {
	rels := pptx.RelationshipsXML{
		XMLName: xml.Name{Space: pptx.NsPackageRels, Local: "Relationships"},
		Xmlns:   pptx.NsPackageRels,
		Relationships: []pptx.RelationshipXML{
			{
				ID:     "rId1",
				Type:   pptx.RelTypeSlide,
				Target: fmt.Sprintf("../slides/slide%d.xml", slideNum),
			},
		},
	}

	relsOutput, err := xml.Marshal(rels)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal notes slide relationships: %w", err)
	}

	return append([]byte(xml.Header), relsOutput...), nil
}

// writeNotesSlides writes all notes slide files and their relationships to the output ZIP.
// It also registers content type overrides for each notes slide.
func (ctx *singlePassContext) writeNotesSlides() error {
	if len(ctx.slideNotes) == 0 {
		return nil // No notes to write
	}

	// Sort slide numbers for deterministic output ordering
	noteSlideNums := make([]int, 0, len(ctx.slideNotes))
	for slideNum := range ctx.slideNotes {
		noteSlideNums = append(noteSlideNums, slideNum)
	}
	sort.Ints(noteSlideNums)

	for _, slideNum := range noteSlideNums {
		notesText := ctx.slideNotes[slideNum]

		// Write the notesSlide XML
		notesData := generateNotesSlideXML(notesText)
		notesPath := NotesSlideXMLPath(slideNum)
		fw, err := utils.ZipCreateDeterministic(ctx.outputWriter, notesPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", notesPath, err)
		}
		if _, err := fw.Write(notesData); err != nil {
			return fmt.Errorf("failed to write %s: %w", notesPath, err)
		}

		// Write the notesSlide relationships
		relsData, err := generateNotesSlideRels(slideNum)
		if err != nil {
			return fmt.Errorf("failed to generate notes rels for slide %d: %w", slideNum, err)
		}
		relsPath := NotesSlideRelsPath(slideNum)
		fw, err = utils.ZipCreateDeterministic(ctx.outputWriter, relsPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", relsPath, err)
		}
		if _, err := fw.Write(relsData); err != nil {
			return fmt.Errorf("failed to write %s: %w", relsPath, err)
		}
	}

	return nil
}

// addNotesSlideContentTypes adds Override entries for notes slides to [Content_Types].xml.
// Each notes slide needs a part-specific override (not a Default extension mapping).
func addNotesSlideContentTypes(ctData []byte, slideNotes map[int]string) ([]byte, error) {
	var contentTypes pptx.ContentTypesXML
	if err := xml.Unmarshal(ctData, &contentTypes); err != nil {
		return nil, fmt.Errorf("failed to parse [Content_Types].xml for notes: %w", err)
	}

	// Sort slide numbers for deterministic content type ordering
	ctSlideNums := make([]int, 0, len(slideNotes))
	for slideNum := range slideNotes {
		ctSlideNums = append(ctSlideNums, slideNum)
	}
	sort.Ints(ctSlideNums)

	// Build set of existing overrides to avoid duplicates.
	// Templates may already have notes slide content type entries.
	existingOverrides := make(map[string]bool)
	for _, ovr := range contentTypes.Overrides {
		existingOverrides[ovr.PartName] = true
	}

	for _, slideNum := range ctSlideNums {
		partName := "/" + NotesSlideXMLPath(slideNum)
		if existingOverrides[partName] {
			continue
		}
		contentTypes.Overrides = append(contentTypes.Overrides, pptx.ContentTypeOverride{
			PartName:    partName,
			ContentType: pptx.ContentTypeNotesSlide,
		})
	}

	modifiedData, err := xml.Marshal(contentTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal [Content_Types].xml with notes: %w", err)
	}

	return append([]byte(xml.Header), modifiedData...), nil
}
