package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func walkComposeSegments(c *ComposeInput, prefix string, visit func(*SegmentInput, string)) {
	if c == nil {
		return
	}
	for i := range c.Segments {
		seg := &c.Segments[i]
		path := fmt.Sprintf("%s/segments/%d", prefix, i)
		visit(seg, path)
		walkComposeSegments(seg.Compose, path+"/compose", visit)
	}
}
func composeHasURLReferences(c *ComposeInput) bool {
	found := false
	walkComposeSegments(c, "", func(seg *SegmentInput, _ string) {
		if seg.Image != nil && seg.Image.URL != "" {
			found = true
		}
		if seg.ShapeGrid != nil && gridHasURLReferences(seg.ShapeGrid) {
			found = true
		}
	})
	return found
}
func resolveComposeURLs(c *ComposeInput, prefix string, index int, resolver urlResolver) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	walkComposeSegments(c, prefix, func(seg *SegmentInput, path string) {
		if seg.Image != nil {
			out = appendURLFinding(out, resolveURLField(&seg.Image.URL, &seg.Image.Path, false, path+"/image/url", "image", index, resolver))
		}
		if seg.ShapeGrid != nil {
			out = append(out, resolveGridURLs(seg.ShapeGrid, path+"/shape_grid", index, resolver)...)
		}
	})
	return out
}
func resolveComposeAssetPaths(c *ComposeInput, prefix, base string, index int, allowList ...string) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	walkComposeSegments(c, prefix, func(seg *SegmentInput, path string) {
		if seg.Image != nil && seg.Image.Path != "" {
			out = append(out, applyLocalAssetPath(&seg.Image.Path, base, diagnostics.CodeImagePath, "image", index, path+"/image/path")...)
		}
		if seg.ShapeGrid != nil {
			out = append(out, resolveGridImageAssets(seg.ShapeGrid, path+"/shape_grid", base, index)...)
			out = append(out, resolveGridIconPaths(seg.ShapeGrid, path+"/shape_grid", base, index, allowList...)...)
		}
	})
	return out
}
