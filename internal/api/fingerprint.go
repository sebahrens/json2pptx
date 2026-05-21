package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
)

// ResponseFingerprintField is the name of the struct field that
// ComputeResponseFingerprint reads from and writes to. Response structs that
// want a fingerprint must declare a settable string field of this name with
// the JSON tag "response_fingerprint".
const ResponseFingerprintField = "ResponseFingerprint"

// ComputeResponseFingerprint sets v.ResponseFingerprint to a sha256 hex digest
// of the canonical JSON of v with the fingerprint field zeroed. v must be a
// non-nil pointer to a struct that declares a settable string field named
// ResponseFingerprint.
//
// The encoding is deterministic: Go's encoding/json emits struct fields in
// declaration order and sorts map keys, which is sufficient as a canonical
// form. Agents can verify or recompute the fingerprint by zeroing the
// response_fingerprint field of a parsed response and re-hashing the JSON.
func ComputeResponseFingerprint(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("ComputeResponseFingerprint: v must be a non-nil pointer to a struct")
	}
	rs := rv.Elem()
	if rs.Kind() != reflect.Struct {
		return fmt.Errorf("ComputeResponseFingerprint: v must point to a struct (got %s)", rs.Kind())
	}
	f := rs.FieldByName(ResponseFingerprintField)
	if !f.IsValid() || f.Kind() != reflect.String || !f.CanSet() {
		return fmt.Errorf("ComputeResponseFingerprint: struct %s lacks settable string field %s", rs.Type(), ResponseFingerprintField)
	}
	f.SetString("")
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("ComputeResponseFingerprint: marshal failed: %w", err)
	}
	sum := sha256.Sum256(data)
	f.SetString(hex.EncodeToString(sum[:]))
	return nil
}
