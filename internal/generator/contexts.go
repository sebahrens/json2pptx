// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// ZipContext holds ZIP I/O state for single-pass generation.
// Responsible for template reading and output writing operations.
type ZipContext struct {
	templateReader *zip.ReadCloser
	templateIndex  utils.ZipIndex // O(1) filename lookups into templateReader
	outputWriter   *zip.Writer
	outputFile     *os.File
	tmpPath        string
	outputPath     string
}

// LogoZone describes a rectangular area occupied by a template logo.
// When a small decorative image (typically a company logo) is detected in the
// top-left corner of a slide layout, its bounding box is recorded here so that
// title and diagram content can be shifted to avoid visual overlap.
// All values are in EMUs (English Metric Units).
type LogoZone struct {
	Right  int64 // Right edge X of the logo (X + Width)
	Bottom int64 // Bottom edge Y of the logo (Y + Height)
}

// SlideContext holds slide tracking state for single-pass generation.
// Responsible for managing existing and new slides.
type SlideContext struct {
	existingSlides        int
	excludeTemplateSlides bool                // When true, template slides are excluded from output
	slideWidth            int64               // Slide width in EMU (from presentation.xml <p:sldSz>)
	slideHeight           int64               // Slide height in EMU (from presentation.xml <p:sldSz>)
	templateSlideData     map[int]*slideXML   // existing slides that might need modification
	newSlideData          map[int][]byte      // newly created slides
	slideSpecs            []SlideSpec         // specs for new slides
	slideContentMap       map[int]SlideSpec   // slideNum -> content
	slideRelIDs           map[int]string      // slideNum -> relationship ID (e.g., "rId15")
	masterBulletLevelCache map[string]int     // masterPath -> first bullet level (cached)
	slideNotes            map[int]string      // slideNum -> speaker notes text (only for slides with notes)
	slideSources          map[int]string      // slideNum -> source attribution text (only for slides with source)
	tableInserts          map[int][]tableInsert      // slideNum -> table XML inserts (replaces placeholder shapes)
	panelShapeInserts     map[int][]panelShapeInsert // slideNum -> native panel shape inserts (replaces placeholder shapes)
	logoZones             map[string]*LogoZone // per-layout logo zones (key = layout basename, e.g. "slideLayout1"); nil if no logos detected
	footerConfig              *FooterConfig       // Footer configuration (nil = disabled)
	footerPositionsByLayout   map[string]map[string]*transformXML // layoutID -> ("dt"/"ftr"/"sldNum" -> position)
	slideBgMedia          map[int]mediaRel    // slideNum -> background image media relationship
	themeFontName         string              // Theme body font (e.g. "Franklin Gothic Book") for text fitting
}

// tableInsert tracks a table that replaces a placeholder shape.
type tableInsert struct {
	placeholderIdx int    // Shape index to remove
	graphicFrameXML string // Complete <p:graphicFrame> XML to insert
}

// panelShapeInsert tracks a native panel shape group that replaces a placeholder shape.
type panelShapeInsert struct {
	placeholderIdx int                // Shape index to remove
	bounds         types.BoundingBox  // Placeholder EMU bounds
	panels         []nativePanelData  // Parsed panel data
	groupXML       string             // Populated during allocatePanelIconRelIDs()
	swotMode       bool               // True for SWOT 2x2 grid layout (vs column panels)
	pestelMode     bool               // True for PESTEL 3x2 grid layout
	nineBoxMode    bool               // True for Nine Box Talent 3x3 grid layout
	rowsMode       bool               // True for horizontal rows layout
	statCardsMode  bool               // True for stat_cards grid layout
	valueChainMode   bool               // True for Value Chain diagram layout
	valueChainMeta   valueChainMeta     // Metadata for value chain layout (primary/support counts, margin)
	kpiDashboardMode bool               // True for KPI Dashboard grid layout
	portersFiveMode  bool               // True for Porter's Five Forces cross layout
	bmcMode          bool               // True for Business Model Canvas 9-box layout
	processFlowMode  bool               // True for Process Flow diagram layout
	processFlowMeta  processFlowMeta    // Metadata for process flow layout
	heatmapMode      bool               // True for Heatmap NxM grid layout
	heatmapMeta      heatmapMeta        // Metadata for heatmap layout (rows, cols)
	pyramidMode      bool               // True for Pyramid stacked trapezoid layout
	houseDiagramMode bool               // True for House Diagram (roof/pillars/foundation) layout
	houseDiagramMeta houseDiagramMeta   // Metadata for house diagram layout (floor structure)
}

// nativePanelData holds parsed data for a single panel in a native panel shape group.
type nativePanelData struct {
	title         string // Panel title
	body          string // May contain \n and "- " bullets
	value         string // Hero value for stat_cards mode (e.g., "10%", "$1.2M")
	iconBytes     []byte // nil if icon load failed
	iconMediaFile string // Set during allocation
	iconRelID     string // Set during allocation
}

