package deckinput

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// withExpander temporarily installs a SplitSlideExpander for the duration of a
// test and restores the previous value (the package-level var is global state).
func withExpander(t *testing.T, fn func(SplitSlideInput) ([]SlideInput, error)) {
	t.Helper()
	prev := SplitSlideExpander
	SplitSlideExpander = fn
	t.Cleanup(func() { SplitSlideExpander = prev })
}

func TestPresentationInputUnmarshalJSON_RegularSlides(t *testing.T) {
	const in = `{
		"template": "midnight-blue",
		"slides": [
			{"slide_type": "title", "content": []},
			{"slide_type": "content", "content": [
				{"placeholder_id": "title", "type": "text", "text_value": "Hello"}
			]}
		]
	}`

	var p PresentationInput
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Template != "midnight-blue" {
		t.Errorf("Template = %q, want midnight-blue", p.Template)
	}
	if len(p.Slides) != 2 {
		t.Fatalf("len(Slides) = %d, want 2", len(p.Slides))
	}
	if p.Slides[0].SlideType != "title" {
		t.Errorf("Slides[0].SlideType = %q, want title", p.Slides[0].SlideType)
	}
	if got := len(p.Slides[1].Content); got != 1 {
		t.Fatalf("Slides[1].Content len = %d, want 1", got)
	}
	if p.Slides[1].Content[0].TextValue == nil || *p.Slides[1].Content[0].TextValue != "Hello" {
		t.Errorf("Slides[1] text_value not decoded: %+v", p.Slides[1].Content[0])
	}
}

