// Package fonts embeds original native fonts and metric-compatible fallback
// fonts for headless environments.
//
// Liberation Sans (SIL Open Font License) is metric-compatible with Arial,
// ensuring accurate text measurement on systems where Arial is not installed
// (e.g., Ubuntu, Alpine, Docker containers without msttcorefonts).
package fonts

import _ "embed"

//go:embed LiberationSans-Regular.ttf
var LiberationSansRegular []byte

//go:embed LiberationSans-Bold.ttf
var LiberationSansBold []byte

// Lora is the original, unmodified native modern-template title family.
// Upstream: cyrealtype/Lora-Cyrillic at 2d53b449b60e185b39f671b44fded83e0910ad30.
// Copyright and SIL Open Font License are retained in Lora-OFL.txt and font metadata.
//
//go:embed Lora-Regular.ttf
var LoraRegular []byte

//go:embed Lora-Bold.ttf
var LoraBold []byte

// Original native modern-template body family, pinned Google Fonts snapshot
// 8b0a1d0f5983c89bc2b93f1b5fb55f9e252744b5. The Light weight and original
// Regular/Italic family metadata are retained; see Poppins-OFL.txt.
//
//go:embed Poppins-Light.ttf
var PoppinsLight []byte

//go:embed Poppins-LightItalic.ttf
var PoppinsLightItalic []byte
