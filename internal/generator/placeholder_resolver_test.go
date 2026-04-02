package generator

import (
	"testing"
)

// makeShape creates a minimal shapeXML for testing.
func makeShape(name string, phType string, phIdx *int, x, y, cx, cy int64) shapeXML {
	shape := shapeXML{
		NonVisualProperties: nonVisualPropertiesXML{
			ConnectionNonVisual: connectionNonVisualXML{Name: name},
			NvPr: nvPrXML{},
		},
	}
	if phType != "" || phIdx != nil {
		shape.NonVisualProperties.NvPr.Placeholder = &placeholderXML{
			Type:  phType,
			Index: phIdx,
		}
	}
	if cx > 0 || cy > 0 {
		shape.ShapeProperties.Transform = &transformXML{
			Offset: offsetXML{X: x, Y: y},
			Extent: extentXML{CX: cx, CY: cy},
		}
	}
	return shape
}

// =============================================================================
// Tier 1: Exact Match
// =============================================================================

func TestResolveWithFallback_Tier1_ExactByName(t *testing.T) {
	shapes := []shapeXML{
		makeShape("title", "title", nil, 0, 0, 1000, 500),
		makeShape("body", "body", nil, 0, 500, 5000, 3000),
	}
	r := newPlaceholderResolver(shapes)

	idx, tier, ok := r.ResolveWithFallback("body")
	if !ok {
		t.Fatal("expected match for 'body'")
	}
	if idx != 1 {
		t.Errorf("idx = %d, want 1", idx)
	}
	if tier != TierExact {
		t.Errorf("tier = %v, want TierExact", tier)
	}
}

func TestResolveWithFallback_Tier1_ExactByIdx(t *testing.T) {
	shapes := []shapeXML{
		makeShape("title", "title", nil, 0, 0, 1000, 500),
		makeShape("body", "body", intPtr(10), 0, 500, 5000, 3000),
	}
	r := newPlaceholderResolver(shapes)

	idx, tier, ok := r.ResolveWithFallback("idx:10")
	if !ok {
		t.Fatal("expected match for 'idx:10'")
	}
	if idx != 1 {
		t.Errorf("idx = %d, want 1", idx)
	}
	if tier != TierExact {
		t.Errorf("tier = %v, want TierExact", tier)
	}
}

// =============================================================================
// Tier 2: Semantic Role Match
// =============================================================================

func TestResolveWithFallback_Tier2_SemanticRole(t *testing.T) {
	// Simulate template with canonical names "body", "body_2"
	// but content uses legacy "Content Placeholder 1" which maps to RoleBodyPrimary.
	shapes := []shapeXML{
		makeShape("title", "title", nil, 0, 0, 9144000, 914400),
		makeShape("accent_number", "body", nil, 457200, 914400, 3000000, 2000000), // smaller
		makeShape("main_content", "body", nil, 457200, 3000000, 8000000, 3500000), // largest body → primary
	}
	r := newPlaceholderResolver(shapes)

	// "Content Placeholder 1" maps to RoleBodyPrimary in semanticAliases.
	// main_content is the largest body placeholder → RoleBodyPrimary.
	idx, tier, ok := r.ResolveWithFallback("Content Placeholder 1")
	if !ok {
		t.Fatal("expected semantic match for 'Content Placeholder 1'")
	}
	if idx != 2 {
		t.Errorf("idx = %d, want 2 (main_content)", idx)
	}
	if tier != TierSemantic {
		t.Errorf("tier = %v, want TierSemantic", tier)
	}
}

func TestResolveWithFallback_Tier2_SemanticBodySecondary(t *testing.T) {
	shapes := []shapeXML{
		makeShape("title", "title", nil, 0, 0, 9144000, 914400),
		makeShape("big_body", "body", nil, 0, 914400, 8000000, 4000000),   // largest
		makeShape("small_body", "body", nil, 0, 5000000, 4000000, 1000000), // second
	}
	r := newPlaceholderResolver(shapes)

	idx, tier, ok := r.ResolveWithFallback("Content Placeholder 2")
	if !ok {
		t.Fatal("expected semantic match for 'Content Placeholder 2'")
	}
	if idx != 2 {
		t.Errorf("idx = %d, want 2 (small_body)", idx)
	}
	if tier != TierSemantic {
		t.Errorf("tier = %v, want TierSemantic", tier)
	}
}

// =============================================================================
// Tier 3: Fuzzy Name Match
// =============================================================================

