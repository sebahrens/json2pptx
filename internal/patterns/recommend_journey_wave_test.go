package patterns

import (
	"strconv"
	"strings"
	"testing"
)

// The four product-only agent journeys of 2026-10 (f-deals-pitch,
// g-risk-consulting, h-tech-data, i-risk-assurance) hit recommend_visual
// with the exact phrasings below and got the wrong first answer every time
// (go-slide-creator-ux1fl). These pin the top candidate for each.

const (
	waveBridgeBesideText = "EBITDA margin bridge 2023 to 2025 as a waterfall taking two thirds of the slide, with a short what-this-means-for-the-bid narrative text column on the remaining third"
	waveRiskAppetite     = "risk appetite status board: five risk types each with a status (within / amber / breached), the current metric and the limit; breached rows visibly distinguished"
	waveITGCResults      = "ITGC results-by-domain board: five domains, each with controls tested, exceptions, and a red/amber/green rating shown as colour or a marker"
	waveAuditFinding     = "one audit finding on its own slide in a structured layout, not bullets: what we found, why it matters, the agreed action, owner and due date"
	waveTieredStack      = "Target lakehouse architecture as a tiered stack, five tiers top to bottom (consumption, serving, processing, storage, ingestion), with security/governance and FinOps as cross-cutting side rails beside the tiers"
	waveRunCostSplit     = "Run cost driver split as a visual: compute 48%, storage 12%, licences 22%, people 18% share of annual run cost"
)

func visualRank(res RecommendVisualResult) map[string]int {
	out := map[string]int{}
	for i, c := range res.Candidates {
		if _, seen := out[c.Name]; !seen {
			out[c.Name] = i
		}
	}
	return out
}

// TestRecommendVisualWave_TopCandidates: the open ranking's first answer.
func TestRecommendVisualWave_TopCandidates(t *testing.T) {
	reg := Default()
	cases := []struct {
		name   string
		intent string
		hints  *VisualHints
		want   string
		// below must rank under want when present.
		below []string
	}{
		{"g status board", waveRiskAppetite, &VisualHints{ContentHints: ContentHints{ItemCount: 5, Columns: 4, HasMetrics: true}}, "table-highlight", []string{"kpi-5up", "kpi-inline", "metric-list"}},
		{"g status board no hints", waveRiskAppetite, nil, "table-highlight", []string{"kpi-3up", "kpi-inline"}},
		{"i results by domain", waveITGCResults, &VisualHints{ContentHints: ContentHints{ItemCount: 5, Columns: 3}}, "table-highlight", []string{"horizontal-bar-with-callouts", "card-grid"}},
		{"i audit finding", waveAuditFinding, &VisualHints{ContentHints: ContentHints{ItemCount: 5}}, "labeled-rows", []string{"content", "next-steps"}},
		{"i audit finding no hints", waveAuditFinding, nil, "labeled-rows", []string{"content"}},
		{"h tiered stack", waveTieredStack, nil, "arch-stack", nil},
		{"h run cost split", waveRunCostSplit, nil, "pie", []string{"driver-tree", "waterfall", "horizontal-bar-with-callouts"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := RecommendVisual(reg, tc.intent, tc.hints, 6)
			if len(res.Candidates) == 0 || res.Candidates[0].Name != tc.want {
				t.Fatalf("top = %+v, want %s", summarizeCandidates(res.Candidates), tc.want)
			}
			r := visualRank(res)
			for _, worse := range tc.below {
				if i, ok := r[worse]; ok && i <= r[tc.want] {
					t.Errorf("%s (rank %d) is not below %s: %v", worse, i, tc.want, summarizeCandidates(res.Candidates))
				}
			}
		})
	}
}

// TestRecommendVisualWave_GaugeGatedOutOfStacks: "target architecture" is
// not a measure against a target; the gauge never ranks for a tiered stack.
func TestRecommendVisualWave_GaugeGatedOutOfStacks(t *testing.T) {
	res := RecommendVisual(Default(), waveTieredStack, nil, 8)
	if c := findCandidate(res.Candidates, "gauge"); c != nil {
		t.Errorf("gauge ranked for a tiered stack: %+v", summarizeCandidates(res.Candidates))
	}
	short := RecommendVisual(Default(), waveTieredStack, nil, 8, &RecommendOptions{Candidates: []string{"gauge", "arch-stack"}})
	if short.Candidates[0].Name != "arch-stack" || findCandidate(short.Candidates, "gauge").Score >= 0.5 {
		t.Errorf("shortlist = %+v, want arch-stack first and gauge under 0.5", summarizeCandidates(short.Candidates))
	}
	// A real single-value target reading still is a gauge.
	one := RecommendVisual(Default(), "progress toward target", &VisualHints{DataPoints: 1}, 3)
	if findCandidate(one.Candidates, "gauge") == nil {
		t.Errorf("gauge lost the single-value target intent: %+v", summarizeCandidates(one.Candidates))
	}
}

