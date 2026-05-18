package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

type sampleResponse struct {
	Foo                 string `json:"foo"`
	Bar                 int    `json:"bar"`
	ResponseFingerprint string `json:"response_fingerprint,omitempty"`
}

func TestComputeResponseFingerprint_SetsHexDigest(t *testing.T) {
	r := &sampleResponse{Foo: "hello", Bar: 7}
	if err := ComputeResponseFingerprint(r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.ResponseFingerprint) != sha256.Size*2 {
		t.Fatalf("expected sha256 hex (64 chars), got %d: %q", len(r.ResponseFingerprint), r.ResponseFingerprint)
	}
	if _, err := hex.DecodeString(r.ResponseFingerprint); err != nil {
		t.Fatalf("fingerprint is not valid hex: %v", err)
	}
}

func TestComputeResponseFingerprint_Deterministic(t *testing.T) {
	a := &sampleResponse{Foo: "hello", Bar: 7}
	b := &sampleResponse{Foo: "hello", Bar: 7}
	if err := ComputeResponseFingerprint(a); err != nil {
		t.Fatalf("a: %v", err)
	}
	if err := ComputeResponseFingerprint(b); err != nil {
		t.Fatalf("b: %v", err)
	}
	if a.ResponseFingerprint != b.ResponseFingerprint {
		t.Fatalf("expected identical fingerprints, got %s vs %s", a.ResponseFingerprint, b.ResponseFingerprint)
	}
}

func TestComputeResponseFingerprint_ChangesWithContent(t *testing.T) {
	a := &sampleResponse{Foo: "hello", Bar: 7}
	b := &sampleResponse{Foo: "hello", Bar: 8}
	if err := ComputeResponseFingerprint(a); err != nil {
		t.Fatalf("a: %v", err)
	}
	if err := ComputeResponseFingerprint(b); err != nil {
		t.Fatalf("b: %v", err)
	}
	if a.ResponseFingerprint == b.ResponseFingerprint {
		t.Fatalf("expected different fingerprints for different content")
	}
}

// TestComputeResponseFingerprint_Recomputable verifies an agent can recompute
// the fingerprint by zeroing the field and re-marshaling — the documented
// contract.
func TestComputeResponseFingerprint_Recomputable(t *testing.T) {
	r := &sampleResponse{Foo: "hello", Bar: 7}
	if err := ComputeResponseFingerprint(r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := r.ResponseFingerprint

	// Marshal full response, parse, zero the fingerprint, re-marshal, hash.
	full, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(full, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	delete(m, "response_fingerprint")
	// Reconstruct in struct form to get the same canonical encoding (field order
	// matters; encoding from a generic map will use sorted keys, which happens
	// to match struct order here).
	stripped := &sampleResponse{Foo: r.Foo, Bar: r.Bar}
	canonical, err := json.Marshal(stripped)
	if err != nil {
		t.Fatalf("canonical marshal: %v", err)
	}
	sum := sha256.Sum256(canonical)
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("fingerprint mismatch:\n got:  %s\n want: %s\n canonical body: %s", got, want, string(canonical))
	}
}

func TestComputeResponseFingerprint_Errors(t *testing.T) {
	t.Run("nil pointer", func(t *testing.T) {
		var r *sampleResponse
		err := ComputeResponseFingerprint(r)
		if err == nil || !strings.Contains(err.Error(), "non-nil pointer") {
			t.Fatalf("expected non-nil pointer error, got %v", err)
		}
	})
	t.Run("value not pointer", func(t *testing.T) {
		r := sampleResponse{Foo: "x"}
		err := ComputeResponseFingerprint(r)
		if err == nil {
			t.Fatalf("expected error for non-pointer arg")
		}
	})
	t.Run("missing field", func(t *testing.T) {
		type noField struct{ X int }
		err := ComputeResponseFingerprint(&noField{X: 1})
		if err == nil || !strings.Contains(err.Error(), "ResponseFingerprint") {
			t.Fatalf("expected missing-field error, got %v", err)
		}
	})
}
