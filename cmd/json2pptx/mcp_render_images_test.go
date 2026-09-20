package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
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

func TestRenderImageHandlersRespectCancelledContext(t *testing.T) {
	pptxPath := filepath.Join(t.TempDir(), "deck.pptx")
	if err := os.WriteFile(pptxPath, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mc := cliMCPConfig("./templates", "./out")
	for _, tc := range []struct {
		name   string
		handle func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{"slide", mc.handleRenderSlideImage},
		{"deck", mc.handleRenderDeckThumbnails},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.handle(ctx, makeRequest(map[string]any{"pptx_path": pptxPath}))
			if err != nil || result == nil || !result.IsError {
				t.Fatalf("result = %+v, err = %v", result, err)
			}
			if !strings.Contains(textContent(result), "CANCELLED") {
				t.Fatalf("missing cancellation code: %s", textContent(result))
			}
			for _, item := range result.Content {
				if _, ok := item.(mcp.ImageContent); ok {
					t.Fatal("cancelled call returned image content")
				}
			}
		})
	}
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
	var progress []map[string]any
	mc.progressSender = func(_ context.Context, params map[string]any) error {
		progress = append(progress, params)
		return nil
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
	res, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{"pptx_path": out.OutputPath, "force": true}))
	if err != nil {
		t.Fatal(err)
	}
	assertImageContentDeck(t, res, 2)
	if len(progress) != 0 {
		t.Fatalf("call without progress token emitted %d notifications", len(progress))
	}
	req := makeRequest(map[string]any{"pptx_path": out.OutputPath})
	req.Params.Meta = &mcp.Meta{ProgressToken: "render-1"}
	res, err = mc.handleRenderDeckThumbnails(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	assertImageContentDeck(t, res, 2)
	if len(progress) != 3 {
		t.Fatalf("progress notifications = %+v, want conversion + 2 slides", progress)
	}
	for i, got := range progress {
		if got["progressToken"] != "render-1" || got["progress"] != i {
			t.Errorf("progress[%d] = %+v", i, got)
		}
		if i > 0 && got["total"] != 2 {
			t.Errorf("progress[%d] total = %v, want 2", i, got["total"])
		}
	}
}

