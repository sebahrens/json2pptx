package quality

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestNativeEvidenceFreezesAuthoredChartBeforeGeneration(t *testing.T) {
	path := filepath.Join(testutil.TemplatesDir(), "modern-yellow.pptx")
	r, err := template.OpenTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	layouts, err := template.ParseLayouts(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	var probe nativeProbe
	for _, layout := range layouts {
		if layout.ID == "slideLayout3" {
			for _, p := range makeNativeProbes(layout, nativeReferenceImage()) {
				if p.Profile == "chart" {
					probe = p
				}
			}
		}
	}
	if probe.ID == "" {
		t.Fatal("native chart probe missing")
	}
	want, err := json.Marshal(probe.Slide)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := newNativeProbeEvidence(probe, 0)
	if err != nil {
		t.Fatal(err)
	}
	status, loss, err := classifyNativePublication(t, path, evidence.nativeProbe, "")
	if err != nil || status != nativeGenerated || loss != nil {
		t.Fatalf("actual generation failed: %s %v %v", status, loss, err)
	}
	effective, err := json.Marshal(evidence.Slide)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(want, effective) {
		t.Fatal("fixture did not exercise generation's derived input mutation")
	}
	charts := 0
	for _, item := range evidence.Slide.Content {
		if item.Type == generator.ContentDiagram {
			d := item.Value.(*types.DiagramSpec)
			if d.Width <= 0 || d.Height <= 0 {
				t.Fatalf("effective chart geometry missing: %+v", d)
			}
			charts++
		}
	}
	if charts != 1 {
		t.Fatalf("expected one chart, got %d", charts)
	}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fields["input"], want) {
		t.Fatal("authored input changed after actual generation")
	}
	if !bytes.Equal(fields["effective_input"], effective) {
		t.Fatal("derived geometry evidence lost")
	}
}

func TestNativeEvidenceRejectsMissingOrUnserializableSource(t *testing.T) {
	if _, err := json.Marshal(nativeProbeEvidence{}); err == nil {
		t.Fatal("missing authored input accepted")
	}
	probe := nativeProbe{ID: "invalid", Slide: generator.SlideSpec{Content: []generator.ContentItem{{Value: make(chan int)}}}}
	if _, err := newNativeProbeEvidence(probe, 0); err == nil {
		t.Fatal("unserializable authored input accepted")
	}
}
