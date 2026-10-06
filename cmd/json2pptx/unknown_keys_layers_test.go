package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A misspelled key inside a layer is dropped by the decoder: the layer then
// renders at a zero frame or without its fill. Each is reported at its path.
func TestUnknownKeys_GridCellLayers(t *testing.T) {
	raw := json.RawMessage(`{"template":"t","slides":[{"shape_grid":{"rows":[{"cells":[
		{"layer":[], "layers":[
			{"frmae":{"x":0,"y":0,"w":1,"h":1}, "shape":{"geometry":"blockArc"}},
			{"name":"badge", "frame":{"x":0.1,"y":0.1,"width":0.2,"h":0.2}, "shape":{"geometry":"ellipse","fil":"accent1","adjustments":{"adj1":1}}}
		]},
		{"layers":[{"frame":{"x":0,"y":0,"w":1,"h":1},"shape":{"geometry":"ellipse"},"z":2}]}
	]}]}}]}`)

	got := map[string]bool{}
	for _, w := range checkInputUnknownKeys(raw) {
		got[w.Path] = true
	}
	base := "/slides/0/shape_grid/rows/0/cells"
	for _, want := range []string{
		base + "/0/layer",
		base + "/0/layers/0/frmae",
		base + "/0/layers/1/frame/width",
		base + "/0/layers/1/shape/fil",
		base + "/1/layers/0/z",
	} {
		if !got[want] {
			t.Errorf("unknown key %s not reported; got %v", want, got)
		}
	}
	for path := range got {
		for _, ok := range []string{"/layers/1/name", "/layers/1/frame/x", "/layers/1/frame/h", "/layers/1/shape/geometry", "/layers/1/shape/adjustments", "/0/layers"} {
			if strings.HasSuffix(path, ok) {
				t.Errorf("working key %s reported as unknown", path)
			}
		}
	}
}

// The discovery schema describes layers, so an agent can find the field.
func TestInputSchemaDescribesLayers(t *testing.T) {
	data, err := json.Marshal(buildInputSchema())
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Defs map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	layers, ok := schema.Defs["GridCellInput"].Properties["layers"]
	if !ok || !strings.Contains(string(layers), "#/$defs/LayerInput") {
		t.Fatalf("GridCellInput.layers = %s, want an array of LayerInput", layers)
	}
	layer := schema.Defs["LayerInput"].Properties
	for _, field := range []string{"frame", "shape", "name"} {
		if _, ok := layer[field]; !ok {
			t.Errorf("LayerInput schema lacks %q: %v", field, layer)
		}
	}
	if !strings.Contains(string(layer["frame"]), "#/$defs/LayerFrameInput") || !strings.Contains(string(layer["shape"]), "#/$defs/ShapeSpecInput") {
		t.Errorf("LayerInput frame / shape are not typed: %s / %s", layer["frame"], layer["shape"])
	}
	frame := schema.Defs["LayerFrameInput"].Properties
	for _, field := range []string{"x", "y", "w", "h"} {
		if _, ok := frame[field]; !ok {
			t.Errorf("LayerFrameInput schema lacks %q", field)
		}
	}
}
