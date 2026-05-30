// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// tableStyleDisplayNames maps well-known OOXML built-in table style GUIDs to
// their canonical PowerPoint display names. The engine default is the only one
// it actually generates today; the others are listed for completeness so that
// custom user-supplied GUIDs that happen to match a built-in render with a
// sensible name in viewers that show style names.
var tableStyleDisplayNames = map[string]string{
	types.DefaultTableStyleID: "Medium Style 2 - Accent 1",
}

// buildTableStylesXML returns a complete ppt/tableStyles.xml that declares a
// <a:tblStyle> element for every supplied GUID and sets the def attribute to
// the engine default style.
//
// The OOXML schema requires <a:tableStyleId> elements inside slides to
// reference a styleId that exists in tableStyles.xml. PowerPoint tolerates
// references to its built-in style GUIDs even when they aren't declared in
// the file, but stricter readers (LibreOffice / Impress) treat the dangling
// reference as a fatal error and refuse to open the presentation. Declaring
// each referenced GUID — even as a minimal <a:tblStyle styleId styleName>
// stub — keeps the file portable.
func buildTableStylesXML(referencedIDs map[string]bool) string {
	ids := make([]string, 0, len(referencedIDs)+1)
	seen := make(map[string]bool, len(referencedIDs)+1)
	// Always emit the engine default so the def attribute resolves.
	ids = append(ids, types.DefaultTableStyleID)
	seen[types.DefaultTableStyleID] = true
	for id := range referencedIDs {
		// Skip empties, duplicates, and any value that is not a well-formed
		// table style GUID so user-controlled text never reaches the
		// styleId="" attribute.
		if id == "" || seen[id] || !types.IsValidTableStyleID(id) {
			continue
		}
		ids = append(ids, id)
		seen[id] = true
	}
	sort.Strings(ids)

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	fmt.Fprintf(&sb,
		`<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="%s">`,
		types.DefaultTableStyleID,
	)
	for _, id := range ids {
		name, ok := tableStyleDisplayNames[id]
		if !ok {
			name = "Custom Table Style"
		}
		fmt.Fprintf(&sb,
			`<a:tblStyle styleId="%s" styleName="%s"/>`,
			id, name,
		)
	}
	sb.WriteString(`</a:tblStyleLst>`)
	return sb.String()
}

// parseTemplateStyleIDs extracts the set of styleId attribute values already
// declared in the template's ppt/tableStyles.xml. Returns an empty map when
// the XML has no <a:tblStyle> children (e.g. the common self-closing stub
// templates use: <a:tblStyleLst def="..."/>).
func parseTemplateStyleIDs(data []byte) map[string]bool {
	ids := make(map[string]bool)
	// Walk the raw bytes looking for styleId="..." occurrences.  We only care
	// about the attribute value, not the full element structure, so a simple
	// byte scan is enough and avoids a full XML decode of potentially large
	// template files.
	const attr = `styleId="`
	remaining := data
	for {
		idx := bytes.Index(remaining, []byte(attr))
		if idx == -1 {
			break
		}
		after := remaining[idx+len(attr):]
		end := bytes.IndexByte(after, '"')
		if end == -1 {
			break
		}
		ids[string(after[:end])] = true
		remaining = after[end+1:]
	}
	return ids
}

// buildStubElements returns the XML fragment for stub <a:tblStyle> elements
// for the given GUIDs, sorted for deterministic output.
func buildStubElements(missingIDs []string) string {
	sorted := make([]string, len(missingIDs))
	copy(sorted, missingIDs)
	sort.Strings(sorted)

	var sb strings.Builder
	for _, id := range sorted {
		// Defensive: never declare a styleId that is not a well-formed GUID.
		if !types.IsValidTableStyleID(id) {
			continue
		}
		name, ok := tableStyleDisplayNames[id]
		if !ok {
			name = "Custom Table Style"
		}
		fmt.Fprintf(&sb, `<a:tblStyle styleId="%s" styleName="%s"/>`, id, name)
	}
	return sb.String()
}

