package generator

import (
	"testing"
)

func TestNewSinglePassContext(t *testing.T) {
	slides := []SlideSpec{
		{LayoutID: "layout1", Content: []ContentItem{}},
		{LayoutID: "layout2", Content: []ContentItem{}},
	}
	allowedPaths := []string{"/path/one", "/path/two"}
	outputPath := "/output/presentation.pptx"

	tests := []struct {
		name                  string
		outputPath            string
		slides                []SlideSpec
		allowedPaths          []string
		excludeTemplateSlides bool
	}{
		{
			name:                  "basic initialization",
			outputPath:            outputPath,
			slides:                slides,
			allowedPaths:          allowedPaths,
			excludeTemplateSlides: false,
		},
		{
			name:                  "with exclude template slides",
			outputPath:            outputPath,
			slides:                slides,
			allowedPaths:          allowedPaths,
			excludeTemplateSlides: true,
		},
		{
			name:                  "empty slides",
			outputPath:            outputPath,
			slides:                []SlideSpec{},
			allowedPaths:          allowedPaths,
			excludeTemplateSlides: false,
		},
		{
			name:                  "empty allowed paths",
			outputPath:            outputPath,
			slides:                slides,
			allowedPaths:          []string{},
			excludeTemplateSlides: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newSinglePassContext(tt.outputPath, tt.slides, tt.allowedPaths, tt.excludeTemplateSlides, nil)

			if ctx == nil {
				t.Fatal("newSinglePassContext returned nil")
			}

			// Verify ZipContext
			if ctx.outputPath != tt.outputPath {
				t.Errorf("outputPath = %q, want %q", ctx.outputPath, tt.outputPath)
			}
			if ctx.tmpPath != tt.outputPath+".tmp" {
				t.Errorf("tmpPath = %q, want %q", ctx.tmpPath, tt.outputPath+".tmp")
			}

			// Verify SlideContext
			if len(ctx.slideSpecs) != len(tt.slides) {
				t.Errorf("slideSpecs length = %d, want %d", len(ctx.slideSpecs), len(tt.slides))
			}
			if ctx.excludeTemplateSlides != tt.excludeTemplateSlides {
				t.Errorf("excludeTemplateSlides = %v, want %v", ctx.excludeTemplateSlides, tt.excludeTemplateSlides)
			}
			if ctx.templateSlideData == nil {
				t.Error("templateSlideData is nil, want initialized map")
			}
			if ctx.newSlideData == nil {
				t.Error("newSlideData is nil, want initialized map")
			}
			if ctx.slideContentMap == nil {
				t.Error("slideContentMap is nil, want initialized map")
			}
			if ctx.slideRelIDs == nil {
				t.Error("slideRelIDs is nil, want initialized map")
			}

			// Verify MediaContext
			if ctx.media == nil {
				t.Error("media allocator is nil, want initialized")
			} else if ctx.nextMediaNum() != 1 {
				t.Errorf("nextMediaNum() = %d, want 1", ctx.nextMediaNum())
			}
			if ctx.mediaFiles == nil {
				t.Error("mediaFiles is nil, want initialized map")
			}
			if ctx.usedExtensions == nil {
				t.Error("usedExtensions is nil, want initialized map")
			}
			if ctx.slideRelUpdates == nil {
				t.Error("slideRelUpdates is nil, want initialized map")
			}

			// Verify SVGContext
			if ctx.nativeSVGInserts == nil {
				t.Error("nativeSVGInserts is nil, want initialized map")
			}

			// Verify SecurityContext
			if len(ctx.allowedImagePaths) != len(tt.allowedPaths) {
				t.Errorf("allowedImagePaths length = %d, want %d", len(ctx.allowedImagePaths), len(tt.allowedPaths))
			}

			// Verify OutputContext
			if ctx.modifiedFiles == nil {
				t.Error("modifiedFiles is nil, want initialized map")
			}
			if len(ctx.warnings) != 0 {
				t.Errorf("warnings = %v, want nil or empty", ctx.warnings)
			}
		})
	}
}

