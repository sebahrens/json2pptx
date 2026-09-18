package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder for image.Decode
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	xdraw "golang.org/x/image/draw"
)

// MCPImage is one image to deliver as a native MCP ImageContent block.
type MCPImage struct {
	// MIMEType is the image media type (e.g. "image/jpeg", "image/png").
	MIMEType string
	// Data is the raw (not base64) image bytes.
	Data []byte
}

// Defaults for EncodeImageForMCP: large enough for a vision model to read
// slide text, small enough that a 10-slide deck stays well under typical MCP
// client message budgets.
const (
	MCPImageMaxWidth    = 1280
	MCPImageJPEGQuality = 80
)

// EncodeImageForMCP decodes a raster image (PNG or JPEG), downscales it so its
// width is at most maxWidth pixels (aspect ratio preserved; never upscaled) and
// re-encodes it as JPEG at the given quality. It returns the JPEG bytes and the
// output dimensions.
func EncodeImageForMCP(data []byte, maxWidth, quality int) (MCPImage, int, int, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return MCPImage{}, 0, 0, fmt.Errorf("decode image: %w", err)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return MCPImage{}, 0, 0, fmt.Errorf("image has empty bounds")
	}
	out := src
	if maxWidth > 0 && w > maxWidth {
		nh := h * maxWidth / w
		if nh < 1 {
			nh = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, maxWidth, nh))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
		out = dst
		w, h = maxWidth, nh
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: quality}); err != nil {
		return MCPImage{}, 0, 0, fmt.Errorf("encode jpeg: %w", err)
	}
	return MCPImage{MIMEType: "image/jpeg", Data: buf.Bytes()}, w, h, nil
}

// MCPImageResult builds a CallToolResult that delivers images as native MCP
// ImageContent blocks so the client model can actually see them, instead of
// burying base64 inside a JSON text payload the model cannot view.
//
// Content layout: Content[0] is a TextContent carrying the JSON-encoded meta
// (which must NOT contain base64 image data), followed by one ImageContent
// per image in order. StructuredContent is set to meta.
func MCPImageResult(ctx context.Context, meta any, images []MCPImage) (*mcp.CallToolResult, error) {
	textJSON, err := MarshalMCPResponse(ctx, meta)
	if err != nil {
		return nil, err
	}
	content := make([]mcp.Content, 0, 1+len(images))
	content = append(content, mcp.TextContent{Type: "text", Text: string(textJSON)})
	for _, img := range images {
		mime := img.MIMEType
		if mime == "" {
			mime = http.DetectContentType(img.Data)
		}
		content = append(content, mcp.NewImageContent(base64.StdEncoding.EncodeToString(img.Data), mime))
	}
	return &mcp.CallToolResult{
		Content:           content,
		StructuredContent: meta,
	}, nil
}
