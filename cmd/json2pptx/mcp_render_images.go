// mcp_render_images.go delivers rendered slide PNGs as native MCP ImageContent
// blocks (go-slide-creator-cn3h). Before this, render_slide_image /
// render_deck_thumbnails / render_slide_image_from_json returned base64 PNGs
// inside a JSON TextContent payload — tens of thousands of tokens the client
// model could not actually look at. The default now emits one downscaled JPEG
// ImageContent per slide plus a small JSON metadata TextContent without any
// base64. The legacy base64-in-JSON envelope stays available via
// include_base64_json=true (the CLI render subcommands always use it).
package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png" // register PNG decoder for image.DecodeConfig
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/render"
)

// argIncludeBase64JSON is the opt-in flag that restores the legacy
// base64-PNG-in-JSON response envelope for clients that cannot consume MCP
// ImageContent.
const argIncludeBase64JSON = "include_base64_json"

// deliveryImageContent marks a response whose pixels travel as MCP ImageContent.
const deliveryImageContent = "image_content"

// includeBase64JSONOption is the shared tool-parameter definition.
func includeBase64JSONOption() mcp.ToolOption {
	return mcp.WithBoolean(argIncludeBase64JSON,
		mcp.Description("Legacy: true returns png_base64 (or path) inside the JSON instead of MCP image blocks, for clients that cannot display images."),
	)
}

// wantsBase64JSON reports whether the caller opted into the legacy envelope.
func wantsBase64JSON(request mcp.CallToolRequest) bool {
	v, ok := request.GetArguments()[argIncludeBase64JSON].(bool)
	return ok && v
}

// renderedSlideMeta is the per-slide metadata emitted alongside an
// ImageContent block. It never carries base64 pixel data.
type renderedSlideMeta struct {
	Index int `json:"index"`
	// ID is the slide's stable DeckSpec id, when the deck's ids are known for
	// this file (go-slide-creator-1w3uo).
	ID string `json:"id,omitempty"`
	// Path is a content-addressed PNG artifact on disk (full resolution). It
	// is always populated in image_content mode so the image can be handed to
	// inspect_slide_images or opened locally. The image block delivered
	// alongside is a downscaled JPEG (ImageMIMEType), not this file.
	Path        string `json:"path,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	SizeError   string `json:"size_error,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
	SourceHash  string `json:"source_hash,omitempty"`
	Cleanup     string `json:"cleanup,omitempty"`
	// ImageContentIndex is the position of this slide's ImageContent block in
	// the tool result's content array (content[0] is this JSON metadata).
	ImageContentIndex int    `json:"image_content_index,omitempty"`
	ImageMIMEType     string `json:"image_mime_type,omitempty"`
	ImageWidth        int    `json:"image_width,omitempty"`
	ImageHeight       int    `json:"image_height,omitempty"`
	// Unchanged marks a slide whose content_hash the caller named in
	// known_hashes: the entry is index, id and content_hash, with no image
	// block and no path (go-slide-creator-yosa8).
	Unchanged bool `json:"unchanged,omitempty"`
}

// renderedSlideImageResponse is the image_content-mode response for
// render_slide_image and render_slide_image_from_json.
type renderedSlideImageResponse struct {
	renderedSlideMeta
	Delivery string `json:"delivery"`
}

// renderedDeckThumbnailsResponse is the image_content-mode response for
// render_deck_thumbnails.
// Deck-wide values (source_hash, cleanup) are hoisted to the top level instead
// of being repeated per slide, keeping the metadata small for large decks.
type renderedDeckThumbnailsResponse struct {
	Slides    []renderedSlideMeta `json:"slides"`
	Truncated bool                `json:"truncated"`
	Delivery  string              `json:"delivery"`
	// SlideCount is the deck's slide count, whatever came back. With
	// slide_indices the two differ, and "2 images" says nothing on its own.
	SlideCount int `json:"slide_count,omitempty"`
	// Selected echoes the 0-based indices a slide_indices render returned,
	// ascending. Absent on a full-deck render.
	Selected   []int  `json:"selected,omitempty"`
	SourceHash string `json:"source_hash,omitempty"`
	Cleanup    string `json:"cleanup,omitempty"`
	// ImageMIMEType is the encoding of every image block (image/jpeg),
	// hoisted from slides[]. slides[].path stays the full-resolution PNG.
	ImageMIMEType string `json:"image_mime_type,omitempty"`
	// NextToolCall points at submit_visual_review bound to this exact PPTX
	// revision, the step that closes the render loop (go-slide-creator-z3pbp).
	NextToolCall *patterns.ToolCallSuggestion `json:"next_tool_call,omitempty"`
	// LargerRender is the call that renders one slide larger, offered when
	// the image blocks are below the delivery width: at the default density a
	// slide is 667px wide and 12pt body text cannot be judged
	// (go-slide-creator-jn6vj).
	LargerRender *largerRenderCall `json:"larger_render,omitempty"`
}

