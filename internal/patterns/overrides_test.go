package patterns

import (
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
