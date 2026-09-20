# Connector viewer fixture

`fixture.json` has six slides in a fixed order: a four-branch driver tree, a
six-row dual-organisation ladder, a process flow with a decision, a swimlane
with two handoff arrows, native Porter forces, and an explicit three-row
shape grid with a row-spanning parent. The grid exercises fan-out and elbow
routing. Each deck is generated from the same input with strict output and
unknown-key validation.

From the repository root:

```bash
for template in midnight-blue warm-coral; do
  go run ./cmd/json2pptx generate \
    --json tests/quality/evidence/connectors/fixture.json \
    --template "$template" --templates-dir templates \
    --output "tests/quality/evidence/connectors/$template" \
    --design-mode=free --strict-unknown-keys
  soffice -env:UserInstallation="file:///private/tmp/json2pptx-connectors-$template" \
    --headless --convert-to pdf \
    --outdir "tests/quality/evidence/connectors/$template" \
    "tests/quality/evidence/connectors/$template/connector-fixture.pptx"
  pdftoppm -r 90 -png \
    "tests/quality/evidence/connectors/$template/connector-fixture.pdf" \
    "tests/quality/evidence/connectors/$template/libreoffice-slide"
done
```

Both PPTX files pass the CLI's strict OOXML validator. The slide XML contains
13, 6, 4, 2, 4 and 6 `p:cxnSp` connectors respectively. The committed
LibreOffice PNGs provide the pre-PowerPoint reference: connector endpoints
meet their intended shape edges, no elbow crosses a shape, and every slide's
text remains visible. The PowerPoint comparison and edit/gluing check belong
to Bead `go-slide-creator-rzu9.2`.
