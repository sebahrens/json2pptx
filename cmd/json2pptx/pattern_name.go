package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// patternName reads the discriminator without treating malformed input as a
// different kind of cell. Callers retain the location when surfacing the error.
func patternName(raw json.RawMessage) (string, error) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}
	if strings.TrimSpace(p.Name) == "" {
		return "", fmt.Errorf("pattern.name is required")
	}
	return p.Name, nil
}
