// Package generator provides PPTX file generation from slide specifications.
package generator

import "strings"

// insertSlideEntriesIntoPresentationXML inserts new slide entries into presentation.xml
// while preserving the original XML structure, namespaces, and formatting.
func insertSlideEntriesIntoPresentationXML(presentationXML string, newSlideEntries []string) string {
	if len(newSlideEntries) == 0 {
		return presentationXML
	}

	insertion := strings.Join(newSlideEntries, "")

	// Find the closing </p:sldIdLst> tag
	closingTag := "</p:sldIdLst>"
	closingIdx := strings.Index(presentationXML, closingTag)
	if closingIdx == -1 {
		// Fallback: try without namespace prefix
		closingTag = "</sldIdLst>"
		closingIdx = strings.Index(presentationXML, closingTag)
	}

	if closingIdx != -1 {
		// Insert new entries before the closing tag
		return presentationXML[:closingIdx] + insertion + presentationXML[closingIdx:]
	}

	// Handle self-closing <p:sldIdLst/> or <sldIdLst/>
	if result, ok := expandSelfClosingSldIDLst(presentationXML, insertion); ok {
		return result
	}

	// Handle completely missing sldIdLst by inserting before <p:sldSz or </p:presentation>
	return insertNewSldIDLst(presentationXML, insertion)
}

// replaceSlideListInPresentationXML replaces the entire slide list in presentation.xml
// with only the new slide entries. Used when excluding template slides.
func replaceSlideListInPresentationXML(presentationXML string, newSlideEntries []string) string {
	newContent := strings.Join(newSlideEntries, "")

	// Find the opening <p:sldIdLst> tag
	openingTag := "<p:sldIdLst>"
	openingIdx := strings.Index(presentationXML, openingTag)
	if openingIdx == -1 {
		// Fallback: try without namespace prefix
		openingTag = "<sldIdLst>"
		openingIdx = strings.Index(presentationXML, openingTag)
	}

	// Find the closing </p:sldIdLst> tag
	closingTag := "</p:sldIdLst>"
	closingIdx := strings.Index(presentationXML, closingTag)
	if closingIdx == -1 {
		// Fallback: try without namespace prefix
		closingTag = "</sldIdLst>"
		closingIdx = strings.Index(presentationXML, closingTag)
	}

	if openingIdx != -1 && closingIdx != -1 {
		// Replace everything between opening and closing tags
		// Keep the opening tag, replace content, keep closing tag
		return presentationXML[:openingIdx+len(openingTag)] + newContent + presentationXML[closingIdx:]
	}

	// Handle self-closing <p:sldIdLst/> or <sldIdLst/>
	if result, ok := expandSelfClosingSldIDLst(presentationXML, newContent); ok {
		return result
	}

	// Handle completely missing sldIdLst by inserting before <p:sldSz or </p:presentation>
	return insertNewSldIDLst(presentationXML, newContent)
}

// expandSelfClosingSldIDLst handles the case where the template has a self-closing
// <p:sldIdLst/> or <sldIdLst/> element. It expands it into an open/close pair with
// the new entries inside.
func expandSelfClosingSldIDLst(presentationXML string, entries string) (string, bool) {
	for _, selfClosing := range []string{"<p:sldIdLst/>", "<sldIdLst/>"} {
		idx := strings.Index(presentationXML, selfClosing)
		if idx != -1 {
			// Determine the namespace prefix to use for the expanded tags
			prefix := "p:"
			if selfClosing == "<sldIdLst/>" {
				prefix = ""
			}
			replacement := "<" + prefix + "sldIdLst>" + entries + "</" + prefix + "sldIdLst>"
			return presentationXML[:idx] + replacement + presentationXML[idx+len(selfClosing):], true
		}
	}
	// Also handle variants with whitespace before />
	for _, prefix := range []string{"p:", ""} {
		tag := "<" + prefix + "sldIdLst"
		idx := strings.Index(presentationXML, tag)
		if idx != -1 {
			// Find the end of this tag
			endIdx := strings.Index(presentationXML[idx:], "/>")
			if endIdx != -1 {
				fullEnd := idx + endIdx + 2
				replacement := "<" + prefix + "sldIdLst>" + entries + "</" + prefix + "sldIdLst>"
				return presentationXML[:idx] + replacement + presentationXML[fullEnd:], true
			}
		}
	}
	return "", false
}

// insertNewSldIDLst handles the case where the template has no sldIdLst element at all.
// It inserts a new <p:sldIdLst> element at the correct position in the XML structure.
// Per OOXML spec, sldIdLst comes after sldMasterIdLst/notesMasterIdLst/handoutMasterIdLst
// and before sldSz.
func insertNewSldIDLst(presentationXML string, entries string) string {
	newElement := "<p:sldIdLst>" + entries + "</p:sldIdLst>"

	// Try to insert before <p:sldSz (the element that follows sldIdLst in the spec)
	for _, marker := range []string{"<p:sldSz", "<sldSz"} {
		idx := strings.Index(presentationXML, marker)
		if idx != -1 {
			return presentationXML[:idx] + newElement + presentationXML[idx:]
		}
	}

	// Fallback: insert before </p:presentation> or </presentation>
	for _, marker := range []string{"</p:presentation>", "</presentation>"} {
		idx := strings.Index(presentationXML, marker)
		if idx != -1 {
			return presentationXML[:idx] + newElement + presentationXML[idx:]
		}
	}

	// Last resort: return original (should not happen with valid OOXML)
	return presentationXML
}
