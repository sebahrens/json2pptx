// Package pptx provides low-level PPTX file manipulation primitives.
//
// The pptx package handles direct manipulation of Office Open XML (OOXML)
// package files that comprise PowerPoint presentations. It provides abstractions
// for common operations while maintaining access to the underlying XML structure.
//
// # Core Types
//
// The main types are:
//
//   - Package: Represents an open PPTX archive with access to all parts
//   - Document: High-level API wrapping Package for common operations
//   - Slide: Represents a single slide with its XML structure
//   - RectEmu: Rectangle dimensions in EMUs (English Metric Units)
//
// # Architecture
//
// PPTX files are ZIP archives containing XML parts:
//
//	presentation.xml      → Presentation structure
//	ppt/slides/slide*.xml → Individual slides
//	ppt/slideLayouts/     → Layout definitions
//	ppt/slideMasters/     → Master slides with theming
//	ppt/media/            → Embedded images and media
//
// The package provides safe read/write access to these parts.
//
// # Usage
//
//	pkg, err := pptx.Open(templatePath)
//	if err != nil {
//	    return err
//	}
//	defer pkg.Close()
//
//	doc := pptx.NewDocument(pkg)
//	doc.InsertImage(slideIdx, imagePath, bounds)
//
//	return pkg.Save(outputPath)
//
// # EMU Units
//
// PowerPoint uses EMUs (English Metric Units) for all dimensions.
// 914400 EMUs = 1 inch. Use types.FromPixels() for conversions.
//
// # Validation
//
// The validator submodule provides structural validation of generated
// PPTX files to ensure they will open correctly in PowerPoint.
package pptx
