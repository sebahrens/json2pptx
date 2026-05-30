package semantic

import "testing"

func TestSlideKindValid(t *testing.T) {
	if !KindTitle.Valid() {
		t.Error("KindTitle should be valid")
	}
	if SlideKind("nope").Valid() {
		t.Error("unknown kind should be invalid")
	}
}

func TestAllSlideKindsCoverRegistryAndAreSorted(t *testing.T) {
	kinds := AllSlideKinds()
	if len(kinds) != len(slideKindRegistry) {
		t.Fatalf("AllSlideKinds returned %d, registry has %d", len(kinds), len(slideKindRegistry))
	}
	for i := 1; i < len(kinds); i++ {
		if kinds[i-1] >= kinds[i] {
			t.Errorf("AllSlideKinds not sorted: %q >= %q", kinds[i-1], kinds[i])
		}
	}
	for _, k := range kinds {
		if info, ok := LookupKind(k); !ok || info.Kind != k {
			t.Errorf("LookupKind(%q) inconsistent", k)
		}
	}
}

func TestArchetypeValid(t *testing.T) {
	if !ArchetypeQBR.Valid() {
		t.Error("ArchetypeQBR should be valid")
	}
	if Archetype("nope").Valid() {
		t.Error("unknown archetype should be invalid")
	}
	if got := len(AllArchetypes()); got != len(archetypeRegistry) {
		t.Errorf("AllArchetypes returned %d, registry has %d", got, len(archetypeRegistry))
	}
}