// MediaContext holds media tracking state for single-pass generation.
// Responsible for managing image files, extensions, and relationships.
type MediaContext struct {
	media           *pptx.MediaAllocator
	mediaFiles      map[string]string  // image path -> media filename
	usedExtensions  map[string]bool    // image extensions for Content_Types.xml
	slideRelUpdates map[int][]mediaRel // slideNum -> relationships to add
}

// SVGContext holds SVG conversion state for single-pass generation.
// Responsible for SVG conversion strategies and native SVG handling.
type SVGContext struct {
	svgConverter     *SVGConverter // SVG converter with configured strategy
	svgCleanupFuncs  []func()              // cleanup functions for converted SVG temp files
	nativeSVGInserts map[int][]nativeSVGInsert
}

// SecurityContext holds security-related configuration.
type SecurityContext struct {
	allowedImagePaths []string // allowed base paths for image loading
}

// ChartContext holds chart rendering state.
type ChartContext struct {
	// Theme colors from template for chart styling
	themeColors []types.ThemeColor

	// themeOverride contains per-deck color/font overrides from frontmatter.
	// Applied after template theme is parsed, before chart rendering.
	themeOverride *types.ThemeOverride

	// strictFit controls how chart/diagram fit findings affect generation.
	// Threaded to svggen via RequestEnvelope.Output.StrictFit.
	strictFit string
}

// OutputContext holds output tracking state.
type OutputContext struct {
	// Files that need modification
	modifiedFiles map[string][]byte // path -> content

	// Synthetic layout files from SynthesisManifest.
	// Maps ZIP paths (e.g., "ppt/slideLayouts/slideLayout99.xml") to XML bytes.
	// Checked before the template ZIP in readLayoutFile and getMasterPositionsForLayout.
	syntheticFiles map[string][]byte

	// Warnings accumulated during generation
	warnings []string

	// Structured validation errors emitted during generation (e.g. placeholder_not_found)
	validationErrors []*patterns.ValidationError

	// Structured media failures (diagrams, images, tables that failed to render)
	mediaFailures []MediaFailure
}

// singlePassContext holds all state needed for single-pass ZIP generation.
// This consolidates what was previously done in 3 separate passes.
//
// The struct embeds focused sub-contexts to maintain single responsibility:
//   - ZipContext: ZIP I/O operations
//   - SlideContext: Slide tracking and manipulation
//   - MediaContext: Media file management
//   - SVGContext: SVG conversion and embedding
//   - SecurityContext: Security configuration
//   - ChartContext: Chart rendering
//   - OutputContext: Output files and warnings
type singlePassContext struct {
	ctx context.Context // propagated from Generate() for cancellation and timeouts
	ZipContext
	SlideContext
	MediaContext
	SVGContext
	SecurityContext
	ChartContext
	OutputContext
}

// newSinglePassContext creates a new singlePassContext with initialized maps.
func newSinglePassContext(outputPath string, slides []SlideSpec, allowedPaths []string, excludeTemplateSlides bool, syntheticFiles map[string][]byte) *singlePassContext {
	return &singlePassContext{
		ZipContext: ZipContext{
			outputPath: outputPath,
			tmpPath:    outputPath + ".tmp",
		},
		SlideContext: SlideContext{
			slideSpecs:             slides,
			excludeTemplateSlides:  excludeTemplateSlides,
			templateSlideData:      make(map[int]*slideXML),
			newSlideData:           make(map[int][]byte),
			slideContentMap:        make(map[int]SlideSpec),
			slideRelIDs:            make(map[int]string),
			masterBulletLevelCache: make(map[string]int),
			slideNotes:             make(map[int]string),
			slideSources:           make(map[int]string),
			tableInserts:           make(map[int][]tableInsert),
			panelShapeInserts:      make(map[int][]panelShapeInsert),
			slideBgMedia:           make(map[int]mediaRel),
		},
		MediaContext: MediaContext{
			media:           pptx.NewMediaAllocator(),
			mediaFiles:      make(map[string]string),
			usedExtensions:  make(map[string]bool),
			slideRelUpdates: make(map[int][]mediaRel),
		},
		SVGContext: SVGContext{
			nativeSVGInserts: make(map[int][]nativeSVGInsert),
		},
		SecurityContext: SecurityContext{
			allowedImagePaths: allowedPaths,
		},
		ChartContext: ChartContext{},
		OutputContext: OutputContext{
			modifiedFiles:  make(map[string][]byte),
			syntheticFiles: syntheticFiles,
		},
	}
}

// mediaRel represents a media relationship to add.
// For streaming ZIP writes, it supports both file paths and in-memory byte data.
// This eliminates temp file creation for charts, keeping memory bounded.
type mediaRel struct {
	imagePath     string // File path (used when data is nil)
	mediaFileName string
	data          []byte // Direct byte data (used for charts, eliminates temp files)

	// Position/size for p:pic element insertion (from placeholder)
	offsetX, offsetY   int64
	extentCX, extentCY int64

	// Relationship ID (allocated during write)
	relID string

	// Shape ID for p:pic element (allocated during write)
	shapeID uint32

	// Placeholder index to remove from slide (required for p:pic insertion)
	placeholderIdx int
}

