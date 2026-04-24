// Package config provides configuration management for the slide generator service.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/safeyaml"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Config holds all configuration for the service.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Templates TemplatesConfig `yaml:"templates"`
	Storage   StorageConfig   `yaml:"storage"`
	Images    ImageConfig     `yaml:"images"`
	SVG       types.SVGConfig  `yaml:"svg"`
}

// ValidationMode defines how template validation errors are handled.
type ValidationMode string

const (
	ValidationModeStrict ValidationMode = "strict"
	ValidationModeSoft   ValidationMode = "soft"
)

// ImageConfig holds configuration for image handling and security.
type ImageConfig struct {
	AllowedBasePaths []string `yaml:"allowed_base_paths"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	// PprofPort is the port for the pprof debug server. 0 disables pprof.
	PprofPort int    `yaml:"pprof_port"`
	PprofBind string `yaml:"pprof_bind"`
}

// TemplatesConfig holds template directory configuration.
type TemplatesConfig struct {
	Dir            string         `yaml:"dir"`
	CacheDir       string         `yaml:"cache_dir"`
	ValidationMode ValidationMode `yaml:"validation_mode"`
}

// IsStrictValidation returns true if template validation is in strict mode.
func (tc *TemplatesConfig) IsStrictValidation() bool {
	return tc.ValidationMode == ValidationModeStrict
}

// StorageConfig holds file storage configuration.
type StorageConfig struct {
	OutputDir       string        `yaml:"output_dir"`
	FileRetention   time.Duration `yaml:"file_retention"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	TempFileMaxAge  time.Duration `yaml:"temp_file_max_age"`
}

// DefaultConfig returns configuration with default values.
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    60 * time.Second,
			ShutdownTimeout: 10 * time.Second,
			PprofPort:       0,
			PprofBind:       "127.0.0.1",
		},
		Templates: TemplatesConfig{
			Dir:            "./templates",
			CacheDir:       "./cache/templates",
			ValidationMode: ValidationModeSoft,
		},
		Storage: StorageConfig{
			OutputDir:       "./output",
			FileRetention:   1 * time.Hour,
			CleanupInterval: 5 * time.Minute,
			TempFileMaxAge:  1 * time.Hour,
		},
		Images: ImageConfig{
			AllowedBasePaths: []string{},
		},
		SVG: types.SVGConfig{
			Strategy:              types.SVGStrategyNative,
			Scale:                 types.DefaultSVGScale,
			NativeCompatibility:   types.SVGCompatIgnore,
			PreferredPNGConverter: types.PNGConverterAuto,
			MaxPNGWidth:           types.DefaultMaxPNGWidth,
		},
	}
}

// Load loads configuration from a YAML file, with environment variable overrides.
// When path is non-empty but the file does not exist, Load silently falls back to
// defaults. This is intentional: the Dockerfile hardcodes --config /app/config.yaml
// in CMD, so containers start cleanly with defaults when no config is mounted.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return Config{}, fmt.Errorf("read config file: %w", err)
			}
			// File not found — use defaults (see comment above).
		} else {
			if err := safeyaml.Unmarshal(data, &cfg); err != nil {
				return Config{}, fmt.Errorf("parse config file: %w", err)
			}
		}
	}

	applyEnvOverrides(&cfg)

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config.
func applyEnvOverrides(cfg *Config) {
	applyServerEnvOverrides(&cfg.Server)
	applyTemplateEnvOverrides(&cfg.Templates)
	applyStorageEnvOverrides(&cfg.Storage)
	applyImageEnvOverrides(&cfg.Images)
	applySVGEnvOverrides(&cfg.SVG)
}

// applyServerEnvOverrides applies server-related environment variable overrides.
func applyServerEnvOverrides(cfg *ServerConfig) {
	if v := os.Getenv("PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		} else {
			slog.Warn("ignoring unparseable PORT env var, using default", "value", v, "default", cfg.Port, "error", err)
		}
	}
	applyPprofEnvOverrides(cfg)
}

// applyPprofEnvOverrides applies pprof debug server environment variable overrides.
func applyPprofEnvOverrides(cfg *ServerConfig) {
	if v := os.Getenv("PPROF_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port >= 0 && port <= 65535 {
			cfg.PprofPort = port
		} else if err != nil {
			slog.Warn("ignoring unparseable PPROF_PORT env var, using default", "value", v, "default", cfg.PprofPort, "error", err)
		} else {
			slog.Warn("ignoring out-of-range PPROF_PORT env var, using default", "value", v, "default", cfg.PprofPort)
		}
	}
	if v := os.Getenv("PPROF_BIND"); v != "" {
		cfg.PprofBind = v
	}
}

// applyTemplateEnvOverrides applies template-related environment variable overrides.
func applyTemplateEnvOverrides(cfg *TemplatesConfig) {
	if v := os.Getenv("TEMPLATES_DIR"); v != "" {
		cfg.Dir = v
	}
	if v := os.Getenv("TEMPLATE_VALIDATION_MODE"); v != "" {
		mode := ValidationMode(strings.ToLower(v))
		if mode == ValidationModeStrict || mode == ValidationModeSoft {
			cfg.ValidationMode = mode
		} else {
			slog.Warn("ignoring invalid TEMPLATE_VALIDATION_MODE env var, using default", "value", v, "valid", "strict, soft", "default", cfg.ValidationMode)
		}
	}
}

