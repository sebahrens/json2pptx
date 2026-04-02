package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "invalid port zero",
			modify: func(c *Config) {
				c.Server.Port = 0
			},
			wantErr: true,
		},
		{
			name: "invalid port too high",
			modify: func(c *Config) {
				c.Server.Port = 70000
			},
			wantErr: true,
		},
		{
			name: "empty templates dir",
			modify: func(c *Config) {
				c.Templates.Dir = ""
			},
			wantErr: true,
		},
		{
			name: "empty output dir",
			modify: func(c *Config) {
				c.Storage.OutputDir = ""
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadNonExistent(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Errorf("Load() should not error on missing file, got %v", err)
	}

	// Should return defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := `
server:
  port: 9090
templates:
  dir: "/custom/templates"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}

	if cfg.Templates.Dir != "/custom/templates" {
		t.Errorf("expected templates dir /custom/templates, got %s", cfg.Templates.Dir)
	}
}

func TestEnvOverrides(t *testing.T) {
	// Set env vars
	t.Setenv("PORT", "3000")
	t.Setenv("TEMPLATES_DIR", "/env/templates")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("expected port 3000 from env, got %d", cfg.Server.Port)
	}

	if cfg.Templates.Dir != "/env/templates" {
		t.Errorf("expected templates dir from env, got %s", cfg.Templates.Dir)
	}
}

// TestParseCommaSeparated validates the parseCommaSeparated helper function.
func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "single value",
			input: "value1",
			want:  []string{"value1"},
		},
		{
			name:  "multiple values",
			input: "value1,value2,value3",
			want:  []string{"value1", "value2", "value3"},
		},
		{
			name:  "values with spaces",
			input: "value1 , value2 , value3",
			want:  []string{"value1", "value2", "value3"},
		},
		{
			name:  "empty values filtered",
			input: "value1,,value2,  ,value3",
			want:  []string{"value1", "value2", "value3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommaSeparated(tt.input)

			if tt.want == nil && got != nil {
				t.Errorf("expected nil, got %v", got)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("expected %d values, got %d: %v", len(tt.want), len(got), got)
				return
			}

			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("value[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// A16: SVG Configuration Tests

// TestDefaultConfigHasSVGDefaults validates default SVG config values.
func TestDefaultConfigHasSVGDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.SVG.Strategy != types.SVGStrategyNative {
		t.Errorf("expected default SVG strategy 'native', got %q", cfg.SVG.Strategy)
	}

	if cfg.SVG.Scale != types.DefaultSVGScale {
		t.Errorf("expected default SVG scale %.1f, got %.1f", types.DefaultSVGScale, cfg.SVG.Scale)
	}
}

// TestSVGStrategyFromEnv validates SVG_STRATEGY environment variable parsing.
func TestSVGStrategyFromEnv(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		wantStrategy types.SVGConversionStrategy
	}{
		{
			name:         "png strategy",
			envValue:     "png",
			wantStrategy: types.SVGStrategyPNG,
		},
		{
			name:         "emf strategy",
			envValue:     "emf",
			wantStrategy: types.SVGStrategyEMF,
		},
		{
			name:         "PNG uppercase",
			envValue:     "PNG",
			wantStrategy: types.SVGStrategyPNG,
		},
		{
			name:         "EMF uppercase",
			envValue:     "EMF",
			wantStrategy: types.SVGStrategyEMF,
		},
		{
			name:         "native strategy",
			envValue:     "native",
			wantStrategy: types.SVGStrategyNative,
		},
		{
			name:         "NATIVE uppercase",
			envValue:     "NATIVE",
			wantStrategy: types.SVGStrategyNative,
		},
		{
			name:         "invalid strategy keeps default",
			envValue:     "invalid",
			wantStrategy: types.SVGStrategyNative, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SVG_STRATEGY", tt.envValue)

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.SVG.Strategy != tt.wantStrategy {
				t.Errorf("expected SVG strategy %q, got %q", tt.wantStrategy, cfg.SVG.Strategy)
			}
		})
	}
}

