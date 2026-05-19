package generator

import "strings"

// stripSelfClosingElement removes a self-closing XML element by tag name.
// E.g., stripSelfClosingElement(`<a:latin typeface="Arial"/>`, "a:latin") returns "".
// Used to splice out conflicting font elements before injecting replacements.
func stripSelfClosingElement(xml, tag string) string {
	for {
		start := strings.Index(xml, "<"+tag+" ")
		if start == -1 {
			start = strings.Index(xml, "<"+tag+"/>")
		}
		if start == -1 {
			return xml
		}
		end := strings.Index(xml[start:], "/>")
		if end == -1 {
			return xml
		}
		xml = xml[:start] + xml[start+end+2:]
	}
}
