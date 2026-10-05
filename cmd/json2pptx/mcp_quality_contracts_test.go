package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

func TestAgendaSelectionSchemaAndCompiler(t *testing.T) {
	if got := slideKindBrief(semantic.KindAgenda)["current"]; got != "string | integer" {
		t.Errorf("current brief = %q", got)
	}
	if got := slideKindBrief(semantic.KindTable)["totals_row"]; got != "boolean" {
		t.Errorf("totals brief = %q", got)
	}
	for _, key := range []string{"current", "current_section", "highlight", "active"} {
		for _, tc := range []struct {
			value any
			valid bool
		}{{2, true}, {"Risks", true}, {1.5, false}} {
			t.Run(fmt.Sprintf("%s/%v", key, tc.value), func(t *testing.T) {
				item := map[string]any{"kind": "agenda", "sections": []any{"Performance", "Risks", "Investment"}, key: tc.value}
				schemas := map[string]map[string]any{"full": semantic.KindItemSchema(semantic.KindAgenda)}
				if key == "current" {
					schemas["compact"] = semantic.KindItemSchemaCompact(semantic.KindAgenda)
				}
				for name, s := range schemas {
					raw, err := json.Marshal(s)
					if err != nil {
						t.Fatal(err)
					}
					err = compileToolInputSchema(t, name, raw).Validate(roundTripJSON(t, item))
					if (err == nil) != tc.valid {
						t.Errorf("%s accepted=%v, want %v: %v", name, err == nil, tc.valid, err)
					}
				}
				want := 0
				if tc.valid {
					want = 2
				}
				if got := slides.AgendaCurrentIndex(item, slides.AgendaSections(item)); got != want {
					t.Errorf("selected %d, want %d", got, want)
				}
			})
		}
	}
}

func TestTableTotalsContractAndRenderedCells(t *testing.T) {
	for _, tc := range []struct {
		name          string
		value         any
		include, bold bool
	}{{"true", true, true, true}, {"false", false, true, false}, {"omitted", nil, false, false}, {"string", "true", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			item := map[string]any{"kind": "table", "title": "Revenue by team", "headers": []any{"Team", "Revenue"}, "rows": []any{[]any{"North", "100"}, []any{"South", "200"}, []any{"Combined", "300"}}}
			if tc.include {
				item["totals_row"] = tc.value
			}
			for name, s := range map[string]map[string]any{"full": semantic.KindItemSchema(semantic.KindTable), "compact": semantic.KindItemSchemaCompact(semantic.KindTable)} {
				raw, err := json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				err = compileToolInputSchema(t, name, raw).Validate(roundTripJSON(t, item))
				if (err != nil) != (tc.name == "string") {
					t.Errorf("%s schema error: %v", name, err)
				}
			}
			spec := map[string]any{"meta": map[string]any{"title": "Revenue", "template": "midnight-blue"}, "slides": []any{item}}
			if tc.name == "string" {
				res, err := testValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec, "strict": "warn"}))
				if err != nil {
					t.Fatal(err)
				}
				var env diagnostics.FindingEnvelope
				structuredInto(t, res.StructuredContent, &env)
				found := false
				for _, f := range env.Findings {
					if strings.HasSuffix(f.Code, diagnostics.CodeSemanticFieldType) && f.Path != nil && *f.Path == "/slides/0/totals_row" {
						found = true
					}
				}
				if !found {
					t.Errorf("missing totals_row type finding: %+v", env.Findings)
				}
				return
			}
			out := renderDeckSpecCall(t, semanticTestConfig(t), map[string]any{"spec": spec})
			if !out.Success {
				t.Fatalf("render failed: %s %+v", out.Error, out.Diagnostics)
			}
			xml := string(pptxParts(t, out.PptxPath)["ppt/slides/slide1.xml"])
			rows := regexp.MustCompile(`(?s)<a:tr\b[^>]*>.*?</a:tr>`).FindAllString(xml, -1)
			if len(rows) != 4 {
				t.Fatalf("got %d table rows", len(rows))
			}
			cells := regexp.MustCompile(`(?s)<a:tc>.*?</a:tc>`).FindAllString(rows[3], -1)
			if len(cells) != 2 || !strings.Contains(cells[0], "Combined") || !strings.Contains(cells[1], ">300<") {
				t.Fatalf("wrong last row: %s", rows[3])
			}
			bold := strings.Contains(cells[1], `b="1"`)
			separator := regexp.MustCompile(`(?s)<a:lnT w="12700"[^>]*>\s*<a:solidFill>`).MatchString(cells[1])
			if bold != tc.bold || separator != tc.bold {
				t.Errorf("numeric cell bold=%v separator=%v, want %v: %s", bold, separator, tc.bold, cells[1])
			}
		})
	}
}

