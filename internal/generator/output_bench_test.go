package generator

import (
	"strings"
	"testing"
)

// BenchmarkSpliceBytes benchmarks the new spliceBytes helper vs the old
// string-based approach of insertTableFrames.
func BenchmarkSpliceBytes(b *testing.B) {
	// Simulate a ~50KB slide XML
	slideData := make([]byte, 0, 50000)
	slideData = append(slideData, []byte("<p:sld><p:cSld><p:spTree>")...)
	slideData = append(slideData, make([]byte, 49900)...)
	slideData = append(slideData, []byte("</p:spTree></p:cSld></p:sld>")...)

	insertion := "<p:graphicFrame>table content here</p:graphicFrame>"

	b.Run("bytes_splice", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			pos := findLastClosingSpTree(slideData)
			spliceBytes(slideData, pos, insertion)
		}
	})

	b.Run("string_roundtrip", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			s := string(slideData)
			pos := strings.LastIndex(s, "</p:spTree>")
			_ = []byte(s[:pos] + "\n" + insertion + "\n" + s[pos:])
		}
	})
}