// TestSVGScaleFromEnv validates SVG_SCALE environment variable parsing.
func TestSVGScaleFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		wantScale float64
	}{
		{
			name:      "scale 1.0",
			envValue:  "1.0",
			wantScale: 1.0,
		},
		{
			name:      "scale 3.0",
			envValue:  "3.0",
			wantScale: 3.0,
		},
		{
			name:      "scale integer",
			envValue:  "4",
			wantScale: 4.0,
		},
		{
			name:      "invalid scale keeps default",
			envValue:  "invalid",
			wantScale: types.DefaultSVGScale,
		},
		{
			name:      "zero scale keeps default",
			envValue:  "0",
			wantScale: types.DefaultSVGScale,
		},
		{
			name:      "negative scale keeps default",
			envValue:  "-1.0",
			wantScale: types.DefaultSVGScale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SVG_SCALE", tt.envValue)

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.SVG.Scale != tt.wantScale {
				t.Errorf("expected SVG scale %.1f, got %.1f", tt.wantScale, cfg.SVG.Scale)
			}
		})
	}
}

// TestSVGConfigFromYAML validates SVG config from YAML file.
func TestSVGConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := `
server:
  port: 8080
templates:
  dir: "/templates"
storage:
  output_dir: "/output"
svg:
  strategy: emf
  scale: 3.0
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SVG.Strategy != types.SVGStrategyEMF {
		t.Errorf("expected SVG strategy 'emf', got %q", cfg.SVG.Strategy)
	}

	if cfg.SVG.Scale != 3.0 {
		t.Errorf("expected SVG scale 3.0, got %.1f", cfg.SVG.Scale)
	}
}

// TestSVGNativeStrategyFromYAML validates native SVG strategy from YAML file.
func TestSVGNativeStrategyFromYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := `
server:
  port: 8080
templates:
  dir: "/templates"
storage:
  output_dir: "/output"
svg:
  strategy: native
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SVG.Strategy != types.SVGStrategyNative {
		t.Errorf("expected SVG strategy 'native', got %q", cfg.SVG.Strategy)
	}
}

// A18: Template Validation Mode Configuration Tests

// TestDefaultConfigHasSoftValidationMode validates default config uses soft validation mode.
func TestDefaultConfigHasSoftValidationMode(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Templates.ValidationMode != ValidationModeSoft {
		t.Errorf("expected default validation mode 'soft', got %q", cfg.Templates.ValidationMode)
	}
}