func TestMediaRelStruct(t *testing.T) {
	// Test that mediaRel struct can hold both file path and byte data
	tests := []struct {
		name     string
		rel      mediaRel
		hasData  bool
		hasPath  bool
	}{
		{
			name: "file path based media",
			rel: mediaRel{
				imagePath:     "/path/to/image.png",
				mediaFileName: "image1.png",
				relID:         "rId10",
				shapeID:       5,
			},
			hasData: false,
			hasPath: true,
		},
		{
			name: "byte data based media (charts)",
			rel: mediaRel{
				data:          []byte{0x89, 0x50, 0x4E, 0x47}, // PNG magic bytes
				mediaFileName: "image2.png",
				relID:         "rId11",
				shapeID:       6,
			},
			hasData: true,
			hasPath: false,
		},
		{
			name: "with position data",
			rel: mediaRel{
				imagePath:      "/path/to/image.png",
				mediaFileName:  "image3.png",
				offsetX:        100,
				offsetY:        200,
				extentCX:       300,
				extentCY:       400,
				placeholderIdx: 2,
			},
			hasData: false,
			hasPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasData && tt.rel.data == nil {
				t.Error("expected data to be set")
			}
			if !tt.hasData && tt.rel.data != nil {
				t.Error("expected data to be nil")
			}
			if tt.hasPath && tt.rel.imagePath == "" {
				t.Error("expected imagePath to be set")
			}
			if !tt.hasPath && tt.rel.imagePath != "" {
				t.Error("expected imagePath to be empty")
			}
		})
	}
}

func TestNativeSVGInsertStruct(t *testing.T) {
	// Test that nativeSVGInsert struct can hold SVG+PNG pairs
	insert := nativeSVGInsert{
		svgPath:      "/path/to/chart.svg",
		pngPath:      "/path/to/chart.png",
		svgMediaFile: "image1.svg",
		pngMediaFile: "image2.png",
		svgRelID:     "rId10",
		pngRelID:     "rId11",
		offsetX:      1000,
		offsetY:      2000,
		extentCX:     3000,
		extentCY:     4000,
		shapeID:      7,
		placeholderIdx: 3,
	}

	// Verify all fields are accessible
	if insert.svgPath != "/path/to/chart.svg" {
		t.Errorf("svgPath = %q, want /path/to/chart.svg", insert.svgPath)
	}
	if insert.pngPath != "/path/to/chart.png" {
		t.Errorf("pngPath = %q, want /path/to/chart.png", insert.pngPath)
	}
	if insert.svgMediaFile != "image1.svg" {
		t.Errorf("svgMediaFile = %q, want image1.svg", insert.svgMediaFile)
	}
	if insert.pngMediaFile != "image2.png" {
		t.Errorf("pngMediaFile = %q, want image2.png", insert.pngMediaFile)
	}
	if insert.offsetX != 1000 {
		t.Errorf("offsetX = %d, want 1000", insert.offsetX)
	}
	if insert.extentCX != 3000 {
		t.Errorf("extentCX = %d, want 3000", insert.extentCX)
	}
}

func TestContextMapsAreIndependent(t *testing.T) {
	// Test that multiple contexts don't share maps (important for concurrency)
	ctx1 := newSinglePassContext("/output1.pptx", []SlideSpec{}, nil, false, nil)
	ctx2 := newSinglePassContext("/output2.pptx", []SlideSpec{}, nil, false, nil)

	// Modify ctx1's maps
	ctx1.mediaFiles["test.png"] = "media1.png"
	ctx1.modifiedFiles["file.xml"] = []byte("content")
	ctx1.slideRelIDs[1] = "rId10"

	// Verify ctx2's maps are not affected
	if len(ctx2.mediaFiles) != 0 {
		t.Error("ctx2.mediaFiles should be empty")
	}
	if len(ctx2.modifiedFiles) != 0 {
		t.Error("ctx2.modifiedFiles should be empty")
	}
	if len(ctx2.slideRelIDs) != 0 {
		t.Error("ctx2.slideRelIDs should be empty")
	}
}
