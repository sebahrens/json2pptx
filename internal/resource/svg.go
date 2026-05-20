package resource

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// SVGValidationError is a structured error produced when SVG content fails
// the strict XML safety / shape checks performed before caching. Callers can
// retrieve the diagnostic Code via errors.As so MCP / CLI responses surface
// the precise reason (SVG_INVALID_ROOT, SVG_UNSAFE_XML, SVG_PARSE_ERROR)
// rather than a generic URL_FETCH_FAILED.
type SVGValidationError struct {
	Code    diagnostics.Code
	Message string
	Cause   error
}

func (e *SVGValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *SVGValidationError) Unwrap() error { return e.Cause }

// SVGValidationCode extracts the structured diagnostic Code from err if it
// (or anything it wraps) is a *SVGValidationError. Returns "" when not found.
func SVGValidationCode(err error) diagnostics.Code {
	var sve *SVGValidationError
	if errors.As(err, &sve) {
		return sve.Code
	}
	return ""
}

// validateSVG parses data with a strict XML decoder and rejects payloads that
// are not well-formed, declare a DOCTYPE (the carrier for entity expansion /
// XXE attacks), or whose root element is not <svg>. It is the only validator
// gating SVG bytes into the resource cache.
func validateSVG(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return &SVGValidationError{
			Code:    diagnostics.CodeSVGParseError,
			Message: "content is empty",
		}
	}
	// xml.Decoder happily accepts text-only payloads as a stream of CharData
	// (no error, no StartElement). Require an XML-shaped prefix so that
	// plain text surfaces as SVG_PARSE_ERROR rather than SVG_INVALID_ROOT.
	if trimmed[0] != '<' {
		return &SVGValidationError{
			Code:    diagnostics.CodeSVGParseError,
			Message: "content does not begin with an XML tag",
		}
	}

	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	// Entity == nil keeps the decoder to the 5 predefined XML entities
	// (amp, lt, gt, apos, quot). Any custom entity reference will surface
	// as a parse error rather than silently expanding.
	dec.Entity = nil

	sawRoot := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return &SVGValidationError{
				Code:    diagnostics.CodeSVGParseError,
				Message: "malformed XML",
				Cause:   err,
			}
		}

		switch t := tok.(type) {
		case xml.Directive:
			// xml.Directive carries the body of <!...> markup without the
			// leading <! or trailing >. We reject anything declaring a
			// DOCTYPE (entity expansion / external subset / XXE carrier).
			trimmed := bytes.TrimSpace(t)
			if hasPrefixFold(trimmed, []byte("DOCTYPE")) {
				return &SVGValidationError{
					Code:    diagnostics.CodeSVGUnsafeXML,
					Message: "DOCTYPE declarations are not permitted in SVG content",
				}
			}
			if hasPrefixFold(trimmed, []byte("ENTITY")) {
				return &SVGValidationError{
					Code:    diagnostics.CodeSVGUnsafeXML,
					Message: "ENTITY declarations are not permitted in SVG content",
				}
			}
		case xml.StartElement:
			if !sawRoot {
				if t.Name.Local != "svg" {
					return &SVGValidationError{
						Code:    diagnostics.CodeSVGInvalidRoot,
						Message: fmt.Sprintf("root element is %q, expected \"svg\"", t.Name.Local),
					}
				}
				sawRoot = true
			}
		}
	}

	if !sawRoot {
		return &SVGValidationError{
			Code:    diagnostics.CodeSVGInvalidRoot,
			Message: "no <svg> root element found",
		}
	}
	return nil
}

// hasPrefixFold reports whether b begins with prefix using ASCII
// case-insensitive comparison. Used to match XML directive keywords
// (DOCTYPE / ENTITY) that XML allows in either case.
func hasPrefixFold(b, prefix []byte) bool {
	if len(b) < len(prefix) {
		return false
	}
	for i := range prefix {
		bc := b[i]
		pc := prefix[i]
		if bc >= 'a' && bc <= 'z' {
			bc -= 'a' - 'A'
		}
		if pc >= 'a' && pc <= 'z' {
			pc -= 'a' - 'A'
		}
		if bc != pc {
			return false
		}
	}
	return true
}