// largerRenderCall says when a slide should be rendered again larger and how:
// render_deck_thumbnails' own slide_indices and density.
type largerRenderCall struct {
	When string `json:"when"`
	patterns.ToolCallSuggestion
}

// largerRenderDensity is the DPI at which a 16:9 slide fills the delivered
// image width (api.MCPImageMaxWidth, 1280px = 96 DPI): about twice the linear
// size of the default pass. A higher density adds bytes on disk, not pixels in
// the image block.
const largerRenderDensity = 100

// largerRenderFor is the larger_render block of a thumbnails response whose
// image blocks came out narrower than the delivery width; nil when they are
// already as large as an image block gets, or nothing was delivered.
func largerRenderFor(pptxPath string, slides []renderedSlideMeta) *largerRenderCall {
	narrow := false
	for _, s := range slides {
		if s.ImageWidth > 0 && s.ImageWidth < api.MCPImageMaxWidth {
			narrow = true
			break
		}
	}
	if !narrow || pptxPath == "" {
		return nil
	}
	return &largerRenderCall{
		When: "text of 12pt or less cannot be read at this size: render that slide larger before judging it",
		ToolCallSuggestion: patterns.ToolCallSuggestion{Tool: "render_deck_thumbnails", ArgsTemplate: map[string]any{
			"pptx_path":     pptxPath,
			"slide_indices": "<[index or id] of the slide to read>",
			"density":       largerRenderDensity,
		}},
	}
}

// slideImageToMCP converts one rendered SlideImage into its metadata record and
// a downscaled JPEG MCPImage. contentIndex is the ImageContent position the
// image will occupy in the result.
func slideImageToMCP(img render.SlideImage, contentIndex int) (renderedSlideMeta, api.MCPImage, error) {
	meta := renderedSlideMeta{
		Index:       img.Index,
		Width:       img.Width,
		Height:      img.Height,
		SizeError:   img.SizeErr,
		ContentHash: img.ContentHash,
		SourceHash:  img.SourceHash,
	}
	data, path := materializeThumbnail(img)
	if len(data) == 0 {
		return meta, api.MCPImage{}, fmt.Errorf("slide index %d: rendered image bytes unavailable", img.Index)
	}
	meta.Path = path
	if path != "" {
		meta.Cleanup = render.ArtifactCleanupPolicy
	}
	if meta.Width == 0 || meta.Height == 0 {
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			meta.Width, meta.Height = cfg.Width, cfg.Height
		}
	}
	enc, w, h, err := api.EncodeImageForMCP(data, api.MCPImageMaxWidth, api.MCPImageJPEGQuality)
	if err != nil {
		return meta, api.MCPImage{}, fmt.Errorf("slide index %d: %w", img.Index, err)
	}
	meta.ImageContentIndex = contentIndex
	meta.ImageMIMEType = enc.MIMEType
	meta.ImageWidth = w
	meta.ImageHeight = h
	return meta, enc, nil
}

// slideImageMCPResult builds the result for a single rendered slide, honoring
// the include_base64_json legacy opt-in.
func slideImageMCPResult(ctx context.Context, request mcp.CallToolRequest, img *render.SlideImage) *mcp.CallToolResult {
	if err := ctx.Err(); err != nil {
		return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error())
	}
	if wantsBase64JSON(request) {
		res, err := api.MCPSuccessResult(ctx, img)
		if err != nil {
			return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err))
		}
		if err := ctx.Err(); err != nil {
			return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error())
		}
		return res
	}
	meta, enc, err := slideImageToMCP(*img, 1)
	if err != nil {
		return api.MCPSimpleError("RENDER_FAILED", err.Error())
	}
	one := []renderedSlideMeta{meta}
	stampSlideIDs(request.GetString("pptx_path", ""), one)
	res, err := api.MCPImageResult(ctx, renderedSlideImageResponse{renderedSlideMeta: one[0], Delivery: deliveryImageContent}, []api.MCPImage{enc})
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err))
	}
	if err := ctx.Err(); err != nil {
		return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error())
	}
	return res
}

