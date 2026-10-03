package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

func imageCaseBody(overlay map[string]any) map[string]any {
	body := map[string]any{
		"title":   "How Northbank made the deadline",
		"eyebrow": "Case study",
		"heading": "Two clearers migrated in one weekend",
		"body":    "Northbank ran the cutover on the rehearsed plan.",
	}
	for k, v := range overlay {
		if v == nil {
			delete(body, k)
			continue
		}
		body[k] = v
	}
	return body
}

// The case study slide every proposal ends with had no DeckSpec kind, so an
// agent that wanted one dropped to raw_json2pptx (go-slide-creator-q31s).
func TestCompileImageCaseUsesTheSplitWhenItFits(t *testing.T) {
	body := imageCaseBody(map[string]any{
		"bullets": []any{"Nine months from mandate to first wave"},
		"metrics": []any{
			map[string]any{"value": "2", "label": "clearers migrated"},
			map[string]any{"value": "0", "label": "settlement breaks"},
		},
		"caption": "The cutover room, March 2026",
	})
	if got := ImageCasePattern(body); got != "image-text-split" {
		t.Fatalf("ImageCasePattern = %q, want image-text-split", got)
	}
	slide, _, err := CompileImageCase(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "image-text-split" {
		t.Fatalf("pattern = %+v", slide.Pattern)
	}
	var values imageCaseValues
	if err := json.Unmarshal(slide.Pattern.Values, &values); err != nil {
		t.Fatalf("values %s: %v", slide.Pattern.Values, err)
	}
	if values.Eyebrow != "Case study" || values.Heading != "Two clearers migrated in one weekend" {
		t.Errorf("text column = %+v", values)
	}
	if len(values.Metrics) != 2 || values.Metrics[0].Value != "2" {
		t.Errorf("metrics = %+v", values.Metrics)
	}
	// No label is invented for a missing picture: filling the placeholder with
	// the caption printed the caption twice, once in the box and once beneath it.
	if values.ImageLabel != "" {
		t.Errorf("an image label was invented: %q", values.ImageLabel)
	}
}

// go-slide-creator-n3j96: callouts compile to callout overlays anchored on the
// pattern's picture — column 0, or 1 when the picture sits on the right — each
// mapped back to the callout that was authored. The fallbacks, which cannot
// anchor to a picture, keep the labels as text.
func TestImageCaseCalloutsCompileToAnchoredOverlays(t *testing.T) {
	callouts := []any{
		map[string]any{"label": "Delayed queue", "x": float64(560), "y": float64(484), "units": "px"},
		map[string]any{"label": "", "x": 0.2, "y": 0.2}, // unusable: no label
		map[string]any{"label": "Oldest message", "x": 0.62, "y": 0.54},
	}
	for side, col := range map[string]int{"": 0, "left": 0, "right": 1} {
		body := imageCaseBody(map[string]any{"image": "shots/console.png", "callouts": callouts, "image_side": side})
		slide, links, err := CompileImageCase(Input{Body: body})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		if slide.Pattern == nil || len(slide.Overlays) != 2 {
			t.Fatalf("side %q: pattern %v, %d overlays, want the split and 2 overlays", side, slide.Pattern != nil, len(slide.Overlays))
		}
		first, second := slide.Overlays[0], slide.Overlays[1]
		if first.Kind != "callout" || first.Text != "Delayed queue" || first.From != nil {
			t.Errorf("overlay = %+v, want a callout that places its own label", first)
		}
		if a := first.To.AnchorImage; a == nil || a.Row != 0 || a.Col != col || a.X != 560 || a.Y != 484 || a.Units != "px" {
			t.Errorf("side %q: anchor = %+v, want row 0 col %d at 560,484 px", side, a, col)
		}
		if a := second.To.AnchorImage; a.X != 0.62 || a.Y != 0.54 || a.Units != "" {
			t.Errorf("anchor = %+v", a)
		}
		if got := semanticFor(links, "slides[0].overlays[1]"); got != "slides[0].callouts[2]" {
			t.Errorf("overlay 1 maps to %q, want the authored slides[0].callouts[2]", got)
		}
	}

	t.Run("no picture, no overlays", func(t *testing.T) {
		slide, _, err := CompileImageCase(Input{Body: imageCaseBody(map[string]any{"callouts": callouts})})
		if err != nil || len(slide.Overlays) != 0 {
			t.Errorf("overlays = %d, err = %v", len(slide.Overlays), err)
		}
	})

	t.Run("fallback keeps the labels", func(t *testing.T) {
		body := imageCaseBody(map[string]any{"image": "shots/console.png", "callouts": callouts, "body": strings.Repeat("long ", 80)})
		slide, _, err := CompileImageCase(Input{Body: body})
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(slide.Content)
		if slide.Pattern != nil || len(slide.Overlays) != 0 || !strings.Contains(string(encoded), "Delayed queue") || !strings.Contains(string(encoded), "Oldest message") {
			t.Errorf("fallback should carry the callout labels as text: %s", encoded)
		}
	})
}

// A picture is a path, a url or an object, and the side it sits on is an
// override rather than a second pattern.
func TestImageCasePictureForms(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    any
		wantPath string
		wantURL  string
	}{
		{name: "a path", value: "/assets/cutover.png", wantPath: "/assets/cutover.png"},
		{name: "a url", value: "https://example.com/cutover.png", wantURL: "https://example.com/cutover.png"},
		{name: "an object", value: map[string]any{"path": "/assets/cutover.png", "alt": "The cutover room"}, wantPath: "/assets/cutover.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			img := imageCaseFrom(imageCaseBody(map[string]any{"image": tc.value})).Image
			if img == nil {
				t.Fatal("no picture resolved")
			}
			if img.Path != tc.wantPath || img.URL != tc.wantURL {
				t.Errorf("image = %+v", img)
			}
		})
	}

	slide, _, err := CompileImageCase(Input{Body: imageCaseBody(map[string]any{"image_side": "right"})})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var ovr imageCaseOverrides
	if err := json.Unmarshal(slide.Pattern.Overrides, &ovr); err != nil {
		t.Fatalf("overrides %s: %v", slide.Pattern.Overrides, err)
	}
	if ovr.ImageSide != "right" {
		t.Errorf("image_side = %q, want right", ovr.ImageSide)
	}
	// A side nobody asked for is not set, so the pattern keeps its own default.
	plain, _, err := CompileImageCase(Input{Body: imageCaseBody(nil)})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(plain.Pattern.Overrides) != 0 {
		t.Errorf("unexpected overrides: %s", plain.Pattern.Overrides)
	}
}

