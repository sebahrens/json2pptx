package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// Default asset-size caps. SVG inputs typically carry per-symbol detail so a
// tighter soft cap is appropriate; raster images are allowed to be heavier
// before warning. The hard cap is uniform — anything beyond is rejected
// regardless of kind because it indicates an upload mistake.
const (
	defaultSVGSoftBytes    int64 = 2 * 1024 * 1024  // 2 MB
	defaultSVGHardBytes    int64 = 25 * 1024 * 1024 // 25 MB
	defaultRasterSoftBytes int64 = 8 * 1024 * 1024  // 8 MB
	defaultRasterHardBytes int64 = 25 * 1024 * 1024 // 25 MB
)

// Env-var overrides for the four cap thresholds. Each accepts a positive
// integer count of bytes; non-positive or unparseable values fall back to the
// default. Reading happens per call so test setups using t.Setenv take effect
// without process-wide caching.
const (
	envMaxSVGSoftBytes    = "JSON2PPTX_MAX_SVG_SOFT_BYTES"
	envMaxSVGHardBytes    = "JSON2PPTX_MAX_SVG_HARD_BYTES"
	envMaxRasterSoftBytes = "JSON2PPTX_MAX_RASTER_SOFT_BYTES"
	envMaxRasterHardBytes = "JSON2PPTX_MAX_RASTER_HARD_BYTES"
)

// assetSizeLimits is the resolved {soft, hard} pair for one asset kind.
type assetSizeLimits struct {
	SoftBytes int64
	HardBytes int64
}

// svgSizeLimits returns the active soft and hard caps for SVG assets, applying
// env-var overrides on top of the defaults.
func svgSizeLimits() assetSizeLimits {
	return assetSizeLimits{
		SoftBytes: envOrInt64(envMaxSVGSoftBytes, defaultSVGSoftBytes),
		HardBytes: envOrInt64(envMaxSVGHardBytes, defaultSVGHardBytes),
	}
}

// rasterSizeLimits returns the active soft and hard caps for raster
// (PNG/JPEG/GIF/BMP/TIFF/WEBP) assets, applying env-var overrides on top of
// the defaults.
func rasterSizeLimits() assetSizeLimits {
	return assetSizeLimits{
		SoftBytes: envOrInt64(envMaxRasterSoftBytes, defaultRasterSoftBytes),
		HardBytes: envOrInt64(envMaxRasterHardBytes, defaultRasterHardBytes),
	}
}

// limitsForExtension picks the right {soft, hard} pair based on a file
// extension (case-insensitive, leading dot expected — e.g. ".svg"). Unknown
// extensions are treated as raster so the hard cap still applies.
func limitsForExtension(ext string) assetSizeLimits {
	if strings.EqualFold(ext, ".svg") {
		return svgSizeLimits()
	}
	return rasterSizeLimits()
}