// mergeTemplateWithStubs takes the template's raw tableStyles.xml and returns
// a merged version that preserves all existing <a:tblStyle> elements (with
// their full formatting rules) while appending stub elements for every GUID
// in missingIDs.
//
// Handles both the common self-closing form:
//
//	<a:tblStyleLst def="..."/>
//
// and the full open/close form:
//
//	<a:tblStyleLst def="...">...</a:tblStyleLst>
//
// Returns nil when the template XML cannot be recognised as a valid
// tblStyleLst element (caller should fall back to the synthetic approach).
func mergeTemplateWithStubs(templateXML []byte, missingIDs []string) []byte {
	if len(missingIDs) == 0 {
		return templateXML
	}
	stubs := buildStubElements(missingIDs)

	// Case 1: template has a proper closing tag — insert stubs before it.
	const closing = "</a:tblStyleLst>"
	if closeIdx := bytes.LastIndex(templateXML, []byte(closing)); closeIdx != -1 {
		result := make([]byte, 0, len(templateXML)+len(stubs))
		result = append(result, templateXML[:closeIdx]...)
		result = append(result, []byte(stubs)...)
		result = append(result, templateXML[closeIdx:]...)
		return result
	}

	// Case 2: template uses a self-closing <a:tblStyleLst ... /> — convert to
	// open/close form and inject the stubs.  The self-closing case means the
	// element has no children, so finding `/>` here is unambiguous.
	const selfClose = "/>"
	if scIdx := bytes.LastIndex(templateXML, []byte(selfClose)); scIdx != -1 {
		if bytes.Contains(templateXML[:scIdx+2], []byte("<a:tblStyleLst")) {
			result := make([]byte, 0, len(templateXML)+len(stubs)+len(closing))
			result = append(result, templateXML[:scIdx]...)
			result = append(result, '>')
			result = append(result, []byte(stubs)...)
			result = append(result, []byte(closing)...)
			return result
		}
	}

	return nil // unrecognised format; caller falls back to synthetic
}

// installTableStylesOverride writes a populated ppt/tableStyles.xml into
// syntheticFiles when at least one rendered table referenced a style GUID.
// writeTemplateFiles will then skip copying the template's original file and
// writeSyntheticFiles will emit this version instead.
//
// When the template already declares a full <a:tblStyle> element for every
// referenced GUID, no override is written and the template's own file passes
// through unchanged — preserving brand-specific formatting rules that the
// synthetic approach would otherwise destroy.
//
// Called from writeOutput before writeTemplateFiles. A no-op when no tables
// rendered.
func (ctx *singlePassContext) installTableStylesOverride() {
	if len(ctx.tableStyleIDsUsed) == 0 {
		return
	}

	// Try to read the template's existing tableStyles.xml so we can preserve
	// full <a:tblStyle> elements (which carry brand-specific formatting rules)
	// rather than replacing them with empty stubs.
	templateXML, err := utils.ReadFileFromZipIndex(ctx.templateIndex, PathTableStyles)
	if err == nil && len(templateXML) > 0 {
		defined := parseTemplateStyleIDs(templateXML)

		// Collect GUIDs referenced by the rendered tables that the template
		// does not already declare.
		var missing []string
		for id := range ctx.tableStyleIDsUsed {
			if id != "" && types.IsValidTableStyleID(id) && !defined[id] {
				missing = append(missing, id)
			}
		}

		if len(missing) == 0 {
			// Template already declares all referenced GUIDs — no override
			// needed.  writeTemplateFiles will copy the original file as-is,
			// preserving every <a:tblStyle> formatting rule.
			return
		}

		// Template exists but is missing some GUIDs — merge: preserve all
		// existing <a:tblStyle> elements and append stubs only for the new
		// GUIDs that the template doesn't declare.
		if merged := mergeTemplateWithStubs(templateXML, missing); merged != nil {
			if ctx.syntheticFiles == nil {
				ctx.syntheticFiles = make(map[string][]byte)
			}
			ctx.syntheticFiles[PathTableStyles] = merged
			return
		}
	}

	// No template file, empty template, or unrecognised format — fall back to
	// the fully-synthetic approach (stubs only, engine-default def attribute).
	if ctx.syntheticFiles == nil {
		ctx.syntheticFiles = make(map[string][]byte)
	}
	ctx.syntheticFiles[PathTableStyles] = []byte(buildTableStylesXML(ctx.tableStyleIDsUsed))
}