// TestRecommendVisualWave_ShareSplitGatesToPartToWhole: parts that sum to
// 100% are a composition, not a driver tree or a bridge.
func TestRecommendVisualWave_ShareSplitGatesToPartToWhole(t *testing.T) {
	reg := Default()
	res := RecommendVisual(reg, waveRunCostSplit, nil, 8)
	r := visualRank(res)
	for _, name := range []string{"pie", "donut", "stacked_bar"} {
		if _, ok := r[name]; !ok {
			t.Errorf("%s missing from a four-part share split: %v", name, summarizeCandidates(res.Candidates))
		}
	}
	// Six or more parts overload a pie: the stacked bar leads.
	six := RecommendVisual(reg, "cost base split: compute 30%, storage 15%, network 10%, licences 20%, people 15%, other 10%", nil, 8)
	if six.Candidates[0].Name != "stacked_bar" {
		t.Errorf("six-part split top = %+v, want stacked_bar", summarizeCandidates(six.Candidates))
	}
	if c := findCandidate(six.Candidates, "pie"); c != nil && c.Score >= 0.5 {
		t.Errorf("six-slice pie still recommended: %+v", c)
	}
	// Shortlist mode applies the same gate.
	short := RecommendVisual(reg, waveRunCostSplit, nil, 8, &RecommendOptions{Candidates: []string{"driver-tree", "pie", "waterfall"}})
	if short.Candidates[0].Name != "pie" {
		t.Errorf("shortlist top = %+v, want pie", summarizeCandidates(short.Candidates))
	}
	// Percentages that do not sum to a whole are not a share split.
	notSplit := RecommendVisual(reg, "cost driver tree: run cost grew 12% on compute and 8% on licences", nil, 8)
	if notSplit.Candidates[0].Name != "driver-tree" {
		t.Errorf("driver tree lost its own intent: %+v", summarizeCandidates(notSplit.Candidates))
	}
}

// TestRecommendVisualWave_BridgeBesideTextIsRegions: a waterfall taking two
// thirds beside a narrative third is one slide of two views — the regions
// kind (chart region type waterfall + text region) first, its compose form
// just below, every single view under both.
func TestRecommendVisualWave_BridgeBesideTextIsRegions(t *testing.T) {
	reg := Default()
	hints := &VisualHints{ContentHints: ContentHints{HasChart: true, DensityHint: "medium"}, DataPoints: 6}
	res := RecommendVisual(reg, waveBridgeBesideText, hints, 6)
	if len(res.Candidates) < 3 {
		t.Fatalf("candidates = %+v", summarizeCandidates(res.Candidates))
	}
	kind, comp := res.Candidates[0], res.Candidates[1]
	if kind.Category != VisualCategoryKind || kind.Name != RegionsKindName {
		t.Fatalf("top = %s %q, want the regions kind: %v", kind.Category, kind.Name, summarizeCandidates(res.Candidates))
	}
	if comp.Category != VisualCategoryCompose || !strings.Contains(comp.Name, "chart:waterfall") {
		t.Fatalf("second = %s %q, want the compose form with chart:waterfall: %v", comp.Category, comp.Name, summarizeCandidates(res.Candidates))
	}
	for _, c := range res.Candidates[2:] {
		if c.Score >= comp.Score {
			t.Errorf("single view %s %q (%v) ties or beats the composition (%v)", c.Category, c.Name, c.Score, comp.Score)
		}
	}
	for _, c := range []VisualCandidate{kind, comp} {
		cm := c.Composition
		if cm == nil || cm.Direction != "horizontal" || len(cm.Regions) != 2 {
			t.Fatalf("%s: composition = %+v, want a horizontal pair", c.Name, cm)
		}
		left, right := cm.Regions[0], cm.Regions[1]
		if left.Category != VisualCategoryChart || left.Name != "waterfall" || left.SizePct != 67 {
			t.Errorf("%s: left = %+v, want the waterfall chart at two thirds", c.Name, left)
		}
		if RegionKindFor(&right) != "text" || right.SizePct != 33 {
			t.Errorf("%s: right = %+v, want a text region on the remaining third", c.Name, right)
		}
		sharesSum(t, cm)
	}
	if !strings.Contains(kind.Rationale, "waterfall chart") || !strings.Contains(kind.Rationale, "text") {
		t.Errorf("regions rationale does not name the two regions: %s", kind.Rationale)
	}

	// The same views phrased with the other same-slide words.
	for _, intent := range []string{
		"EBITDA bridge as a waterfall beside a short narrative column",
		"waterfall chart on the left, so-what commentary on the right",
		"a waterfall paired with three implication bullets, side by side",
		"margin bridge next to a short text column",
	} {
		res := RecommendVisual(reg, intent, nil, 6)
		if len(res.Candidates) == 0 || res.Candidates[0].Name != RegionsKindName {
			t.Errorf("%q: top = %v, want regions", intent, summarizeCandidates(res.Candidates))
		}
	}
	// A whole-slide bridge stays a bridge.
	alone := RecommendVisual(reg, "EBITDA margin bridge 2023 to 2025 as a waterfall", nil, 6)
	for _, c := range alone.Candidates {
		if c.Category == VisualCategoryKind || c.Category == VisualCategoryCompose {
			t.Errorf("a lone waterfall grew a composition: %+v", summarizeCandidates(alone.Candidates))
		}
	}
}

