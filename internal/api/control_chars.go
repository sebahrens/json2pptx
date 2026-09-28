package api

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/sebahrens/json2pptx/internal/bidisafe"
)

// neutralizeRequestControlChars removes invisible bidi override / isolate
// controls (U+202A-U+202E, U+2066-U+2069) and byte-order marks (U+FEFF) from
// every string in req and records one INPUT_CONTROL_CHARS_REMOVED warning per
// changed value (go-slide-creator-7oz3c). The request was decoded from JSON,
// so a JSON round trip is lossless; it only runs when a character is present.
func neutralizeRequestControlChars(req *ConvertRequest) {
	raw, err := json.Marshal(req)
	if err != nil || !bidisafe.MayContain(raw) {
		return
	}
	clean, sites := bidisafe.Neutralize(raw)
	if len(sites) == 0 {
		return
	}
	var cleaned ConvertRequest
	if err := json.Unmarshal(clean, &cleaned); err != nil {
		slog.Warn("control-character neutralisation round trip failed", "error", err)
		return
	}
	warnings := make([]string, 0, len(sites))
	for _, s := range sites {
		warnings = append(warnings, fmt.Sprintf("INPUT_CONTROL_CHARS_REMOVED: removed %d invisible bidi control / byte-order-mark character(s) from %s", s.Removed, s.Path))
	}
	cleaned.controlCharWarnings = warnings
	*req = cleaned
}
