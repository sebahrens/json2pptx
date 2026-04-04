package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// BenchmarkInsertIntoSpTree benchmarks the centralized spTree insertion.
func BenchmarkInsertIntoSpTree(b *testing.B) {
	// Simulate a ~50KB slide XML
	slideData := make([]byte, 0, 50000)
	slideData = append(slideData, []byte("<p:sld><p:cSld><p:spTree>")...)
	slideData = append(slideData, make([]byte, 49900)...)
	slideData = append(slideData, []byte("</p:spTree></p:cSld></p:sld>")...)

	insertion := []byte("<p:graphicFrame>table content here</p:graphicFrame>")

	b.Run("insert_at_end", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			pptx.InsertIntoSpTree(slideData, insertion, pptx.InsertAtEnd)
		}
	})
}