// TestRecommendVisualWave_ShortlistScoresLikeOpenRanking: a shortlist is the
// open model restricted to the names given — the consulting routing that
// lifts a name in the open ranking lifts it in the shortlist, and the top
// name is the same.
func TestRecommendVisualWave_ShortlistScoresLikeOpenRanking(t *testing.T) {
	reg := Default()
	cases := []struct {
		intent string
		hints  *VisualHints
		names  []string
		want   string
		scored []string // must score above 0.5 in the shortlist
	}{
		{waveRiskAppetite, nil, []string{"table-highlight", "table", "comparison-2col", "labeled-rows"}, "table-highlight", []string{"table-highlight", "table"}},
		{waveAuditFinding, &VisualHints{ContentHints: ContentHints{ItemCount: 5}}, []string{"labeled-rows", "scqa-summary", "framework-grid", "card-grid", "before-after", "text-sidebar"}, "labeled-rows", []string{"labeled-rows"}},
		{waveITGCResults, &VisualHints{ContentHints: ContentHints{ItemCount: 5}}, []string{"capability-heatmap", "card-grid", "kpi-5up", "stylish-panels", "table-highlight"}, "table-highlight", []string{"table-highlight"}},
		{"key risks and mitigations with an owner per risk", nil, []string{"card-grid", "table", "table-highlight"}, "table", []string{"table", "table-highlight"}},
	}
	for _, tc := range cases {
		t.Run(tc.intent[:24], func(t *testing.T) {
			open := RecommendVisual(reg, tc.intent, tc.hints, 8)
			short := RecommendVisual(reg, tc.intent, tc.hints, 8, &RecommendOptions{Candidates: tc.names})
			if len(short.Candidates) != len(tc.names) {
				t.Fatalf("shortlist returned %d of %d names: %v", len(short.Candidates), len(tc.names), summarizeCandidates(short.Candidates))
			}
			if short.Candidates[0].Name != tc.want {
				t.Errorf("shortlist top = %v, want %s", summarizeCandidates(short.Candidates), tc.want)
			}
			for _, name := range tc.scored {
				s := candidateScore(short.Candidates, name)
				if s < 0.5 {
					t.Errorf("%s scored %v in the shortlist, want the open ranking's model (>= 0.5)", name, s)
				}
				if o := findCandidate(open.Candidates, name); o != nil && o.Score != s {
					t.Errorf("%s: shortlist score %v differs from open-ranking score %v", name, s, o.Score)
				}
			}
		})
	}
}

// TestRecommendVisualWave_NegatedBulletsDemoteContent: "not bullets" is not
// a request for the bullets layout.
func TestRecommendVisualWave_NegatedBulletsDemoteContent(t *testing.T) {
	res := RecommendVisual(Default(), waveAuditFinding, nil, 6)
	if c := findCandidate(res.Candidates, "content"); c != nil && c.Score >= 0.7 {
		t.Errorf("content still scores %v for an intent that says \"not bullets\"", c.Score)
	}
}

func summarizeCandidates(cs []VisualCandidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = string(c.Category) + ":" + c.Name + "=" + strconv.FormatFloat(c.Score, 'f', -1, 64)
	}
	return out
}