func TestResolveWithFallback_Tier3_FuzzyMatch(t *testing.T) {
	shapes := []shapeXML{
		makeShape("title", "title", nil, 0, 0, 9144000, 914400),
		makeShape("body_content", "body", nil, 0, 914400, 8000000, 4000000),
	}
	r := newPlaceholderResolver(shapes)

	// "body content" (space instead of underscore) should fuzzy-match "body_content"
	// after normalization both become "body content" / "body_content" → close enough.
	idx, tier, ok := r.ResolveWithFallback("body content")
	if !ok {
		t.Fatal("expected fuzzy match for 'body content'")
	}
	if idx != 1 {
		t.Errorf("idx = %d, want 1", idx)
	}
	if tier != TierFuzzy {
		t.Errorf("tier = %v, want TierFuzzy", tier)
	}
}

// =============================================================================
// Tier 4: Positional Fallback
// =============================================================================

func TestResolveWithFallback_Tier4_SlotPositional(t *testing.T) {
	// Three-column layout: body placeholders at x=100, x=3000, x=6000
	shapes := []shapeXML{
		makeShape("title", "title", nil, 0, 0, 9144000, 914400),
		makeShape("col_right", "body", nil, 6000000, 914400, 3000000, 4000000),
		makeShape("col_left", "body", nil, 100000, 914400, 3000000, 4000000),
		makeShape("col_center", "body", nil, 3000000, 914400, 3000000, 4000000),
	}
	r := newPlaceholderResolver(shapes)

	// "slot1" → 0-indexed slot 0 → leftmost content placeholder (col_left at x=100000)
	idx, tier, ok := r.ResolveWithFallback("slot1")
	if !ok {
		t.Fatal("expected positional match for 'slot1'")
	}
	if idx != 2 { // col_left is shape index 2
		t.Errorf("slot1: idx = %d, want 2 (col_left)", idx)
	}
	if tier != TierPositional {
		t.Errorf("slot1: tier = %v, want TierPositional", tier)
	}

	// "slot2" → 0-indexed slot 1 → center column
	idx, tier, ok = r.ResolveWithFallback("slot2")
	if !ok {
		t.Fatal("expected positional match for 'slot2'")
	}
	if idx != 3 { // col_center is shape index 3
		t.Errorf("slot2: idx = %d, want 3 (col_center)", idx)
	}
	if tier != TierPositional {
		t.Errorf("slot2: tier = %v, want TierPositional", tier)
	}
}

func TestResolveWithFallback_Tier4_BodyLeftRight(t *testing.T) {
	shapes := []shapeXML{
		makeShape("left_body", "body", nil, 100000, 914400, 4000000, 4000000),
		makeShape("right_body", "body", nil, 5000000, 914400, 4000000, 4000000),
	}
	r := newPlaceholderResolver(shapes)

	idx, tier, ok := r.ResolveWithFallback("body_left")
	if !ok {
		t.Fatal("expected positional match for 'body_left'")
	}
	if idx != 0 {
		t.Errorf("body_left: idx = %d, want 0", idx)
	}
	if tier != TierPositional {
		t.Errorf("tier = %v, want TierPositional", tier)
	}

	idx, _, ok = r.ResolveWithFallback("body_right")
	if !ok {
		t.Fatal("expected positional match for 'body_right'")
	}
	if idx != 1 {
		t.Errorf("body_right: idx = %d, want 1", idx)
	}
}

// =============================================================================
// Tier 5: Topmost Placeholder Fallback (title only)
// =============================================================================

func TestResolveWithFallback_Tier5_TitleToTopmostBody(t *testing.T) {
	// Layout without any title-type placeholder.
	// Has only body placeholders at different Y positions.
	shapes := []shapeXML{
		makeShape("ftr", "ftr", nil, 0, 6500000, 9144000, 300000),              // footer at bottom
		makeShape("body", "body", nil, 300000, 1200000, 9000000, 5000000),       // body at y=1200000
		makeShape("body_2", "body", nil, 300000, 300000, 4000000, 800000),       // body_2 at y=300000 (topmost)
	}
	r := newPlaceholderResolver(shapes)

	// Requesting "title" should fall through all tiers and resolve to the
	// topmost content placeholder (body_2 at y=300000).
	idx, tier, ok := r.ResolveWithFallback("title")
	if !ok {
		t.Fatal("expected topmost fallback match for 'title'")
	}
	if idx != 2 { // body_2 is shape index 2
		t.Errorf("idx = %d, want 2 (body_2, topmost content placeholder)", idx)
	}
	if tier != TierTopmost {
		t.Errorf("tier = %v, want TierTopmost", tier)
	}
}