// TestSlideIndicesArg covers the argument's own contract before any rendering:
// what is accepted, what is refused, and that refusals name the argument.
func TestSlideIndicesArg(t *testing.T) {
	cases := []struct {
		name    string
		args    map[string]any
		want    []int
		present bool
		errText string
	}{
		{name: "absent", args: map[string]any{}, present: false},
		{name: "null", args: map[string]any{"slide_indices": nil}, present: false},
		{name: "one slide", args: map[string]any{"slide_indices": []any{4}}, want: []int{4}, present: true},
		{name: "sorted and deduped", args: map[string]any{"slide_indices": []any{9.0, 4.0, 9.0}}, want: []int{4, 9}, present: true},
		{name: "empty", args: map[string]any{"slide_indices": []any{}}, present: true, errText: "slide_indices is empty"},
		{name: "not an array", args: map[string]any{"slide_indices": 4}, present: true, errText: "array of integers"},
		{name: "fractional", args: map[string]any{"slide_indices": []any{4.5}}, present: true, errText: "must contain integers"},
		// A client that stringifies numbers is taken at its word; a string that
		// is not a number is not. Same leniency as score_deck, which shares the
		// parser.
		{name: "a numeric string", args: map[string]any{"slide_indices": []any{"4"}}, want: []int{4}, present: true},
		{name: "a non-numeric string", args: map[string]any{"slide_indices": []any{"four"}}, present: true, errText: "array of integers"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, present, errRes := slideIndicesArg(makeRequest(tc.args))
			if present != tc.present {
				t.Errorf("present = %v, want %v", present, tc.present)
			}
			if tc.errText != "" {
				if errRes == nil {
					t.Fatalf("want an error mentioning %q, got indices %v", tc.errText, got)
				}
				if text := resultText(errRes); !strings.Contains(text, tc.errText) {
					t.Errorf("error should mention %q, got:\n%s", tc.errText, text)
				}
				return
			}
			if errRes != nil {
				t.Fatalf("unexpected error: %s", resultText(errRes))
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("indices = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRenderDeckThumbnails_SlideIndicesRejectsMaxSlides pins the exclusivity:
// one argument names slides and the other caps a prefix, and together they do
// not say what the caller wants.
func TestRenderDeckThumbnails_SlideIndicesRejectsMaxSlides(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()}
	pptx := filepath.Join(t.TempDir(), "deck.pptx")
	if err := os.WriteFile(pptx, []byte("not really a deck"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{
		"pptx_path":     pptx,
		"slide_indices": []any{1},
		"max_slides":    3,
	}))
	if err != nil {
		t.Fatalf("go error: %v", err)
	}
	if !res.IsError {
		t.Fatal("slide_indices + max_slides must be refused")
	}
	if text := resultText(res); !strings.Contains(text, "not both") {
		t.Errorf("refusal should say the two are exclusive, got:\n%s", text)
	}
}

// TestRenderDeckThumbnails_SlideIndicesIntegration is the go-slide-creator-2018
// acceptance test: slide_indices:[1,3] on a 5-slide deck returns exactly two
// image blocks for slides 1 and 3, reports the deck's real size, and costs a
// fraction of the full-deck payload. An index the deck does not have is an
// error naming the real count.
func TestRenderDeckThumbnails_SlideIndicesIntegration(t *testing.T) {
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
	slides := make([]any, 0, 5)
	for i := 0; i < 5; i++ {
		slides = append(slides, map[string]any{
			"slide_type": "content",
			"content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": fmt.Sprintf("Slide %d", i)},
				map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": []any{"one", "two"}},
			},
		})
	}
	gen, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": map[string]any{"template": "midnight-blue", "slides": slides},
	}))
	if err != nil || gen.IsError {
		t.Fatalf("generate failed: %v %s", err, textContent(gen))
	}
	var out JSONOutput
	if err := json.Unmarshal([]byte(textContent(gen)), &out); err != nil {
		t.Fatal(err)
	}

	full, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{"pptx_path": out.OutputPath}))
	if err != nil {
		t.Fatal(err)
	}
	fullResp := assertImageContentDeck(t, full, 5)
	if fullResp.SlideCount != 5 {
		t.Errorf("full render slide_count = %d, want 5", fullResp.SlideCount)
	}
	if fullResp.Selected != nil {
		t.Errorf("a full-deck render should not report selected, got %v", fullResp.Selected)
	}

	// force:true drives the cache-miss path, where the PNGs live in a temp
	// directory that must outlive the read: deleting it too early turns every
	// slide into "rendered image bytes unavailable".
	sub, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{
		"pptx_path":     out.OutputPath,
		"slide_indices": []any{3, 1},
		"force":         true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	subResp := assertImageContentDeck(t, sub, 2)
	if got := []int{subResp.Slides[0].Index, subResp.Slides[1].Index}; !slices.Equal(got, []int{1, 3}) {
		t.Errorf("returned slides %v, want [1 3] ascending", got)
	}
	if !slices.Equal(subResp.Selected, []int{1, 3}) {
		t.Errorf("selected = %v, want [1 3]", subResp.Selected)
	}
	if subResp.SlideCount != 5 {
		t.Errorf("subset slide_count = %d, want the deck's 5", subResp.SlideCount)
	}

	// The point of the argument: the payload shrinks with the selection.
	fullBytes, subBytes := contentPayloadBytes(full), contentPayloadBytes(sub)
	if subBytes*2 > fullBytes {
		t.Errorf("subset payload %d bytes vs full %d — narrowing to 2 of 5 slides should cost well under half", subBytes, fullBytes)
	}
	t.Logf("payload: full=%d bytes, slide_indices=[1,3]=%d bytes", fullBytes, subBytes)

	bad, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{
		"pptx_path":     out.OutputPath,
		"slide_indices": []any{1, 9},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !bad.IsError {
		t.Fatal("an index outside the deck must be an error, not a silent omission")
	}
	if text := resultText(bad); !strings.Contains(text, "has 5 slides") {
		t.Errorf("out-of-range error should name the deck's real size, got:\n%s", text)
	}
}

// contentPayloadBytes sums the text and image bytes a tool result carries.
func contentPayloadBytes(res *mcp.CallToolResult) int {
	total := 0
	for _, c := range res.Content {
		switch v := c.(type) {
		case mcp.TextContent:
			total += len(v.Text)
		case mcp.ImageContent:
			total += len(v.Data)
		}
	}
	return total
}
