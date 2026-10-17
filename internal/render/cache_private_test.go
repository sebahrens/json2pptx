package render

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pixelhash"
)

func auditPNG(t *testing.T, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: shade, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestGraphicsAuditCacheRejectsUntrustedEntries(t *testing.T) {
	root := withTempCache(t)
	outside := t.TempDir()
	key := strings.Repeat("a", 64) + "-d72"
	if err := os.Symlink(outside, filepath.Join(root, key)); err != nil {
		t.Fatal(err)
	}
	if got := getCachedPNGs(key); got != nil {
		t.Fatal("trusted symlink entry")
	}
	if err := os.Remove(filepath.Join(root, key)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, key), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, key, cacheCompleteMarker), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outside, "victim.png")
	if err := os.WriteFile(victim, auditPNG(t, 32), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(root, key, "slide-0.png")); err != nil {
		t.Fatal(err)
	}
	if got := getCachedPNGs(key); got != nil {
		t.Fatal("trusted symlink image")
	}
	if got := CachedSlideHashes(strings.Repeat("a", 64)); len(got) != 0 {
		t.Fatal("forged image accepted as visual-review evidence")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(root, 0777); err != nil { //nolint:gosec // Deliberately hostile shared cache permissions.
			t.Fatal(err)
		}
		if _, err := WriteArtifact(auditPNG(t, 42)); err == nil {
			t.Fatal("wrote into shared cache")
		}
		if err := InvalidateCache(); err == nil {
			t.Fatal("invalidated untrusted shared directory")
		}
		if err := os.Chmod(root, 0700); err != nil { //nolint:gosec // Restore a private directory for cleanup.
			t.Fatal(err)
		}
	}
}

func TestGraphicsAuditArtifactAtomicAndPrivate(t *testing.T) {
	root := withTempCache(t)
	data := auditPNG(t, 23)
	if err := os.Mkdir(filepath.Join(root, "artifacts"), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "artifacts", "slide-"+pixelhash.Sum(data)+".png")
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("KEEP"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, target); err != nil {
		t.Fatal(err)
	}
	path, err := WriteArtifact(data)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(victim); string(got) != "KEEP" {
		t.Fatal("artifact writer followed symlink")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatal("artifact remains a symlink")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("artifact is not private: %v", info.Mode())
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, len(data)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteArtifact(data); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, data) {
		t.Fatal("same-sized corrupt artifact was trusted")
	}
}

func TestGraphicsAuditConcurrentCacheLifecycle(t *testing.T) {
	root := withTempCache(t)
	data := auditPNG(t, 51)
	source := filepath.Join(t.TempDir(), "source.png")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("b", 64) + "-d72"
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				storeCachePNGs(key, []string{source})
				paths := getCachedPNGs(key)
				if len(paths) != 1 {
					t.Error("concurrent writer exposed incomplete entry")
					return
				}
				if got, err := os.ReadFile(paths[0]); err != nil || !bytes.Equal(got, data) {
					t.Errorf("cache read changed: %v", err)
					return
				}
				if _, err := WriteArtifact(data); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Error("temporary cache entry leaked")
		}
	}
	if err := InvalidateCache(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("cache invalidation left root")
	}
	storeCachePNGs(key, []string{source})
	if len(getCachedPNGs(key)) != 1 {
		t.Fatal("cache could not be recreated after invalidation")
	}
}