// go-slide-creator-zifd3: image.fit survives DeckSpec compile into the
// pattern values, and into the two-column fallback's image.
func TestImageCaseImageFitReachesThePattern(t *testing.T) {
	img := map[string]any{"path": "/assets/queue.png", "alt": "Queue", "fit": "Contain"}
	slide, _, err := CompileImageCase(Input{Body: imageCaseBody(map[string]any{"image": img})})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var vals imageCaseValues
	if err := json.Unmarshal(slide.Pattern.Values, &vals); err != nil {
		t.Fatal(err)
	}
	if vals.Image == nil || vals.Image.Fit != "contain" {
		t.Fatalf("values.image = %+v, want fit contain", vals.Image)
	}
	plain, _, err := CompileImageCase(Input{Body: imageCaseBody(map[string]any{"image": "/assets/photo.jpg"})})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain.Pattern.Values), `"fit"`) {
		t.Errorf("no fit asked for, but values carry one: %s", plain.Pattern.Values)
	}
	long := imageCaseBody(map[string]any{"image": img, "body": strings.Repeat("A long story. ", 30)})
	fallback, _, err := CompileImageCase(Input{Body: long})
	if err != nil {
		t.Fatal(err)
	}
	sawImage := false
	for _, c := range fallback.Content {
		if c.ImageValue != nil {
			sawImage = true
			if c.ImageValue.Fit != "contain" {
				t.Errorf("fallback image fit = %q, want contain", c.ImageValue.Fit)
			}
		}
	}
	if !sawImage {
		t.Error("fallback dropped the picture")
	}
}

// A figure with no words, or words with no figure, is half a claim.
func TestImageCaseDropsHalfMetrics(t *testing.T) {
	body := imageCaseBody(map[string]any{"metrics": []any{
		map[string]any{"value": "2", "label": "clearers migrated"},
		map[string]any{"value": "41"},
		map[string]any{"label": "settlement breaks"},
	}})
	if got := imageCaseMetrics(body); len(got) != 1 || got[0].Label != "clearers migrated" {
		t.Errorf("metrics = %+v, want the one complete claim", got)
	}
}

