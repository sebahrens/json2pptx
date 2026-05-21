// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
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
		if id == "" || seen[id] {
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

// installTableStylesOverride writes a populated ppt/tableStyles.xml into
// syntheticFiles when at least one rendered table referenced a style GUID.
// writeTemplateFiles will then skip copying the template's original file and
// writeSyntheticFiles will emit this version instead.
//
// Called from writeOutput before writeTemplateFiles. A no-op when no tables
// rendered.
func (ctx *singlePassContext) installTableStylesOverride() {
	if len(ctx.tableStyleIDsUsed) == 0 {
		return
	}
	if ctx.syntheticFiles == nil {
		ctx.syntheticFiles = make(map[string][]byte)
	}
	ctx.syntheticFiles[PathTableStyles] = []byte(buildTableStylesXML(ctx.tableStyleIDsUsed))
}
