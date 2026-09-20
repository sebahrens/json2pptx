package generator

import (
	"encoding/xml"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// TitleFitContext holds the state for measured title fitting: the theme's
// heading font and a per-master cache of inherited title styles.
type TitleFitContext struct {
	titleFontName   string                                 // theme major (heading) font
	titleStyleCache map[string]template.InheritedTextStyle // masterPath -> inherited title style (lazy)
	bodyStyleCache  map[string]template.InheritedTextStyle // masterPath -> inherited body style (lazy)
	viewingMode     tokens.ViewingMode                     // readability policy mode (go-slide-creator-vbic)
	// masterXMLCache holds raw master XML per path, for the inherited-color
	// resolution the contrast pass needs (go-slide-creator-ucmgr).
	masterXMLCache map[string][]byte
}

// masterPathForLayout returns the ZIP path of the slide master a layout
// inherits from, or "" when it cannot be resolved.
func (ctx *singlePassContext) masterPathForLayout(layoutID string) string {
	relsData, err := ctx.readFileWithSyntheticFallback(LayoutRelsPath(layoutID))
	if err != nil {
		return ""
	}
	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		return ""
	}
	for _, rel := range rels.Relationships {
		if rel.Type == pptx.RelTypeSlideMaster {
			return template.ResolveRelativePath(strings.TrimSuffix(PathSlideLayouts, "/"), rel.Target)
		}
	}
	return ""
}

// inheritedTitleStyle returns the title text style (size, caps, line spacing,
// typeface) the layout's title placeholder inherits from its slide master.
// Results are cached per master. ok is false when the master has no usable
// title style.
func (ctx *singlePassContext) inheritedTitleStyle(layoutID string) (template.InheritedTextStyle, bool) {
	masterPath := ctx.masterPathForLayout(layoutID)
	if masterPath == "" {
		return template.InheritedTextStyle{}, false
	}
	if ctx.titleStyleCache == nil {
		ctx.titleStyleCache = make(map[string]template.InheritedTextStyle)
	}
	st, cached := ctx.titleStyleCache[masterPath]
	if !cached {
		if data, err := utils.ReadFileFromZipIndex(ctx.templateIndex, masterPath); err == nil {
			st = template.ParseMasterTitleStyle(data)
		}
		ctx.titleStyleCache[masterPath] = st
	}
	return st, st.SizeHPt > 0
}

// titleAutofitOptions returns the autofit options for a title placeholder on
// the given layout: the inherited master title style (so measured fit uses the
// real size / caps / line spacing) and the title role (TITLE_OVERFLOW instead
// of paragraph trimming).
func (ctx *singlePassContext) titleAutofitOptions(layoutID string) []autofitOption {
	opts := []autofitOption{withTitleRole()}
	if st, ok := ctx.inheritedTitleStyle(layoutID); ok {
		fontName := resolveStyleFontName(st.Typeface, ctx.titleFontName, ctx.themeFontName)
		if fontName == "" {
			fontName = ctx.titleFontName
		}
		opts = append(opts, withInheritedTextStyle(st, fontName))
	}
	return opts
}

// inheritedBodyStyle returns the body text style (size, line spacing,
// space-before, typeface) the layout's body placeholders inherit from their
// slide master. Cached per master alongside the title style.
//
// Without it the body autofit guessed: no size (textfit fell back to its own
// default) and a flat 12pt of per-paragraph spacing where midnight-blue's
// master declares 8pt. The guess is what made validate and generate predict
// different font scales for the same fourteen bullets (go-slide-creator-nlrg).
func (ctx *singlePassContext) inheritedBodyStyle(layoutID string) (template.InheritedTextStyle, bool) {
	masterPath := ctx.masterPathForLayout(layoutID)
	if masterPath == "" {
		return template.InheritedTextStyle{}, false
	}
	if ctx.bodyStyleCache == nil {
		ctx.bodyStyleCache = make(map[string]template.InheritedTextStyle)
	}
	st, cached := ctx.bodyStyleCache[masterPath]
	if !cached {
		if data, err := utils.ReadFileFromZipIndex(ctx.templateIndex, masterPath); err == nil {
			st = template.ParseMasterBodyStyle(data)
		}
		ctx.bodyStyleCache[masterPath] = st
	}
	// A master's bodyStyle often declares spacing and line height without a
	// size, and the spacing alone changes the measured fit.
	return st, st.SizeHPt > 0 || st.SpcBefPt > 0 || st.LineSpacingPct > 0
}

// bodyAutofitOptions returns the autofit options for a body placeholder on the
// given layout: the inherited master body style, so the measured fit uses the
// real size, line spacing and space-before instead of defaults.
func (ctx *singlePassContext) bodyAutofitOptions(layoutID string) []autofitOption {
	st, ok := ctx.inheritedBodyStyle(layoutID)
	if !ok {
		return nil
	}
	fontName := resolveStyleFontName(st.Typeface, ctx.themeFontName, ctx.themeFontName)
	return []autofitOption{withInheritedTextStyle(st, fontName)}
}

// isTitleShape reports whether a slide shape is a title placeholder
// (type="title" or "ctrTitle").
func isTitleShape(shape *shapeXML) bool {
	ph := shape.NonVisualProperties.NvPr.Placeholder
	if ph == nil {
		return false
	}
	return ph.Type == "title" || ph.Type == "ctrTitle"
}
