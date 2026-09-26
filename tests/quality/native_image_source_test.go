package quality

import (
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestNativeImageSourceDefaultAndSupplemental(t *testing.T) {
	t.Setenv("NATIVE_LAYOUT_IMAGE_SOURCE", "")
	want := filepath.Join(testutil.RepoRoot(), "tests", "quality", "evidence", "connectors", "midnight-blue", "powerpoint-slide-4.png")
	if got := nativeReferenceImage(); got != want {
		t.Fatalf("default adversarial screenshot source changed: %q", got)
	}
	photo := filepath.Join(t.TempDir(), "photo.png")
	t.Setenv("NATIVE_LAYOUT_IMAGE_SOURCE", photo)
	if got := nativeReferenceImage(); got != photo {
		t.Fatalf("supplemental source not honored: %q", got)
	}
}
