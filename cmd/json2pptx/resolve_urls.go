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
		for j := range slides[i].Content {
			if slides[i].Content[j].Type == "image" && slides[i].Content[j].ImageValue != nil && slides[i].Content[j].ImageValue.URL != "" {
				return true
			}
		}
		if slides[i].ShapeGrid == nil {
			continue
		}
		for j := range slides[i].ShapeGrid.Rows {
			for k := range slides[i].ShapeGrid.Rows[j].Cells {
				cell := slides[i].ShapeGrid.Rows[j].Cells[k]
				if cell == nil {
					continue
				}
				if cell.Image != nil && cell.Image.URL != "" {
					return true
				}
				if cell.Icon != nil && cell.Icon.URL != "" {
					return true
				}
				if cell.Shape != nil && cell.Shape.Icon != nil && cell.Shape.Icon.URL != "" {
					return true
				}
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

		// Shape grid URLs
		if slides[i].ShapeGrid == nil {
			continue
		}
		for j := range slides[i].ShapeGrid.Rows {
			for k := range slides[i].ShapeGrid.Rows[j].Cells {
				cell := slides[i].ShapeGrid.Rows[j].Cells[k]
				if cell == nil {
					continue
				}

				// Grid image URL
				if cell.Image != nil && cell.Image.URL != "" {
					path, err := resolver.ResolveImage(cell.Image.URL)
					if err != nil {
						findings = append(findings, urlFetchDiagnostic(
							slidepath.GridCellField(i, j, k, "image/url"),
							"image", "image", cell.Image.URL, i, err,
						))
					} else {
						cell.Image.Path = path
						cell.Image.URL = ""
					}
				}

				// Icon URL (cell-level)
				if cell.Icon != nil && cell.Icon.URL != "" {
					path, err := resolver.ResolveSVG(cell.Icon.URL)
					if err != nil {
						findings = append(findings, urlFetchDiagnostic(
							slidepath.GridCellField(i, j, k, "icon/url"),
							"icon", "svg", cell.Icon.URL, i, err,
						))
					} else {
						cell.Icon.Path = path
						cell.Icon.URL = ""
					}
				}

				// Icon URL nested inside shape
				if cell.Shape != nil && cell.Shape.Icon != nil && cell.Shape.Icon.URL != "" {
					path, err := resolver.ResolveSVG(cell.Shape.Icon.URL)
					if err != nil {
						findings = append(findings, urlFetchDiagnostic(
							slidepath.GridCellField(i, j, k, "shape/icon/url"),
							"icon", "svg", cell.Shape.Icon.URL, i, err,
						))
					} else {
						cell.Shape.Icon.Path = path
						cell.Shape.Icon.URL = ""
					}
				}
			}
		}
	}
	return findings
}

// urlFetchDiagnostic builds a structured URL_FETCH_FAILED diagnostic with the
// fields agents need to repair a broken URL reference: JSON Pointer path,
// asset kind, expected content type, the offending URL, and the underlying
// error message.
func urlFetchDiagnostic(jsonPath, assetKind, expectedContent, rawURL string, slideIdx int, cause error) diagnostics.Diagnostic {
	return diagnostics.Diagnostic{
		Code:     urlFetchCode,
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
