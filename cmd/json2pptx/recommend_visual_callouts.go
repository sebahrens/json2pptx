package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/policy/placeholder"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// Screenshot-with-callouts recipes (go-slide-creator-n3j96).
//
// image_case takes callouts [{label, x, y, units?}] anchored on its picture,
// but recommend_visual answered "screenshot with callouts" with the plain
// case-study example: a placeholder and no callouts, so the one field the
// intent was about had to be found in the kind's schema. For such an intent
// every candidate that renders as image_case now carries callouts in its
// runnable recipe. Callouts point at a real picture (a placeholder has no
// pixels to point at), so the recipe's image is a small sample screenshot the
// server writes on demand; the contract says to replace it.

// calloutIntentWords mark an intent that asks for labels pointing at parts of
// a picture.
var calloutIntentWords = []string{"callout", "annotat", "pointing at", "point at", "points at", "labelled screenshot", "labeled screenshot"}

// calloutIntent reports whether the recommendation query asks for callouts.
func calloutIntent(query string) bool {
	q := strings.ToLower(query)
	for _, w := range calloutIntentWords {
		if strings.Contains(q, w) {
			return true
		}
	}
	return false
}

// sampleScreenshotCallouts are the recipe's callouts, placed on the two
// regions sampleScreenshot draws.
func sampleScreenshotCallouts() []any {
	return []any{
		map[string]any{"label": "Navigation rail", "x": 0.07, "y": 0.37},
		map[string]any{"label": "Delayed queue", "x": 0.84, "y": 0.56},
	}
}

// attachCalloutRecipe turns an image_case recipe into a screenshot with
// callouts. It leaves the candidate alone when it is not an image_case recipe
// or the sample picture cannot be written.
func attachCalloutRecipe(c *patterns.VisualCandidate) {
	if c.DeckSpec == nil || c.DeckSpec.Kind != string(semantic.KindImageCase) || c.NextToolCall == nil {
		return
	}
	spec, _ := c.NextToolCall.ArgsTemplate["spec"].(map[string]any)
	slides, _ := spec["slides"].([]any)
	if len(slides) == 0 {
		return
	}
	slide, _ := slides[0].(map[string]any)
	path, err := sampleScreenshot()
	if slide == nil || err != nil {
		return
	}
	// The whole screenshot is kept (fit contain), so no callout is cropped.
	slide["image"] = map[string]any{"path": path, "alt": "Operations console showing the delayed queue", "fit": "contain"}
	slide["callouts"] = sampleScreenshotCallouts()
	delete(slide, "image_label")
	slide["title"] = placeholder.RecipeActionTitle("screenshot")
	slide["heading"] = "The delay sits in a single queue"
	slide["body"] = "Retries pile up behind one slow downstream call; every other queue is healthy."
	slide["caption"] = "Operations console, 09:40"
	slide["takeaway"] = "The fix is scoped to one queue."
	delete(slide, "eyebrow")
	delete(slide, "bullets")
	delete(slide, "metrics")
	if c.DataContract != nil {
		c.DataContract.Description += " callouts [{label (≤40 chars), x, y, units?}] point at parts of the picture: x / y are fractions of the image (0–1 from its top-left corner) or, with units \"px\", its own pixels. image.path here is a sample screenshot the server wrote — replace it with yours; image.fit \"contain\" keeps every callout target visible."
	}
}

// sampleScreenshotPNG is the sample screenshot's bytes: the picture is drawn
// by this binary, so the file on disk can be checked against it.
var sampleScreenshotPNG = sync.OnceValues(func() ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, drawSampleScreenshot()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
})

// sampleScreenshotPath is where the sample screenshot is written. It lives in
// the temp directory, like the render cache.
func sampleScreenshotPath() string {
	return filepath.Join(os.TempDir(), "json2pptx-samples", "sample-screenshot.png")
}

