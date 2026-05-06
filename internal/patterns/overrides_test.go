package patterns

import (
	"errors"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestResolveAccent(t *testing.T) {
	meta := &types.TemplateMetadata{
		SemanticAccents: map[string]string{
			"positive": "accent3",
			"negative": "accent2",
			"neutral":  "accent4",
		},
	}

	tests := []struct {
		name           string
		accent         string
		semanticAccent string
		metadata       *types.TemplateMetadata
		want           string
	}{
		{"default fallback", "", "", nil, "accent1"},
		{"explicit accent wins", "accent5", "", nil, "accent5"},
		{"explicit accent beats semantic", "accent5", "positive", meta, "accent5"},
		{"semantic resolves via metadata", "", "positive", meta, "accent3"},
		{"semantic negative", "", "negative", meta, "accent2"},
		{"semantic neutral", "", "neutral", meta, "accent4"},
		{"semantic with nil metadata falls back", "", "positive", nil, "accent1"},
		{"semantic with empty map falls back", "", "positive", &types.TemplateMetadata{}, "accent1"},
		{"unknown semantic role falls back", "", "danger", meta, "accent1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAccent(tt.accent, tt.semanticAccent, tt.metadata)
			if got != tt.want {
				t.Errorf("ResolveAccent(%q, %q, ...) = %q, want %q", tt.accent, tt.semanticAccent, got, tt.want)
			}
		})
	}
}

func TestAccentForStrategy(t *testing.T) {
	tests := []struct {
		name         string
		strategy     AccentStrategy
		slideIndex   int
		sectionIndex int
		want         string
	}{
		{"primary always accent1", AccentStrategyPrimary, 0, 0, "accent1"},
		{"primary ignores slide index", AccentStrategyPrimary, 5, 3, "accent1"},
		{"empty strategy defaults to accent1", "", 5, 3, "accent1"},
		{"rotate slide 0", AccentStrategyRotate, 0, 0, "accent1"},
		{"rotate slide 1", AccentStrategyRotate, 1, 0, "accent2"},
		{"rotate slide 5", AccentStrategyRotate, 5, 0, "accent6"},
		{"rotate wraps at 6", AccentStrategyRotate, 6, 0, "accent1"},
		{"rotate wraps at 7", AccentStrategyRotate, 7, 0, "accent2"},
		{"section-keyed section 0", AccentStrategySectionKeyed, 0, 0, "accent1"},
		{"section-keyed section 2", AccentStrategySectionKeyed, 5, 2, "accent3"},
		{"section-keyed wraps at 6", AccentStrategySectionKeyed, 10, 6, "accent1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AccentForStrategy(tt.strategy, tt.slideIndex, tt.sectionIndex)
			if got != tt.want {
				t.Errorf("AccentForStrategy(%q, %d, %d) = %q, want %q",
					tt.strategy, tt.slideIndex, tt.sectionIndex, got, tt.want)
			}
		})
	}
}

func TestExpandContext_ResolveAccent(t *testing.T) {
	meta := &types.TemplateMetadata{
		SemanticAccents: map[string]string{
			"positive": "accent3",
		},
	}

	tests := []struct {
		name           string
		ctx            ExpandContext
		accent         string
		semanticAccent string
		want           string
	}{
		{
			name: "explicit accent wins over rotate",
			ctx:  ExpandContext{Metadata: meta, AccentStrategy: AccentStrategyRotate, SlideIndex: 3},
			accent: "accent5", want: "accent5",
		},
		{
			name: "semantic wins over rotate",
			ctx:  ExpandContext{Metadata: meta, AccentStrategy: AccentStrategyRotate, SlideIndex: 3},
			semanticAccent: "positive", want: "accent3",
		},
		{
			name: "rotate used when no explicit accent",
			ctx:  ExpandContext{AccentStrategy: AccentStrategyRotate, SlideIndex: 3},
			want: "accent4",
		},
		{
			name: "section-keyed used when no explicit accent",
			ctx:  ExpandContext{AccentStrategy: AccentStrategySectionKeyed, SectionIndex: 2},
			want: "accent3",
		},
		{
			name: "primary is default",
			ctx:  ExpandContext{SlideIndex: 5},
			want: "accent1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.ResolveAccent(tt.accent, tt.semanticAccent)
			if got != tt.want {
				t.Errorf("ctx.ResolveAccent(%q, %q) = %q, want %q",
					tt.accent, tt.semanticAccent, got, tt.want)
			}
		})
	}
}

func TestResolveSurface(t *testing.T) {
	meta := &types.TemplateMetadata{
		SurfaceTints: map[string]string{
			"subtle":   "lt2",
			"paper":    "lt1",
			"elevated": "accent5",
			"inverse":  "dk2",
		},
	}

	tests := []struct {
		name         string
		role         string
		metadata     *types.TemplateMetadata
		defaultColor string
		want         string
	}{
		{name: "role found in metadata", role: "subtle", metadata: meta, defaultColor: "lt1", want: "lt2"},
		{name: "paper role", role: "paper", metadata: meta, defaultColor: "lt2", want: "lt1"},
		{name: "elevated role", role: "elevated", metadata: meta, defaultColor: "lt2", want: "accent5"},
		{name: "role not found falls back", role: "unknown", metadata: meta, defaultColor: "dk1", want: "dk1"},
		{name: "nil metadata falls back", role: "subtle", metadata: nil, defaultColor: "lt1", want: "lt1"},
		{name: "empty tints falls back", role: "subtle", metadata: &types.TemplateMetadata{}, defaultColor: "lt1", want: "lt1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSurface(tt.role, tt.metadata, tt.defaultColor)
			if got != tt.want {
				t.Errorf("ResolveSurface(%q, meta, %q) = %q, want %q",
					tt.role, tt.defaultColor, got, tt.want)
			}
		})
	}
}

