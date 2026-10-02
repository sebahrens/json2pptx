// Package pixelhash computes the canonical identity of a rendered slide image:
// a hash of its decoded pixels, not of the file bytes that carry them
// (go-slide-creator-sr3xk).
//
// A PNG's bytes include metadata that has nothing to do with what the image
// shows: ImageMagick copies the PDF's XMP creation/modification timestamps into
// the file and stamps tIME / date:* chunks with the wall clock. Two renders of
// the same slide of the same PPTX therefore produced different file hashes, and
// a forced thumbnail refresh invalidated a review that had inspected exactly
// those pixels. Hashing the decoded image makes the identity depend only on the
// dimensions and RGBA values — the thing a reviewer actually looked at.
package pixelhash

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	"image/draw"
	"image/png"
)

// domain separates a pixel hash from a plain sha256 of file bytes, so the two
// identities can never collide.
const domain = "json2pptx-pixels-rgba8-v1\x00"

// Sum returns the canonical identity of an image file's contents as 64 hex
// digits.
//
// When data decodes as a PNG, the hash covers only the image dimensions and its
// pixels converted to 8-bit premultiplied RGBA: chunk order, compression level,
// palette vs truecolor encoding, and every metadata chunk (tEXt, zTXt, iTXt,
// tIME, XMP) are ignored. Anything that does not decode as a PNG falls back to
// the sha256 of its bytes, so non-image input still has a stable identity.
func Sum(data []byte) string {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	return SumImage(img)
}

// SumImage returns the canonical pixel identity of a decoded image.
func SumImage(img image.Image) string {
	b := img.Bounds()
	rgba, ok := img.(*image.RGBA)
	if !ok || rgba.Rect.Min != (image.Point{}) || rgba.Stride != 4*b.Dx() {
		rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Rect, img, b.Min, draw.Src)
	}
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	var dims [8]byte
	binary.BigEndian.PutUint32(dims[0:4], uint32(b.Dx())) //nolint:gosec // image dimensions are non-negative and far below 2^32
	binary.BigEndian.PutUint32(dims[4:8], uint32(b.Dy())) //nolint:gosec // image dimensions are non-negative and far below 2^32
	_, _ = h.Write(dims[:])
	_, _ = h.Write(rgba.Pix[:4*b.Dx()*b.Dy()])
	return hex.EncodeToString(h.Sum(nil))
}
