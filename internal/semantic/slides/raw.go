package slides

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// CompileRaw compiles a raw_json2pptx escape-hatch slide by decoding its "slide"
// payload directly into a deckinput.SlideInput, bypassing the semantic
// abstraction. The whole slide maps coarsely back to the semantic source so a
// downstream finding still resolves to the author's escape-hatch block.
func CompileRaw(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	rawSlide, ok := in.Body["slide"]
	if !ok {
		return nil, nil, fmt.Errorf("raw_json2pptx slide is missing its %q payload", "slide")
	}
	encoded, err := json.Marshal(rawSlide)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal raw slide payload: %w", err)
	}
	var slide deckinput.SlideInput
	if err := json.Unmarshal(encoded, &slide); err != nil {
		// Don't surface the raw json.Unmarshal error: it names the internal Go
		// target type (deckinput.SlideInput), which leaks an implementation
		// detail to the caller. validateRawEscapeHatch blocks non-object/invalid
		// payloads before compile, so this is defensive; keep the message in the
		// author's vocabulary ("slide" payload) rather than Go's.
		return nil, nil, fmt.Errorf("raw_json2pptx %q payload is not a valid json2pptx slide object", "slide")
	}
	links := []SourceLink{{
		RawPath:      in.rawSlide(),
		SemanticPath: in.semSlide() + ".slide",
	}}
	return &slide, links, nil
}