// TestTemplatesConfig_IsStrictValidation validates the IsStrictValidation helper method.
func TestTemplatesConfig_IsStrictValidation(t *testing.T) {
	tests := []struct {
		name string
		mode ValidationMode
		want bool
	}{
		{
			name: "strict mode returns true",
			mode: ValidationModeStrict,
			want: true,
		},
		{
			name: "soft mode returns false",
			mode: ValidationModeSoft,
			want: false,
		},
		{
			name: "empty mode returns false",
			mode: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := TemplatesConfig{ValidationMode: tt.mode}
			if got := tc.IsStrictValidation(); got != tt.want {
				t.Errorf("IsStrictValidation() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidationModeFromEnv validates TEMPLATE_VALIDATION_MODE environment variable parsing.
func TestValidationModeFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		wantMode ValidationMode
	}{
		{
			name:     "strict mode",
			envValue: "strict",
			wantMode: ValidationModeStrict,
		},
		{
			name:     "soft mode",
			envValue: "soft",
			wantMode: ValidationModeSoft,
		},
		{
			name:     "STRICT uppercase",
			envValue: "STRICT",
			wantMode: ValidationModeStrict,
		},
		{
			name:     "SOFT uppercase",
			envValue: "SOFT",
			wantMode: ValidationModeSoft,
		},
		{
			name:     "invalid mode keeps default",
			envValue: "invalid",
			wantMode: ValidationModeSoft, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEMPLATE_VALIDATION_MODE", tt.envValue)

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Templates.ValidationMode != tt.wantMode {
				t.Errorf("expected validation mode %q, got %q", tt.wantMode, cfg.Templates.ValidationMode)
			}
		})
	}
}

// TestValidationModeFromYAML validates validation_mode in YAML config (A18).
func TestValidationModeFromYAML(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantMode    ValidationMode
	}{
		{
			name: "strict mode from YAML",
			yamlContent: `
server:
  port: 8080
templates:
  dir: "/templates"
  validation_mode: strict
storage:
  output_dir: "/output"
`,
			wantMode: ValidationModeStrict,
		},
		{
			name: "soft mode from YAML",
			yamlContent: `
server:
  port: 8080
templates:
  dir: "/templates"
  validation_mode: soft
storage:
  output_dir: "/output"
`,
			wantMode: ValidationModeSoft,
		},
		{
			name: "no validation_mode defaults to soft",
			yamlContent: `
server:
  port: 8080
templates:
  dir: "/templates"
storage:
  output_dir: "/output"
`,
			wantMode: ValidationModeSoft,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")

			if err := os.WriteFile(configPath, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Templates.ValidationMode != tt.wantMode {
				t.Errorf("expected validation mode %q, got %q", tt.wantMode, cfg.Templates.ValidationMode)
			}
		})
	}
}

// SVG Native Compatibility Tests

// TestDefaultConfigHasIgnoreNativeCompatibility validates default config uses ignore mode.
func TestDefaultConfigHasIgnoreNativeCompatibility(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.SVG.NativeCompatibility != types.SVGCompatIgnore {
		t.Errorf("expected default native compatibility 'ignore', got %q", cfg.SVG.NativeCompatibility)
	}
}

// TestSVGNativeCompatibilityFromEnv validates SVG_NATIVE_COMPATIBILITY environment variable parsing.
func TestSVGNativeCompatibilityFromEnv(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		wantCompat types.SVGNativeCompatibility
	}{
		{
			name:       "warn compatibility",
			envValue:   "warn",
			wantCompat: types.SVGCompatWarn,
		},
		{
			name:       "WARN uppercase",
			envValue:   "WARN",
			wantCompat: types.SVGCompatWarn,
		},
		{
			name:       "fallback compatibility",
			envValue:   "fallback",
			wantCompat: types.SVGCompatFallback,
		},
		{
			name:       "strict compatibility",
			envValue:   "strict",
			wantCompat: types.SVGCompatStrict,
		},
		{
			name:       "ignore compatibility",
			envValue:   "ignore",
			wantCompat: types.SVGCompatIgnore,
		},
		{
			name:       "invalid uses default",
			envValue:   "invalid",
			wantCompat: types.SVGCompatIgnore, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SVG_NATIVE_COMPATIBILITY", tt.envValue)

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.SVG.NativeCompatibility != tt.wantCompat {
				t.Errorf("expected native compatibility %q, got %q", tt.wantCompat, cfg.SVG.NativeCompatibility)
			}
		})
	}
}

// TestSVGNativeCompatibilityFromYAML validates native_compatibility from YAML file.
func TestSVGNativeCompatibilityFromYAML(t *testing.T) {
	tests := []struct {
		name       string
		yamlValue  string
		wantCompat types.SVGNativeCompatibility
	}{
		{
			name:       "fallback from yaml",
			yamlValue:  "fallback",
			wantCompat: types.SVGCompatFallback,
		},
		{
			name:       "strict from yaml",
			yamlValue:  "strict",
			wantCompat: types.SVGCompatStrict,
		},
		{
			name:       "ignore from yaml",
			yamlValue:  "ignore",
			wantCompat: types.SVGCompatIgnore,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")

			content := `
server:
  port: 8080
templates:
  dir: "/templates"
storage:
  output_dir: "/output"
svg:
  strategy: native
  native_compatibility: ` + tt.yamlValue + `
`
			if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.SVG.NativeCompatibility != tt.wantCompat {
				t.Errorf("expected native compatibility %q, got %q", tt.wantCompat, cfg.SVG.NativeCompatibility)
			}
		})
	}
}

// Pprof Configuration Tests

// TestDefaultConfigHasPprofDisabled validates default config has pprof disabled.
func TestDefaultConfigHasPprofDisabled(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.PprofPort != 0 {
		t.Errorf("expected default pprof port 0 (disabled), got %d", cfg.Server.PprofPort)
	}

	if cfg.Server.PprofBind != "127.0.0.1" {
		t.Errorf("expected default pprof bind 127.0.0.1, got %q", cfg.Server.PprofBind)
	}
}