func TestResolveWithFallback_Tier5_NotAppliedToNonTitle(t *testing.T) {
	// The topmost fallback should only trigger for "title" placeholder ID.
	shapes := []shapeXML{
		makeShape("body", "body", nil, 300000, 300000, 9000000, 5000000),
	}
	r := newPlaceholderResolver(shapes)

	// "subtitle" should not fall back to topmost.
	_, _, ok := r.ResolveWithFallback("subtitle")
	if ok {
		t.Error("expected no match for 'subtitle' — topmost fallback is title-only")
	}
}

func TestResolveWithFallback_Tier5_NoContentPlaceholders(t *testing.T) {
	// Layout with no content placeholders at all (only utility).
	shapes := []shapeXML{
		makeShape("ftr", "ftr", nil, 0, 6500000, 9144000, 300000),
		makeShape("sldNum", "sldNum", nil, 8000000, 6500000, 1000000, 300000),
	}
	r := newPlaceholderResolver(shapes)

	// "title" should fail even with topmost fallback since no content placeholders exist.
	_, _, ok := r.ResolveWithFallback("title")
	if ok {
		t.Error("expected no match when no content placeholders exist")
	}
}

// =============================================================================
// No Match
// =============================================================================

func TestResolveWithFallback_NoMatch(t *testing.T) {
	shapes := []shapeXML{
		makeShape("title", "title", nil, 0, 0, 9144000, 914400),
	}
	r := newPlaceholderResolver(shapes)

	_, _, ok := r.ResolveWithFallback("completely_unknown_placeholder")
	if ok {
		t.Error("expected no match for unknown placeholder")
	}
}

// =============================================================================
// Semantic Role Classification
// =============================================================================

func TestBuildSemanticRoleMap(t *testing.T) {
	shapes := []shapeXML{
		makeShape("title", "title", nil, 0, 0, 9144000, 914400),
		makeShape("subtitle", "subTitle", nil, 0, 914400, 9144000, 914400),
		makeShape("large_body", "body", nil, 0, 1828800, 8000000, 4000000), // area = 32e12
		makeShape("medium_body", "body", nil, 0, 5828800, 6000000, 2000000), // area = 12e12
		makeShape("small_body", "body", nil, 0, 7828800, 3000000, 1000000), // area = 3e12
	}

	roles := buildSemanticRoleMap(shapes)

	tests := []struct {
		idx  int
		want PlaceholderSemanticRole
	}{
		{0, RoleTitle},
		{1, RoleSubtitle},
		{2, RoleBodyPrimary},   // largest body
		{3, RoleBodySecondary}, // second largest
		{4, RoleBodyTertiary},  // third
	}

	for _, tt := range tests {
		got, ok := roles[tt.idx]
		if !ok {
			t.Errorf("roles[%d] not found", tt.idx)
			continue
		}
		if got != tt.want {
			t.Errorf("roles[%d] = %d, want %d", tt.idx, got, tt.want)
		}
	}
}

// =============================================================================
// Name Normalization
// =============================================================================

func TestNormalizePlaceholderName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Content Placeholder 1", "content"},
		{"Text Placeholder 15", "text"},
		{"body", "body"},
		{"Body_2", "body_"},
		{"TITLE", "title"},
	}

	for _, tt := range tests {
		got := normalizePlaceholderName(tt.input)
		if got != tt.want {
			t.Errorf("normalizePlaceholderName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// =============================================================================
// Slot Index Parsing
// =============================================================================

func TestParseSlotIndex(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"slot1", 0},
		{"slot2", 1},
		{"slot3", 2},
		{"body_left", 0},
		{"body_right", 1},
		{"left", 0},
		{"right", 1},
		{"body", -1},         // not a slot reference
		{"unknown", -1},      // not a slot reference
		{"slot0", -1},        // invalid (1-indexed)
		{"slot10", -1},       // only single digit supported
	}

	for _, tt := range tests {
		got := parseSlotIndex(tt.input)
		if got != tt.want {
			t.Errorf("parseSlotIndex(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// =============================================================================
// ResolutionTier String
// =============================================================================

func TestResolutionTierString(t *testing.T) {
	tests := []struct {
		tier ResolutionTier
		want string
	}{
		{TierExact, "exact"},
		{TierSemantic, "semantic"},
		{TierFuzzy, "fuzzy"},
		{TierPositional, "positional"},
		{TierTopmost, "topmost"},
		{ResolutionTier(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.want {
			t.Errorf("ResolutionTier(%d).String() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}