// Past the column's budgets the story degrades rather than being truncated.
func TestImageCaseDegradesWithAReason(t *testing.T) {
	cases := []struct {
		name   string
		body   map[string]any
		reason string
	}{
		{
			name:   "a story too long for the column",
			body:   imageCaseBody(map[string]any{"body": strings.Repeat("a", 310)}),
			reason: "body is 310 characters",
		},
		{
			name:   "an eyebrow written as a sentence",
			body:   imageCaseBody(map[string]any{"eyebrow": strings.Repeat("a", 40)}),
			reason: "eyebrow is 40 characters",
		},
		{
			name:   "six bullets",
			body:   imageCaseBody(map[string]any{"bullets": []any{"a", "b", "c", "d", "e", "f"}}),
			reason: "has 6 bullets",
		},
		{
			name: "four result metrics",
			body: imageCaseBody(map[string]any{"metrics": []any{
				map[string]any{"value": "1", "label": "a"}, map[string]any{"value": "2", "label": "b"},
				map[string]any{"value": "3", "label": "c"}, map[string]any{"value": "4", "label": "d"},
			}}),
			reason: "has 4 result metrics",
		},
		{
			name:   "a picture with nothing said about it",
			body:   map[string]any{"title": "A photo", "image": "/assets/cutover.png"},
			reason: "nothing said about it",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ImageCasePattern(c.body); got != "" {
				t.Errorf("ImageCasePattern = %q, want the content fallback", got)
			}
			if over := ImageCaseOverBudget(c.body); !strings.Contains(over, c.reason) {
				t.Errorf("ImageCaseOverBudget = %q, want it to mention %q", over, c.reason)
			}
			slide, _, err := CompileImageCase(Input{Body: c.body})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if slide.Pattern != nil {
				t.Errorf("expected the content fallback, got pattern %s", slide.Pattern.Name)
			}
		})
	}
}

// The picture cannot come to a text slide, so the caption stands in for it
// rather than the slide pretending there was never an image.
func TestImageCaseFallbackKeepsTheStoryAndThePicture(t *testing.T) {
	body := imageCaseBody(map[string]any{
		"body":    strings.Repeat("a", 310),
		"bullets": []any{"Nine months from mandate to first wave"},
		"metrics": []any{map[string]any{"value": "2", "label": "clearers migrated"}},
		"caption": "The cutover room, March 2026",
	})
	slide, _, err := CompileImageCase(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	encoded, err := json.Marshal(slide.Content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	for _, want := range []string{
		"Case study", "Two clearers migrated in one weekend",
		"Nine months from mandate to first wave", "2 — clearers migrated",
		"Image: The cutover room, March 2026",
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the fallback dropped %q: %s", want, encoded)
		}
	}
}

// The spellings an author reaches for all resolve.
func TestImageCaseAliases(t *testing.T) {
	for _, key := range []string{"body", "text", "story", "description"} {
		body := map[string]any{key: "Northbank ran the cutover."}
		if got := imageCaseFrom(body).Body; got != "Northbank ran the cutover." {
			t.Errorf("%s: body = %q", key, got)
		}
	}
	for _, key := range []string{"bullets", "points", "highlights"} {
		body := map[string]any{key: []any{"A point"}}
		if got := imageCaseBullets(body); len(got) != 1 {
			t.Errorf("%s: resolved %v", key, got)
		}
	}
	for _, key := range []string{"metrics", "results", "outcomes"} {
		body := map[string]any{key: []any{map[string]any{"value": "2", "label": "clearers"}}}
		if got := imageCaseMetrics(body); len(got) != 1 {
			t.Errorf("%s: resolved %v", key, got)
		}
	}
	for _, key := range []string{"image", "photo", "screenshot"} {
		if img := imageCaseImageFrom(map[string]any{key: "/a.png"}); img == nil || img.Path != "/a.png" {
			t.Errorf("%s: resolved %+v", key, img)
		}
	}
}

// A slide with neither body nor bullets has no case to make.
func TestImageCaseWithNoNarrative(t *testing.T) {
	if n := UsableImageCaseNarrative(map[string]any{"title": "A photo"}); n != 0 {
		t.Errorf("UsableImageCaseNarrative = %d, want 0", n)
	}
	if n := UsableImageCaseNarrative(map[string]any{"bullets": []any{"A point"}}); n != 1 {
		t.Errorf("UsableImageCaseNarrative = %d, want 1", n)
	}
	// With nothing at all, the budget rule stays quiet: that is the
	// required-field gate's business.
	if over := ImageCaseOverBudget(map[string]any{"title": "A photo"}); over != "" {
		t.Errorf("an empty case reported %q", over)
	}
}
