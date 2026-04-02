package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/resource"
)

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
// background.url, image_value.url) to local cached files via the Resolver.
// After this function returns, all URL fields have been cleared and the
// corresponding Path fields point to local files.
func resolveURLs(slides []SlideInput, resolver *resource.Resolver) error { //nolint:gocognit
	for i := range slides {
		// Background image URL
		if slides[i].Background != nil && slides[i].Background.URL != "" {
			path, err := resolver.ResolveImage(slides[i].Background.URL)
			if err != nil {
				return fmt.Errorf("slide %d background: %w", i+1, err)
			}
			slides[i].Background.Image = path
			slides[i].Background.URL = ""
		}

		// Content-level image URLs
		for j := range slides[i].Content {
			c := &slides[i].Content[j]
			if c.Type == "image" && c.ImageValue != nil && c.ImageValue.URL != "" {
				path, err := resolver.ResolveImage(c.ImageValue.URL)
				if err != nil {
					return fmt.Errorf("slide %d, content %d: %w", i+1, j+1, err)
				}
				c.ImageValue.Path = path
				c.ImageValue.URL = ""
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
						return fmt.Errorf("slide %d, grid row %d cell %d image: %w", i+1, j+1, k+1, err)
					}
					cell.Image.Path = path
					cell.Image.URL = ""
				}

				// Icon URL (cell-level)
				if cell.Icon != nil && cell.Icon.URL != "" {
					path, err := resolver.ResolveSVG(cell.Icon.URL)
					if err != nil {
						return fmt.Errorf("slide %d, grid row %d cell %d icon: %w", i+1, j+1, k+1, err)
					}
					cell.Icon.Path = path
					cell.Icon.URL = ""
				}

				// Icon URL nested inside shape
				if cell.Shape != nil && cell.Shape.Icon != nil && cell.Shape.Icon.URL != "" {
					path, err := resolver.ResolveSVG(cell.Shape.Icon.URL)
					if err != nil {
						return fmt.Errorf("slide %d, grid row %d cell %d shape icon: %w", i+1, j+1, k+1, err)
					}
					cell.Shape.Icon.Path = path
					cell.Shape.Icon.URL = ""
				}
			}
		}
	}
	return nil
}
