package patterns

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCyclePhaseStatusPreservesNativeGeometry(t *testing.T) {
	for _, arrows := range []bool{false, true} {
		spec := newRingArcSpec(4, -60, 240, true).withBand(.9, .18)
		items, err := spec.items()
		if err != nil {
			t.Fatal(err)
		}
		paint := ringPaint{Arrows: arrows, Highlight: 0, Accents: []string{"accent2", "accent2", "accent2", "accent2"}}
		plain := ringSegmentLayers(ExpandContext{}, spec, items, paint)
		paint.Statuses = []string{"done", "planned", "planned", "planned"}
		got := ringSegmentLayers(ExpandContext{}, spec, items, paint)
		for i := range got {
			if got[i].Frame != plain[i].Frame || got[i].Shape.Geometry != plain[i].Shape.Geometry || !reflect.DeepEqual(got[i].Shape.Adjustments, plain[i].Shape.Adjustments) {
				t.Fatalf("status changed loop geometry: %d", i)
			}
			if i == 0 {
				if !reflect.DeepEqual(got[i], plain[i]) {
					t.Fatal("done changed default highlight")
				}
				continue
			}
			if string(got[i].Shape.Fill) != `"none"` || !strings.Contains(string(got[i].Shape.Line), `"dash":"dash"`) {
				t.Fatalf("planned segment lacks native outline: %+v", got[i])
			}
		}
	}
}

func TestCycleStatusValidation(t *testing.T) {
	for _, name := range []string{"cycle-ring", "cycle-figure-eight"} {
		p, _ := Default().Get(name)
		for _, status := range []string{"done", "planned", "unknown"} {
			v := p.NewValues()
			raw := `{"phases":[{"label":"Plan","highlight":true},{"label":"Build"},{"label":"Run","status":"` + status + `"},{"label":"Learn","status":"planned"}]}`
			if err := json.Unmarshal([]byte(raw), v); err != nil {
				t.Fatal(err)
			}
			err := p.Validate(v, nil, nil)
			if (err != nil) != (status == "unknown") {
				t.Fatalf("%s %s: %v", name, status, err)
			}
		}
	}
}

func TestFigureEightRightLoopPlanned(t *testing.T) {
	v := cfeValues(8, 4)
	v.Phases[0].Highlight = true
	for i := range v.Phases {
		v.Phases[i].Status = "done"
		if i >= 4 {
			v.Phases[i].Status = "planned"
		}
	}
	grid := cfeExpand(t, kpiTestCtx(), v, nil)
	lobes := cfeLobes(t, grid)
	planned := 0
	for li, lobe := range lobes {
		for _, layer := range lobe.Layers {
			if !strings.Contains(layer.Name, "segment-") {
				continue
			}
			if li == 1 {
				planned++
				if string(layer.Shape.Fill) != `"none"` || !strings.Contains(string(layer.Shape.Line), `"dash":"dash"`) {
					t.Fatalf("right lobe phase not planned: %s", layer.Name)
				}
			} else if string(layer.Shape.Fill) == `"none"` {
				t.Fatal("done lobe outlined")
			}
		}
	}
	if planned != 4 {
		t.Fatalf("planned segments %d", planned)
	}
}
