package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Every next_tool_call an agent receives must name a tool the active profile
// advertises: a client model cannot emit a call to a tool absent from tools/list
// (go-slide-creator-mvny).
func TestNextCallHelpersStayInProfile(t *testing.T) {
	helpers := map[string]func() *patterns.ToolCallSuggestion{
		"nextCallGetInputSchema":     nextCallGetInputSchema,
		"nextCallListTemplates":      nextCallListTemplates,
		"nextCallListPatterns":       nextCallListPatterns,
		"nextCallInspectSlideImages": nextCallInspectSlideImages,
		"nextCallReadPresentation":   func() *patterns.ToolCallSuggestion { return nextCallReadPresentation("/tmp/deck.pptx") },
		"nextCallValidateOutput":     func() *patterns.ToolCallSuggestion { return nextCallValidateOutput("/tmp/deck.pptx") },
	}
	for _, profile := range []string{toolProfileCore, toolProfileAll} {
		t.Run(profile, func(t *testing.T) {
			withToolProfile(t, profile)
			for name, build := range helpers {
				s := build()
				if s == nil {
					continue // dropping a suggestion is allowed; naming a hidden tool is not
				}
				if !toolIsAdvertised(s.Tool) {
					t.Errorf("%s suggests %q, which profile %s does not advertise", name, s.Tool, profile)
				}
			}
		})
	}
}

// The substitution must carry the file path over, not lose it: a suggestion the
// agent has to fill in by hand is a worse hop than the one it replaced.
func TestReadPresentationSubstituteKeepsThePath(t *testing.T) {
	withToolProfile(t, toolProfileCore)
	s := nextCallReadPresentation("/tmp/out/deck.pptx")
	if s == nil {
		t.Fatal("core profile dropped the read_presentation hop entirely")
	}
	if s.Tool != "render_deck_thumbnails" {
		t.Errorf("tool = %q, want render_deck_thumbnails", s.Tool)
	}
	if got := s.ArgsTemplate["pptx_path"]; got != "/tmp/out/deck.pptx" {
		t.Errorf("pptx_path = %v, want the original path", got)
	}

	withToolProfile(t, toolProfileAll)
	if s := nextCallReadPresentation("/tmp/out/deck.pptx"); s == nil || s.Tool != "read_presentation" {
		t.Errorf("full profile must keep read_presentation, got %+v", s)
	}
}

// patterns.AttachNextToolCalls points adopt_pattern / swap_pattern findings at
// recommend_pattern, which core hides. The retarget pass rewrites it to
// recommend_visual with the item count carried into content_hints.
func TestRetargetRewritesRecommendPatternForCore(t *testing.T) {
	newFindings := func() []patterns.FitFinding {
		return []patterns.FitFinding{{
			ValidationError: patterns.ValidationError{
				Pattern: "shape_grid",
				Path:    slidepath.ShapeGrid(2),
				Code:    patterns.ErrCodeSparseLayout,
				Message: "raw grid content is sparse",
				Fix: &patterns.FixSuggestion{
					Kind:   "adopt_pattern",
					Params: map[string]any{"filled_slots": 6},
				},
			},
			Action: "review",
		}}
	}

	withToolProfile(t, toolProfileCore)
	findings := newFindings()
	patterns.AttachNextToolCalls(findings, slidepath.SlideIndex)
	if got := findings[0].NextToolCall; got == nil || got.Tool != "recommend_pattern" {
		t.Fatalf("precondition: AttachNextToolCalls should emit recommend_pattern, got %+v", got)
	}
	retargetUnadvertisedSuggestions(findings)
	next := findings[0].NextToolCall
	if next == nil {
		t.Fatal("core profile dropped the recommendation hop; recommend_visual is in core")
	}
	if next.Tool != "recommend_visual" {
		t.Errorf("tool = %q, want recommend_visual", next.Tool)
	}
	hints, ok := next.ArgsTemplate["content_hints"].(map[string]any)
	if !ok {
		t.Fatalf("args_template.content_hints missing or wrong type: %#v", next.ArgsTemplate)
	}
	if hints["item_count"] != 6 {
		t.Errorf("content_hints.item_count = %v, want the finding's 6 filled slots", hints["item_count"])
	}
	if _, ok := next.ArgsTemplate["intent"]; !ok {
		t.Error("recommend_visual requires intent; the substitute must template it")
	}

	withToolProfile(t, toolProfileAll)
	findings = newFindings()
	patterns.AttachNextToolCalls(findings, slidepath.SlideIndex)
	retargetUnadvertisedSuggestions(findings)
	if got := findings[0].NextToolCall; got == nil || got.Tool != "recommend_pattern" {
		t.Errorf("full profile must keep recommend_pattern, got %+v", got)
	}
}