// applyStorageEnvOverrides applies storage-related environment variable overrides.
func applyStorageEnvOverrides(cfg *StorageConfig) {
	if v := os.Getenv("OUTPUT_DIR"); v != "" {
		cfg.OutputDir = v
	}
	if v := os.Getenv("TEMP_FILE_MAX_AGE"); v != "" {
		if d, err := parseDurationOrSeconds(v); err == nil && d > 0 {
			cfg.TempFileMaxAge = d
		} else if err != nil {
			slog.Warn("ignoring unparseable TEMP_FILE_MAX_AGE env var, using default", "value", v, "default", cfg.TempFileMaxAge, "error", err)
		} else {
			slog.Warn("ignoring non-positive TEMP_FILE_MAX_AGE env var, using default", "value", v, "default", cfg.TempFileMaxAge)
		}
	}
	if v := os.Getenv("TEMP_CLEANUP_INTERVAL"); v != "" {
		if d, err := parseDurationOrSeconds(v); err == nil && d > 0 {
			cfg.CleanupInterval = d
		} else if err != nil {
			slog.Warn("ignoring unparseable TEMP_CLEANUP_INTERVAL env var, using default", "value", v, "default", cfg.CleanupInterval, "error", err)
		} else {
			slog.Warn("ignoring non-positive TEMP_CLEANUP_INTERVAL env var, using default", "value", v, "default", cfg.CleanupInterval)
		}
	}
}

// applyImageEnvOverrides applies image-related environment variable overrides.
func applyImageEnvOverrides(cfg *ImageConfig) {
	if v := os.Getenv("ALLOWED_IMAGE_PATHS"); v != "" {
		cfg.AllowedBasePaths = parseCommaSeparated(v)
	}
}

// applySVGEnvOverrides applies SVG-related environment variable overrides.
func applySVGEnvOverrides(cfg *types.SVGConfig) {
	if v := os.Getenv("SVG_STRATEGY"); v != "" {
		strategy := types.SVGConversionStrategy(strings.ToLower(v))
		if strategy == types.SVGStrategyPNG || strategy == types.SVGStrategyEMF || strategy == types.SVGStrategyNative {
			cfg.Strategy = strategy
		} else {
			slog.Warn("ignoring invalid SVG_STRATEGY env var, using default", "value", v, "valid", "png, emf, native", "default", cfg.Strategy)
		}
	}
	if v := os.Getenv("SVG_SCALE"); v != "" {
		if scale, err := strconv.ParseFloat(v, 64); err == nil && scale > 0 {
			cfg.Scale = scale
		} else if err != nil {
			slog.Warn("ignoring unparseable SVG_SCALE env var, using default", "value", v, "default", cfg.Scale, "error", err)
		} else {
			slog.Warn("ignoring non-positive SVG_SCALE env var, using default", "value", v, "default", cfg.Scale)
		}
	}
	if v := os.Getenv("SVG_NATIVE_COMPATIBILITY"); v != "" {
		compat := types.SVGNativeCompatibility(strings.ToLower(v))
		switch compat {
		case types.SVGCompatWarn, types.SVGCompatFallback, types.SVGCompatStrict, types.SVGCompatIgnore:
			cfg.NativeCompatibility = compat
		default:
			slog.Warn("ignoring invalid SVG_NATIVE_COMPATIBILITY env var, using default", "value", v, "valid", "warn, fallback, strict, ignore", "default", cfg.NativeCompatibility)
		}
	}
	if v := os.Getenv("SVG_PNG_CONVERTER"); v != "" {
		converter := strings.ToLower(v)
		switch converter {
		case types.PNGConverterAuto, types.PNGConverterRsvg, types.PNGConverterResvg:
			cfg.PreferredPNGConverter = converter
		default:
			slog.Warn("ignoring invalid SVG_PNG_CONVERTER env var, using default", "value", v, "valid", "auto, rsvg-convert, resvg", "default", cfg.PreferredPNGConverter)
		}
	}
	if v := os.Getenv("MAX_PNG_WIDTH"); v != "" {
		if width, err := strconv.Atoi(v); err == nil && width >= 0 {
			cfg.MaxPNGWidth = width
		} else if err != nil {
			slog.Warn("ignoring unparseable MAX_PNG_WIDTH env var, using default", "value", v, "default", cfg.MaxPNGWidth, "error", err)
		} else {
			slog.Warn("ignoring negative MAX_PNG_WIDTH env var, using default", "value", v, "default", cfg.MaxPNGWidth)
		}
	}
}

// parseDurationOrSeconds parses a duration string using time.ParseDuration.
// If the value has no unit suffix (e.g., "1800"), it falls back to interpreting
// it as an integer number of seconds for backward compatibility.
func parseDurationOrSeconds(v string) (time.Duration, error) {
	// Try time.ParseDuration first (supports "30m", "1h", "90s", etc.)
	if d, err := time.ParseDuration(v); err == nil {
		return d, nil
	}
	// Fall back to integer seconds for backward compatibility
	secs, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("not a valid duration or integer seconds: %q", v)
	}
	return time.Duration(secs) * time.Second, nil
}

// parseCommaSeparated splits a comma-separated string into a slice of trimmed strings.
func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Templates.Dir == "" {
		return fmt.Errorf("templates directory is required")
	}
	if c.Storage.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}
	return nil
}
