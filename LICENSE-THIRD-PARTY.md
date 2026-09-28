# Third-Party Licenses

This file documents the licenses of all third-party dependencies and bundled
assets used by json2pptx. The project itself is licensed under the Apache
License 2.0 (see `LICENSE` and `NOTICE`).

All dependencies are compatible with the Apache License 2.0. Versions below
are those pinned in `go.mod` and `svggen/go.mod`; regenerate these tables from
`go list -m all` in both modules (or `go-licenses report ./...`) whenever those
files change.

## Direct Dependencies

| Module | Version | License | URL |
|--------|---------|---------|-----|
| github.com/google/uuid | v1.6.0 | BSD-3-Clause | https://github.com/google/uuid/blob/v1.6.0/LICENSE |
| github.com/mark3labs/mcp-go | v0.45.0 | MIT | https://github.com/mark3labs/mcp-go/blob/v0.45.0/LICENSE |
| github.com/tdewolff/canvas | v0.0.0-20260109 | MIT | https://github.com/tdewolff/canvas/blob/main/LICENSE.md |
| golang.org/x/image | v0.45.0 | BSD-3-Clause | https://cs.opensource.google/go/x/image/+/v0.45.0:LICENSE |
| golang.org/x/sync (svggen module) | v0.22.0 | BSD-3-Clause | https://cs.opensource.google/go/x/sync/+/v0.22.0:LICENSE |
| gopkg.in/yaml.v3 | v3.0.1 | MIT | https://github.com/go-yaml/yaml/blob/v3.0.1/LICENSE |

## Indirect Dependencies

| Module | Version | License | URL |
|--------|---------|---------|-----|
| codeberg.org/go-pdf/fpdf | v0.11.1 | MIT | https://codeberg.org/go-pdf/fpdf/src/tag/v0.11.1/LICENSE |
| github.com/BurntSushi/freetype-go | v0.0.0-20160129 | FTL (elected) | https://github.com/BurntSushi/freetype-go/blob/master/LICENSE |
| github.com/BurntSushi/graphics-go | v0.0.0-20160129 | BSD-3-Clause | https://github.com/BurntSushi/graphics-go/blob/master/LICENSE |
| github.com/BurntSushi/xgb | v0.0.0-20210121 | BSD-3-Clause | https://github.com/BurntSushi/xgb/blob/master/LICENSE |
| github.com/BurntSushi/xgbutil | v0.0.0-20190907 | WTFPL-2.0 | https://github.com/BurntSushi/xgbutil/blob/master/COPYING |
| github.com/ByteArena/poly2tri-go | v0.0.0-20170716 | BSD-3-Clause | https://github.com/ByteArena/poly2tri-go/blob/master/LICENSE |
| github.com/andybalholm/brotli | v1.2.0 | MIT | https://github.com/andybalholm/brotli/blob/v1.2.0/LICENSE |
| github.com/bahlo/generic-list-go | v0.2.0 | BSD-3-Clause | https://github.com/bahlo/generic-list-go/blob/v0.2.0/LICENSE |
| github.com/benoitkugler/textlayout | v0.3.1 | MIT | https://github.com/benoitkugler/textlayout/blob/v0.3.1/LICENSE |
| github.com/benoitkugler/textprocessing | v0.0.3 | LGPL-2.1-or-later | https://github.com/benoitkugler/textprocessing/blob/main/LICENSE |
| github.com/buger/jsonparser | v1.1.2 | MIT | https://github.com/buger/jsonparser/blob/v1.1.2/LICENSE |
| github.com/go-fonts/latin-modern | v0.3.3 | BSD-3-Clause | https://github.com/go-fonts/latin-modern/blob/v0.3.3/LICENSE |
| github.com/go-text/typesetting | v0.3.0 | BSD-3-Clause | https://github.com/go-text/typesetting/blob/v0.3.0/LICENSE |
| github.com/golang/freetype | v0.0.0-20170609 | FTL (elected) | https://github.com/golang/freetype/blob/master/LICENSE |
| github.com/invopop/jsonschema | v0.13.0 | MIT | https://github.com/invopop/jsonschema/blob/v0.13.0/COPYING |
| github.com/mailru/easyjson | v0.7.7 | MIT | https://github.com/mailru/easyjson/blob/v0.7.7/LICENSE |
| github.com/spf13/cast | v1.7.1 | MIT | https://github.com/spf13/cast/blob/v1.7.1/LICENSE |
| github.com/srwiley/rasterx | v0.0.0-20220730 | BSD-3-Clause | https://github.com/srwiley/rasterx/blob/master/LICENSE |
| github.com/srwiley/scanx | v0.0.0-20190309 | FTL (see note) | https://github.com/srwiley/scanx |
| github.com/tdewolff/font | v0.0.0-20250902 | MIT | https://github.com/tdewolff/font/blob/main/LICENSE.md |
| github.com/tdewolff/minify/v2 | v2.24.4 | MIT | https://github.com/tdewolff/minify/blob/v2.24.4/LICENSE |
| github.com/tdewolff/parse/v2 | v2.8.4 | MIT | https://github.com/tdewolff/parse/blob/v2.8.4/LICENSE.md |
| github.com/wk8/go-ordered-map/v2 | v2.1.8 | Apache-2.0 | https://github.com/wk8/go-ordered-map/blob/v2.1.8/LICENSE |
| github.com/yosida95/uritemplate/v3 | v3.0.2 | BSD-3-Clause | https://github.com/yosida95/uritemplate/blob/v3.0.2/LICENSE |
| github.com/yuin/goldmark | v1.7.17 | MIT | https://github.com/yuin/goldmark/blob/v1.7.17/LICENSE |
| golang.org/x/net | v0.56.0 | BSD-3-Clause | https://cs.opensource.google/go/x/net/+/v0.56.0:LICENSE |
| golang.org/x/sys | v0.47.0 | BSD-3-Clause | https://cs.opensource.google/go/x/sys/+/v0.47.0:LICENSE |
| golang.org/x/text | v0.41.0 | BSD-3-Clause | https://cs.opensource.google/go/x/text/+/v0.41.0:LICENSE |
| modernc.org/knuth | v0.5.5 | BSD-3-Clause | https://gitlab.com/cznic/knuth/blob/v0.5.5/LICENSE-STAR-TEX |
| modernc.org/token | v1.1.0 | BSD-3-Clause | https://gitlab.com/cznic/token/blob/v1.1.0/LICENSE |
| star-tex.org/x/tex | v0.7.1 | BSD-3-Clause | https://git.sr.ht/~sbinet/star-tex/tree/v0.7.1/LICENSE |