func TestDeckMonotonyNextCallUsesRecommendVisualContentHints(t *testing.T) {
	withToolProfile(t, toolProfileCore)
	findings := []patterns.FitFinding{{
		ValidationError: patterns.ValidationError{
			Path: slidepath.ShapeGrid(2),
			Code: patterns.ErrCodeDeckMonotony,
			Fix:  &patterns.FixSuggestion{Kind: "swap_pattern"},
		},
		Action: "review",
	}}
	patterns.AttachNextToolCalls(findings, slidepath.SlideIndex)
	retargetUnadvertisedSuggestions(findings)
	next := findings[0].NextToolCall
	if next == nil || next.Tool != "recommend_visual" {
		t.Fatalf("DECK_MONOTONY next_tool_call = %+v, want recommend_visual", next)
	}
	if _, ok := next.ArgsTemplate["content_hints"].(map[string]any); !ok {
		t.Errorf("DECK_MONOTONY args_template has no content_hints object: %#v", next.ArgsTemplate)
	}
	if _, wrong := next.ArgsTemplate["hints"]; wrong {
		t.Errorf("DECK_MONOTONY still suggests unknown hints argument: %#v", next.ArgsTemplate)
	}
}

// Every profile substitution must name arguments declared by its target tool,
// satisfy required fields, and use the advertised top-level types. This catches
// handoff errors that name an advertised tool but get UNKNOWN_PARAMETER next.
func TestSuggestionSubstitutesMatchTargetInputSchemas(t *testing.T) {
	if len(suggestionSubstitutes) == 0 {
		t.Fatal("no profile substitutions to validate")
	}
	constructors := toolConstructors()
	sourceArgs := map[string]any{
		"item_count": 6,
		"pptx_path":  "/tmp/deck.pptx",
		"path":       "/tmp/deck.pptx",
	}
	for source, build := range suggestionSubstitutes {
		t.Run(source, func(t *testing.T) {
			s := build(sourceArgs)
			if s == nil {
				t.Fatal("substitute returned nil")
			}
			ctor, ok := constructors[s.Tool]
			if !ok {
				t.Fatalf("suggested tool %q has no registered constructor", s.Tool)
			}
			tool := ctor()
			for _, required := range tool.InputSchema.Required {
				if _, ok := s.ArgsTemplate[required]; !ok {
					t.Errorf("%s requires %q, missing from args_template", s.Tool, required)
				}
			}
			for arg, value := range s.ArgsTemplate {
				prop, ok := tool.InputSchema.Properties[arg]
				if !ok {
					t.Errorf("%s args_template has unknown parameter %q", s.Tool, arg)
					continue
				}
				raw, err := json.Marshal(prop)
				if err != nil {
					t.Fatal(err)
				}
				var schema struct {
					Type string `json:"type"`
				}
				if err := json.Unmarshal(raw, &schema); err != nil {
					t.Fatal(err)
				}
				switch schema.Type {
				case "string":
					if _, ok := value.(string); !ok {
						t.Errorf("%s.%s must be string, got %T", s.Tool, arg, value)
					}
				case "object":
					if _, ok := value.(map[string]any); !ok {
						t.Errorf("%s.%s must be object, got %T", s.Tool, arg, value)
					}
				default:
					t.Fatalf("%s.%s has unhandled schema type %q", s.Tool, arg, schema.Type)
				}
			}
		})
	}
}

// A tool with no in-profile equivalent loses its suggestion rather than naming
// something uncallable.
func TestSubstituteDropsWhenNoEquivalentExists(t *testing.T) {
	withToolProfile(t, toolProfileCore)
	got := substituteUnadvertised(&patterns.ToolCallSuggestion{Tool: "register_template_setting"})
	if got != nil {
		t.Errorf("substituteUnadvertised = %+v, want nil for a hidden tool with no equivalent", got)
	}
}
