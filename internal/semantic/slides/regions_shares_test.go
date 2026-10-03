package slides

import (
	"encoding/json"
	"testing"
)

func regionsBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

const (
	labelledLine = `{"type":"line_chart","data":{"categories":["Q1","Q2"],"series":[{"name":"Revenue","values":[1,2]}]}}`
	stackedMix   = `{"type":"stacked_bar_chart","data":{"categories":["Q1","Q2"],"series":[{"name":"A","values":[1,2]},{"name":"B","values":[1,2]}]}}`
)

// A chart's minimum share counts its decoration: a labelled line reads in
// 50%, a heading over it costs 15%, and a stacked chart's value axis and
// legend another 35% (go-slide-creator-9re9p).
func TestRegionChartMinPct(t *testing.T) {
	for _, tc := range []struct {
		region string
		want   float64
	}{
		{`{"kind":"chart","chart":` + labelledLine + `}`, 50},
		{`{"kind":"chart","heading":"Revenue","chart":` + labelledLine + `}`, 65},
		{`{"kind":"chart","chart":` + stackedMix + `}`, 80},
		{`{"kind":"chart","heading":"Mix","chart":` + stackedMix + `}`, RegionMaxSizePct},
		{`{"kind":"chart","chart":{"type":"pie_chart","data":{"values":[1,2]}}}`, 30},
	} {
		if got := regionMinHeightPct(regionsBody(t, tc.region), false); got != tc.want {
			t.Errorf("%s: min = %g, want %g", tc.region, got, tc.want)
		}
	}
}

// Unset shares give a stacked chart the height its decoration needs when the
// other regions can spare it, and leave it short — to be reported by
// chart.plot_area_collapsed — rather than squeeze a timeline under its own
// minimum when they cannot (go-slide-creator-qpd9c).
func TestRegionGroupSharesChartYields(t *testing.T) {
	rows := regionsBody(t, `{"arrangement":"rows","regions":[{"kind":"chart","heading":"Revenue","chart":`+labelledLine+`},{"kind":"text","body":"Note."}]}`)
	axis, _, err := RegionGroupShares(rows)
	if err != nil {
		t.Fatal(err)
	}
	if axis[0] < 65 {
		t.Errorf("rows: chart share %g, want its 65%% minimum", axis[0])
	}

	top := regionsBody(t, `{"arrangement":"main_top","regions":[{"kind":"chart","heading":"Revenue","chart":`+labelledLine+`},{"kind":"stat","value":"32%","label":"Margin"},{"kind":"timeline","milestones":[{"label":"A","date":"Oct"},{"label":"B","date":"Nov"},{"label":"C","date":"Dec"}]}]}`)
	axis, _, err = RegionGroupShares(top)
	if err != nil {
		t.Fatal(err)
	}
	if need := regionMinHeightPct(RegionList(top)[2], false); axis[1] < need {
		t.Errorf("main_top: band share %g, the timeline needs %g%%", axis[1], need)
	}
}
