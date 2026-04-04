// Package pptx provides PPTX file manipulation primitives.
package pptx

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// spTreePattern matches the opening and closing spTree tags.
// Both prefixed (p:spTree) and unprefixed (spTree) forms are supported
// because Go's encoding/xml marshals without namespace prefixes;
// fixOOXMLNamespaces adds the p: prefix later in the pipeline.
var (
	spTreeOpenPattern  = regexp.MustCompile(`<(?:p:)?spTree[^>]*>`)
	spTreeClosePattern = regexp.MustCompile(`</(?:p:)?spTree>`)

	// Shape element patterns - these are the child elements that can appear in spTree
	// after the mandatory nvGrpSpPr and grpSpPr elements.
	// Order: sp, grpSp, graphicFrame, cxnSp, pic, contentPart
	// Supports both prefixed (p:sp) and unprefixed (sp) forms.
	shapeElementPattern = regexp.MustCompile(`<(?:p:)?(sp|grpSp|graphicFrame|cxnSp|pic|contentPart)\b`)
)

// InsertPosition specifies where to insert a new element in the spTree.
type InsertPosition int

const (
	// InsertAtEnd appends the element at the end of spTree (on top visually).
	InsertAtEnd InsertPosition = -1

	// InsertAtStart inserts after the mandatory group properties (at bottom visually).
	InsertAtStart InsertPosition = 0
)

// InsertIntoSpTree inserts the given element XML into the spTree at the specified position.
//
// The slideXML is the raw slide XML content.
// The elementXML is the complete XML for the element to insert (e.g., a p:pic element).
// The position specifies where to insert:
//   - InsertAtEnd (-1): append at the end (shape will be on top)
//   - 0: insert after mandatory group properties (shape will be at the bottom)
//   - 1, 2, ...: insert after that many existing shape elements
//
// Returns the modified slide XML with proper indentation.
func InsertIntoSpTree(slideXML []byte, elementXML []byte, position InsertPosition) ([]byte, error) {
	// Find spTree opening and closing tags
	openMatch := spTreeOpenPattern.FindIndex(slideXML)
	if openMatch == nil {
		return nil, fmt.Errorf("no p:spTree element found in slide XML")
	}

	closeMatch := spTreeClosePattern.FindIndex(slideXML)
	if closeMatch == nil {
		return nil, fmt.Errorf("no closing </p:spTree> found in slide XML")
	}

	if closeMatch[0] < openMatch[1] {
		return nil, fmt.Errorf("malformed spTree: closing tag before opening tag ends")
	}

	// Content between <p:spTree...> and </p:spTree>
	spTreeContent := slideXML[openMatch[1]:closeMatch[0]]

	// Find insertion point
	insertOffset := findInsertionPoint(spTreeContent, position)

	// Detect indentation from the context
	indent := detectIndentation(spTreeContent, insertOffset)

	// Build the new slide XML
	var result bytes.Buffer

	// Content before spTree content
	result.Write(slideXML[:openMatch[1]])

	// spTree content up to insertion point
	result.Write(spTreeContent[:insertOffset])

	// Ensure proper line ending before new element
	if insertOffset > 0 && spTreeContent[insertOffset-1] != '\n' {
		result.WriteByte('\n')
	}

	// Write the element with proper indentation
	result.WriteString(indent)
	result.Write(indentElement(elementXML, indent))

	// Rest of spTree content
	result.Write(spTreeContent[insertOffset:])

	// Closing tag and beyond
	result.Write(slideXML[closeMatch[0]:])

	return result.Bytes(), nil
}

// findInsertionPoint finds the byte offset within spTree content where
// a new element should be inserted based on the position.
func findInsertionPoint(content []byte, position InsertPosition) int {
	// Find all shape elements
	matches := shapeElementPattern.FindAllIndex(content, -1)

	// If no shape elements exist, insert at the end of content
	if len(matches) == 0 {
		return len(content)
	}

	// For InsertAtEnd, insert at the end of content (after all shapes)
	if position == InsertAtEnd {
		return len(content)
	}

	// For position 0, insert before the first shape
	if int(position) == 0 {
		return matches[0][0]
	}

	// For position N, insert after the Nth shape
	// We need to find the end of the Nth shape element
	shapeIndex := int(position)
	if shapeIndex >= len(matches) {
		// If requested position is beyond available shapes, insert at end
		return len(content)
	}

	// Return the position of the requested shape (we insert before it)
	return matches[shapeIndex][0]
}

