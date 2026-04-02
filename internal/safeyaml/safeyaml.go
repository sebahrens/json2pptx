// Package safeyaml provides secure YAML parsing with size and depth limits.
// This protects against YAML bombs (billion laughs attack), memory exhaustion
// through deeply nested structures, and CPU exhaustion through recursive parsing.
//
// See: CWE-776 (Improper Restriction of Recursive Entity References)
// See: OWASP API4:2023 - Unrestricted Resource Consumption
package safeyaml

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Default limits for YAML parsing.
const (
	// DefaultMaxSize is the maximum YAML content size in bytes (64KB).
	// This is sufficient for typical chart/diagram definitions while
	// preventing memory exhaustion attacks.
	DefaultMaxSize = 64 * 1024

	// DefaultMaxDepth is the maximum nesting depth allowed (20 levels).
	// Charts and diagrams rarely need more than 5-6 levels of nesting.
	DefaultMaxDepth = 20

	// DefaultMaxAliases is the maximum number of YAML aliases allowed (50).
	// This prevents billion laughs attacks using alias expansion.
	DefaultMaxAliases = 50
)

// Common errors returned by safe YAML parsing functions.
var (
	// ErrYAMLTooLarge is returned when YAML content exceeds MaxSize.
	ErrYAMLTooLarge = errors.New("YAML content exceeds maximum allowed size")

	// ErrYAMLTooDeep is returned when YAML nesting exceeds MaxDepth.
	ErrYAMLTooDeep = errors.New("YAML content exceeds maximum nesting depth")

	// ErrYAMLTooManyAliases is returned when alias count exceeds MaxAliases.
	ErrYAMLTooManyAliases = errors.New("YAML content contains too many aliases")
)

// Limits defines the constraints for safe YAML parsing.
type Limits struct {
	// MaxSize is the maximum content size in bytes.
	MaxSize int

	// MaxDepth is the maximum nesting depth.
	MaxDepth int

	// MaxAliases is the maximum number of YAML aliases.
	MaxAliases int
}

// DefaultLimits returns the default safe parsing limits.
func DefaultLimits() Limits {
	return Limits{
		MaxSize:    DefaultMaxSize,
		MaxDepth:   DefaultMaxDepth,
		MaxAliases: DefaultMaxAliases,
	}
}

// Unmarshal safely parses YAML content with default limits.
// It validates size before parsing and checks depth after parsing.
func Unmarshal(data []byte, v interface{}) error {
	return UnmarshalWithLimits(data, v, DefaultLimits())
}

// UnmarshalString safely parses a YAML string with default limits.
func UnmarshalString(s string, v interface{}) error {
	return Unmarshal([]byte(s), v)
}

// UnmarshalWithLimits safely parses YAML content with custom limits.
func UnmarshalWithLimits(data []byte, v interface{}, limits Limits) error {
	// Check size limit before parsing
	if len(data) > limits.MaxSize {
		return fmt.Errorf("%w: %d bytes exceeds limit of %d bytes",
			ErrYAMLTooLarge, len(data), limits.MaxSize)
	}

	// Parse into yaml.Node first to check structure
	var node yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&node); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	// Validate depth and alias count
	aliasCount := 0
	if err := validateNode(&node, 0, limits.MaxDepth, &aliasCount, limits.MaxAliases); err != nil {
		return err
	}

	// Now unmarshal to target type
	return yaml.Unmarshal(data, v)
}

// validateNode recursively checks YAML node depth and alias count.
func validateNode(node *yaml.Node, depth int, maxDepth int, aliasCount *int, maxAliases int) error {
	if node == nil {
		return nil
	}

	// Check depth
	if depth > maxDepth {
		return fmt.Errorf("%w: depth %d exceeds limit of %d",
			ErrYAMLTooDeep, depth, maxDepth)
	}

	// Count aliases
	if node.Kind == yaml.AliasNode {
		*aliasCount++
		if *aliasCount > maxAliases {
			return fmt.Errorf("%w: %d aliases exceeds limit of %d",
				ErrYAMLTooManyAliases, *aliasCount, maxAliases)
		}
	}

	// Recursively check children
	for _, child := range node.Content {
		if err := validateNode(child, depth+1, maxDepth, aliasCount, maxAliases); err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalStrict safely parses YAML content with default limits and strict mode.
// In strict mode, unknown fields cause an error (uses KnownFields(true)).
func UnmarshalStrict(data []byte, v interface{}) error {
	return UnmarshalStrictWithLimits(data, v, DefaultLimits())
}

// UnmarshalStrictWithLimits safely parses YAML with custom limits and strict mode.
// Unknown fields will cause an error when strict mode is enabled.
func UnmarshalStrictWithLimits(data []byte, v interface{}, limits Limits) error {
	// Check size limit before parsing
	if len(data) > limits.MaxSize {
		return fmt.Errorf("%w: %d bytes exceeds limit of %d bytes",
			ErrYAMLTooLarge, len(data), limits.MaxSize)
	}

	// Parse into yaml.Node first to check structure
	var node yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&node); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	// Validate depth and alias count
	aliasCount := 0
	if err := validateNode(&node, 0, limits.MaxDepth, &aliasCount, limits.MaxAliases); err != nil {
		return err
	}

	// Now unmarshal with strict mode using a new decoder
	strictDecoder := yaml.NewDecoder(strings.NewReader(string(data)))
	strictDecoder.KnownFields(true)
	return strictDecoder.Decode(v)
}

// ValidateSize checks if YAML content size is within limits.
// This can be used for early rejection before parsing.
func ValidateSize(data []byte, maxSize int) error {
	if len(data) > maxSize {
		return fmt.Errorf("%w: %d bytes exceeds limit of %d bytes",
			ErrYAMLTooLarge, len(data), maxSize)
	}
	return nil
}

// ValidateSizeString checks if a YAML string size is within limits.
func ValidateSizeString(s string, maxSize int) error {
	return ValidateSize([]byte(s), maxSize)
}

// ExtractMapKeyOrder parses YAML and returns the ordered keys of the map
// at the given top-level field name. This preserves the source document
// ordering that is lost when unmarshaling to map[string]interface{}.
// Returns nil if the field is not found or is not a mapping node.
func ExtractMapKeyOrder(yamlContent string, fieldName string) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		return nil
	}
	// doc is a document node; its first child is the root mapping
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	// Find the field in key-value pairs
	for i := 0; i+1 < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valNode := root.Content[i+1]
		if keyNode.Value == fieldName && valNode.Kind == yaml.MappingNode {
			keys := make([]string, 0, len(valNode.Content)/2)
			for j := 0; j+1 < len(valNode.Content); j += 2 {
				keys = append(keys, valNode.Content[j].Value)
			}
			return keys
		}
	}
	return nil
}
