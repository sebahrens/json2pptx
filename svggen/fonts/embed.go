// Package fonts embeds metric-compatible fallback fonts for headless environments.
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
