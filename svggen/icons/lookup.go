package icons

import (
	"fmt"
	"io/fs"
	"strings"
)

// DefaultSet is the icon set used when no prefix is specified.
const DefaultSet = "outline"

// Lookup retrieves SVG bytes for a bundled icon by name.
//
// Name formats:
//   - "chart-pie"         → outline/chart-pie.svg
//   - "outline:chart-pie" → outline/chart-pie.svg
//   - "filled:chart-pie"  → filled/chart-pie.svg
//
// Returns the raw SVG content or an error if the icon is not found.
func Lookup(name string) ([]byte, error) {
	set, base := parseName(name)
	path := set + "/" + base + ".svg"
	data, err := fs.ReadFile(FS, path)
	if err != nil {
		return nil, fmt.Errorf("icon %q not found (tried %s): %w", name, path, err)
	}
	return data, nil
}

// Exists reports whether a bundled icon with the given name exists.
func Exists(name string) bool {
	set, base := parseName(name)
	path := set + "/" + base + ".svg"
	_, err := fs.Stat(FS, path)
	return err == nil
}

// List returns all icon names in the given set ("filled" or "outline").
// Names are returned without the set prefix or .svg extension.
func List(set string) ([]string, error) {
	entries, err := fs.ReadDir(FS, set)
	if err != nil {
		return nil, fmt.Errorf("listing icon set %q: %w", set, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".svg") {
			names = append(names, strings.TrimSuffix(n, ".svg"))
		}
	}
	return names, nil
}

// parseName splits "set:name" into (set, name).
// If no prefix is present, DefaultSet is used.
func parseName(name string) (set, base string) {
	name = strings.TrimSpace(name)
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[:i], name[i+1:]
	}
	return DefaultSet, name
}