// detectIndentation attempts to detect the indentation used in the XML.
func detectIndentation(content []byte, nearOffset int) string {
	// Look backwards from the offset to find the start of the line
	lineStart := nearOffset
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}

	// Extract indentation (whitespace at start of line)
	indent := ""
	for i := lineStart; i < len(content) && (content[i] == ' ' || content[i] == '\t'); i++ {
		indent += string(content[i])
	}

	// If we couldn't detect indentation, use a sensible default
	if indent == "" {
		indent = "      " // 6 spaces, typical for spTree children
	}

	return indent
}

// indentElement adds proper indentation to a multi-line XML element.
func indentElement(element []byte, baseIndent string) []byte {
	// Trim leading/trailing whitespace from the element
	element = bytes.TrimSpace(element)

	// Split into lines
	lines := bytes.Split(element, []byte("\n"))
	if len(lines) <= 1 {
		// Single line element
		return append(element, '\n')
	}

	// Re-indent each line (except the first which gets baseIndent)
	var result bytes.Buffer
	for i, line := range lines {
		trimmedLine := bytes.TrimLeft(line, " \t")
		if len(trimmedLine) == 0 {
			continue
		}

		if i > 0 {
			result.WriteString(baseIndent)
		}
		result.Write(trimmedLine)
		result.WriteByte('\n')
	}

	return result.Bytes()
}

// CountShapesInSpTree counts the number of shape elements in the spTree.
// This includes p:sp, p:pic, p:grpSp, p:graphicFrame, p:cxnSp, and p:contentPart.
func CountShapesInSpTree(slideXML []byte) (int, error) {
	// Find spTree
	openMatch := spTreeOpenPattern.FindIndex(slideXML)
	if openMatch == nil {
		return 0, fmt.Errorf("no p:spTree element found in slide XML")
	}

	closeMatch := spTreeClosePattern.FindIndex(slideXML)
	if closeMatch == nil {
		return 0, fmt.Errorf("no closing </p:spTree> found in slide XML")
	}

	if closeMatch[0] < openMatch[1] {
		return 0, fmt.Errorf("malformed spTree")
	}

	content := slideXML[openMatch[1]:closeMatch[0]]
	matches := shapeElementPattern.FindAllIndex(content, -1)
	return len(matches), nil
}

// ExtractSpTree extracts just the spTree content from slide XML.
// Returns the content between <p:spTree...> and </p:spTree>.
func ExtractSpTree(slideXML []byte) ([]byte, error) {
	openMatch := spTreeOpenPattern.FindIndex(slideXML)
	if openMatch == nil {
		return nil, fmt.Errorf("no p:spTree element found")
	}

	closeMatch := spTreeClosePattern.FindIndex(slideXML)
	if closeMatch == nil {
		return nil, fmt.Errorf("no closing </p:spTree> found")
	}

	// Return including the tags
	return slideXML[openMatch[0]:closeMatch[1]], nil
}

// ValidateSpTree performs basic validation on spTree structure.
func ValidateSpTree(slideXML []byte) error {
	openMatch := spTreeOpenPattern.FindIndex(slideXML)
	if openMatch == nil {
		return fmt.Errorf("no p:spTree element found")
	}

	closeMatch := spTreeClosePattern.FindIndex(slideXML)
	if closeMatch == nil {
		return fmt.Errorf("no closing </p:spTree> found")
	}

	if closeMatch[0] < openMatch[1] {
		return fmt.Errorf("malformed spTree: closing tag before content")
	}

	// Check for required group properties
	content := slideXML[openMatch[1]:closeMatch[0]]
	if !strings.Contains(string(content), "nvGrpSpPr") {
		return fmt.Errorf("spTree missing required nvGrpSpPr element")
	}
	if !strings.Contains(string(content), "grpSpPr") {
		return fmt.Errorf("spTree missing required grpSpPr element")
	}

	return nil
}
