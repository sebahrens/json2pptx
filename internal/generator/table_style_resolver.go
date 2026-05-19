package generator

import "github.com/sebahrens/json2pptx/internal/types"

// templateDefaultSentinel mirrors template.TemplateDefaultSentinel so the
// resolver can recognise the sentinel without importing the template package
// (which would otherwise pull in a transitive dependency for callers that only
// need the in-package default resolver).
const templateDefaultSentinel = "@template-default"

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
	//   - ""                                     → engine-default (types.DefaultTableStyleID)
	ResolveTableStyleID(authored string) string
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
