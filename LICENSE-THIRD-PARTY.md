# Third-Party Licenses

This file documents the licenses of all third-party dependencies used by
go-slide-creator. The project itself is licensed under the MIT License.

All dependencies are compatible with the MIT License.

## Direct Dependencies

| Module | Version | License | URL |
|--------|---------|---------|-----|
| github.com/fsnotify/fsnotify | v1.9.0 | BSD-3-Clause | https://github.com/fsnotify/fsnotify/blob/v1.9.0/LICENSE |
| github.com/tdewolff/canvas | v0.0.0-20260109 | MIT | https://github.com/tdewolff/canvas/blob/main/LICENSE.md |
| golang.org/x/image | v0.32.0 | BSD-3-Clause | https://cs.opensource.google/go/x/image/+/v0.32.0:LICENSE |
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
| github.com/benoitkugler/textlayout | v0.3.1 | MIT | https://github.com/benoitkugler/textlayout/blob/v0.3.1/LICENSE |
| github.com/benoitkugler/textprocessing | v0.0.3 | LGPL-2.1-or-later | https://github.com/benoitkugler/textprocessing/blob/main/LICENSE |
| github.com/go-fonts/latin-modern | v0.3.3 | BSD-3-Clause | https://github.com/go-fonts/latin-modern/blob/v0.3.3/LICENSE |
| github.com/go-text/typesetting | v0.3.0 | BSD-3-Clause | https://github.com/go-text/typesetting/blob/v0.3.0/LICENSE |
| github.com/golang/freetype | v0.0.0-20170609 | FTL (elected) | https://github.com/golang/freetype/blob/master/LICENSE |
| github.com/kr/text | v0.2.0 | MIT | https://github.com/kr/text/blob/v0.2.0/LICENSE |
| github.com/srwiley/rasterx | v0.0.0-20220730 | BSD-3-Clause | https://github.com/srwiley/rasterx/blob/master/LICENSE |
| github.com/srwiley/scanx | v0.0.0-20190309 | FTL (see note) | https://github.com/srwiley/scanx |
| github.com/tdewolff/font | v0.0.0-20250902 | MIT | https://github.com/tdewolff/font/blob/main/LICENSE.md |
| github.com/tdewolff/minify/v2 | v2.24.4 | MIT | https://github.com/tdewolff/minify/blob/v2.24.4/LICENSE |
| github.com/tdewolff/parse/v2 | v2.8.4 | MIT | https://github.com/tdewolff/parse/blob/v2.8.4/LICENSE.md |
| github.com/yuin/goldmark | v1.7.13 | MIT | https://github.com/yuin/goldmark/blob/v1.7.13/LICENSE |
| golang.org/x/net | v0.46.0 | BSD-3-Clause | https://cs.opensource.google/go/x/net/+/v0.46.0:LICENSE |
| golang.org/x/sys | v0.37.0 | BSD-3-Clause | https://cs.opensource.google/go/x/sys/+/v0.37.0:LICENSE |
| golang.org/x/text | v0.30.0 | BSD-3-Clause | https://cs.opensource.google/go/x/text/+/v0.30.0:LICENSE |
| modernc.org/knuth | v0.5.5 | BSD-3-Clause | https://gitlab.com/cznic/knuth/blob/v0.5.5/LICENSE-STAR-TEX |
| modernc.org/token | v1.1.0 | BSD-3-Clause | https://gitlab.com/cznic/token/blob/v1.1.0/LICENSE |
| star-tex.org/x/tex | v0.7.1 | BSD-3-Clause | https://git.sr.ht/~sbinet/star-tex/tree/v0.7.1/LICENSE |

## Bundled Fonts

| Font | Version | License | URL |
|------|---------|---------|-----|
| Liberation Sans | 2.1.5 | SIL Open Font License 1.1 | https://github.com/liberationfonts/liberation-fonts |

Liberation Sans is bundled at `fonts/LiberationSans-Regular.ttf` and embedded
into the binary via `go:embed`. It is metric-compatible with Arial, ensuring
accurate text measurement on headless/Docker environments where Arial is not
installed. The full license text is at `fonts/LICENSE-LiberationSans`.

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
MIT-licensed code. For open-source distribution this is straightforward. For
closed-source binary distribution, LGPL Section 6 requires providing means for
users to relink with a modified version of the library. Since go-slide-creator
is open source, this is satisfied by source availability.

### github.com/BurntSushi/xgbutil (WTFPL-2.0)

The WTFPL is maximally permissive with no restrictions. Some organizations
consider it unusual but it imposes no compliance obligations.

## FreeType Attribution

Portions of this software are copyright The FreeType Project
(www.freetype.org). All rights reserved.
