package semantic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

func quoteSpec(body map[string]any) *DeckSpec {
	return &DeckSpec{Meta: DeckMeta{Title: "Customer voices"}, Slides: []SlideSpec{{Kind: KindQuote, Body: body}}}
}

func quoteItems(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = map[string]any{"text": "Faster month end", "name": "Speaker"}
	}
	return out
}

func TestQuoteCompileAndPreflight(t *testing.T) {
	cases := []struct {
		name, pattern string
		body          map[string]any
		degraded      bool
	}{
		{"single shorthand", "pull-quote", map[string]any{"title": "Customer voice", "quote": "Faster month end", "attribution": "A. Lee"}, false},
		{"three voices", "quote-cluster", map[string]any{"title": "Customer voices", "quotes": quoteItems(3)}, false},
		{"eight voices", "quote-cluster", map[string]any{"title": "Customer voices", "testimonials": quoteItems(8)}, false},
		{"two voices", "", map[string]any{"title": "Customer voices", "quotes": quoteItems(2)}, true},
		{"nine voices", "", map[string]any{"title": "Customer voices", "voices": quoteItems(9)}, true},
		{"missing attribution", "", map[string]any{"title": "Customer voice", "quote": "Faster month end"}, true},
		{"long single quote", "", map[string]any{"title": "Customer voice", "quote": strings.Repeat("word ", 110), "attribution": "A. Lee"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input, result, err := Compile(quoteSpec(tc.body), CompileOptions{})
			if err != nil {
				t.Fatalf("compile: %v; diagnostics: %+v", err, result.Diagnostics)
			}
			if len(input.Slides) != 1 {
				t.Fatalf("slides = %d, want 1", len(input.Slides))
			}
			slide := input.Slides[0]
			got := ""
			if slide.Pattern != nil {
				got = slide.Pattern.Name
				if _, ok := patterns.Default().Get(got); !ok {
					t.Fatalf("compiled pattern %q is not registered", got)
				}
			}
			if got != tc.pattern {
				t.Errorf("pattern = %q, want %q", got, tc.pattern)
			}
			if tc.degraded {
				foundBullets := false
				for _, c := range slide.Content {
					if c.PlaceholderID == "body" && c.BulletsValue != nil && len(*c.BulletsValue) > 0 {
						foundBullets = true
					}
				}
				if !foundBullets {
					t.Errorf("fallback lost the quote text: %+v", slide.Content)
				}
				if !hasCode(result.Diagnostics, diagnostics.CodeSemanticPatternDegraded) {
					t.Errorf("fallback has no degrade finding: %+v", result.Diagnostics)
				}
			} else if hasCode(result.Diagnostics, diagnostics.CodeSemanticPatternDegraded) {
				t.Errorf("valid visual reported degradation: %+v", result.Diagnostics)
			}
		})
	}
}

func TestQuoteMissingPayloadHasSemanticPath(t *testing.T) {
	ds := Validate(quoteSpec(map[string]any{"title": "Customer voices", "quotes": []any{map[string]any{"text": ""}}}), StrictnessWarn)
	if !hasCode(ds, diagnostics.CodeSemanticRequired) {
		t.Fatalf("missing usable quote text was accepted: %+v", ds)
	}
	for _, d := range ds {
		if d.Code == string(diagnostics.CodeSemanticRequired) && !strings.HasPrefix(d.Path, "slides[0].quotes") {
			t.Errorf("required finding path = %q, want slides[0].quotes", d.Path)
		}
	}
}

func TestQuoteSlideTitleDoesNotBecomeSpeakerRole(t *testing.T) {
	input, _, err := Compile(quoteSpec(map[string]any{
		"title": "Customer voice", "quote": "Month end is faster", "attribution": "A. Lee",
	}), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var values struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(input.Slides[0].Pattern.Values, &values); err != nil {
		t.Fatal(err)
	}
	if values.Role != "" {
		t.Errorf("speaker role = %q, want empty", values.Role)
	}
}

func TestQuoteMixedInvalidItemsBlockInsteadOfDisappearing(t *testing.T) {
	spec := quoteSpec(map[string]any{"quotes": []any{
		map[string]any{"text": "Good quote", "name": "A. Lee"},
		map[string]any{"text": ""},
		map[string]any{"text": 42},
		12,
	}})
	input, result, err := Compile(spec, CompileOptions{})
	if err == nil || input != nil {
		t.Fatalf("malformed quote items compiled: input=%+v, err=%v", input, err)
	}
	want := map[string]bool{
		"slides[0].quotes[1].text": false,
		"slides[0].quotes[2].text": false,
		"slides[0].quotes[3]":      false,
	}
	for _, d := range result.Diagnostics {
		if _, ok := want[d.Path]; ok && d.Severity == diagnostics.SeverityError {
			want[d.Path] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("no blocking finding at %s: %+v", path, result.Diagnostics)
		}
	}
}

func TestQuoteCompetingSourcesAreRejected(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
		path string
	}{
		{"list and shorthand", map[string]any{"quotes": quoteItems(3), "quote": "A fourth voice"}, "slides[0].quote"},
		{"two list aliases", map[string]any{"quotes": quoteItems(3), "voices": quoteItems(3)}, "slides[0].voices"},
		{"two shorthand aliases", map[string]any{"quote": "First", "text": "Second", "attribution": "A. Lee"}, "slides[0].text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input, result, err := Compile(quoteSpec(tc.body), CompileOptions{})
			if err == nil || input != nil {
				t.Fatalf("competing sources compiled: input=%+v, err=%v", input, err)
			}
			found := false
			for _, d := range result.Diagnostics {
				if d.Code == string(diagnostics.CodeAmbiguousInput) && d.Path == tc.path {
					found = true
				}
			}
			if !found {
				t.Errorf("missing ambiguity finding at %s: %+v", tc.path, result.Diagnostics)
			}
		})
	}
}

func TestQuoteBlankCanonicalSourceUsesPopulatedAlias(t *testing.T) {
	input, result, err := Compile(quoteSpec(map[string]any{
		"quotes": []any{}, "testimonials": quoteItems(3),
	}), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v; diagnostics: %+v", err, result.Diagnostics)
	}
	if input.Slides[0].Pattern == nil || input.Slides[0].Pattern.Name != "quote-cluster" {
		t.Errorf("populated alias did not reach quote-cluster: %+v", input.Slides[0].Pattern)
	}
	if path, _, ok := result.SourceMap.ResolveSemantic("slides[0].pattern.values.quotes"); !ok || path != "slides[0].testimonials" {
		t.Errorf("source map path = %q, mapped=%v", path, ok)
	}
}
