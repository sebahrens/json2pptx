package patterns

import (
	"encoding/json"
	"fmt"
)

// Peer structural fills (go-slide-creator-at7ij).
//
// dual-org-ladder coloured its two org headers accent1 and accent2;
// process-grid-2row coloured its two rows accent1 and accent3. Templates pick
// accent2 and accent3 for contrast with accent1, not kinship with it, so on
// forest-green the pair came out green beside bright orange and green beside
// bright blue; on midnight-blue, navy beside red.
//
// Neither pair is a contrast failure — both are perfectly distinguishable. The
// problem is that the two halves are PEERS (two organisations in one
// engagement, two parallel tracks of one process) and two unrelated brand hues
// say the opposite. One accent at two luminances says "two of the same kind".

const (
	// peerToneLumMod darkens the accent for the second of two peer fills. A
	// lower luminance of the same hue keeps the pair harmonious while staying
	// clearly distinguishable, and it stays dark enough for the light text both
	// patterns put on these fills.
	peerToneLumMod = 65000
)

// peerTone is the second of two peer structural fills: the same accent, darker.
func peerTone(accent string) fillTone {
	return fillTone{Color: accent, LumMod: peerToneLumMod}
}

// peerFillJSON renders peerTone as a shape fill.
func peerFillJSON(accent string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"color": %q, "lumMod": %d}`, accent, peerToneLumMod))
}

// accentFillJSON renders a plain scheme-colour fill, the first of the pair.
func accentFillJSON(accent string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`%q`, accent))
}
