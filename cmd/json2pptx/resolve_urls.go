package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// urlFetchCode is the structured diagnostic code emitted when a URL reference
// (background.url, image_value.url, icon.url, grid image.url, nested
// shape.icon.url) cannot be downloaded or validated.
const urlFetchCode = diagnostics.CodeURLFetchFailed

// urlResolver is the subset of *resource.Resolver that resolveURLs uses.
// Defined as an interface so tests can stub it without standing up a real
// HTTP server.
type urlResolver interface {
	ResolveImage(rawURL string) (string, error)
	ResolveSVG(rawURL string) (string, error)
}

// hasURLReferences returns true if any slide contains a URL reference that needs resolution.
func hasURLReferences(slides []SlideInput) bool { //nolint:gocognit
	for i := range slides {
		if slides[i].Background != nil && slides[i].Background.URL != "" {
			return true
		}
		if patternHasImageURL(&slides[i]) {
			return true
		}
		for j := range slides[i].Content {
			if slides[i].Content[j].Type == "image" && slides[i].Content[j].ImageValue != nil && slides[i].Content[j].ImageValue.URL != "" {
				return true
			}
		}
		if slides[i].ShapeGrid != nil && gridHasURLReferences(slides[i].ShapeGrid) {
			return true
		}
	}
	return false
}

// gridHasURLReferences reports whether grid or any nested cell grid carries
// an image / icon URL.
func gridHasURLReferences(grid *ShapeGridInput) bool {
	for j := range grid.Rows {
		for _, cell := range grid.Rows[j].Cells {
			if cell == nil {
				continue
			}
			if (cell.Image != nil && cell.Image.URL != "") ||
				(cell.Icon != nil && cell.Icon.URL != "") ||
				(cell.Shape != nil && cell.Shape.Icon != nil && cell.Shape.Icon.URL != "") {
				return true
			}
			if cell.Grid != nil && gridHasURLReferences(cell.Grid) {
				return true
			}
		}
	}
	return false
}

// resolveURLs walks all slides and resolves URL references (icon.url, image.url,
// background.url, image_value.url, nested shape.icon.url) to local cached files
// via the resolver. Successful resolutions clear the URL field and populate the
// corresponding Path/Image field. Each failure is recorded as a structured
// diagnostic so CLI and MCP callers can surface all broken references in one
// pass instead of bailing on the first one.
func resolveURLs(slides []SlideInput, resolver urlResolver) []diagnostics.Diagnostic { //nolint:gocognit
	var findings []diagnostics.Diagnostic
	for i := range slides {
		// Background image URL
		if slides[i].Background != nil && slides[i].Background.URL != "" {
			path, err := resolver.ResolveImage(slides[i].Background.URL)
			if err != nil {
				findings = append(findings, urlFetchDiagnostic(
					slidepath.SlideField(i, "background/url"),
					"background", "image", slides[i].Background.URL, i, err,
				))
			} else {
				slides[i].Background.Image = path
				slides[i].Background.URL = ""
			}
		}

		// Pattern-level image URLs (e.g. image-text-split values.image.url)
		findings = append(findings, resolvePatternImageURLs(&slides[i], i, resolver)...)

		// Content-level image URLs
		for j := range slides[i].Content {
			c := &slides[i].Content[j]
			if c.Type == "image" && c.ImageValue != nil && c.ImageValue.URL != "" {
				path, err := resolver.ResolveImage(c.ImageValue.URL)
				if err != nil {
					findings = append(findings, urlFetchDiagnostic(
						slidepath.ContentField(i, j, "image_value/url"),
						"image", "image", c.ImageValue.URL, i, err,
					))
				} else {
					c.ImageValue.Path = path
					c.ImageValue.URL = ""
				}
			}
		}

		// Shape grid URLs, including nested cell grids.
		if slides[i].ShapeGrid != nil {
			findings = append(findings, resolveGridURLs(slides[i].ShapeGrid, slidepath.ShapeGrid(i), i, resolver)...)
		}
	}
	return findings
}

