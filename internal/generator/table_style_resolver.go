package generator

import "github.com/sebahrens/json2pptx/internal/types"

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
type defaultTableStyleResolver struct{}

func (defaultTableStyleResolver) ResolveTableStyleID(authored string) string {
	if authored == "" {
		return types.DefaultTableStyleID
	}
	return authored
}
