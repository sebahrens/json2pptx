package main

import "testing"

// TestSchemaFingerprintMatchesVersion ensures that any change to the contract
// surfaces (PresentationInput fields, MCP tools, Fix.Kind vocab) is detected.
// If this test fails, you MUST:
//  1. Bump SchemaVersion in main.go (minor for additions, major for removals/renames).
//  2. Add a changelog entry in docs/SCHEMA_CHANGELOG.md.
//  3. Update wantFingerprint below to the new fingerprint value.
func TestSchemaFingerprintMatchesVersion(t *testing.T) {
	// Last advanced at SchemaVersion "4.41.0". The fingerprint covers
	// PresentationInput fields, MCP tool NAMES, and Fix.Kind vocab. The 4.41.0
	// addition of the examine_template MCP tool changes the tool-name set, so the
	// hash advanced from 873ef2aca515871c (4.40.0). 4.42.0 only added response
	// fields to the render tools (render.SlideImage), which are not part of the
	// fingerprint surface, so the hash is unchanged.
	// If this fails, see file header comment.
	const wantFingerprint = "e597f18f06d5bf07"

	got := schemaFingerprint()

	if wantFingerprint == "" {
		t.Logf("Schema fingerprint (pin this value): %s", got)
		t.Log("Set wantFingerprint in this test and commit alongside a SchemaVersion bump.")
		return
	}

	if got != wantFingerprint {
		t.Errorf("Schema fingerprint changed: got %q, want %q.\n"+
			"This means the contract surface changed. You MUST:\n"+
			"  1. Bump SchemaVersion in main.go\n"+
			"  2. Add entry to docs/SCHEMA_CHANGELOG.md\n"+
			"  3. Update wantFingerprint in this test to %q", got, wantFingerprint, got)
	}
}

func TestSchemaFingerprintStable(t *testing.T) {
	// Calling schemaFingerprint twice must produce the same result.
	a := schemaFingerprint()
	b := schemaFingerprint()
	if a != b {
		t.Errorf("fingerprint is non-deterministic: %q != %q", a, b)
	}
}

func TestFixKindVocabularySorted(t *testing.T) {
	kinds := fixKindVocabulary()
	for i := 1; i < len(kinds); i++ {
		if kinds[i] < kinds[i-1] {
			t.Errorf("fixKindVocabulary not sorted: %q before %q", kinds[i-1], kinds[i])
		}
	}
}
