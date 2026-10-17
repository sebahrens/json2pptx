package generator

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// writtenRelationshipsForTest reads the actual ZIP entry emitted by the live
// writer so relationship assertions cover the serialization boundary too.
func writtenRelationshipsForTest(t *testing.T, ctx *singlePassContext, slideNum int, layout string) pptx.RelationshipsXML {
	t.Helper()
	var output bytes.Buffer
	ctx.outputWriter = zip.NewWriter(&output)
	if err := ctx.writeNewSlideRelationships(slideNum, layout); err != nil {
		t.Fatal(err)
	}
	if err := ctx.outputWriter.Close(); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	file, err := archive.Open(SlideRelsPath(slideNum))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(data, &rels); err != nil {
		t.Fatal(err)
	}
	return rels
}
