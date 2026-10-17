package template

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

var defaultAnalysisCache = NewMemoryCacheWithMaxSize(time.Hour, 128)

// Analyze returns the shared, normalized template analysis used by discovery,
// validation and rendering. Cached analyses are read-only; per-deck overrides
// belong on a copy of Theme, outside this function.
func Analyze(path string, cache types.TemplateCache, strictMode bool) (*types.TemplateAnalysis, error) {
	if cache == nil {
		cache = defaultAnalysisCache
	}
	var cached *types.TemplateAnalysis
	var ok bool
	if fast, supported := cache.(types.FastValidationCache); supported {
		cached, ok = fast.GetWithFastValidation(path)
	} else {
		cached, ok = cache.Get(path)
	}
	if ok {
		if strictMode && len(cached.Warnings) > 0 {
			return nil, fmt.Errorf("template validation failed: %s", cached.Warnings[0].Message)
		}
		return cached, nil
	}
	reader, err := OpenTemplate(path)
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := ParseLayouts(reader)
	if err != nil {
		return nil, fmt.Errorf("parse template layouts: %w", err)
	}
	validation := ValidateTemplateMetadata(reader, strictMode)
	if !validation.Valid {
		return nil, fmt.Errorf("template validation failed: %v", validation.AllIssues())
	}
	metadata := validation.Metadata
	ApplyMetadataHints(layouts, metadata)
	theme := ParseTheme(reader)
	if metadata != nil {
		theme.SemanticAccents = metadata.SemanticAccents
		theme.SurfaceTints = metadata.SurfaceTints
	}
	width, height := ParseSlideDimensions(reader)
	aspect := AspectRatio(width, height)
	if metadata != nil && metadata.AspectRatio != "" {
		aspect = metadata.AspectRatio
	}
	analysis := &types.TemplateAnalysis{
		TemplatePath: path, Hash: reader.Hash(), AnalyzedAt: time.Now(),
		SlideWidth: width, SlideHeight: height, AspectRatio: aspect,
		Layouts: layouts, Theme: theme, Metadata: metadata,
	}
	for _, d := range validation.Diagnostics {
		analysis.Warnings = append(analysis.Warnings, types.TemplateWarning{Code: d.Code, Message: d.Message})
		slog.Warn("template metadata warning", "template", path, "code", d.Code, "message", d.Message)
	}
	for _, style := range reader.TableStyles() {
		analysis.TableStyles = append(analysis.TableStyles, types.TableStyleInfo{ID: style.ID, Name: style.Name})
	}
	SynthesizeIfNeeded(reader, analysis)
	normalized, err := NormalizeLayoutFiles(reader, analysis.Layouts)
	if err != nil {
		return nil, fmt.Errorf("normalize template layouts: %w", err)
	}
	if len(normalized) > 0 {
		if analysis.Synthesis == nil {
			analysis.Synthesis = &types.SynthesisManifest{}
		}
		if analysis.Synthesis.SyntheticFiles == nil {
			analysis.Synthesis.SyntheticFiles = make(map[string][]byte)
		}
		for name, data := range normalized {
			analysis.Synthesis.SyntheticFiles[name] = data
		}
	}
	if fast, supported := cache.(types.FastValidationCache); supported {
		if info, statErr := (OSFileStat{}).Stat(path); statErr == nil && info.Exists {
			fast.SetWithModTime(path, analysis, info.ModTime)
		} else {
			cache.Set(path, analysis)
		}
	} else {
		cache.Set(path, analysis)
	}
	return analysis, nil
}

// AnalysisFindings preserves template-level advice on both cold and cached
// analyses. Native layout normalization never masquerades as synthesis.
func AnalysisFindings(analysis *types.TemplateAnalysis) []patterns.FitFinding {
	if analysis == nil {
		return nil
	}
	findings := synthesisFindings(analysis)
	for _, warning := range analysis.Warnings {
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{Code: warning.Code, Message: warning.Message, Path: "template.metadata"},
			Action:          "review",
		})
	}
	return findings
}

func synthesisFindings(analysis *types.TemplateAnalysis) []patterns.FitFinding {
	if analysis.Synthesis == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for _, id := range analysis.Synthesis.LayoutIDs {
		name := id
		for _, layout := range analysis.Layouts {
			if layout.ID == id {
				name = layout.Name
				break
			}
		}
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Code: "layout_synthesized", Path: "template.layouts",
				Message: fmt.Sprintf("layout %q synthesized from base layout %q — visual identity may differ from template brand", name, analysis.Synthesis.BaseLayoutID),
				Fix: &patterns.FixSuggestion{Kind: "review_layout", Params: map[string]any{
					"layout_id": id, "capability": name, "base_layout_id": analysis.Synthesis.BaseLayoutID,
				}},
			},
			Action: "review",
		})
	}
	return findings
}
