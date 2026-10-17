package main

import (
	"encoding/json"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// rawRenderResponse exposes the semantic field names while retaining the
// original raw keys for the compatibility cycle. It is a transport wrapper:
// JSONOutput is also embedded in CLI failures that carry their own envelope.
type rawRenderResponse struct {
	JSONOutput
	OK             bool                  `json:"ok"`
	PPTXPath       string                `json:"pptx_path,omitempty"`
	QualitySummary *QualityScore         `json:"quality_summary,omitempty"`
	Diagnostics    []patterns.FitFinding `json:"diagnostics"`
}

func canonicalRawRender(o JSONOutput) rawRenderResponse {
	findings := o.FitFindings
	if findings == nil {
		findings = []patterns.FitFinding{}
	}
	return rawRenderResponse{o, o.Success, o.OutputPath, o.Quality, findings}
}

// MarshalJSON puts the validation envelope at the top level, as on the
// semantic path. legacy_findings preserves the former nested envelope for
// clients migrating during this schema cycle.
func (o dryRunOutput) MarshalJSON() ([]byte, error) {
	type original dryRunOutput
	return json.Marshal(struct {
		original
		diagnostics.FindingEnvelope
	}{original(o), o.Findings})
}
