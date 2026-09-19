package patterns

import (
	"sort"
	"strings"
)

// Read accessors for Schema (go-slide-creator-20jm).
//
// Schema is constructor-built and its backing struct is private, which was
// fine while the only consumer was json.Marshal for discovery output. Pattern
// input diagnostics need to READ the schema — to name the shape a field should
// have without leaking Go type names into agent-facing messages — so the
// keywords the diagnostics use are exposed here. Kept read-only on purpose:
// schemas stay constructor-built.

// TypeName returns the schema's "type" keyword, or "" when unset (which a
// oneOf branch schema or a bare $ref legitimately leaves empty).
func (s *Schema) TypeName() SchemaType {
	if s == nil {
		return ""
	}
	return s.raw.Type
}

// Description returns the schema's description, or "".
func (s *Schema) Description() string {
	if s == nil {
		return ""
	}
	return s.raw.Description
}

// Properties returns the schema's declared object properties. The returned map
// is the live one; callers must not mutate it.
func (s *Schema) Properties() map[string]*Schema {
	if s == nil {
		return nil
	}
	return s.raw.Properties
}

// PropertyNames returns the declared property names, sorted, so messages and
// did-you-mean suggestions are deterministic across runs.
func (s *Schema) PropertyNames() []string {
	if s == nil || len(s.raw.Properties) == 0 {
		return nil
	}
	out := make([]string, 0, len(s.raw.Properties))
	for k := range s.raw.Properties {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Property returns the schema for one declared property, or nil.
func (s *Schema) Property(name string) *Schema {
	if s == nil {
		return nil
	}
	return s.raw.Properties[name]
}

// RequiredNames returns the schema's "required" list (a copy).
func (s *Schema) RequiredNames() []string {
	if s == nil || len(s.raw.Required) == 0 {
		return nil
	}
	return append([]string(nil), s.raw.Required...)
}

// IsRequired reports whether name appears in the schema's required list.
func (s *Schema) IsRequired(name string) bool {
	if s == nil {
		return false
	}
	for _, r := range s.raw.Required {
		if r == name {
			return true
		}
	}
	return false
}

// ItemSchema returns the schema for array items, or nil.
func (s *Schema) ItemSchema() *Schema {
	if s == nil {
		return nil
	}
	return s.raw.Items
}

// OneOfBranches returns the schema's oneOf alternatives, or nil. A pattern uses
// oneOf to document a tolerated shorthand ("$4.2M | ARR" as well as {big,
// small}), so any shape check has to consider every branch before deciding a
// value is wrong.
func (s *Schema) OneOfBranches() []*Schema {
	if s == nil {
		return nil
	}
	return s.raw.OneOf
}

// EnumValues returns the schema's enum values (a copy), or nil.
func (s *Schema) EnumValues() []string {
	if s == nil || len(s.raw.Enum) == 0 {
		return nil
	}
	return append([]string(nil), s.raw.Enum...)
}

// PatternPropertySchemas returns the schema's patternProperties map, used by
// cell_overrides ("^[0-9]+$"). The returned map is live; do not mutate.
func (s *Schema) PatternPropertySchemas() map[string]*Schema {
	if s == nil {
		return nil
	}
	return s.raw.PatternProperties
}

// Defs returns the schema's $defs map. Only a root schema carries one.
func (s *Schema) Defs() map[string]*Schema {
	if s == nil {
		return nil
	}
	return s.raw.Defs
}

// RefName returns the $defs entry name a $ref points at ("cellOverride" for
// "#/$defs/cellOverride"), or "" when the schema is not a local $defs ref.
func (s *Schema) RefName() string {
	if s == nil || s.raw.Ref == "" {
		return ""
	}
	return strings.TrimPrefix(s.raw.Ref, "#/$defs/")
}

// Deref resolves a local $defs reference against root, returning s unchanged
// when it is not a reference (or when the target is missing).
func (s *Schema) Deref(root *Schema) *Schema {
	if s == nil {
		return nil
	}
	name := s.RefName()
	if name == "" {
		return s
	}
	if target, ok := root.Defs()[name]; ok && target != nil {
		return target
	}
	return s
}
