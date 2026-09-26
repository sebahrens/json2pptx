package generator

import "testing"

func TestDisclosureCannotBeClaimedByGenericFallback(t *testing.T) {
	for _, kind := range []string{"subTitle", "body", "obj", ""} {
		t.Run(kind, func(t *testing.T) {
			legal := makeShape("legal_disclosure", kind, intPtr(1), 5000000, 4800000, 6000000, 1600000)
			for _, requested := range []string{"section_number", "section_no", "large_number", "legal_disclosures", "legal_disclosur"} {
				r := newPlaceholderResolver([]shapeXML{legal}, "Section Divider")
				if idx, tier, ok := r.ResolveWithFallback(requested); ok {
					t.Errorf("generic %q claimed required legal shape %d via %s", requested, idx, tier)
				}
			}
			r := newPlaceholderResolver([]shapeXML{legal})
			if idx, tier, ok := r.ResolveWithFallback("idx:1"); !ok || idx != 0 || tier != TierExact {
				t.Fatal("explicit idx targeting must remain available")
			}
		})
	}
}

func TestDisclosureRequestRequiresExplicitLegalTextSlot(t *testing.T) {
	for _, kind := range []string{"subTitle", "body", "pic", "title"} {
		t.Run(kind, func(t *testing.T) {
			shapes := []shapeXML{makeShape("legal_disclosures", kind, intPtr(1), 0, 0, 1000000, 1000000)}
			if idx, tier, ok := newPlaceholderResolver(shapes).ResolveWithFallback("legal_disclosure"); ok {
				t.Fatalf("missing required legal slot fuzzily resolved to incidental shape %d via %s", idx, tier)
			}
		})
	}
}

func TestDisclosureAliasAndOrdinarySectionFallbackRemainAvailable(t *testing.T) {
	shapes := []shapeXML{
		makeShape(" Legal Disclosure ", "subTitle", intPtr(1), 0, 5000000, 6000000, 1000000),
		makeShape("Section Number", "body", intPtr(7), 5000000, 0, 3000000, 3000000),
		makeShape("Main Content", "body", intPtr(2), 0, 1000000, 6000000, 3000000),
	}
	for _, requested := range []string{"legal_disclosure", "legal disclosure", " LEGAL DISCLOSURE "} {
		if idx, _, ok := newPlaceholderResolver(shapes).ResolveWithFallback(requested); !ok || idx != 0 {
			t.Errorf("explicit alias %q failed: idx=%d found=%v", requested, idx, ok)
		}
	}
	if idx, _, ok := newPlaceholderResolver(shapes, "Section Divider").ResolveWithFallback("section_number"); !ok || idx != 1 {
		t.Fatal("ordinary explicit section-number lookup changed")
	}
	ordinary := []shapeXML{makeShape("content", "body", intPtr(1), 0, 0, 6000000, 3000000)}
	if idx, _, ok := newPlaceholderResolver(ordinary, "Section Divider").ResolveWithFallback("section_number"); !ok || idx != 0 {
		t.Fatal("ordinary legacy idx1 section fallback changed")
	}
}
