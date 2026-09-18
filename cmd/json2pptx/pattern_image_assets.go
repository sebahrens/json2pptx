package main

import (
	"encoding/json"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// patternImageRef is one image reference inside a slide-level pattern's
// values (e.g. image-text-split's values.image), with its JSON path.
type patternImageRef struct {
	img  *jsonschema.GridImageInput
	path string // JSON pointer of the image object, e.g. /slides/2/pattern/values/image
}

// patternImageRefs decodes a slide's pattern values and returns the image
// references of patterns implementing patterns.ImageAssetPattern, plus a
// commit func that writes modified values back into the slide.
func patternImageRefs(slide *SlideInput, slideIdx int) ([]patternImageRef, func()) {
	if slide.Pattern == nil || len(slide.Pattern.Values) == 0 {
		return nil, nil
	}
	pat, ok := patterns.Default().Get(slide.Pattern.Name)
	if !ok {
		return nil, nil
	}
	ia, ok := pat.(patterns.ImageAssetPattern)
	if !ok {
		return nil, nil
	}
	values := pat.NewValues()
	if err := json.Unmarshal(slide.Pattern.Values, values); err != nil {
		return nil, nil // pattern validation reports the malformed values
	}
	var refs []patternImageRef
	for _, a := range ia.ImageAssets(values) {
		if a.Image != nil {
			refs = append(refs, patternImageRef{img: a.Image, path: slidepath.SlideField(slideIdx, "pattern/values/"+a.Field)})
		}
	}
	commit := func() {
		if data, err := json.Marshal(values); err == nil {
			slide.Pattern.Values = data
		}
	}
	return refs, commit
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