// resolveGridURLs resolves the image / icon URLs of grid's cells and,
// recursively, of every nested cell grid under the JSON pointer prefix.
// Nested grids used to be skipped, leaving their URLs unfetched.
func resolveGridURLs(grid *ShapeGridInput, prefix string, slideIdx int, resolver urlResolver) []diagnostics.Diagnostic {
	var findings []diagnostics.Diagnostic
	for j := range grid.Rows {
		for k, cell := range grid.Rows[j].Cells {
			if cell == nil {
				continue
			}
			cellPath := fmt.Sprintf("%s/rows/%d/cells/%d", prefix, j, k)
			if cell.Image != nil {
				findings = appendURLFinding(findings, resolveURLField(&cell.Image.URL, &cell.Image.Path, false, cellPath+"/image/url", "image", slideIdx, resolver))
			}
			if cell.Icon != nil {
				findings = appendURLFinding(findings, resolveURLField(&cell.Icon.URL, &cell.Icon.Path, true, cellPath+"/icon/url", "icon", slideIdx, resolver))
			}
			if cell.Shape != nil && cell.Shape.Icon != nil {
				findings = appendURLFinding(findings, resolveURLField(&cell.Shape.Icon.URL, &cell.Shape.Icon.Path, true, cellPath+"/shape/icon/url", "icon", slideIdx, resolver))
			}
			if cell.Grid != nil {
				findings = append(findings, resolveGridURLs(cell.Grid, cellPath+"/grid", slideIdx, resolver)...)
			}
		}
	}
	return findings
}

// resolveURLField downloads *url (an SVG when svg is set, else a raster
// image) and, on success, moves the cached path into *path and clears *url.
// Returns the failure diagnostic, or nil on success / when *url is empty.
func resolveURLField(url, path *string, svg bool, jsonPath, assetKind string, slideIdx int, resolver urlResolver) *diagnostics.Diagnostic {
	if *url == "" {
		return nil
	}
	resolve, expected := resolver.ResolveImage, "image"
	if svg {
		resolve, expected = resolver.ResolveSVG, "svg"
	}
	p, err := resolve(*url)
	if err != nil {
		d := urlFetchDiagnostic(jsonPath, assetKind, expected, *url, slideIdx, err)
		return &d
	}
	*path, *url = p, ""
	return nil
}

func appendURLFinding(findings []diagnostics.Diagnostic, d *diagnostics.Diagnostic) []diagnostics.Diagnostic {
	if d == nil {
		return findings
	}
	return append(findings, *d)
}

// urlFetchDiagnostic builds a structured diagnostic with the fields agents
// need to repair a broken URL reference: JSON Pointer path, asset kind,
// expected content type, the offending URL, and the underlying error message.
// The default code is URL_FETCH_FAILED; SVG-specific validation failures
// surface the more precise SVG_INVALID_ROOT / SVG_UNSAFE_XML / SVG_PARSE_ERROR
// codes via the structured error returned by resource.ResolveSVG.
func urlFetchDiagnostic(jsonPath, assetKind, expectedContent, rawURL string, slideIdx int, cause error) diagnostics.Diagnostic {
	code := urlFetchCode
	if svgCode := resource.SVGValidationCode(cause); svgCode != "" {
		code = svgCode
	}
	return diagnostics.Diagnostic{
		Code:     code,
		Path:     jsonPath,
		Message:  fmt.Sprintf("%s URL %q: %v", assetKind, rawURL, cause),
		Severity: diagnostics.SeverityError,
		Details: map[string]any{
			"slide_index":      slideIdx,
			"asset_kind":       assetKind,
			"expected_content": expectedContent,
			"input_url":        rawURL,
			"remediation":      "verify the URL is reachable, returns the expected content type, and is not behind auth; or replace with a local path",
		},
	}
}

// Compile-time assertion: the production resolver satisfies the interface.
var _ urlResolver = (*resource.Resolver)(nil)