// nativeSVGInsert tracks a native SVG+PNG insert for a slide.
// This is used for native SVG embedding (both external SVG images and svggen-rendered charts).
type nativeSVGInsert struct {
	// File paths (for external SVG images loaded from disk)
	svgPath string // Path to original SVG file
	pngPath string // Path to generated PNG fallback

	// Byte data (for svggen-rendered charts/diagrams — in-memory, no temp files)
	svgData []byte // SVG XML content
	pngData []byte // PNG fallback image

	// Media filenames in ZIP
	svgMediaFile string // e.g., "image1.svg"
	pngMediaFile string // e.g., "image2.png"

	// Relationship IDs (allocated during writeOutput)
	svgRelID string
	pngRelID string

	// Shape position/size (from placeholder)
	offsetX, offsetY   int64
	extentCX, extentCY int64

	// Shape ID (allocated during writeOutput)
	shapeID uint32

	// Placeholder to remove from slide
	placeholderIdx int
}

// transparentPNG1x1 is a minimal 1x1 transparent PNG image (67 bytes).
// Used as the OOXML a:blip fallback reference when native SVG embedding is active.
// The OOXML spec requires a:blip r:embed to reference a valid image, but PowerPoint
// 2016+ renders the asvg:svgBlip extension instead. This avoids rasterizing each
// diagram just to produce a never-displayed fallback PNG.
var transparentPNG1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, // RGBA, CRC
	0x89,
	0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, // IDAT chunk
	0x78, 0x9c, 0x62, 0x00, 0x00, 0x00, 0x02, 0x00, // zlib compressed
	0x01, 0xe5, 0x27, 0xde, 0xfc,
	0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, // IEND chunk
	0xae, 0x42, 0x60, 0x82,
}

// allocSVGPNGPair allocates a paired SVG+PNG media filename slot via the MediaAllocator.
// sourceID should uniquely identify the content (e.g., "chart-slide3-idx2").
// Returns (svgMediaFile, pngMediaFile) basenames like ("image5.svg", "image6.png").
func (ctx *singlePassContext) allocSVGPNGPair(sourceID string) (svgMediaFile, pngMediaFile string) {
	svgPath := ctx.media.Allocate("svg", sourceID+":svg")
	pngPath := ctx.media.Allocate("png", sourceID+":png")
	ctx.usedExtensions["svg"] = true
	ctx.usedExtensions["png"] = true
	return pptx.MediaFilename(svgPath), pptx.MediaFilename(pngPath)
}

// allocPNG allocates a single PNG media filename slot via the MediaAllocator.
// Returns the basename like "image5.png".
func (ctx *singlePassContext) allocPNG(sourceID string) string {
	mediaPath := ctx.media.AllocatePNG(sourceID)
	ctx.usedExtensions["png"] = true
	return pptx.MediaFilename(mediaPath)
}

// nextMediaNum returns the next image number that would be allocated.
func (ctx *singlePassContext) nextMediaNum() int {
	return ctx.media.NextImageNum()
}

// allocPNGForFile allocates a media filename for an image file path, with deduplication.
// If the same file path was already allocated, returns the existing filename.
// Returns the basename like "image5.png".
func (ctx *singlePassContext) allocPNGForFile(imagePath string) string {
	if existing, present := ctx.mediaFiles[imagePath]; present {
		return existing
	}
	ext := filepath.Ext(imagePath)
	if ext == "" {
		ext = ".png"
	}
	mediaPath := ctx.media.Allocate(strings.TrimPrefix(ext, "."), "file:"+imagePath)
	mediaFileName := pptx.MediaFilename(mediaPath)
	ctx.mediaFiles[imagePath] = mediaFileName

	extLower := strings.TrimPrefix(strings.ToLower(ext), ".")
	ctx.usedExtensions[extLower] = true
	return mediaFileName
}

// getPlaceholderBounds extracts bounds from shape transform or uses explicit bounds.
func getPlaceholderBounds(shape *shapeXML, explicitBounds *types.BoundingBox) types.BoundingBox {
	var placeholderBounds types.BoundingBox
	if shape.ShapeProperties.Transform != nil {
		placeholderBounds = types.BoundingBox{
			X:      shape.ShapeProperties.Transform.Offset.X,
			Y:      shape.ShapeProperties.Transform.Offset.Y,
			Width:  shape.ShapeProperties.Transform.Extent.CX,
			Height: shape.ShapeProperties.Transform.Extent.CY,
		}
	}
	if explicitBounds != nil {
		placeholderBounds = *explicitBounds
	}
	return placeholderBounds
}
