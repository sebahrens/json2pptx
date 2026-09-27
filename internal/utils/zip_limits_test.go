package utils

import (
	"archive/zip"
	"bytes"
	"testing"
)

// TestCheckZipLimits_RejectsBomb verifies a highly compressible large entry is
// refused at open instead of being decompressed (go-slide-creator-csclk.85).
func TestCheckZipLimits_RejectsBomb(t *testing.T) {
	build := func(size int) *zip.Reader {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		fw, err := w.Create("ppt/slides/slide1.xml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(make([]byte, size)); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatal(err)
		}
		return zr
	}
	if err := CheckZipLimits(build(11 << 20).File); err == nil {
		t.Fatal("expected bomb-ratio entry to be rejected")
	}
	if err := CheckZipLimits(build(64 << 10).File); err != nil {
		t.Fatalf("small entry rejected: %v", err)
	}
}
