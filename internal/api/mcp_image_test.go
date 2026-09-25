package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestEncodeImageForMCP_DownscalesToJPEG(t *testing.T) {
	enc, w, h, err := EncodeImageForMCP(testPNG(t, 2000, 1125), MCPImageMaxWidth, MCPImageJPEGQuality)
	if err != nil {
		t.Fatal(err)
	}
	if enc.MIMEType != "image/jpeg" {
		t.Fatalf("mime = %q, want image/jpeg", enc.MIMEType)
	}
	if w != 1280 || h != 720 {
		t.Fatalf("dims = %dx%d, want 1280x720", w, h)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(enc.Data))
	if err != nil {
		t.Fatalf("output is not a JPEG: %v", err)
	}
	if cfg.Width != 1280 || cfg.Height != 720 {
		t.Fatalf("decoded dims = %dx%d", cfg.Width, cfg.Height)
	}
}

func TestEncodeImageForMCP_NoUpscale(t *testing.T) {
	_, w, h, err := EncodeImageForMCP(testPNG(t, 640, 360), MCPImageMaxWidth, MCPImageJPEGQuality)
	if err != nil {
		t.Fatal(err)
	}
	if w != 640 || h != 360 {
		t.Fatalf("dims = %dx%d, want unchanged 640x360", w, h)
	}
}

func TestEncodeImageForMCP_RejectsGarbage(t *testing.T) {
	if _, _, _, err := EncodeImageForMCP([]byte("not an image"), 100, 80); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestMCPImageResult_Layout(t *testing.T) {
	meta := map[string]any{"slides": []int{0, 1}}
	imgs := []MCPImage{
		{MIMEType: "image/jpeg", Data: []byte{0xff, 0xd8, 0xff}},
		{Data: testPNG(t, 4, 4)}, // MIME sniffed
	}
	res, err := MCPImageResult(context.Background(), meta, imgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) != 3 {
		t.Fatalf("content len = %d, want 3", len(res.Content))
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok || !strings.Contains(tc.Text, "slides") {
		t.Fatalf("content[0] = %#v, want JSON metadata text", res.Content[0])
	}
	ic, ok := res.Content[1].(mcp.ImageContent)
	if !ok || ic.Type != "image" || ic.MIMEType != "image/jpeg" {
		t.Fatalf("content[1] = %#v", res.Content[1])
	}
	if got, _ := base64.StdEncoding.DecodeString(ic.Data); !bytes.Equal(got, imgs[0].Data) {
		t.Fatal("content[1] data does not round-trip")
	}
	ic2 := res.Content[2].(mcp.ImageContent)
	if ic2.MIMEType != "image/png" {
		t.Fatalf("sniffed mime = %q, want image/png", ic2.MIMEType)
	}
	if res.StructuredContent == nil {
		t.Fatal("StructuredContent must carry meta")
	}
}

func TestMCPImageResult_ModernSummaryDoesNotDuplicateImage(t *testing.T) {
	withMode(t, TextFallbackAuto)
	ctx := ctxForSession("modern-image")
	RecordProtocolVersion(ctx, "2025-06-18")
	t.Cleanup(func() { ForgetProtocolVersion(ctx) })
	img := MCPImage{MIMEType: "image/png", Data: testPNG(t, 4, 4)}
	res, err := MCPImageResult(ctx, map[string]any{"ok": true, "summary": "one slide", "payload": strings.Repeat("x", 3000)}, []MCPImage{img})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) != 2 {
		t.Fatalf("content blocks = %d, want one summary and one image", len(res.Content))
	}
	text := res.Content[0].(mcp.TextContent).Text
	if len(text) > maxMCPTextSummaryBytes || !strings.Contains(text, "one slide") || strings.Contains(text, strings.Repeat("x", 100)) {
		t.Errorf("image metadata synopsis = %q", text)
	}
	if content, ok := res.Content[1].(mcp.ImageContent); !ok || content.MIMEType != "image/png" || content.Data != base64.StdEncoding.EncodeToString(img.Data) {
		t.Errorf("image block = %#v, want exactly one native image", res.Content[1])
	}
}