func TestExpandContext_ResolveSurface(t *testing.T) {
	ctx := ExpandContext{
		Metadata: &types.TemplateMetadata{
			SurfaceTints: map[string]string{
				"subtle": "accent3",
				"paper":  "lt2",
			},
		},
	}

	if got := ctx.ResolveSurface("subtle", "lt1"); got != "accent3" {
		t.Errorf("ctx.ResolveSurface(subtle) = %q, want accent3", got)
	}
	if got := ctx.ResolveSurface("paper", "lt1"); got != "lt2" {
		t.Errorf("ctx.ResolveSurface(paper) = %q, want lt2", got)
	}
	if got := ctx.ResolveSurface("inverse", "dk1"); got != "dk1" {
		t.Errorf("ctx.ResolveSurface(inverse) = %q, want dk1 (fallback)", got)
	}
}

func TestResolveCellAccent(t *testing.T) {
	tests := []struct {
		name string
		base string
		idx  int
		mode string
		want string
	}{
		// uniform (default)
		{"uniform explicit", "accent1", 0, "uniform", "accent1"},
		{"uniform idx=3", "accent3", 3, "uniform", "accent3"},
		{"empty mode = uniform", "accent2", 5, "", "accent2"},

		// alternate: base, base+1, base, base+1, ...
		{"alternate idx=0 base=accent1", "accent1", 0, "alternate", "accent1"},
		{"alternate idx=1 base=accent1", "accent1", 1, "alternate", "accent2"},
		{"alternate idx=2 base=accent1", "accent1", 2, "alternate", "accent1"},
		{"alternate idx=3 base=accent1", "accent1", 3, "alternate", "accent2"},
		{"alternate idx=0 base=accent5", "accent5", 0, "alternate", "accent5"},
		{"alternate idx=1 base=accent5", "accent5", 1, "alternate", "accent6"},
		{"alternate idx=0 base=accent6", "accent6", 0, "alternate", "accent6"},
		{"alternate idx=1 base=accent6 wraps", "accent6", 1, "alternate", "accent1"},

		// progressive: base, base+1, base+2, ...
		{"progressive idx=0 base=accent1", "accent1", 0, "progressive", "accent1"},
		{"progressive idx=1 base=accent1", "accent1", 1, "progressive", "accent2"},
		{"progressive idx=2 base=accent1", "accent1", 2, "progressive", "accent3"},
		{"progressive idx=5 base=accent1", "accent1", 5, "progressive", "accent6"},
		{"progressive idx=6 base=accent1 wraps", "accent1", 6, "progressive", "accent1"},
		{"progressive idx=0 base=accent3", "accent3", 0, "progressive", "accent3"},
		{"progressive idx=1 base=accent3", "accent3", 1, "progressive", "accent4"},
		{"progressive idx=4 base=accent3 wraps", "accent3", 4, "progressive", "accent1"},
		{"progressive idx=5 base=accent3", "accent3", 5, "progressive", "accent2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveCellAccent(tt.base, tt.idx, tt.mode)
			if got != tt.want {
				t.Errorf("ResolveCellAccent(%q, %d, %q) = %q, want %q",
					tt.base, tt.idx, tt.mode, got, tt.want)
			}
		})
	}
}

func TestResolveCellAccent_WithStrategy(t *testing.T) {
	// Verify cell_accent_mode operates on top of strategy-resolved base
	ctx := ExpandContext{
		AccentStrategy: AccentStrategySectionKeyed,
		SectionIndex:   2, // -> accent3
	}
	base := ctx.ResolveAccent("", "") // accent3

	// Progressive from accent3: accent3, accent4, accent5, accent6, accent1, accent2
	for i, want := range []string{"accent3", "accent4", "accent5", "accent6", "accent1", "accent2"} {
		got := ResolveCellAccent(base, i, "progressive")
		if got != want {
			t.Errorf("idx=%d: ResolveCellAccent(%q, %d, progressive) = %q, want %q",
				i, base, i, got, want)
		}
	}
}

func TestValidateCellAccentMode(t *testing.T) {
	// Valid modes
	for _, mode := range []string{"", "uniform", "alternate", "progressive"} {
		if err := ValidateCellAccentMode("test", mode); err != nil {
			t.Errorf("ValidateCellAccentMode(%q) = %v, want nil", mode, err)
		}
	}

	// Invalid mode
	err := ValidateCellAccentMode("test", "invalid")
	if err == nil {
		t.Fatal("ValidateCellAccentMode(invalid) = nil, want error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.Code != "invalid_enum" {
		t.Errorf("error code = %q, want invalid_enum", ve.Code)
	}
}

func TestAccentNumber(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"accent1", 1},
		{"accent2", 2},
		{"accent6", 6},
		{"accent0", 1},  // out of range, falls back
		{"accent7", 1},  // out of range, falls back
		{"dk1", 1},      // wrong format, falls back
		{"", 1},         // empty, falls back
		{"accent", 1},   // no digit
	}
	for _, tt := range tests {
		got := accentNumber(tt.name)
		if got != tt.want {
			t.Errorf("accentNumber(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}