// deckThumbnailsMCPResult builds the result for a rendered deck, honoring the
// include_base64_json legacy opt-in.
func deckThumbnailsMCPResult(ctx context.Context, request mcp.CallToolRequest, deck *render.DeckResult) *mcp.CallToolResult {
	if err := ctx.Err(); err != nil {
		return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error())
	}
	known, bad := knownHashesArg(request)
	if bad != nil {
		return bad
	}
	held := func(s render.SlideImage) bool { return s.ContentHash != "" && known[s.ContentHash] }
	recordDeckThumbnails(request, deck)
	if wantsBase64JSON(request) {
		var payload any = deck
		if len(known) > 0 {
			payload = legacyDeckWithoutKnown(deck, held)
		}
		res, err := api.MCPSuccessResult(ctx, payload)
		if err != nil {
			return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err))
		}
		if err := ctx.Err(); err != nil {
			return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error())
		}
		return res
	}
	resp := renderedDeckThumbnailsResponse{
		Slides:     make([]renderedSlideMeta, 0, len(deck.Slides)),
		Truncated:  deck.Truncated,
		Delivery:   deliveryImageContent,
		SlideCount: deck.SlideCount,
		Selected:   deck.Selected,
	}
	images := make([]api.MCPImage, 0, len(deck.Slides))
	for _, s := range deck.Slides {
		if err := ctx.Err(); err != nil {
			return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error())
		}
		if held(s) {
			// The caller holds this image: the hash says so, nothing is sent.
			if resp.SourceHash == "" {
				resp.SourceHash = s.SourceHash
			}
			resp.Slides = append(resp.Slides, renderedSlideMeta{Index: s.Index, ContentHash: s.ContentHash, Unchanged: true})
			continue
		}
		meta, enc, err := slideImageToMCP(s, len(images)+1)
		if err != nil {
			return api.MCPSimpleError("RENDER_FAILED", err.Error())
		}
		if resp.SourceHash == "" {
			resp.SourceHash = meta.SourceHash
		}
		if meta.SourceHash == resp.SourceHash {
			meta.SourceHash = ""
		}
		if meta.Cleanup != "" {
			resp.Cleanup = meta.Cleanup
			meta.Cleanup = ""
		}
		// Every block is encoded the same way, so the MIME type is stated once
		// at the top level rather than per slide beside a .png path, which read
		// as a mismatch (go-slide-creator-ppned).
		if resp.ImageMIMEType == "" {
			resp.ImageMIMEType = meta.ImageMIMEType
		}
		if meta.ImageMIMEType == resp.ImageMIMEType {
			meta.ImageMIMEType = ""
		}
		resp.Slides = append(resp.Slides, meta)
		images = append(images, enc)
	}
	if pptxPath := request.GetString("pptx_path", ""); pptxPath != "" {
		stampSlideIDs(pptxPath, resp.Slides)
		if artifact, aerr := describeArtifact(pptxPath, "pptx"); aerr == nil {
			resp.NextToolCall = nextCallSubmitVisualReview(pptxPath, artifact.SHA256)
		}
		resp.LargerRender = largerRenderFor(pptxPath, resp.Slides)
	}
	res, err := api.MCPImageResult(ctx, resp, images)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err))
	}
	if err := ctx.Err(); err != nil {
		return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error())
	}
	return res
}

// renderProgressReporter sends only when the client supplied an MCP progress
// token. The initial update covers conversion, then the render loop reports
// each assembled thumbnail against the actual number selected for delivery.
func (mc *mcpConfig) renderProgressReporter(ctx context.Context, request mcp.CallToolRequest) func(done, total int) {
	if request.Params.Meta == nil || request.Params.Meta.ProgressToken == nil || mc.progressSender == nil {
		return nil
	}
	token := request.Params.Meta.ProgressToken
	_ = mc.progressSender(ctx, map[string]any{
		"progressToken": token,
		"progress":      0,
		"message":       "Converting or loading slide images",
	})
	return func(done, total int) {
		_ = mc.progressSender(ctx, map[string]any{
			"progressToken": token,
			"progress":      done,
			"total":         total,
			"message":       fmt.Sprintf("Prepared thumbnail %d of %d", done, total),
		})
	}
}