// envOrInt64 reads a positive int64 from the named environment variable,
// returning fallback when unset, empty, non-numeric, or non-positive.
func envOrInt64(name string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// assetKindForExt classifies an extension as "svg" or "raster" for diagnostic
// details so agents can tell which limit applied. The hard cap is the same in
// both today but the kind helps callers tune per-cap overrides.
func assetKindForExt(ext string) string {
	if strings.EqualFold(ext, ".svg") {
		return "svg"
	}
	return "raster"
}

// checkAssetSize evaluates a file's size against the soft/hard caps for its
// extension. Returns nil when the file is within the soft cap, a
// warning-severity finding when between soft and hard, and an error-severity
// finding when above the hard cap. resolvedPath is the on-disk path actually
// stat'd; rawInput echoes the agent-supplied value so diagnostics can quote
// what was submitted. assetKind, slideIdx, jsonPath, and code follow the
// surrounding diagnostic conventions.
func checkAssetSize(resolvedPath, rawInput, assetKind string, slideIdx int, jsonPath string, code diagnostics.Code) *diagnostics.Diagnostic {
	info, err := os.Stat(resolvedPath)
	if err != nil {
		// A missing or unreadable file is caught upstream by EvalSymlinks;
		// reaching here means a concurrent change. Don't double-report — the
		// caller's stat-based path resolution would have flagged it.
		return nil
	}
	size := info.Size()
	ext := strings.ToLower(filepath.Ext(resolvedPath))
	limits := limitsForExtension(ext)
	kind := assetKindForExt(ext)
	return assetSizeFinding(size, limits, kind, rawInput, assetKind, slideIdx, jsonPath, code)
}

// checkAssetSizeBytes is the in-memory equivalent of checkAssetSize for inline
// SVG markup (icon.svg_data) where no file exists on disk. The kind is forced
// to "svg" because inline data is always SVG markup.
func checkAssetSizeBytes(size int64, rawInput, assetKind string, slideIdx int, jsonPath string, code diagnostics.Code) *diagnostics.Diagnostic {
	limits := svgSizeLimits()
	return assetSizeFinding(size, limits, "svg", rawInput, assetKind, slideIdx, jsonPath, code)
}

// assetSizeFinding builds the ASSET_TOO_LARGE diagnostic from a measured size
// and resolved limits. Returns nil when the size is at or below the soft cap.
func assetSizeFinding(size int64, limits assetSizeLimits, mediaKind, rawInput, assetKind string, slideIdx int, jsonPath string, code diagnostics.Code) *diagnostics.Diagnostic {
	if size <= limits.SoftBytes {
		return nil
	}
	severity := diagnostics.SeverityWarning
	if size > limits.HardBytes {
		severity = diagnostics.SeverityError
	}
	capBytes := limits.SoftBytes
	capKind := "soft"
	if severity == diagnostics.SeverityError {
		capBytes = limits.HardBytes
		capKind = "hard"
	}
	msg := fmt.Sprintf(
		"%s %q is %s (%d bytes), exceeding the %s cap of %s for %s assets",
		assetKind, rawInput, humanizeBytes(size), size, capKind, humanizeBytes(capBytes), mediaKind,
	)
	remediation := "shrink the asset (optimize the SVG, downscale the raster) or supply a smaller alternative"
	if severity == diagnostics.SeverityWarning {
		remediation = "consider shrinking the asset; the engine will still embed it but the deck size and downstream rendering speed will suffer"
	}
	return &diagnostics.Diagnostic{
		Code:     diagnostics.CodeAssetTooLarge,
		Message:  msg,
		Path:     jsonPath,
		Severity: severity,
		Details: map[string]any{
			"slide_index":     slideIdx,
			"asset_kind":      assetKind,
			"media_kind":      mediaKind,
			"input_value":     rawInput,
			"size_bytes":      size,
			"soft_cap_bytes":  limits.SoftBytes,
			"hard_cap_bytes":  limits.HardBytes,
			"exceeded_cap":    capKind,
			"remediation":     remediation,
		},
	}
}

// checkInlineSVGSize evaluates inline svg_data markup against the SVG caps.
// Returns the diagnostic (or nil if within the soft cap) and a blocked flag
// that is true when the diagnostic is error-severity, so callers can decide
// whether to short-circuit downstream resolution.
func checkInlineSVGSize(svgData string, slideIdx int, jsonPath string) (*diagnostics.Diagnostic, bool) {
	preview := svgData
	if len(preview) > 80 {
		preview = preview[:80] + "..."
	}
	diag := checkAssetSizeBytes(int64(len(svgData)), preview, "icon", slideIdx, jsonPath, diagnostics.CodeAssetTooLarge)
	if diag == nil {
		return nil, false
	}
	return diag, diag.Severity == diagnostics.SeverityError
}

// humanizeBytes renders an int64 byte count as a short decimal-prefix string
// ("1.5 MB"). The threshold ladder mirrors common UI conventions: B → KB → MB
// → GB. Used for human-facing diagnostic messages only; structured details
// always carry the raw byte count.
func humanizeBytes(n int64) string {
	const (
		kb = 1024
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
