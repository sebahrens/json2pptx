package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempConfig writes a YAML config file into a temp dir and returns its path.
func writeTempConfig(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestLoadRunConfig_EnvAppliesWithoutConfigFile verifies that environment
// overrides take effect even when no --config file is supplied (regression:
// the CLI path previously started from DefaultConfig() and skipped Load("")).
func TestLoadRunConfig_EnvAppliesWithoutConfigFile(t *testing.T) {
	t.Setenv("OUTPUT_DIR", "/env/output")
	t.Setenv("TEMPLATES_DIR", "/env/templates")

	cfg, err := loadRunConfig("", "", "", false)
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if cfg.Storage.OutputDir != "/env/output" {
		t.Errorf("OutputDir = %q, want /env/output (env override should apply without --config)", cfg.Storage.OutputDir)
	}
	if cfg.Templates.Dir != "/env/templates" {
		t.Errorf("Templates.Dir = %q, want /env/templates (env override should apply without --config)", cfg.Templates.Dir)
	}
}

// TestLoadRunConfig_ConfigNotOverwrittenByDefaults verifies that config-file
// values survive when the directory flags were not explicitly set (passed as
// empty strings). Previously the non-empty default flag values clobbered them.
func TestLoadRunConfig_ConfigNotOverwrittenByDefaults(t *testing.T) {
	path := writeTempConfig(t, "storage:\n  output_dir: /cfg/output\ntemplates:\n  dir: /cfg/templates\n")

	cfg, err := loadRunConfig(path, "", "", false)
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if cfg.Storage.OutputDir != "/cfg/output" {
		t.Errorf("OutputDir = %q, want /cfg/output (default flag value must not overwrite config)", cfg.Storage.OutputDir)
	}
	if cfg.Templates.Dir != "/cfg/templates" {
		t.Errorf("Templates.Dir = %q, want /cfg/templates (default flag value must not overwrite config)", cfg.Templates.Dir)
	}
}

// TestLoadRunConfig_ExplicitFlagWins verifies that explicitly provided CLI
// values (non-empty) override both config-file and environment values.
func TestLoadRunConfig_ExplicitFlagWins(t *testing.T) {
	t.Setenv("OUTPUT_DIR", "/env/output")
	t.Setenv("TEMPLATES_DIR", "/env/templates")
	path := writeTempConfig(t, "storage:\n  output_dir: /cfg/output\ntemplates:\n  dir: /cfg/templates\n")

	cfg, err := loadRunConfig(path, "/cli/templates", "/cli/output", false)
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if cfg.Storage.OutputDir != "/cli/output" {
		t.Errorf("OutputDir = %q, want /cli/output (explicit CLI flag must win)", cfg.Storage.OutputDir)
	}
	if cfg.Templates.Dir != "/cli/templates" {
		t.Errorf("Templates.Dir = %q, want /cli/templates (explicit CLI flag must win)", cfg.Templates.Dir)
	}
}

// TestLoadRunConfig_EnvOverridesConfigFile verifies env > config-file precedence
// is preserved (config.Load applies env overrides after parsing the file).
func TestLoadRunConfig_EnvOverridesConfigFile(t *testing.T) {
	t.Setenv("OUTPUT_DIR", "/env/output")
	path := writeTempConfig(t, "storage:\n  output_dir: /cfg/output\n")

	cfg, err := loadRunConfig(path, "", "", false)
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	if cfg.Storage.OutputDir != "/env/output" {
		t.Errorf("OutputDir = %q, want /env/output (env should override config file)", cfg.Storage.OutputDir)
	}
}