## Bundled Fonts

All bundled fonts live in `svggen/fonts/` and are embedded into the binary via
`go:embed` (`svggen/fonts/embed.go`). Each is redistributed unmodified under the
SIL Open Font License 1.1, whose full text ships next to the font files.

| Font | Files | Version | License | License text | Upstream |
|------|-------|---------|---------|--------------|----------|
| Liberation Sans | `LiberationSans-Regular.ttf`, `LiberationSans-Bold.ttf` | 2.1.5 | SIL Open Font License 1.1 | `svggen/fonts/LiberationSans-OFL.txt` | https://github.com/liberationfonts/liberation-fonts |
| Lora | `Lora-Regular.ttf`, `Lora-Bold.ttf` | upstream commit 2d53b449 | SIL Open Font License 1.1 | `svggen/fonts/Lora-OFL.txt` | https://github.com/cyrealtype/Lora-Cyrillic |
| Poppins | `Poppins-Light.ttf`, `Poppins-LightItalic.ttf` | Google Fonts snapshot 8b0a1d0f | SIL Open Font License 1.1 | `svggen/fonts/Poppins-OFL.txt` | https://github.com/itfoundry/Poppins |

Liberation Sans is metric-compatible with Arial, ensuring accurate text
measurement on headless/Docker environments where Arial is not installed.
Lora and Poppins are the native title and body families of the
`modern-template` template, embedded so its text is measured with the real
font metrics.

## License Notes

### FreeType License (FTL) Elections

**github.com/BurntSushi/freetype-go** and **github.com/golang/freetype** are
dual-licensed under your choice of the FreeType License (FTL) or GPL-2.0+. We
elect the **FreeType License**, which is a permissive BSD-like license
requiring attribution. The FTL requires:

- Acknowledgment in documentation that FreeType code is used
- Binary redistribution includes a disclaimer noting FreeType usage

### github.com/srwiley/scanx

The repository has no root LICENSE file. The primary source file (`scan.go`)
contains a FreeType-Go copyright header granting use under FTL or GPL-2.0+ (we
elect FTL). The `span.go` file has no explicit license header. This is a
formal gap but low practical risk as an indirect dependency via tdewolff/canvas.

### github.com/benoitkugler/textprocessing (LGPL-2.1-or-later)

The `fribidi` sub-package is LGPL-2.1-or-later. LGPL permits linking from
Apache-2.0-licensed code. For open-source distribution this is straightforward.
For closed-source binary distribution, LGPL Section 6 requires providing means
for users to relink with a modified version of the library. Since json2pptx is
open source, this is satisfied by source availability.

### github.com/BurntSushi/xgbutil (WTFPL-2.0)

The WTFPL is maximally permissive with no restrictions. Some organizations
consider it unusual but it imposes no compliance obligations.

## FreeType Attribution

Portions of this software are copyright The FreeType Project
(www.freetype.org). All rights reserved.
