package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func nativeEngineTreeHash(root string) (string, error) {
	paths := []string{"go.mod", "go.sum"}
	for _, scope := range []string{"internal", "cmd", "svggen"} {
		err := filepath.WalkDir(filepath.Join(root, scope), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("source fingerprint refuses symlink %s", path)
			}
			if entry.IsDir() || (!strings.HasSuffix(path, ".go") && entry.Name() != "go.mod" && entry.Name() != "go.sum") {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("source fingerprint requires regular file %s", path)
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(relative))
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("inventory %s: %w", scope, err)
		}
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		info, err := os.Lstat(absolute)
		if err != nil {
			return "", fmt.Errorf("fingerprint %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("source fingerprint requires regular file %s", path)
		}
		body, err := os.ReadFile(absolute)
		if err != nil {
			return "", fmt.Errorf("fingerprint %s: %w", path, err)
		}
		fmt.Fprintf(hash, "%s:%d:", path, len(body))
		_, _ = hash.Write(body)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func TestNativeEngineTreeHash(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"go.mod", "go.sum", "internal/a.go", "cmd/b.go", "svggen/go.mod", "svggen/c.go"} {
		write(name, name)
	}
	fingerprint := func() string {
		t.Helper()
		h, err := nativeEngineTreeHash(root)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	baseline := fingerprint()
	if fingerprint() != baseline {
		t.Fatal("unstable fingerprint")
	}
	write(".gitignore", "internal/new/\n")
	if fingerprint() != baseline {
		t.Fatal("Git metadata changes source fingerprint")
	}
	write("internal/new/repair.go", "first implementation")
	added := fingerprint()
	if added == baseline {
		t.Fatal("new ignored source omitted")
	}
	write("internal/new/repair.go", "second implementation")
	modified := fingerprint()
	if modified == added {
		t.Fatal("new source modification omitted")
	}
	if err := os.Rename(filepath.Join(root, "internal/new/repair.go"), filepath.Join(root, "internal/new/renamed.go")); err != nil {
		t.Fatal(err)
	}
	renamed := fingerprint()
	if renamed == modified {
		t.Fatal("source path omitted")
	}
	write("svggen/go.mod", "changed nested module")
	if fingerprint() == renamed {
		t.Fatal("nested module omitted")
	}
	if err := os.Remove(filepath.Join(root, "go.sum")); err != nil {
		t.Fatal(err)
	}
	if _, err := nativeEngineTreeHash(root); err == nil {
		t.Fatal("missing module checksum silently accepted")
	}
	if err := os.Symlink("go.mod", filepath.Join(root, "go.sum")); err != nil {
		t.Fatal(err)
	}
	if _, err := nativeEngineTreeHash(root); err == nil {
		t.Fatal("symlink root module silently accepted")
	}
	if err := os.Remove(filepath.Join(root, "go.sum")); err != nil {
		t.Fatal(err)
	}
	write("go.sum", "restored")
	if err := os.Symlink("renamed.go", filepath.Join(root, "internal/new/link.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := nativeEngineTreeHash(root); err == nil {
		t.Fatal("symlink source silently accepted")
	}
	if _, err := nativeEngineTreeHash(t.TempDir()); err == nil {
		t.Fatal("missing source tree silently accepted")
	}
}
