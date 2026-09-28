package semantic

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// typedDecodeDiagnostics turns a failed typed decode of rawDeck into a
// path-scoped diagnostic. The decoder's own message leaked Go internals —
// "cannot unmarshal bool into Go struct field ChromeSpec.meta.chrome.page_numbers
// of type semantic.PageNumbersSpec" — which names no field the author wrote in
// the form they wrote it (go-slide-creator-6p9mm). root is the generic decode
// of the same document; YAML decode errors are re-derived through JSON so both
// syntaxes get the same field path.
func typedDecodeDiagnostics(root any, err error) Diagnostics {
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) {
		if encoded, merr := json.Marshal(root); merr == nil {
			var raw rawDeck
			if jerr := json.Unmarshal(encoded, &raw); jerr == nil || !errors.As(jerr, &typeErr) {
				return parseErrorDiagnostics(err)
			}
		} else {
			return parseErrorDiagnostics(err)
		}
	}
	path := typeErr.Field
	if path == "" {
		return parseErrorDiagnostics(err)
	}
	got, found := lookupDecodedPath(root, path)
	gotName := jsonShapeName(got)
	if !found {
		gotName = "a " + typeErr.Value
	}
	msg := fmt.Sprintf("%s must be %s, got %s", path, describeExpectedType(typeErr.Type), gotName)
	if hint := didYouMeanForType(typeErr.Type, got); hint != "" {
		msg += "; did you mean " + hint + "?"
	}
	code := CodeParseError
	if strings.HasPrefix(path, "meta.") {
		code = CodeInvalidMeta
	}
	var ds Diagnostics
	ds.add(path, code, msg)
	return ds
}

// lookupDecodedPath follows a dotted field path through a generic decode.
func lookupDecodedPath(root any, path string) (any, bool) {
	node := root
	for _, part := range strings.Split(path, ".") {
		m, ok := node.(map[string]any)
		if !ok {
			return nil, false
		}
		if node, ok = m[part]; !ok {
			return nil, false
		}
	}
	return node, true
}

// describeExpectedType names a Go decode target in JSON terms; an object lists
// its fields so the author can see the shape without the schema.
func describeExpectedType(t reflect.Type) string {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil {
		return "a different type"
	}
	switch t.Kind() {
	case reflect.Struct:
		if fields := jsonFieldNames(t); len(fields) > 0 {
			return "an object with keys " + strings.Join(fields, ", ")
		}
		return "an object"
	case reflect.Map:
		return "an object"
	case reflect.Slice, reflect.Array:
		return "an array"
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "a boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "a number"
	default:
		return "a different type"
	}
}

func jsonFieldNames(t reflect.Type) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// didYouMeanForType suggests the object form of a boolean written where an
// on/off object is expected (page_numbers: true → {"enabled": true}).
func didYouMeanForType(t reflect.Type, got any) string {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	b, isBool := got.(bool)
	if t == nil || t.Kind() != reflect.Struct || !isBool {
		return ""
	}
	for i := 0; i < t.NumField(); i++ {
		if strings.Split(t.Field(i).Tag.Get("json"), ",")[0] == "enabled" {
			return fmt.Sprintf(`{"enabled": %t}`, b)
		}
	}
	return ""
}