// TestPprofPortFromEnv validates PPROF_PORT environment variable parsing.
func TestPprofPortFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		wantPort int
	}{
		{
			name:     "valid port 6060",
			envValue: "6060",
			wantPort: 6060,
		},
		{
			name:     "valid port 9090",
			envValue: "9090",
			wantPort: 9090,
		},
		{
			name:     "disable with 0",
			envValue: "0",
			wantPort: 0,
		},
		{
			name:     "max port",
			envValue: "65535",
			wantPort: 65535,
		},
		{
			name:     "invalid port too high",
			envValue: "65536",
			wantPort: 0, // keeps default
		},
		{
			name:     "invalid negative",
			envValue: "-1",
			wantPort: 0, // keeps default
		},
		{
			name:     "invalid string",
			envValue: "invalid",
			wantPort: 0, // keeps default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PPROF_PORT", tt.envValue)

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Server.PprofPort != tt.wantPort {
				t.Errorf("expected pprof port %d, got %d", tt.wantPort, cfg.Server.PprofPort)
			}
		})
	}
}

// TestPprofBindFromEnv validates PPROF_BIND environment variable parsing.
func TestPprofBindFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		wantBind string
	}{
		{
			name:     "localhost",
			envValue: "localhost",
			wantBind: "localhost",
		},
		{
			name:     "loopback IPv4",
			envValue: "127.0.0.1",
			wantBind: "127.0.0.1",
		},
		{
			name:     "all interfaces (dangerous)",
			envValue: "0.0.0.0",
			wantBind: "0.0.0.0",
		},
		{
			name:     "specific IP",
			envValue: "10.0.0.1",
			wantBind: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PPROF_BIND", tt.envValue)

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Server.PprofBind != tt.wantBind {
				t.Errorf("expected pprof bind %q, got %q", tt.wantBind, cfg.Server.PprofBind)
			}
		})
	}
}

// TestPprofConfigFromYAML validates pprof config from YAML file.
func TestPprofConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := `
server:
  port: 8080
  pprof_port: 6060
  pprof_bind: "127.0.0.1"
templates:
  dir: "/templates"
storage:
  output_dir: "/output"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.PprofPort != 6060 {
		t.Errorf("expected pprof port 6060, got %d", cfg.Server.PprofPort)
	}

	if cfg.Server.PprofBind != "127.0.0.1" {
		t.Errorf("expected pprof bind 127.0.0.1, got %q", cfg.Server.PprofBind)
	}
}

// TestPprofEnvOverrideYAML validates PPROF_PORT env overrides YAML config.
func TestPprofEnvOverrideYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	// YAML sets port 6060
	content := `
server:
  port: 8080
  pprof_port: 6060
templates:
  dir: "/templates"
storage:
  output_dir: "/output"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Env overrides to 9999
	t.Setenv("PPROF_PORT", "9999")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Env should override YAML
	if cfg.Server.PprofPort != 9999 {
		t.Errorf("expected pprof port 9999 (from env), got %d", cfg.Server.PprofPort)
	}
}

// =============================================================================
// SVG PNG Converter Preference Tests
// =============================================================================

// TestDefaultConfigHasPNGConverterDefault validates default PNG converter config.
func TestDefaultConfigHasPNGConverterDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.SVG.PreferredPNGConverter != types.PNGConverterAuto {
		t.Errorf("expected default PNG converter 'auto', got %q", cfg.SVG.PreferredPNGConverter)
	}
}

// TestSVGPNGConverterFromEnv validates SVG_PNG_CONVERTER environment variable parsing.
func TestSVGPNGConverterFromEnv(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		wantConverter string
	}{
		{
			name:          "auto",
			envValue:      "auto",
			wantConverter: types.PNGConverterAuto,
		},
		{
			name:          "AUTO uppercase",
			envValue:      "AUTO",
			wantConverter: types.PNGConverterAuto,
		},
		{
			name:          "rsvg-convert",
			envValue:      "rsvg-convert",
			wantConverter: types.PNGConverterRsvg,
		},
		{
			name:          "RSVG-CONVERT uppercase",
			envValue:      "RSVG-CONVERT",
			wantConverter: types.PNGConverterRsvg,
		},
		{
			name:          "resvg",
			envValue:      "resvg",
			wantConverter: types.PNGConverterResvg,
		},
		{
			name:          "RESVG uppercase",
			envValue:      "RESVG",
			wantConverter: types.PNGConverterResvg,
		},
		{
			name:          "invalid value keeps default",
			envValue:      "invalid",
			wantConverter: types.PNGConverterAuto, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SVG_PNG_CONVERTER", tt.envValue)

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.SVG.PreferredPNGConverter != tt.wantConverter {
				t.Errorf("expected PNG converter %q, got %q", tt.wantConverter, cfg.SVG.PreferredPNGConverter)
			}
		})
	}
}

