package template

import "github.com/sebahrens/json2pptx/internal/types"

// TemplateDefaultSentinel is the magic value that JSON authors write as
// style_id to opt into the template's declared default table style.
const TemplateDefaultSentinel = "@template-default"

// ResolveTableStyleID maps an authored style_id to the OOXML GUID that should
// be written into <a:tableStyleId>.
//
// Resolution rules:
//   - "@template-default" → declared default from tableStyles.xml def attr,
//     falling back to types.DefaultTableStyleID if the template has none.
//   - "{GUID}" present in the template's index → returned as-is (validated).
//   - Non-empty but unresolvable → returned verbatim so downstream can warn.
//   - "" → types.DefaultTableStyleID (engine-default).
func (r *Reader) ResolveTableStyleID(authored string) string {
	if authored == "" {
		return types.DefaultTableStyleID
	}

	if r.tblStyles == nil {
		r.tblStyles = newTableStyleIndex(r)
	}

	if authored == TemplateDefaultSentinel {
		if guid, ok := r.tblStyles.declaredDefault(); ok {
			return guid
		}
		return types.DefaultTableStyleID
	}

	// GUID-shaped or arbitrary value: return as-is.
	// The index validates that declared GUIDs exist, but we don't reject
	// unknown values — they may still render correctly in PowerPoint.
	return authored
}
