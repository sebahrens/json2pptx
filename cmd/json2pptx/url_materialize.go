package main

import (
	"path/filepath"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/resource"
)

// urlMaterialization is the outcome of downloading a deck's URL references
// into a request-scoped cache. The slides now point at cached files that
// Cleanup deletes, so anything that outlives the request (a deck handle, a
// repair checkpoint, a returned deck) must first call RestoreAuthoredURLs to
// put the authored URLs back (go-slide-creator-b7qqg.9).
type urlMaterialization struct {
	Findings []diagnostics.Diagnostic
	// Cleanup removes the download cache; always non-nil.
	Cleanup func()
	// CacheDir is the download directory ("" when nothing was fetched); pass
	// it to imageAllowList.
	CacheDir string
	// urls maps every spelling of a downloaded file path (as returned by the
	// resolver and after symlink evaluation, which is how resolveLocalAssetPaths
	// rewrites it) to the URL it came from.
	urls map[string]string
}

// recordingResolver wraps a urlResolver and remembers which local file each
// successfully resolved URL was written to.
type recordingResolver struct {
	inner urlResolver
	urls  map[string]string
}

func (r *recordingResolver) record(path, rawURL string) {
	r.urls[path] = rawURL
	if real, err := filepath.EvalSymlinks(path); err == nil {
		r.urls[real] = rawURL
	}
}

func (r *recordingResolver) ResolveImage(rawURL string) (string, error) {
	p, err := r.inner.ResolveImage(rawURL)
	if err == nil {
		r.record(p, rawURL)
	}
	return p, err
}

func (r *recordingResolver) ResolveSVG(rawURL string) (string, error) {
	p, err := r.inner.ResolveSVG(rawURL)
	if err == nil {
		r.record(p, rawURL)
	}
	return p, err
}

// materializeURLs downloads every URL reference in slides through the
// SSRF-safe resource resolver (the same one generate_presentation uses) and
// rewrites the fields to cached local paths, recording the mapping so the
// authored URLs can be restored. A resolver init failure is returned as err.
func (mc *mcpConfig) materializeURLs(slides []SlideInput) (*urlMaterialization, error) {
	m := &urlMaterialization{Cleanup: func() {}}
	if !hasURLReferences(slides) {
		return m, nil
	}
	resolver, err := resource.NewResolver(mc.resolverOpts)
	if err != nil {
		return m, err
	}
	rec := &recordingResolver{inner: resolver, urls: map[string]string{}}
	m.Findings = resolveURLs(slides, rec)
	m.Cleanup = resolver.Close
	m.CacheDir = resolver.Dir()
	m.urls = rec.urls
	return m, nil
}

// RestoreAuthoredURLs puts the authored URL back on every field that
// materializeURLs rewrote to a (request-scoped, soon deleted) cached file,
// clearing the cached path. Fields the author wrote as local paths are left
// untouched. Safe on a nil receiver or when nothing was downloaded.
func (m *urlMaterialization) RestoreAuthoredURLs(slides []SlideInput) {
	if m == nil || len(m.urls) == 0 {
		return
	}
	swap := func(path, url *string) {
		if u, ok := m.urls[*path]; ok && *path != "" {
			*url, *path = u, ""
		}
	}
	for i := range slides {
		s := &slides[i]
		if s.Background != nil {
			swap(&s.Background.Image, &s.Background.URL)
		}
		for j := range s.Content {
			if iv := s.Content[j].ImageValue; iv != nil {
				swap(&iv.Path, &iv.URL)
			}
		}
		if s.ShapeGrid != nil {
			m.restoreGrid(s.ShapeGrid, swap)
		}
		refs, commit := patternImageRefs(s, i)
		changed := false
		for _, r := range refs {
			before := r.img.Path
			swap(&r.img.Path, &r.img.URL)
			changed = changed || r.img.Path != before
		}
		if changed {
			commit()
		}
	}
}

func (m *urlMaterialization) restoreGrid(grid *ShapeGridInput, swap func(path, url *string)) {
	for j := range grid.Rows {
		for _, cell := range grid.Rows[j].Cells {
			if cell == nil {
				continue
			}
			if cell.Image != nil {
				swap(&cell.Image.Path, &cell.Image.URL)
			}
			if cell.Icon != nil {
				swap(&cell.Icon.Path, &cell.Icon.URL)
			}
			if cell.Shape != nil && cell.Shape.Icon != nil {
				swap(&cell.Shape.Icon.Path, &cell.Shape.Icon.URL)
			}
			if cell.Grid != nil {
				m.restoreGrid(cell.Grid, swap)
			}
		}
	}
}
