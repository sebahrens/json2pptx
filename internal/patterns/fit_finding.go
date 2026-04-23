package patterns

// Extent represents a measured or allowed dimension in EMU.
type Extent struct {
	WidthEMU  int64 `json:"width_emu"`
	HeightEMU int64 `json:"height_emu"`
}

// FitFinding is a fit-report finding that embeds ValidationError for
// unification with the existing validation envelope. The embedding flattens
// all ValidationError fields to the top level in JSON output.
type FitFinding struct {
	ValidationError

	// Action is the recommended remediation: "refuse", "shrink_or_split",
	// "review", or "info", ranked from most to least severe.
	Action string `json:"action"`

	// Measured is the actual extent of the content (nil when not applicable).
	Measured *Extent `json:"measured,omitempty"`

	// Allowed is the available extent for the content (nil when not applicable).
	Allowed *Extent `json:"allowed,omitempty"`

	// OverflowRatio is measured/allowed as a fraction (e.g. 1.25 means 25%
	// over). Zero when extents are not available.
	OverflowRatio float64 `json:"overflow_ratio,omitempty"`
}

// actionRanks maps action strings to severity ranks. Higher rank = more severe.
var actionRanks = map[string]int{
	"info":           0,
	"review":         1,
	"shrink_or_split": 2,
	"refuse":         3,
}

// ActionRank returns the severity rank for the given action string.
// Unknown actions return -1.
func ActionRank(action string) int {
	rank, ok := actionRanks[action]
	if !ok {
		return -1
	}
	return rank
}
