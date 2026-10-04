package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

// compiledOf returns the composition the deck's only slide compiled to.
func compiledOf(t *testing.T, spec *DeckSpec) slides.Composition {
	t.Helper()
	in, res, err := Compile(spec, CompileOptions{Strict: StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v (%+v)", err, res.Diagnostics)
	}
	return compiledComposition(&in.Slides[0])
}

// Every composition list_slide_kinds advertises, requested as an override on
// the kind's own example, either compiles to exactly that composition or is
// refused with SEMANTIC_PATTERN_NOT_AVAILABLE — and in both cases the plan
// explain reports is what compile emits. A kind with no pattern / layout
// field lists only the composition it always takes (go-slide-creator-vj549).
func TestEveryListedCompositionCompilesAsReported(t *testing.T) {
	for kind := range kindExamples {
		if kind == KindRawJSON2pptx {
			continue
		}
		_, overridable := kindPayloadFields[kind]["pattern"]
		base := KindExample(kind)
		delete(base, "kind")
		for _, c := range SlideAlternatives(kind, base) {
			want := slides.Composition{Pattern: c.Pattern, Layout: c.Layout}
			t.Run(string(kind)+"/"+compositionLabel(want), func(t *testing.T) {
				body := KindExample(kind)
				delete(body, "kind")
				if !overridable {
					spec := &DeckSpec{Meta: DeckMeta{Title: "D", Template: "midnight-blue"}, Slides: []SlideSpec{{Kind: kind, Body: body}}}
					if got := compiledOf(t, spec); got != want {
						t.Errorf("%s takes no override, so its one composition must be its default: lists %+v, compiles %+v", kind, want, got)
					}
					return
				}
				if c.Pattern != "" {
					body["pattern"] = c.Pattern
				} else {
					body["layout"] = c.Layout
				}
				spec := &DeckSpec{Meta: DeckMeta{Title: "D", Template: "midnight-blue"}, Slides: []SlideSpec{{Kind: kind, Body: body}}}
				refused := findingsWithCode(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticPatternNotAvailable)
				got := compiledOf(t, spec)
				plan := Normalize(spec).Slides[0].Visual
				if planned := (slides.Composition{Pattern: plan.Pattern, Layout: plan.Layout}); planned != got {
					t.Errorf("explain reports %+v but compile emits %+v", planned, got)
				}
				switch {
				case len(refused) == 0 && got != want:
					t.Errorf("override %+v validated clean but compiled to %+v", want, got)
				case len(refused) > 0 && got == want:
					t.Errorf("override %+v compiled as asked but was reported: %s", want, refused[0].Message)
				case len(refused) > 0 && !strings.Contains(refused[0].Message, "compiles to "+compositionLabel(got)):
					t.Errorf("refusal does not name what the slide compiles to (%s): %s", compositionLabel(got), refused[0].Message)
				case len(refused) > 0 && !strings.HasPrefix(refused[0].Message, compositionLabel(want)[:strings.Index(compositionLabel(want), " ")]+" \""):
					// One sentence that opens with the override it is about
					// (go-slide-creator-kc3h1).
					t.Errorf("refusal does not open with the requested override: %s", refused[0].Message)
				case len(refused) > 0 && (strings.Contains(refused[0].Message, "asked for it") || strings.Count(refused[0].Message, "compiles to "+compositionLabel(got)) != 1):
					t.Errorf("refusal repeats itself: %s", refused[0].Message)
				}
			})
		}
	}
}

// processOverrideSpec is the consulting benchmark's process slide
// (go-slide-creator-vj549) with an optional override.
func processOverrideSpec(override map[string]any, descriptions []string) *DeckSpec {
	labels := []string{"Capture", "Assign", "Resolve", "Close"}
	steps := make([]any, 0, len(labels))
	for i, l := range labels {
		steps = append(steps, map[string]any{"label": l, "description": descriptions[i]})
	}
	body := map[string]any{
		"title":    "Four control points give each dispute one owner and an auditable path to closure",
		"steps":    steps,
		"takeaway": "Automate routing and evidence capture; retain human approval for commercial exceptions.",
	}
	for k, v := range override {
		body[k] = v
	}
	return &DeckSpec{Meta: DeckMeta{Title: "D", Template: "modern"}, Slides: []SlideSpec{{Kind: KindProcess, Body: body}}}
}

var (
	disputeStepDescriptions = []string{
		"Finance creates one case record, links the invoice and assigns a reason code.",
		"The market lead names a resolver; cases without an owner escalate after 24 hours.",
		"The resolver agrees the action; credits above EUR25k require a second approval.",
		"Finance verifies cash or credit, closes the case and updates recurring-cause rules.",
	}
	shortStepDescriptions = []string{"One case per dispute.", "Lead names a resolver.", "Second approval above EUR25k.", "Verify cash, update rules."}
)

// layout:content used to be reported by explain while compile still emitted
// the numbered strip. It now compiles to native bullets that keep every step's
// description, with no pattern.
func TestProcessLayoutContentOverrideEmitsNativeBullets(t *testing.T) {
	spec := processOverrideSpec(map[string]any{"layout": "content"}, disputeStepDescriptions)
	in, _, err := Compile(spec, CompileOptions{Strict: StrictnessStrict})
	if err != nil {
		t.Fatal(err)
	}
	s := in.Slides[0]
	if s.Pattern != nil || s.ShapeGrid != nil || s.SlideType != "content" {
		t.Fatalf("compiled pattern=%v slide_type=%q, want a native content slide", s.Pattern, s.SlideType)
	}
	var bullets []string
	for _, c := range s.Content {
		if c.BulletsValue != nil {
			bullets = append(bullets, *c.BulletsValue...)
		}
	}
	text := strings.Join(bullets, "\n")
	for _, d := range disputeStepDescriptions {
		if !strings.Contains(text, d) {
			t.Errorf("content slide lost %q", d)
		}
	}
	if v := Normalize(spec).Slides[0].Visual; v.Pattern != "" || v.Layout != "content" {
		t.Errorf("plan = pattern %q layout %q, want layout content and no pattern", v.Pattern, v.Layout)
	}
}

// A process pattern override the steps fit compiles to that pattern — not to
// whatever the body would have picked; one they do not fit is refused with the
// measured reason and the slide stays on the plan explain reports.
func TestProcessPatternOverrideSelectsItsCompiler(t *testing.T) {
	for _, pattern := range []string{"process-flow", "process-flow-compact", "numbered-step-strip"} {
		spec := processOverrideSpec(map[string]any{"pattern": pattern}, shortStepDescriptions)
		if d := findingsWithCode(Validate(spec, StrictnessStrict), diagnostics.CodeSemanticPatternNotAvailable); len(d) != 0 {
			t.Errorf("%s: a fitting override was reported: %s", pattern, d[0].Message)
		}
		if got := compiledOf(t, spec); got.Pattern != pattern {
			t.Errorf("override %s compiled to %+v", pattern, got)
		}
	}

	long := processOverrideSpec(map[string]any{"pattern": "process-flow"}, disputeStepDescriptions)
	found := findingsWithCode(Validate(long, StrictnessWarn), diagnostics.CodeSemanticPatternNotAvailable)
	if len(found) != 1 {
		t.Fatalf("want one SEMANTIC_PATTERN_NOT_AVAILABLE for a flow the steps overflow, got %+v", found)
	}
	msg := found[0].Message
	for _, want := range []string{`pattern "process-flow" does not fit this "process" payload: step 1 reads 87 characters`, "a flow box holds 80", "so the override is ignored and the slide compiles to pattern numbered-step-strip; this payload takes: pattern numbered-step-strip"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q: %s", want, msg)
		}
	}
	if found[0].Fix == nil {
		t.Fatal("refusal carries no fix")
	}
	allowed, _ := found[0].Fix.Params["allowed"].([]string)
	if strings.Join(allowed, ",") != "numbered-step-strip" {
		t.Errorf("fix.allowed = %v, want only the pattern this payload takes", allowed)
	}
	got := compiledOf(t, long)
	if planned := Normalize(long).Slides[0].Visual.Pattern; got.Pattern != "numbered-step-strip" || planned != got.Pattern {
		t.Errorf("declined override: compiled %+v, planned %q", got, planned)
	}
}
