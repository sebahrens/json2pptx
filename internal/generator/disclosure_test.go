package generator

import "testing"

func TestSubtitleFallbackDoesNotOverwriteDisclosure(t *testing.T) {
	shapes := []shapeXML{
		makeShape("legal_disclosure", "subTitle", intPtr(1), 5000000, 4800000, 6000000, 1600000),
		makeShape("Main Caption", "subTitle", intPtr(2), 400000, 3400000, 6600000, 1000000),
	}
	r := newPlaceholderResolver(shapes)
	idx, _, ok := r.ResolveWithFallback("subtitle")
	if !ok || idx != 1 {
		t.Fatalf("subtitle routed to disclosure: index=%d found=%v", idx, ok)
	}
	idx, tier, ok := r.ResolveWithFallback("legal_disclosure")
	if !ok || idx != 0 || tier != TierExact {
		t.Fatalf("explicit disclosure unavailable: index=%d tier=%v found=%v", idx, tier, ok)
	}
	if got := classifyShapeRole(&shapes[0]); got != RoleDisclosure {
		t.Fatalf("disclosure used as ordinary content: %v", got)
	}
}

func TestDisclosureOnlyLayoutCannotSupplyOrdinarySubtitle(t *testing.T) {
	for _, phType := range []string{"subTitle", "body", "obj", ""} {
		t.Run(phType, func(t *testing.T) {
			shapes := []shapeXML{makeShape("legal_disclosure", phType, intPtr(1), 5000000, 4800000, 6000000, 1600000)}
			r := newPlaceholderResolver(shapes)
			for _, ordinary := range []string{"subtitle", "body", "body_left", "slot1", "title"} {
				if idx, _, ok := r.ResolveWithFallback(ordinary); ok {
					t.Fatalf("ordinary %q incorrectly resolves to disclosure index %d", ordinary, idx)
				}
			}
			if idx, tier, ok := r.ResolveWithFallback("legal_disclosure"); !ok || idx != 0 || tier != TierExact {
				t.Fatalf("explicit disclosure inaccessible: index=%d tier=%v found=%v", idx, tier, ok)
			}
		})
	}
}
