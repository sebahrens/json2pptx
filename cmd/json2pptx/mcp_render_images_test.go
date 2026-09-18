package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/template"
)

// syntheticSlidePNG returns a noisy slide-sized PNG so the encoded payloads
// are representative (flat images compress unrealistically well).
func syntheticSlidePNG(t *testing.T, w, h int, seed uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x*7) ^ seed, uint8(y*13) + seed, uint8((x + y) * 3), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// assertImageContentDeck checks the go-slide-creator-cn3h contract: N native
// image/jpeg ImageContent blocks and a JSON metadata text payload < 5KB that
// carries no base64 pixels.
func assertImageContentDeck(t *testing.T, res *mcp.CallToolResult, wantSlides int) renderedDeckThumbnailsResponse {
	t.Helper()
	if res == nil || res.IsError {
		t.Fatalf("unexpected error result: %s", textContent(res))
	}
	var images int
	for i, c := range res.Content {
		ic, ok := c.(mcp.ImageContent)
		if !ok {
			continue
		}
		images++
		if ic.Type != "image" || ic.MIMEType != "image/jpeg" {
			t.Errorf("content[%d]: type=%q mime=%q, want image image/jpeg", i, ic.Type, ic.MIMEType)
		}
		if _, err := base64.StdEncoding.DecodeString(ic.Data); err != nil {
			t.Errorf("content[%d]: data is not base64: %v", i, err)
		}
	}
	if images != wantSlides {
		t.Fatalf("got %d ImageContent blocks, want %d", images, wantSlides)
	}
	text := textContent(res)
	if len(text) >= 5*1024 {
		t.Fatalf("text payload is %d bytes, want < 5KB", len(text))
	}
	if strings.Contains(text, "png_base64") {
		t.Fatal("text payload must not carry png_base64 in image_content mode")
	}
	var resp renderedDeckThumbnailsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("metadata is not JSON: %v", err)
	}
	if resp.Delivery != deliveryImageContent {
		t.Errorf("delivery = %q, want %q", resp.Delivery, deliveryImageContent)
	}
	for i, s := range resp.Slides {
		if s.ImageContentIndex != i+1 {
			t.Errorf("slides[%d].image_content_index = %d, want %d", i, s.ImageContentIndex, i+1)
		}
		if s.Path == "" {
			t.Errorf("slides[%d].path empty — image_content mode must always materialize a path", i)
		}
		if _, ok := res.Content[s.ImageContentIndex].(mcp.ImageContent); !ok {
			t.Errorf("slides[%d].image_content_index does not point at an ImageContent", i)
		}
	}
	return resp
}

func TestDeckThumbnailsMCPResult_ImageContentDefault(t *testing.T) {
	deck := &render.DeckResult{}
	for i := 0; i < 6; i++ {
		img, err := render.SlideImageFromBytes(i, syntheticSlidePNG(t, 667, 375, uint8(i)), "src")
		if err != nil {
			t.Fatal(err)
		}
		deck.Slides = append(deck.Slides, *img)
	}
	res := deckThumbnailsMCPResult(context.Background(), makeRequest(map[string]any{}), deck)
	resp := assertImageContentDeck(t, res, 6)
	if resp.Slides[0].Width != 667 || resp.Slides[0].ImageWidth != 667 {
		t.Errorf("dims: width=%d image_width=%d, want 667/667", resp.Slides[0].Width, resp.Slides[0].ImageWidth)
	}
}

func TestDeckThumbnailsMCPResult_LegacyBase64JSON(t *testing.T) {
	img, err := render.SlideImageFromBytes(0, syntheticSlidePNG(t, 64, 36, 1), "src")
	if err != nil {
		t.Fatal(err)
	}
	deck := &render.DeckResult{Slides: []render.SlideImage{*img}}
	res := deckThumbnailsMCPResult(context.Background(), makeRequest(map[string]any{argIncludeBase64JSON: true}), deck)
	if res.IsError || len(res.Content) != 1 {
		t.Fatalf("legacy mode: want a single text block, got %d blocks (err=%v)", len(res.Content), res.IsError)
	}
	if !strings.Contains(textContent(res), "png_base64") {
		t.Fatal("legacy mode must keep png_base64 in the JSON text")
	}
}

func TestSlideImageMCPResult_DownscalesLargeSlide(t *testing.T) {
	img, err := render.SlideImageFromBytes(3, syntheticSlidePNG(t, 2000, 1125, 9), "src")
	if err != nil {
		t.Fatal(err)
	}
	res := slideImageMCPResult(context.Background(), makeRequest(nil), img)
	if res.IsError || len(res.Content) != 2 {
		t.Fatalf("want text + 1 image, got %d blocks", len(res.Content))
	}
	var meta renderedSlideImageResponse
	if err := json.Unmarshal([]byte(textContent(res)), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Index != 3 || meta.ImageWidth != 1280 || meta.ImageHeight != 720 || meta.Width != 2000 {
		t.Fatalf("meta = %+v", meta)
	}
	if meta.Delivery != deliveryImageContent || meta.ImageMIMEType != "image/jpeg" {
		t.Fatalf("meta delivery/mime = %q/%q", meta.Delivery, meta.ImageMIMEType)
	}
}

func TestSlideImageMCPResult_UnreadableImage(t *testing.T) {
	res := slideImageMCPResult(context.Background(), makeRequest(nil), &render.SlideImage{Index: 0})
	if !res.IsError {
		t.Fatal("expected error for a SlideImage with no bytes")
	}
}

// TestRenderDeckThumbnails_ImageContentIntegration exercises the real
// LibreOffice render path end-to-end when the toolchain is installed.
func TestRenderDeckThumbnails_ImageContentIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("render integration skipped in -short mode")
	}
	if ok, _ := render.DependencyStatus(); !ok {
		t.Skip("LibreOffice/ImageMagick not installed")
	}
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	deckJSON := `{"template":"midnight-blue","slides":[
	  {"slide_type":"title","content":[{"placeholder_id":"title","type":"text","text_value":"Image content"}]},
	  {"slide_type":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Second"},{"placeholder_id":"body","type":"bullets","bullets_value":["one","two"]}]}]}`
	gen, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": mustParseJSON(deckJSON)}))
	if err != nil || gen.IsError {
		t.Fatalf("generate failed: %v %s", err, textContent(gen))
	}
	var out JSONOutput
	if err := json.Unmarshal([]byte(textContent(gen)), &out); err != nil {
		t.Fatal(err)
	}
	res, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{"pptx_path": out.OutputPath}))
	if err != nil {
		t.Fatal(err)
	}
	assertImageContentDeck(t, res, 2)
}