// TestSVGPNGConverterFromYAML validates preferred_png_converter from YAML file.
func TestSVGPNGConverterFromYAML(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		wantConverter string
	}{
		{
			name: "resvg in yaml",
			yamlContent: `
svg:
  preferred_png_converter: resvg
`,
			wantConverter: types.PNGConverterResvg,
		},
		{
			name: "rsvg-convert in yaml",
			yamlContent: `
svg:
  preferred_png_converter: rsvg-convert
`,
			wantConverter: types.PNGConverterRsvg,
		},
		{
			name: "auto in yaml",
			yamlContent: `
svg:
  preferred_png_converter: auto
`,
			wantConverter: types.PNGConverterAuto,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			content := `
server:
  port: 8080
templates:
  dir: "/templates"
storage:
  output_dir: "/output"
` + tt.yamlContent
			if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.SVG.PreferredPNGConverter != tt.wantConverter {
				t.Errorf("expected PNG converter %q, got %q", tt.wantConverter, cfg.SVG.PreferredPNGConverter)
			}
		})
	}
}

// TestDefaultConfigHasStorageDefaults validates storage configuration defaults.
func TestDefaultConfigHasStorageDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Storage.OutputDir != "./output" {
		t.Errorf("OutputDir = %q, want ./output", cfg.Storage.OutputDir)
	}
	if cfg.Storage.FileRetention != 1*time.Hour {
		t.Errorf("FileRetention = %v, want 1h", cfg.Storage.FileRetention)
	}
	if cfg.Storage.CleanupInterval != 5*time.Minute {
		t.Errorf("CleanupInterval = %v, want 5m", cfg.Storage.CleanupInterval)
	}
	if cfg.Storage.TempFileMaxAge != 1*time.Hour {
		t.Errorf("TempFileMaxAge = %v, want 1h", cfg.Storage.TempFileMaxAge)
	}
}

// TestStorageEnvOverrides validates storage environment variable overrides.
func TestStorageEnvOverrides(t *testing.T) {
	t.Run("OUTPUT_DIR override", func(t *testing.T) {
		t.Setenv("OUTPUT_DIR", "/custom/output")
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Storage.OutputDir != "/custom/output" {
			t.Errorf("OutputDir = %q, want /custom/output", cfg.Storage.OutputDir)
		}
	})

	t.Run("TEMP_FILE_MAX_AGE override", func(t *testing.T) {
		t.Setenv("TEMP_FILE_MAX_AGE", "1800") // 30 minutes in seconds
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Storage.TempFileMaxAge != 30*time.Minute {
			t.Errorf("TempFileMaxAge = %v, want 30m", cfg.Storage.TempFileMaxAge)
		}
	})

	t.Run("TEMP_CLEANUP_INTERVAL override", func(t *testing.T) {
		t.Setenv("TEMP_CLEANUP_INTERVAL", "120") // 2 minutes in seconds
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Storage.CleanupInterval != 2*time.Minute {
			t.Errorf("CleanupInterval = %v, want 2m", cfg.Storage.CleanupInterval)
		}
	})

	t.Run("invalid TEMP_FILE_MAX_AGE ignored", func(t *testing.T) {
		t.Setenv("TEMP_FILE_MAX_AGE", "invalid")
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		// Should use default
		if cfg.Storage.TempFileMaxAge != 1*time.Hour {
			t.Errorf("TempFileMaxAge = %v, want 1h (default)", cfg.Storage.TempFileMaxAge)
		}
	})

	t.Run("zero TEMP_FILE_MAX_AGE ignored", func(t *testing.T) {
		t.Setenv("TEMP_FILE_MAX_AGE", "0")
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		// Should use default because 0 is not > 0
		if cfg.Storage.TempFileMaxAge != 1*time.Hour {
			t.Errorf("TempFileMaxAge = %v, want 1h (default)", cfg.Storage.TempFileMaxAge)
		}
	})

	t.Run("negative TEMP_CLEANUP_INTERVAL ignored", func(t *testing.T) {
		t.Setenv("TEMP_CLEANUP_INTERVAL", "-60")
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		// Should use default because -60 is not > 0
		if cfg.Storage.CleanupInterval != 5*time.Minute {
			t.Errorf("CleanupInterval = %v, want 5m (default)", cfg.Storage.CleanupInterval)
		}
	})
}

