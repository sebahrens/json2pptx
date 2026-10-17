package deckinput

import "encoding/json"

// Preserve an explicit zero gap without changing the historical zero value
// of an omitted gap in Go callers or previously serialized envelopes.
func (c *ComposeInput) UnmarshalJSON(raw []byte) error {
	type plain ComposeInput
	var decoded plain
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	_, decoded.GapSet = fields["gap"]
	*c = ComposeInput(decoded)
	return nil
}

func (c ComposeInput) MarshalJSON() ([]byte, error) {
	type plain ComposeInput
	copy := plain(c)
	var gap *float64
	if c.GapSet || c.Gap != 0 {
		gap = &c.Gap
	}
	return json.Marshal(struct {
		*plain
		Gap *float64 `json:"gap,omitempty"`
	}{plain: &copy, Gap: gap})
}
