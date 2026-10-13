package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

const templateDefaultSentinel = template.TemplateDefaultSentinel

// TableStyleResolver resolves authored table style_id values against a
// template's declared table styles.  The generator depends on this interface
// rather than a concrete template type so that tests can supply stubs and the
// table code remains decoupled from the template package.
type TableStyleResolver interface {
	// ResolveTableStyleID maps an authored style_id to the OOXML GUID that
	// should be written into <a:tableStyleId>.
	//
	// Contract:
	//   - "@template-default"                    → template's declared default GUID, or engine-default
	//   - "{GUID}" present in template index     → returned as-is
	//   - non-empty but unresolvable             → returned verbatim (downstream may warn)
	//   - ""                                     → template default, or engine-default
	ResolveTableStyleID(authored string) string
}

// TableStyleDefinitionChecker is an optional resolver capability. A GUID
// without formatting rules needs explicit cell styling even when the table
// requests use_table_style.
type TableStyleDefinitionChecker interface {
	HasTableStyleDefinition(id string) bool
}

// defaultTableStyleResolver is a no-op resolver that always returns the
// engine-default GUID.  Used when no template-aware resolver is available.
//
// Critically, it resolves the "@template-default" sentinel to the engine
// default GUID rather than leaking the sentinel into the OOXML output. Leaking
// the sentinel produces an invalid <a:tableStyleId>@template-default</a:tableStyleId>
// element that LibreOffice (and strict OOXML readers) reject because the
// element value must be an ST_Guid.
type defaultTableStyleResolver struct{}

func (defaultTableStyleResolver) ResolveTableStyleID(authored string) string {
	if authored == "" || authored == templateDefaultSentinel {
		return types.DefaultTableStyleID
	}
	return authored
}

// resolveTableAppearance preserves the author's style selection and overrides.
// Only a real definition may replace the explicit engine fallback.
func resolveTableAppearance(config *TableRenderConfig) {
	resolver := config.StyleResolver
	if resolver == nil {
		config.Style.StyleID = defaultTableStyleResolver{}.ResolveTableStyleID(config.Style.StyleID)
		config.templateStyle = config.Style.UseTableStyle
		return
	}
	config.Style.StyleID = resolver.ResolveTableStyleID(config.Style.StyleID)
	definitionKnown := false
	if checker, ok := resolver.(TableStyleDefinitionChecker); ok {
		definitionKnown = true
		config.templateStyle = checker.HasTableStyleDefinition(config.Style.StyleID)
	} else if _, standalone := resolver.(defaultTableStyleResolver); !standalone {
		config.templateStyle = config.Style.UseTableStyle
	} else {
		definitionKnown = true
	}
	if config.templateStyle {
		config.engineDefault = false
	} else if definitionKnown || config.Style.UseTableStyle || config.Style.StyleID == resolver.ResolveTableStyleID("") {
		config.Style.UseTableStyle = false
		config.engineDefault = true
	}
}

// templateTableStyleResolver reads the same open ZIP used to build the deck,
// only when a table needs template style resolution or definition lookup.
// This prevents a concurrent template replacement from changing its style.
func (ctx *singlePassContext) templateTableStyleResolver() (TableStyleResolver, error) {
	if ctx.tableStyleReader != nil {
		return ctx.tableStyleReader, nil
	}
	if ctx.templateReader == nil {
		return nil, fmt.Errorf("template ZIP is not open")
	}
	ctx.tableStyleReader = template.BorrowOpenZIP(ctx.templatePath, ctx.templateReader)
	return ctx.tableStyleReader, nil
}

// emittedTableStyleID excludes invalid authored IDs from output bookkeeping.
func emittedTableStyleID(id string) string {
	if types.IsValidTableStyleID(id) {
		return id
	}
	return ""
}
