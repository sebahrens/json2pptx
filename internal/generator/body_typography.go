package generator

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// BodySizePolicy is the presentation-size range for one body-text density.
// Sizes are in hundredths of a point. A template size inside the range is
// preserved; one outside it is normalised to TargetHPt before measured fit.
type BodySizePolicy struct {
	TargetHPt int
	MinHPt    int
	MaxHPt    int
}

// BodySizeForDensity returns the effective base font and allowed range for a
// body placeholder. Density counts visible paragraphs (including group heads).
func BodySizeForDensity(templateHPt, density int) (int, BodySizePolicy) {
	policy := BodySizePolicy{TargetHPt: 1800, MinHPt: 1600, MaxHPt: 2000}
	switch {
	case density >= 10:
		policy = BodySizePolicy{TargetHPt: 1400, MinHPt: 1200, MaxHPt: 1600}
	case density >= 7:
		policy = BodySizePolicy{TargetHPt: 1600, MinHPt: 1400, MaxHPt: 1800}
	}
	if templateHPt >= policy.MinHPt && templateHPt <= policy.MaxHPt {
		return templateHPt, policy
	}
	return policy.TargetHPt, policy
}

func normalizeBodyTypography(shape *shapeXML, cfg *autofitConfig) {
	if !cfg.bodyTypography || shape.TextBody == nil || len(shape.TextBody.Paragraphs) == 0 {
		return
	}
	cfg.bodySourceHPt = extractFontSizeFromShape(shape)
	if cfg.bodySourceHPt == 0 && cfg.inherited != nil {
		cfg.bodySourceHPt = cfg.inherited.SizeHPt
	}
	// Display-size placeholders are deliberate design elements, not regular
	// body copy. Preserve their declared style and usual measured fit.
	if cfg.bodySourceHPt > 4000 {
		cfg.bodyTypography = false
		return
	}
	cfg.bodyBaseHPt, cfg.bodyPolicy = BodySizeForDensity(cfg.bodySourceHPt, len(shape.TextBody.Paragraphs))
	if cfg.bodyBaseHPt != cfg.bodySourceHPt {
		setBodyRunSizes(shape, cfg.bodyBaseHPt)
	}
}

func shouldNormalizeBodyTypography(shape *shapeXML, item ContentItem) bool {
	ph := shape.NonVisualProperties.NvPr.Placeholder
	if item.FontSize != 0 || ph == nil || (ph.Type != "" && ph.Type != "body" && ph.Type != "obj") ||
		isTitleShape(shape) || isTitlePlaceholder(item.PlaceholderID) || strings.Contains(strings.ToLower(item.PlaceholderID), "subtitle") {
		return false
	}
	switch item.Type {
	case ContentText, ContentBullets, ContentBodyAndBullets, ContentBulletGroups:
		return true
	default:
		return false
	}
}

// setBodyRunSizes gives every populated run a concrete size so master and
// layout inheritance cannot reintroduce the off-target font at render time.
// Deeper bullet levels retain a 2pt hierarchy relative to the first level.
func setBodyRunSizes(shape *shapeXML, baseHPt int) {
	if shape.TextBody == nil {
		return
	}
	baseLevel := 0
	foundLevel := false
	for _, para := range shape.TextBody.Paragraphs {
		if para.Properties != nil && para.Properties.Level != nil {
			level := *para.Properties.Level
			if !foundLevel || level < baseLevel {
				baseLevel = level
				foundLevel = true
			}
		}
	}
	for i := range shape.TextBody.Paragraphs {
		para := &shape.TextBody.Paragraphs[i]
		size := baseHPt
		if foundLevel && para.Properties != nil && para.Properties.Level != nil && *para.Properties.Level > baseLevel {
			size -= (*para.Properties.Level - baseLevel) * 200
			if size < 1000 {
				size = 1000
			}
		}
		for j := range para.Runs {
			run := &para.Runs[j]
			if run.RunProperties == nil {
				run.RunProperties = &runPropertiesXML{Lang: "en-US"}
			}
			run.RunProperties.FontSize = fmt.Sprint(size)
		}
		if para.Properties != nil && para.Properties.Inner != "" {
			para.Properties.Inner = szRegexp.ReplaceAllString(para.Properties.Inner, fmt.Sprintf(`sz="%d"`, size))
		}
	}
	// A list style with explicit sizes can size bullet glyphs independently of
	// their text. Bring it into the same range; individual runs above retain the
	// deeper-level hierarchy.
	if shape.TextBody.ListStyle != nil && strings.Contains(shape.TextBody.ListStyle.Inner, `sz="`) {
		shape.TextBody.ListStyle.Inner = szRegexp.ReplaceAllString(shape.TextBody.ListStyle.Inner, fmt.Sprintf(`sz="%d"`, baseHPt))
	}
}

// emitBodySizeFinding distinguishes an automatic template correction from a
// genuinely undersized result after content-aware autofit.
func emitBodySizeFinding(cfg *autofitConfig, scale int) {
	if cfg.findings == nil || !cfg.bodyTypography || cfg.bodyBaseHPt == 0 {
		return
	}
	if finding := NewBodySizeFinding(cfg.findingPath, cfg.bodySourceHPt, cfg.bodyBaseHPt, cfg.bodyPolicy, scale); finding != nil {
		*cfg.findings = append(*cfg.findings, *finding)
	}
}

// NewBodySizeFinding is shared by render and preflight so a template-native
// outlier receives the same code, range, and remedy on either path.
func NewBodySizeFinding(path string, sourceHPt, baseHPt int, policy BodySizePolicy, scale int) *patterns.FitFinding {
	if baseHPt <= 0 {
		return nil
	}
	if scale <= 0 {
		scale = 100000
	}
	effectiveHPt := baseHPt * scale / 100000
	normalized := sourceHPt != baseHPt
	offTarget := effectiveHPt < policy.MinHPt || effectiveHPt > policy.MaxHPt
	if !normalized && !offTarget {
		return nil
	}
	action := "info"
	message := fmt.Sprintf("template body size %.1fpt normalised to %.1fpt for content density", float64(sourceHPt)/100, float64(baseHPt)/100)
	if sourceHPt == 0 {
		message = fmt.Sprintf("unresolved template body size normalised to %.1fpt for content density", float64(baseHPt)/100)
	}
	var fix *patterns.FixSuggestion
	if offTarget {
		action = "review"
		message = fmt.Sprintf("body text fits at %.1fpt, outside the %.1f–%.1fpt target range for content density", float64(effectiveHPt)/100, float64(policy.MinHPt)/100, float64(policy.MaxHPt)/100)
		fix = &patterns.FixSuggestion{Kind: "reduce_text", Params: map[string]any{
			"strategy": "split", "actual_pt": float64(effectiveHPt) / 100,
			"min_pt": float64(policy.MinHPt) / 100,
		}}
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: path, Code: patterns.ErrCodeTextSizeOffTarget,
			Message: message, Fix: fix,
		},
		Action: action,
	}
}
