package pixelhash

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func testImage(w, h int, seed uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x) + seed, G: uint8(y), B: seed, A: 255}) //nolint:gosec // test pattern
		}
	}
	return img
}

func encode(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withChunks inserts ancillary chunks right after IHDR, the way ImageMagick
// writes tEXt (date:create, xmp) and tIME metadata.
func withChunks(t *testing.T, data []byte, chunks ...[2]string) []byte {
	t.Helper()
	const ihdrEnd = 8 + 4 + 4 + 13 + 4 // signature + IHDR length/type/data/crc
	if len(data) < ihdrEnd || string(data[12:16]) != "IHDR" {
		t.Fatal("not a PNG with a leading IHDR")
	}
	var out bytes.Buffer
	out.Write(data[:ihdrEnd])
	for _, c := range chunks {
		var hdr [4]byte
		binary.BigEndian.PutUint32(hdr[:], uint32(len(c[1]))) //nolint:gosec // test data
		out.Write(hdr[:])
		body := append([]byte(c[0]), c[1]...)
		out.Write(body)
		binary.BigEndian.PutUint32(hdr[:], crc32.ChecksumIEEE(body))
		out.Write(hdr[:])
	}
	out.Write(data[ihdrEnd:])
	return out.Bytes()
}

func tIME(year uint16, sec byte) string {
	var b [7]byte
	binary.BigEndian.PutUint16(b[0:2], year)
	b[2], b[3], b[4], b[5], b[6] = 10, 2, 11, 44, sec
	return string(b[:])
}

// TestSum_IgnoresMetadataChunks is the go-slide-creator-sr3xk regression: two
// renders of the same slide differ only in timestamp metadata and must share
// one identity.
func TestSum_IgnoresMetadataChunks(t *testing.T) {
	base := encode(t, testImage(40, 20, 7))
	a := withChunks(t, base,
		[2]string{"tEXt", "date:create\x002026-10-02T11:44:40+00:00"},
		[2]string{"tEXt", "xmp:CreateDate\x002026-10-02T13:44:40+02:00"},
		[2]string{"tIME", tIME(2026, 40)})
	b := withChunks(t, base,
		[2]string{"tEXt", "date:create\x002026-10-02T11:49:03+00:00"},
		[2]string{"tEXt", "xmp:CreateDate\x002026-10-02T13:49:03+02:00"},
		[2]string{"tIME", tIME(2026, 3)})
	if bytes.Equal(a, b) {
		t.Fatal("fixture: metadata variants must differ byte-wise")
	}
	if sha256.Sum256(a) == sha256.Sum256(b) {
		t.Fatal("fixture: file hashes must differ")
	}
	if Sum(a) != Sum(b) || Sum(a) != Sum(base) {
		t.Fatalf("same pixels, different metadata: got %s / %s / %s", Sum(a), Sum(b), Sum(base))
	}
}

// TestSum_IgnoresEncoding: a palette PNG and a truecolor PNG of the same pixels
// are the same image.
func TestSum_IgnoresEncoding(t *testing.T) {
	pal := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{200, 10, 30, 255}, color.RGBA{255, 255, 255, 255}}
	p := image.NewPaletted(image.Rect(0, 0, 9, 5), pal)
	rgba := image.NewRGBA(p.Rect)
	for y := 0; y < 5; y++ {
		for x := 0; x < 9; x++ {
			i := uint8((x + y) % 3) //nolint:gosec // test pattern
			p.SetColorIndex(x, y, i)
			rgba.Set(x, y, p.At(x, y))
		}
	}
	if Sum(encode(t, p)) != Sum(encode(t, rgba)) {
		t.Fatal("palette and truecolor encodings of the same pixels must hash equal")
	}
}

func TestSum_DistinguishesPixelsAndDimensions(t *testing.T) {
	a := Sum(encode(t, testImage(40, 20, 7)))
	if b := Sum(encode(t, testImage(40, 20, 8))); a == b {
		t.Fatal("different pixels must hash differently")
	}
	one := testImage(40, 20, 7)
	one.Set(39, 19, color.RGBA{1, 2, 3, 255})
	if b := Sum(encode(t, one)); a == b {
		t.Fatal("a single changed pixel must change the hash")
	}
	// Same pixel buffer, different shape.
	if Sum(encode(t, image.NewRGBA(image.Rect(0, 0, 4, 2)))) == Sum(encode(t, image.NewRGBA(image.Rect(0, 0, 2, 4)))) {
		t.Fatal("dimensions must be part of the identity")
	}
}

func TestSum_NonPNGFallsBackToByteHash(t *testing.T) {
	data := []byte("pixels-0")
	sum := sha256.Sum256(data)
	if got := Sum(data); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("non-PNG input: got %s, want plain sha256", got)
	}
}

func TestSumImage_SubImageMatchesCopy(t *testing.T) {
	full := testImage(30, 30, 3)
	sub := full.SubImage(image.Rect(5, 5, 15, 12))
	cp := image.NewRGBA(image.Rect(0, 0, 10, 7))
	for y := 0; y < 7; y++ {
		for x := 0; x < 10; x++ {
			cp.Set(x, y, full.At(x+5, y+5))
		}
	}
	if SumImage(sub) != SumImage(cp) {
		t.Fatal("a sub-image and its copy are the same pixels")
	}
}
