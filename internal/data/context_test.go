package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildContext_InlineValues(t *testing.T) {
	fmData := map[string]any{
		"company": "Acme",
		"year":    2026,
	}

	ctx, warnings, err := BuildContext(fmData, nil, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if ctx.Vars["company"] != "Acme" {
		t.Errorf("expected 'Acme', got %v", ctx.Vars["company"])
	}
	if ctx.Vars["year"] != 2026 {
		t.Errorf("expected 2026, got %v", ctx.Vars["year"])
	}
}

func TestBuildContext_CLIOverrides(t *testing.T) {
	fmData := map[string]any{"company": "OldCo"}
	overrides := map[string]string{"company": "NewCo", "extra": "val"}

	ctx, _, err := BuildContext(fmData, overrides, ".")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Vars["company"] != "NewCo" {
		t.Errorf("expected CLI override 'NewCo', got %v", ctx.Vars["company"])
	}
	if ctx.Vars["extra"] != "val" {
		t.Errorf("expected 'val', got %v", ctx.Vars["extra"])
	}
}

func TestBuildContext_JSONFileRef(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "data.json")
	if err := os.WriteFile(jsonPath, []byte(`{"revenue": 42000, "growth": 15.5}`), 0644); err != nil {
		t.Fatal(err)
	}

	fmData := map[string]any{"metrics": "data.json"}
	ctx, warnings, err := BuildContext(fmData, nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	metricsMap, ok := ctx.Vars["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", ctx.Vars["metrics"])
	}
	if metricsMap["revenue"] != float64(42000) {
		t.Errorf("expected 42000, got %v", metricsMap["revenue"])
	}
}

func TestBuildContext_YAMLFileRef(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte("name: Alice\nrole: CEO\n"), 0644); err != nil {
		t.Fatal(err)
	}

	fmData := map[string]any{"profile": "config.yaml"}
	ctx, _, err := BuildContext(fmData, nil, dir)
	if err != nil {
		t.Fatal(err)
	}

	profileMap, ok := ctx.Vars["profile"].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", ctx.Vars["profile"])
	}
	if profileMap["name"] != "Alice" {
		t.Errorf("expected 'Alice', got %v", profileMap["name"])
	}
}

func TestBuildContext_CSVFileRef(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "team.csv")
	if err := os.WriteFile(csvPath, []byte("name,role\nAlice,CEO\nBob,CTO\n"), 0644); err != nil {
		t.Fatal(err)
	}

	fmData := map[string]any{"team": "team.csv"}
	ctx, _, err := BuildContext(fmData, nil, dir)
	if err != nil {
		t.Fatal(err)
	}

	rows, ok := ctx.Vars["team"].([]map[string]string)
	if !ok {
		t.Fatalf("expected []map[string]string, got %T", ctx.Vars["team"])
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "Alice" || rows[0]["role"] != "CEO" {
		t.Errorf("unexpected first row: %v", rows[0])
	}
}

func TestBuildContext_MissingFileRef(t *testing.T) {
	fmData := map[string]any{"missing": "nonexistent.json"}
	ctx, warnings, err := BuildContext(fmData, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if _, ok := ctx.Vars["missing"]; ok {
		t.Error("expected missing key to not be set")
	}
}

func TestBuildContext_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	// Try to access file outside baseDir via .json extension
	fmData := map[string]any{"evil": "../../../etc/secret.json"}
	ctx, warnings, err := BuildContext(fmData, nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	// Should warn about the file (path traversal blocked or not found), not crash
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for path traversal, got %d: %v", len(warnings), warnings)
	}
	if _, ok := ctx.Vars["evil"]; ok {
		t.Error("expected evil key to not be set")
	}
}

func TestIsFileReference(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"data.json", true},
		{"config.yaml", true},
		{"config.yml", true},
		{"team.csv", true},
		{"Acme Corp", false},
		{"2026", false},
		{"hello.txt", false},
	}

	for _, tt := range tests {
		got := isFileReference(tt.input)
		if got != tt.want {
			t.Errorf("isFileReference(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestBuildContext_Empty(t *testing.T) {
	ctx, warnings, err := BuildContext(nil, nil, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(ctx.Vars) != 0 {
		t.Errorf("expected empty vars, got %v", ctx.Vars)
	}
}


func TestBuildContext_FileSizeLimit(t *testing.T) {
	dir := t.TempDir()
	// Create a file just over 1MB
	bigPath := filepath.Join(dir, "big.json")
	bigData := make([]byte, 1024*1024+1)
	for i := range bigData {
		bigData[i] = 'x'
	}
	if err := os.WriteFile(bigPath, bigData, 0644); err != nil {
		t.Fatal(err)
	}

	fmData := map[string]any{"big": "big.json"}
	_, warnings, err := BuildContext(fmData, nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	// Should produce a warning about file size (not silently load the huge file)
	if len(warnings) == 0 {
		t.Error("expected warning for oversized file, got none")
	}
}