// The advertised draft example must produce an artifact without becoming ready.
func TestImageCaseDraftReadiness(t *testing.T) {
	for _, tc := range []struct {
		label, strict string
		ready         bool
	}{
		{"image_label", "warn", false},
		{"image_label", "off", false},
		{"placeholder", "warn", false},
		{"actual_image", "warn", true},
		{"text_only", "warn", true},
	} {
		t.Run(tc.label+"/"+tc.strict, func(t *testing.T) {
			item := semantic.KindExample(semantic.KindImageCase)
			delete(item, "image_label")
			switch tc.label {
			case "image_label", "placeholder":
				item[tc.label] = "Photo of the cutover room"
			case "actual_image":
				path := filepath.Join(t.TempDir(), "photo.png")
				if err := os.WriteFile(path, distinctPNG(t), 0600); err != nil {
					t.Fatal(err)
				}
				item["image"] = path
			case "text_only":
				item = map[string]any{"kind": "executive_summary", "title": "The rehearsal delivered", "points": []any{"Two clearers migrated", "No settlement breaks", "Rehearsed every wave"}}
			}
			spec := map[string]any{"meta": map[string]any{"title": "Cutover", "template": "midnight-blue"}, "slides": []any{item}}
			out := renderDeckSpecCall(t, semanticTestConfig(t), map[string]any{"spec": spec, "strict": tc.strict})
			if !out.Success {
				t.Fatalf("draft cannot render: %s %+v", out.Error, out.Diagnostics)
			}
			if out.DeterministicReady == nil || *out.DeterministicReady != tc.ready {
				t.Errorf("ready=%v want %v: %+v", out.DeterministicReady, tc.ready, out.Diagnostics)
			}
			found := false
			for _, d := range out.Diagnostics {
				if d.Code == diagnostics.CodeSemanticImageMissing {
					found = d.Blocking && d.Severity == "error" && d.address != nil && d.address.Path == "/slides/0" && d.address.Missing == "/slides/0/image"
				}
			}
			if found == tc.ready {
				t.Errorf("missing asset blocker=%v want %v: %+v", found, !tc.ready, out.Diagnostics)
			}
		})
	}
}

func TestImageCaseDraftThumbnailReview(t *testing.T) {
	if testing.Short() {
		t.Skip("requires LibreOffice thumbnails")
	}
	if ok, _ := render.DependencyStatus(); !ok {
		t.Skip("LibreOffice/ImageMagick not installed")
	}
	mc := semanticTestConfig(t)
	listed, err := handleListSlideKinds(context.Background(), makeRequest(map[string]any{"kinds": []any{"image_case"}}))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, listed.StructuredContent, &catalog)
	if len(catalog.SlideKinds) != 1 {
		t.Fatalf("catalog: %+v", catalog)
	}
	spec := map[string]any{"meta": map[string]any{"title": "Case study", "template": "midnight-blue"}, "slides": []any{catalog.SlideKinds[0].Example}}
	out := renderDeckSpecCall(t, mc, map[string]any{"spec": spec})
	if !out.Success || out.DeterministicReady == nil || *out.DeterministicReady {
		t.Fatalf("draft readiness: %+v", out)
	}
	thumbs, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{"pptx_path": out.PptxPath}))
	if err != nil || thumbs.IsError {
		t.Fatalf("thumbnails: %v %+v", err, thumbs)
	}
	var images renderedDeckThumbnailsResponse
	if err := json.Unmarshal([]byte(textContent(thumbs)), &images); err != nil {
		t.Fatal(err)
	}
	if len(images.Slides) != 1 || images.Slides[0].Path == "" {
		t.Fatalf("thumbnails: %+v", images)
	}
	art, err := describeArtifact(out.PptxPath, "pptx")
	if err != nil {
		t.Fatal(err)
	}
	review, err := submitVisualReview(submitVisualReviewInput{PPTXPath: out.PptxPath, PPTXRevision: art.SHA256, Slides: allSlides([]string{images.Slides[0].Path}, "approved")})
	if err != nil {
		t.Fatal(err)
	}
	if review.Publishable == nil || *review.Publishable {
		t.Errorf("visual approval cleared missing-image blocker: %+v", review)
	}
}

// Alias references must remain valid after the MCP schema minifies definitions
// and embeds the DeckSpec beneath an argument, without growing tools/list.
func TestCompactDeckSpecSchemaAliasReferences(t *testing.T) {
	root := map[string]any{"type": "object", "properties": map[string]any{"spec": semantic.CompactSchemaAt("#/properties/spec")}}
	raw, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	validator := compileToolInputSchema(t, "embedded DeckSpec", raw)
	validate := func(item any) error {
		return validator.Validate(roundTripJSON(t, map[string]any{"spec": map[string]any{"slides": []any{item}}}))
	}
	for _, kind := range semantic.AllSlideKinds() {
		t.Run(string(kind), func(t *testing.T) {
			example := semantic.KindExample(kind)
			if err := validate(example); err != nil {
				t.Fatalf("canonical example: %v", err)
			}
			props := semantic.KindItemSchemaCompact(kind)["properties"].(map[string]any)
			for canonical, raw := range props {
				value, present := example[canonical]
				if !present {
					continue
				}
				prop := raw.(map[string]any)
				aliases, _ := prop["aliases"].([]any)
				for _, alias := range aliases {
					item := deepCopyJSON(example).(map[string]any)
					delete(item, canonical)
					item[alias.(string)] = value
					if err := validate(item); err != nil {
						t.Errorf("alias %s: %v", alias, err)
					}
				}
			}
		})
	}
	for _, key := range []string{"image", "photo", "screenshot"} {
		item := map[string]any{"kind": "image_case", "body": "Migration finished on schedule.", key: "image.png"}
		if err := validate(item); err != nil {
			t.Errorf("%s string rejected: %v", key, err)
		}
		item[key] = map[string]any{"path": "image.png", "fit": "contain"}
		if err := validate(item); err != nil {
			t.Errorf("%s object rejected: %v", key, err)
		}
		item[key] = true
		if err := validate(item); err == nil {
			t.Errorf("%s boolean accepted", key)
		}
	}
}
