// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"bytes"
	"strings"
)

// ooxmlReplacer performs bulk namespace prefixing in a single trie-based pass.
// It handles all a: (DrawingML) and p: (PresentationML) element prefixing,
// plus masterClrMapping and ph element fixups.
//
// "ext" is excluded — it requires context-sensitive handling (a:ext for xfrm
// extents vs p:ext inside extLst) done in a second pass.
var ooxmlReplacer = buildOOXMLReplacer()

func buildOOXMLReplacer() *strings.Replacer {
	// DrawingML namespace (a:) elements
	aElements := []string{
		"bodyPr", "lstStyle", "p", "r", "t", "rPr", "pPr",
		"endParaRPr", "xfrm", "off", "chOff", "chExt", "prstGeom",
		"avLst", "solidFill", "schemeClr", "srgbClr", "ln", "noFill",
		"buNone", "buChar", "defRPr", "lvl1pPr", "lvl2pPr", "lvl3pPr",
		"normAutofit", "spLocks", "blip", "stretch", "fillRect", "lumMod",
		"runProperties",
	}

	// Presentation namespace (p:) elements
	pElements := []string{
		"cSld", "spTree", "sp", "nvSpPr", "cNvPr", "cNvSpPr",
		"nvPr", "spPr", "txBody", "nvGrpSpPr", "cNvGrpSpPr", "grpSpPr",
		"clrMapOvr", "pic", "nvPicPr", "cNvPicPr", "blipFill", "extLst",
	}

	// 4 variants per element (open, close, open-with-attrs, self-closing)
	// + masterClrMapping (4) + ph (3) = (28+18)*4 + 4 + 3 = 191 pairs
	pairs := make([]string, 0, 2*((len(aElements)+len(pElements))*4*2+7*2))

	for _, elem := range aElements {
		pairs = append(pairs,
			"<"+elem+">", "<a:"+elem+">",
			"</"+elem+">", "</a:"+elem+">",
			"<"+elem+" ", "<a:"+elem+" ",
			"<"+elem+"/>", "<a:"+elem+"/>",
		)
	}
	for _, elem := range pElements {
		pairs = append(pairs,
			"<"+elem+">", "<p:"+elem+">",
			"</"+elem+">", "</p:"+elem+">",
			"<"+elem+" ", "<p:"+elem+" ",
			"<"+elem+"/>", "<p:"+elem+"/>",
		)
	}

	// masterClrMapping → a:masterClrMapping
	pairs = append(pairs,
		"<masterClrMapping>", "<a:masterClrMapping>",
		"</masterClrMapping>", "</a:masterClrMapping>",
		"<masterClrMapping/>", "<a:masterClrMapping/>",
		"<masterClrMapping></masterClrMapping>", "<a:masterClrMapping/>",
	)

	// ph element (strip inline xmlns, add p: prefix)
	pairs = append(pairs,
		`<ph xmlns="http://schemas.openxmlformats.org/presentationml/2006/main"`, "<p:ph",
		"</ph>", "</p:ph>",
		"<ph/>", "<p:ph/>",
	)

	return strings.NewReplacer(pairs...)
}

// extReplacer handles the context-sensitive "ext" element in a second pass.
// After the bulk replacer has prefixed extLst → p:extLst, we can safely
// distinguish a:ext (xfrm extents with cx=/x= attrs) from p:ext (extensions
// with uri= attrs inside p:extLst).
//
// The trie uses longest-match, so "</ext></p:extLst>" (p:ext closing inside
// extLst) is preferred over the shorter "</ext>" (a:ext closing elsewhere).
var extReplacer = strings.NewReplacer(
	"<ext cx=", "<a:ext cx=",
	"<ext x=", "<a:ext x=",
	"<ext uri=", "<p:ext uri=",
	"</ext></p:extLst>", "</p:ext></p:extLst>",
	"</ext>", "</a:ext>",
	"<ext/>", "<a:ext/>",
)

// fixOOXMLNamespaces transforms generic XML element names to OOXML-prefixed names.
// Go's encoding/xml doesn't properly handle namespace prefixes when marshaling,
// so we fix them via string replacement.
//
// Uses two pre-built strings.Replacer instances (trie-based, single-pass each)
// instead of ~180 sequential strings.ReplaceAll calls, reducing per-call
// allocations from ~180 to 3.
func fixOOXMLNamespaces(data []byte) []byte {
	// Add namespace declarations to root element (once only) — stay in []byte space
	data = bytes.Replace(data, []byte("<sld>"),
		[]byte(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`), 1)
	data = bytes.Replace(data, []byte("</sld>"), []byte("</p:sld>"), 1)

	// Pass 1: bulk namespace prefixing (a: and p: elements, masterClrMapping, ph)
	// strings.Replacer requires string — single conversion for trie-based pass
	s := ooxmlReplacer.Replace(string(data))

	// Pass 2: context-sensitive ext element handling + extLst fixup
	s = extReplacer.Replace(s)

	return []byte(s)
}
