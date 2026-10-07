package main

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// patternImageRef is one image reference inside a pattern's values (e.g.
// image-text-split's values.image), with its JSON path.
type patternImageRef struct {
	img  *jsonschema.GridImageInput
	path string // JSON pointer of the image object, e.g. /slides/2/pattern/values/image
}

// patternHost is one place a slide hosts a named pattern: the slide-level
// pattern, a compose segment (at any compose depth) or a shape_grid cell (at
// any grid depth). path is the JSON pointer of the pattern object; set writes
// modified values back into the slide.
type patternHost struct {
	name   string
	values json.RawMessage
	path   string
	set    func(json.RawMessage)
}

// slidePatternHosts returns every pattern a slide hosts. Asset resolution
// used to look at the slide-level pattern only, so a relative image path in
// a compose segment or a cell pattern reached the generator unresolved and
// unchecked (go-slide-creator-6tgq9).
func slidePatternHosts(slide *SlideInput, slideIdx int) []patternHost {
	var hosts []patternHost
	if p := slide.Pattern; p != nil {
		hosts = append(hosts, patternHost{name: p.Name, values: p.Values,
			path: slidepath.SlideField(slideIdx, "pattern"),
			set:  func(v json.RawMessage) { p.Values = v }})
	}
	hosts = appendComposePatternHosts(hosts, slide.Compose, slidepath.SlideField(slideIdx, "compose"))
	if slide.ShapeGrid != nil {
		hosts = appendGridPatternHosts(hosts, slide.ShapeGrid, slidepath.ShapeGrid(slideIdx))
	}
	return hosts
}

func appendComposePatternHosts(hosts []patternHost, c *ComposeInput, prefix string) []patternHost {
	if c == nil {
		return hosts
	}
	for i := range c.Segments {
		seg := &c.Segments[i]
		segPath := fmt.Sprintf("%s/segments/%d", prefix, i)
		if seg.HasPattern() {
			hosts = append(hosts, patternHost{name: seg.Pattern.Name, values: seg.Pattern.Values,
				path: segPath + "/pattern",
				set:  func(v json.RawMessage) { seg.Pattern.Values = v }})
		}
		hosts = appendComposePatternHosts(hosts, seg.Compose, segPath+"/compose")
	}
	return hosts
}

func appendGridPatternHosts(hosts []patternHost, grid *ShapeGridInput, prefix string) []patternHost {
	for r := range grid.Rows {
		for c, cell := range grid.Rows[r].Cells {
			if cell == nil {
				continue
			}
			cellPath := fmt.Sprintf("%s/rows/%d/cells/%d", prefix, r, c)
			if cell.Grid != nil {
				hosts = appendGridPatternHosts(hosts, cell.Grid, cellPath+"/grid")
			}
			if h, ok := cellPatternHost(cell, cellPath+"/pattern"); ok {
				hosts = append(hosts, h)
			}
		}
	}
	return hosts
}

// cellPatternHost returns the host of a cell's nested pattern. The cell keeps
// its pattern object as authored and only values is replaced on a write, so
// no other key is reshaped on the way through.
func cellPatternHost(cell *jsonschema.GridCellInput, path string) (patternHost, bool) {
	if len(cell.Pattern) == 0 {
		return patternHost{}, false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(cell.Pattern, &obj); err != nil {
		return patternHost{}, false // pattern validation reports the malformed object
	}
	var name string
	if err := json.Unmarshal(obj["name"], &name); err != nil || name == "" {
		return patternHost{}, false
	}
	return patternHost{name: name, values: obj["values"], path: path,
		set: func(v json.RawMessage) {
			obj["values"] = v
			if data, err := json.Marshal(obj); err == nil {
				cell.Pattern = data
			}
		}}, true
}

// patternImageRefs decodes the values of every pattern a slide hosts and
// returns the image references of those implementing
// patterns.ImageAssetPattern, plus a commit func that writes modified values
// back into the slide.
func patternImageRefs(slide *SlideInput, slideIdx int) ([]patternImageRef, func()) {
	var refs []patternImageRef
	var commits []func()
	for _, h := range slidePatternHosts(slide, slideIdx) {
		if len(h.values) == 0 {
			continue
		}
		pat, ok := patterns.Default().Get(h.name)
		if !ok {
			continue
		}
		ia, ok := pat.(patterns.ImageAssetPattern)
		if !ok {
			continue
		}
		values := pat.NewValues()
		if err := json.Unmarshal(h.values, values); err != nil {
			continue // pattern validation reports the malformed values
		}
		found := false
		for _, a := range ia.ImageAssets(values) {
			if a.Image != nil {
				refs = append(refs, patternImageRef{img: a.Image, path: h.path + "/values/" + a.Field})
				found = true
			}
		}
		if found {
			commits = append(commits, func() {
				if data, err := json.Marshal(values); err == nil {
					h.set(data)
				}
			})
		}
	}
	return refs, func() {
		for _, c := range commits {
			c()
		}
	}
}

// resolvePatternImagePaths resolves relative image paths inside pattern
// values against the deck directory, exactly like shape_grid image cells
// (patterns expand after asset resolution, so their images would otherwise
// be looked up relative to the process working directory).
func resolvePatternImagePaths(slides []SlideInput, baseDir string) []diagnostics.Diagnostic {
	var findings []diagnostics.Diagnostic
	for i := range slides {
		refs, commit := patternImageRefs(&slides[i], i)
		changed := false
		for _, r := range refs {
			if r.img.Path == "" {
				continue
			}
			before := r.img.Path
			findings = append(findings, applyLocalAssetPath(&r.img.Path, baseDir,
				diagnostics.CodeImagePath, "image", i, r.path+"/path")...)
			changed = changed || r.img.Path != before
		}
		if changed {
			commit()
		}
	}
	return findings
}

// patternHasImageURL reports whether a slide's pattern values reference an
// image by URL (so the URL resolver must be started).
func patternHasImageURL(slide *SlideInput) bool {
	refs, _ := patternImageRefs(slide, 0)
	for _, r := range refs {
		if r.img.URL != "" {
			return true
		}
	}
	return false
}

// resolvePatternImageURLs downloads pattern image URLs through the resolver
// cache and rewrites them to local paths.
func resolvePatternImageURLs(slide *SlideInput, slideIdx int, resolver urlResolver) []diagnostics.Diagnostic {
	refs, commit := patternImageRefs(slide, slideIdx)
	var findings []diagnostics.Diagnostic
	changed := false
	for _, r := range refs {
		if r.img.URL == "" {
			continue
		}
		path, err := resolver.ResolveImage(r.img.URL)
		if err != nil {
			findings = append(findings, urlFetchDiagnostic(r.path+"/url", "image", "image", r.img.URL, slideIdx, err))
			continue
		}
		r.img.Path, r.img.URL = path, ""
		changed = true
	}
	if changed {
		commit()
	}
	return findings
}