// TestUnparseableEnvVarsLogWarnings validates that unparseable env vars produce slog.Warn messages.
func TestUnparseableEnvVarsLogWarnings(t *testing.T) {
	tests := []struct {
		name    string
		envKey  string
		envVal  string
		wantMsg string
	}{
		{
			name:    "bad PORT logs warning",
			envKey:  "PORT",
			envVal:  "808l",
			wantMsg: "ignoring unparseable PORT env var",
		},
		{
			name:    "bad PPROF_PORT logs warning",
			envKey:  "PPROF_PORT",
			envVal:  "notanumber",
			wantMsg: "ignoring unparseable PPROF_PORT env var",
		},
		{
			name:    "out-of-range PPROF_PORT logs warning",
			envKey:  "PPROF_PORT",
			envVal:  "99999",
			wantMsg: "ignoring out-of-range PPROF_PORT env var",
		},
		{
			name:    "bad TEMP_FILE_MAX_AGE logs warning",
			envKey:  "TEMP_FILE_MAX_AGE",
			envVal:  "abc",
			wantMsg: "ignoring unparseable TEMP_FILE_MAX_AGE env var",
		},
		{
			name:    "zero TEMP_FILE_MAX_AGE logs warning",
			envKey:  "TEMP_FILE_MAX_AGE",
			envVal:  "0",
			wantMsg: "ignoring non-positive TEMP_FILE_MAX_AGE env var",
		},
		{
			name:    "bad TEMP_CLEANUP_INTERVAL logs warning",
			envKey:  "TEMP_CLEANUP_INTERVAL",
			envVal:  "xyz",
			wantMsg: "ignoring unparseable TEMP_CLEANUP_INTERVAL env var",
		},
		{
			name:    "bad SVG_SCALE logs warning",
			envKey:  "SVG_SCALE",
			envVal:  "big",
			wantMsg: "ignoring unparseable SVG_SCALE env var",
		},
		{
			name:    "zero SVG_SCALE logs warning",
			envKey:  "SVG_SCALE",
			envVal:  "0",
			wantMsg: "ignoring non-positive SVG_SCALE env var",
		},
		{
			name:    "invalid SVG_STRATEGY logs warning",
			envKey:  "SVG_STRATEGY",
			envVal:  "bmp",
			wantMsg: "ignoring invalid SVG_STRATEGY env var",
		},
		{
			name:    "invalid TEMPLATE_VALIDATION_MODE logs warning",
			envKey:  "TEMPLATE_VALIDATION_MODE",
			envVal:  "relaxed",
			wantMsg: "ignoring invalid TEMPLATE_VALIDATION_MODE env var",
		},
		{
			name:    "invalid SVG_NATIVE_COMPATIBILITY logs warning",
			envKey:  "SVG_NATIVE_COMPATIBILITY",
			envVal:  "maybe",
			wantMsg: "ignoring invalid SVG_NATIVE_COMPATIBILITY env var",
		},
		{
			name:    "invalid SVG_PNG_CONVERTER logs warning",
			envKey:  "SVG_PNG_CONVERTER",
			envVal:  "imagemagick",
			wantMsg: "ignoring invalid SVG_PNG_CONVERTER env var",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)

			// Capture log output
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
			old := slog.Default()
			slog.SetDefault(logger)
			t.Cleanup(func() { slog.SetDefault(old) })

			_, err := Load("")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if !strings.Contains(buf.String(), tt.wantMsg) {
				t.Errorf("expected log to contain %q, got %q", tt.wantMsg, buf.String())
			}
		})
	}
}