// legacyDeckThumbnails is the include_base64_json envelope of a render that
// named known_hashes: render.DeckResult with a held slide reduced to its
// index and hashes.
type legacyDeckThumbnails struct {
	Slides     []legacySlideThumbnail `json:"slides"`
	Truncated  bool                   `json:"truncated"`
	SlideCount int                    `json:"slide_count,omitempty"`
	Selected   []int                  `json:"selected,omitempty"`
}

type legacySlideThumbnail struct {
	render.SlideImage
	Unchanged bool `json:"unchanged,omitempty"`
}

func legacyDeckWithoutKnown(deck *render.DeckResult, held func(render.SlideImage) bool) legacyDeckThumbnails {
	out := legacyDeckThumbnails{Truncated: deck.Truncated, SlideCount: deck.SlideCount, Selected: deck.Selected,
		Slides: make([]legacySlideThumbnail, 0, len(deck.Slides))}
	for _, s := range deck.Slides {
		if held(s) {
			s = render.SlideImage{Index: s.Index, ContentHash: s.ContentHash, SourceHash: s.SourceHash}
			out.Slides = append(out.Slides, legacySlideThumbnail{SlideImage: s, Unchanged: true})
			continue
		}
		out.Slides = append(out.Slides, legacySlideThumbnail{SlideImage: s})
	}
	return out
}

// argKnownHashes names the content hashes the caller already holds.
const argKnownHashes = "known_hashes"

// knownHashesArg decodes render_deck_thumbnails' known_hashes: the
// content_hash values of slides the caller has already looked at. A final
// full-deck pass after a one-slide repair resent every image (232 KB with 6 of
// 7 hashes unchanged, go-slide-creator-yosa8); with the hashes named, the
// unchanged slides come back as hash-only entries. Hashes are compared without
// case or a "sha256:" prefix, and one that matches nothing is ignored: it is
// a slide that changed.
func knownHashesArg(request mcp.CallToolRequest) (map[string]bool, *mcp.CallToolResult) {
	raw, ok := request.GetArguments()[argKnownHashes]
	if !ok || raw == nil {
		return nil, nil
	}
	bad := func(msg string) (map[string]bool, *mcp.CallToolResult) {
		return nil, argInvalidValue("render_deck_thumbnails", diagnostics.CodeInvalidParameter, argKnownHashes,
			msg, "array", []string{"<slides[].content_hash of an earlier render>"}, nil)
	}
	var list []any
	switch v := raw.(type) {
	case []any:
		list = v
	case []string:
		for _, h := range v {
			list = append(list, h)
		}
	default:
		return bad("known_hashes must be an array of content_hash strings")
	}
	known := make(map[string]bool, len(list))
	for _, item := range list {
		h, ok := item.(string)
		if !ok {
			return bad("known_hashes must contain only content_hash strings")
		}
		if h = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(h)), "sha256:"); h != "" {
			known[h] = true
		}
	}
	return known, nil
}

// slideIndicesArg decodes render_deck_thumbnails' slide_indices, sharing
// score_deck's parser so the two tools read the same argument the same way. An
// empty array is refused rather than treated as "the whole deck": a caller that
// narrowed to nothing wants to be told, not handed fifteen images
// (go-slide-creator-2018).
func slideIndicesArg(request mcp.CallToolRequest) (indices []int, present bool, errResult *mcp.CallToolResult) {
	parsed, ok, err := parseSlideIndices(request)
	if err != nil {
		return nil, true, argInvalidValue("render_deck_thumbnails", diagnostics.CodeInvalidParameter, "slide_indices",
			err.Error(), "array", []int{4, 9}, nil)
	}
	if !ok {
		return nil, false, nil
	}
	if len(parsed) == 0 {
		return nil, true, argInvalidValue("render_deck_thumbnails", diagnostics.CodeInvalidParameter, "slide_indices",
			"slide_indices is empty: name the slides to render, or omit it to render the whole deck", "array", []int{4, 9}, nil)
	}
	return parsed, true, nil
}