// sampleScreenshotIntact reports whether path is a regular file holding
// exactly the picture this binary draws: not a link, not another image
// somebody put there.
func sampleScreenshotIntact(path string) bool {
	want, err := sampleScreenshotPNG()
	if err != nil {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(want)) {
		return false
	}
	got, err := os.ReadFile(path) //nolint:gosec // G304: the server's own sample path
	return err == nil && bytes.Equal(got, want)
}

// serverSampleImages lists the pictures the server itself wrote for its
// recipes, as allow-list entries. Under ALLOWED_IMAGE_PATHS the temp directory
// is not an allowed root, so the screenshot-with-callouts recipe was refused
// as it stood (go-slide-creator-bcefj). The entry is the file, not its
// directory, and only while its bytes are the ones this binary draws, so the
// restriction admits nothing an operator did not already ship.
func serverSampleImages() []string {
	path := sampleScreenshotPath()
	if !sampleScreenshotIntact(path) {
		return nil
	}
	out := []string{path}
	// A loader that resolved the temp directory's links (macOS /var →
	// /private/var) must find the same file allowed.
	if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != path {
		out = append(out, resolved)
	}
	return out
}

// sampleScreenshot returns the path of the sample screenshot, writing it on
// first use and again when the file there is not the sample.
func sampleScreenshot() (string, error) {
	path := sampleScreenshotPath()
	dir := filepath.Dir(path)
	if sampleScreenshotIntact(path) {
		return path, nil
	}
	data, err := sampleScreenshotPNG()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: a shared sample, same mode as the render cache
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "sample-*.png")
	if err != nil {
		return "", err
	}
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		// Rename so a concurrent reader never sees a half-written file.
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return path, nil
}

// drawSampleScreenshot draws a 1280x720 mock of an operations console: a
// navigation rail on the left, a header bar, and a table of queues with one
// row marked as delayed. It is a stand-in picture for a recipe, never part of
// a template's look.
func drawSampleScreenshot() image.Image {
	const w, h = 1280, 720
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		draw.Draw(img, image.Rect(x0, y0, x1, y1), image.NewUniform(c), image.Point{}, draw.Src)
	}
	var (
		page   = color.RGBA{0xF4, 0xF5, 0xF7, 0xFF}
		rail   = color.RGBA{0x2B, 0x30, 0x3A, 0xFF}
		railOn = color.RGBA{0x5A, 0x64, 0x78, 0xFF}
		panel  = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
		rule   = color.RGBA{0xDD, 0xE0, 0xE5, 0xFF}
		ink    = color.RGBA{0x9A, 0xA1, 0xAD, 0xFF}
		alert  = color.RGBA{0xF6, 0xD9, 0xD5, 0xFF}
		alertI = color.RGBA{0xC6, 0x4B, 0x3C, 0xFF}
	)
	fill(0, 0, w, h, page)
	// Navigation rail with five entries, the third selected.
	fill(0, 0, 180, h, rail)
	for i := 0; i < 5; i++ {
		c := color.RGBA{0x40, 0x47, 0x55, 0xFF}
		if i == 2 {
			c = railOn
		}
		fill(24, 120+i*64, 156, 120+i*64+36, c)
	}
	// Header bar.
	fill(180, 0, w, 72, panel)
	fill(180, 72, w, 74, rule)
	fill(212, 26, 520, 46, ink)
	// The queue table: a header and seven rows, the fourth delayed.
	fill(212, 110, w-40, h-40, panel)
	fill(236, 134, 420, 150, ink)
	for i := 0; i < 7; i++ {
		y := 180 + i*64
		if i == 3 {
			fill(212, y, w-40, y+64, alert)
			fill(980, y+22, 1180, y+42, alertI)
		} else {
			fill(980, y+22, 1100, y+42, rule)
		}
		fill(212, y, w-40, y+2, rule)
		fill(236, y+22, 560, y+42, ink)
		fill(640, y+22, 860, y+42, rule)
	}
	return img
}
