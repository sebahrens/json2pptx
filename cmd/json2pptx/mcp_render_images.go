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

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
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
		mcp.Description("Legacy opt-in. Default false: each rendered slide is returned as a native MCP image content block (JPEG, max 1280px wide) that you can look at directly, plus a small JSON metadata block (no base64). Set true only for clients that cannot display MCP images — the response then carries png_base64 (or path) inside the JSON text instead."),
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
	// Path is a content-addressed PNG artifact on disk (full resolution). It
	// is always populated in image_content mode so the image can be handed to
	// inspect_slide_images or opened locally.
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
		return meta, api.MCPImage{}, fmt.Errorf("slide %d: rendered image bytes unavailable", img.Index)
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
		return meta, api.MCPImage{}, fmt.Errorf("slide %d: %w", img.Index, err)
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
	res, err := api.MCPImageResult(ctx, renderedSlideImageResponse{renderedSlideMeta: meta, Delivery: deliveryImageContent}, []api.MCPImage{enc})
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
	if wantsBase64JSON(request) {
		res, err := api.MCPSuccessResult(ctx, deck)
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
		resp.Slides = append(resp.Slides, meta)
		images = append(images, enc)
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
