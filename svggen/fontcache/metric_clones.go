package fontcache

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tdewolff/canvas"
)

// metricClones maps proprietary Office fonts to their open, metric-compatible
// clones (identical advance widths), which LibreOffice ships and renders with
// when the original is not installed.
var metricClones = map[string]string{
	"calibri": "Carlito",
	"cambria": "Caladea",
}

// MetricClone returns the metric-compatible clone family for name, or "".
func MetricClone(name string) string {
	return metricClones[strings.ToLower(strings.TrimSpace(name))]
}

// cloneFontDirs lists directories searched for clone font files when the
// clone is not registered as a system font: LibreOffice's bundled fonts
// (macOS app bundle, Linux packages) and the Debian/Fedora crosextra
// packages. A variable so tests can point it elsewhere.
var cloneFontDirs = func() []string {
	dirs := []string{
		"/Applications/LibreOffice.app/Contents/Resources/fonts/truetype",
		"/usr/lib/libreoffice/share/fonts/truetype",
		"/usr/lib64/libreoffice/share/fonts/truetype",
		"/opt/libreoffice/share/fonts/truetype",
		"/usr/share/fonts/truetype/crosextra",
		"/usr/share/fonts/crosextra",
		"/usr/share/fonts/google-carlito-fonts",
		"/usr/share/fonts/google-crosextra-carlito-fonts",
		"/usr/share/fonts/google-crosextra-caladea-fonts",
	}
	if matches, err := filepath.Glob("/opt/libreoffice*/share/fonts/truetype"); err == nil {
		dirs = append(dirs, matches...)
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Library", "Fonts"), filepath.Join(home, ".local", "share", "fonts"))
	}
	return dirs
}

// loadMetricClone loads name's metric-compatible clone into ff, first as a
// system font, then from the clone font directories. Reports the clone's
// family name.
func loadMetricClone(ff *canvas.FontFamily, name string) (string, bool) {
	clone := MetricClone(name)
	if clone == "" {
		return "", false
	}
	if err := ff.LoadSystemFont(clone, canvas.FontRegular); err == nil {
		_ = ff.LoadSystemFont(clone, canvas.FontBold) // best-effort bold
		return clone, true
	}
	for _, dir := range cloneFontDirs() {
		regular := filepath.Join(dir, clone+"-Regular.ttf")
		if _, err := os.Stat(regular); err != nil {
			continue
		}
		if err := ff.LoadFontFile(regular, canvas.FontRegular); err != nil {
			continue
		}
		_ = ff.LoadFontFile(filepath.Join(dir, clone+"-Bold.ttf"), canvas.FontBold) // best-effort bold
		return clone, true
	}
	return "", false
}