func TestPresentationInputUnmarshalJSON_NilSlides(t *testing.T) {
	var p PresentationInput
	if err := json.Unmarshal([]byte(`{"template":"t","slides":[]}`), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Slides != nil {
		t.Errorf("Slides = %+v, want nil for empty input", p.Slides)
	}
}

func TestPresentationInputUnmarshalJSON_MalformedTopLevel(t *testing.T) {
	var p PresentationInput
	if err := json.Unmarshal([]byte(`{"template": 5}`), &p); err == nil {
		t.Fatal("expected error for malformed top-level JSON, got nil")
	}
}

func TestPresentationInputUnmarshalJSON_MalformedSlideProbe(t *testing.T) {
	// A slide whose "type" cannot be probed (it is not an object) must surface
	// a slide-indexed error.
	var p PresentationInput
	err := json.Unmarshal([]byte(`{"template":"t","slides":[42]}`), &p)
	if err == nil {
		t.Fatal("expected error for non-object slide, got nil")
	}
}

func TestPresentationInputUnmarshalJSON_MalformedRegularSlide(t *testing.T) {
	// Probe succeeds (object with no/other type) but the full SlideInput decode
	// fails because content is the wrong shape.
	var p PresentationInput
	err := json.Unmarshal([]byte(`{"template":"t","slides":[{"content": 7}]}`), &p)
	if err == nil {
		t.Fatal("expected error for malformed regular slide, got nil")
	}
}

func TestPresentationInputUnmarshalJSON_SplitSlideNoExpander(t *testing.T) {
	withExpander(t, nil)
	var p PresentationInput
	err := json.Unmarshal([]byte(`{"template":"t","slides":[{"type":"split_slide"}]}`), &p)
	if err == nil {
		t.Fatal("expected error when split_slide encountered with nil expander")
	}
}

func TestPresentationInputUnmarshalJSON_SplitSlideExpands(t *testing.T) {
	called := false
	withExpander(t, func(ss SplitSlideInput) ([]SlideInput, error) {
		called = true
		if ss.Type != "split_slide" {
			t.Errorf("expander got Type %q, want split_slide", ss.Type)
		}
		return []SlideInput{
			{SlideType: "content"},
			{SlideType: "content"},
		}, nil
	})

	const in = `{
		"template": "t",
		"slides": [
			{"slide_type": "title", "content": []},
			{"type": "split_slide", "base": {"slide_type": "content"}, "split": {"by": "table.rows", "group_size": 5}}
		]
	}`
	var p PresentationInput
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !called {
		t.Error("expander was not invoked")
	}
	// 1 title slide + 2 expanded slides.
	if len(p.Slides) != 3 {
		t.Fatalf("len(Slides) = %d, want 3", len(p.Slides))
	}
}

func TestPresentationInputUnmarshalJSON_SplitSlideExpanderError(t *testing.T) {
	sentinel := errors.New("boom")
	withExpander(t, func(SplitSlideInput) ([]SlideInput, error) {
		return nil, sentinel
	})
	var p PresentationInput
	err := json.Unmarshal([]byte(`{"template":"t","slides":[{"type":"split_slide"}]}`), &p)
	if err == nil {
		t.Fatal("expected error propagated from expander, got nil")
	}
}

func TestPresentationInputUnmarshalJSON_SplitSlideMalformed(t *testing.T) {
	withExpander(t, func(SplitSlideInput) ([]SlideInput, error) {
		t.Fatal("expander should not be reached for malformed split_slide")
		return nil, nil
	})
	// "split" must be an object; a string makes the SplitSlideInput decode fail
	// before the expander is consulted.
	var p PresentationInput
	err := json.Unmarshal([]byte(`{"template":"t","slides":[{"type":"split_slide","split":"nope"}]}`), &p)
	if err == nil {
		t.Fatal("expected error for malformed split_slide, got nil")
	}
}

func strPtr(s string) *string { return &s }

func TestContentInputUsesLegacyValue(t *testing.T) {
	tests := []struct {
		name string
		c    ContentInput
		want bool
	}{
		{"no value at all", ContentInput{Type: "text"}, false},
		{"text legacy", ContentInput{Type: "text", Value: json.RawMessage(`"x"`)}, true},
		{"text typed wins", ContentInput{Type: "text", Value: json.RawMessage(`"x"`), TextValue: strPtr("y")}, false},
		{"bullets legacy", ContentInput{Type: "bullets", Value: json.RawMessage(`["a"]`)}, true},
		{"table legacy", ContentInput{Type: "table", Value: json.RawMessage(`{}`)}, true},
		{"unknown type", ContentInput{Type: "weird", Value: json.RawMessage(`"x"`)}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.c.UsesLegacyValue(); got != tc.want {
				t.Errorf("UsesLegacyValue() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestContentInputResolveValue_TypedFields(t *testing.T) {
	t.Run("text typed", func(t *testing.T) {
		c := ContentInput{Type: "text", TextValue: strPtr("hi")}
		got, err := c.ResolveValue()
		if err != nil {
			t.Fatalf("ResolveValue: %v", err)
		}
		if got != "hi" {
			t.Errorf("got %v, want hi", got)
		}
	})

	t.Run("bullets typed", func(t *testing.T) {
		b := []string{"a", "b"}
		c := ContentInput{Type: "bullets", BulletsValue: &b}
		got, err := c.ResolveValue()
		if err != nil {
			t.Fatalf("ResolveValue: %v", err)
		}
		if !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Errorf("got %v, want [a b]", got)
		}
	})

	t.Run("body_and_bullets typed", func(t *testing.T) {
		c := ContentInput{Type: "body_and_bullets", BodyAndBulletsValue: &BodyAndBulletsInput{Body: "x"}}
		got, err := c.ResolveValue()
		if err != nil {
			t.Fatalf("ResolveValue: %v", err)
		}
		if _, ok := got.(*BodyAndBulletsInput); !ok {
			t.Errorf("got %T, want *BodyAndBulletsInput", got)
		}
	})
}

func TestContentInputResolveValue_LegacyDecode(t *testing.T) {
	t.Run("text legacy", func(t *testing.T) {
		c := ContentInput{Type: "text", Value: json.RawMessage(`"legacy"`)}
		got, err := c.ResolveValue()
		if err != nil {
			t.Fatalf("ResolveValue: %v", err)
		}
		if got != "legacy" {
			t.Errorf("got %v, want legacy", got)
		}
	})

	t.Run("bullets legacy", func(t *testing.T) {
		c := ContentInput{Type: "bullets", Value: json.RawMessage(`["p","q"]`)}
		got, err := c.ResolveValue()
		if err != nil {
			t.Fatalf("ResolveValue: %v", err)
		}
		if !reflect.DeepEqual(got, []string{"p", "q"}) {
			t.Errorf("got %v, want [p q]", got)
		}
	})

	t.Run("table legacy", func(t *testing.T) {
		c := ContentInput{Type: "table", Value: json.RawMessage(`{"rows":[]}`)}
		got, err := c.ResolveValue()
		if err != nil {
			t.Fatalf("ResolveValue: %v", err)
		}
		if _, ok := got.(*TableInput); !ok {
			t.Errorf("got %T, want *TableInput", got)
		}
	})
}

func TestContentInputResolveValue_Errors(t *testing.T) {
	tests := []struct {
		name string
		c    ContentInput
	}{
		{"text missing both", ContentInput{Type: "text"}},
		{"bullets missing both", ContentInput{Type: "bullets"}},
		{"body_and_bullets missing both", ContentInput{Type: "body_and_bullets"}},
		{"body_and_lead missing both", ContentInput{Type: "body_and_lead"}},
		{"bullet_groups missing both", ContentInput{Type: "bullet_groups"}},
		{"table missing both", ContentInput{Type: "table"}},
		{"text malformed legacy", ContentInput{Type: "text", Value: json.RawMessage(`{`)}},
		{"bullets malformed legacy", ContentInput{Type: "bullets", Value: json.RawMessage(`"notarray"`)}},
		{"unknown type", ContentInput{Type: "nope"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.c.ResolveValue(); err == nil {
				t.Errorf("ResolveValue() error = nil, want error")
			}
		})
	}
}

func TestContentInputResolveValue_NilSignalPaths(t *testing.T) {
	// chart/diagram/image with no typed value return (nil, nil) to signal the
	// caller should take the legacy decode path.
	for _, typ := range []string{"chart", "diagram", "image"} {
		t.Run(typ, func(t *testing.T) {
			c := ContentInput{Type: typ}
			got, err := c.ResolveValue()
			if err != nil {
				t.Fatalf("ResolveValue: %v", err)
			}
			if got != nil {
				t.Errorf("got %v, want nil signal", got)
			}
		})
	}
}

func TestThemeInputToThemeOverride(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var th *ThemeInput
		if got := th.ToThemeOverride(); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("populated", func(t *testing.T) {
		th := &ThemeInput{
			Colors:    map[string]string{"accent1": "FF0000"},
			TitleFont: "Inter",
			BodyFont:  "Roboto",
		}
		got := th.ToThemeOverride()
		if got == nil {
			t.Fatal("got nil, want override")
		}
		if got.TitleFont != "Inter" || got.BodyFont != "Roboto" {
			t.Errorf("fonts = %q/%q, want Inter/Roboto", got.TitleFont, got.BodyFont)
		}
		if got.Colors["accent1"] != "FF0000" {
			t.Errorf("Colors = %v, want accent1=FF0000", got.Colors)
		}
	})
}

func TestSegmentInputHasPatternHasDiagram(t *testing.T) {
	empty := SegmentInput{}
	if empty.HasPattern() {
		t.Error("empty segment HasPattern() = true, want false")
	}
	if empty.HasDiagram() {
		t.Error("empty segment HasDiagram() = true, want false")
	}

	withPattern := SegmentInput{Pattern: PatternInput{Name: "kpi-3up"}}
	if !withPattern.HasPattern() {
		t.Error("pattern segment HasPattern() = false, want true")
	}
	if withPattern.HasDiagram() {
		t.Error("pattern segment HasDiagram() = true, want false")
	}
}
