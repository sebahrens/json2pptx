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
LibreOffice PNGs and native PowerPoint PNGs provide paired references for both
templates. PowerPoint images were exported on PowerPoint for Mac 16.113.1 by
copying each slide and saving the native PNG clipboard representation.

The paired renders agree on connector position, direction, and routing. The
four Five Forces arrows on slide 5 initially exposed `go-slide-creator-7ec2s`:
PowerPoint did not render connectors whose attached endpoints were accompanied
by a 1x1 EMU placeholder transform. The rebuilt fixtures store resolved bounds
and render all four arrows in both applications.

PowerPoint's object model reports both endpoints attached for the shape-grid
connectors on slides 1, 2, 3 and 6 (13/13, 6/6, 4/4 and 6/6). Slide 4's two
handoff arrows are coordinate overlays and intentionally have no shape
attachments. After temporarily ungrouping slide 5 in a disposable session,
PowerPoint reports 4/4 Porter connectors attached to their named source and
target boxes. Moving the New Entrants box by 20 points changed its connector
width from approximately 0 to 20 points while both endpoints remained
attached; the box was restored and the disposable session was closed without
saving. No file-repair prompt appeared for either template.
