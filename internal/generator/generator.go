// Package generator provides PPTX file generation from slide specifications.
//
// This package integrates all PPTX generation components:
//   - Slide creation from layouts (slides.go)
//   - Text and bullet content population (text.go)
//   - Image embedding and scaling (images.go)
//
// The main entry point is the Generate function, which takes a GenerationRequest
// and produces a complete PPTX file.
//
// Example usage:
//
//	req := GenerationRequest{
//	    TemplatePath: "templates/corporate.pptx",
//	    OutputPath:   "output/presentation.pptx",
//	    Slides: []SlideSpec{
//	        {
//	            LayoutID: "slideLayout2",
//	            Content: []ContentItem{
//	                {
//	                    PlaceholderID: "title",
//	                    Type:          ContentText,
//	                    Value:         "My Presentation",
//	                },
//	                {
//	                    PlaceholderID: "body",
//	                    Type:          ContentBullets,
//	                    Value:         []string{"First", "Second", "Third"},
//	                },
//	            },
//	        },
//	    },
//	}
//
//	result, err := Generate(ctx, req)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Generated %d slides in %v\n", result.SlideCount, result.Duration)
//	fmt.Printf("Output: %s (%d bytes)\n", result.OutputPath, result.FileSize)
package generator

// The Generate function and related types are defined in slides.go.
// This file serves as the main package documentation and entry point.
//
// Key types:
//   - GenerationRequest: Specifies what to generate
//   - GenerationResult: Contains generation output and metrics
//   - SlideSpec: Defines a single slide to create
//   - ContentItem: Represents content to place in a placeholder
//   - ContentType: The kind of content (text, bullets, image, chart)
//   - ImageContent: Image content to embed
//
// See slides.go for the Generate function implementation.
