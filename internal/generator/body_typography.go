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

// LeadStepHPt is the scale step between a body_and_lead lead paragraph and
// its supporting bullets; LeadMinHPt is the lead's floor.
const (
	LeadStepHPt = 200
	LeadMinHPt  = 1600
)

// LeadSizeFor returns the lead-paragraph size for a bullet base size.
func LeadSizeFor(bulletHPt int) int {
	lead := bulletHPt + LeadStepHPt
	if lead < LeadMinHPt {
		lead = LeadMinHPt
	}
	return lead
}

// applyLeadParagraphSizes sizes the first cfg.leadParagraphs paragraphs one
// step above the size the bullets actually render at: their explicit run
// size (density-normalised or authored), else the shape list style's size
// for their level, else the density base or the inherited size.
func applyLeadParagraphSizes(shape *shapeXML, cfg *autofitConfig) {
	if cfg.leadParagraphs <= 0 || shape.TextBody == nil {
		return
	}
	rest := shape.TextBody.Paragraphs[min(cfg.leadParagraphs, len(shape.TextBody.Paragraphs)):]
	base := extractFontSizeFromParagraphs(rest)
	if base == 0 && len(rest) > 0 && shape.TextBody.ListStyle != nil {
		level := 0
		if rest[0].Properties != nil && rest[0].Properties.Level != nil {
			level = *rest[0].Properties.Level
		}
		base = listStyleLevelSize(shape.TextBody.ListStyle.Inner, level)
	}
	if base == 0 {
		base = cfg.bodyBaseHPt
	}
	if base == 0 {
		base = cfg.authoredFontSizeHPt
	}
	if base == 0 && cfg.inherited != nil {
		base = cfg.inherited.SizeHPt
	}
	if base == 0 {
		return
	}
	size := fmt.Sprint(LeadSizeFor(base))
	for i := 0; i < cfg.leadParagraphs && i < len(shape.TextBody.Paragraphs); i++ {
		para := &shape.TextBody.Paragraphs[i]
		for j := range para.Runs {
			if para.Runs[j].RunProperties == nil {
				para.Runs[j].RunProperties = &runPropertiesXML{Lang: "en-US"}
			}
			para.Runs[j].RunProperties.FontSize = size
		}
	}
}

// listStyleLevelSize returns the defRPr size declared for a 0-based level
// in a list style, or 0.
func listStyleLevelSize(inner string, level int) int {
	tag := fmt.Sprintf("a:lvl%dpPr", level+1)
	start := strings.Index(inner, "<"+tag)
	if start < 0 {
		return 0
	}
	block := inner[start:]
	if end := strings.Index(block, "</"+tag+">"); end >= 0 {
		block = block[:end]
	}
	return parseSzAttr(block)
}

// extractFontSizeFromParagraphs returns the first explicit run size, or 0.
func extractFontSizeFromParagraphs(paras []paragraphXML) int {
	for _, p := range paras {
		for _, r := range p.Runs {
			if r.RunProperties != nil && r.RunProperties.FontSize != "" {
				var v int
				if _, err := fmt.Sscan(r.RunProperties.FontSize, &v); err == nil && v > 0 {
					return v
				}
			}
		}
	}
	return 0
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
		classifyShapeRole(shape) == RoleDisclosure || isTitleShape(shape) || isTitlePlaceholder(item.PlaceholderID) || strings.Contains(strings.ToLower(item.PlaceholderID), "subtitle") {
		return false
	}
	switch item.Type {
	case ContentText, ContentBullets, ContentBodyAndBullets, ContentBodyAndLead, ContentBulletGroups:
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
