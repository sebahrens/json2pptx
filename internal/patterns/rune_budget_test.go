package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every maxLength budget in this package is written in characters and the error
// message says "chars", but the checks counted bytes: "€980.2M" is 7 characters
// and 9 bytes, so it was rejected against kpi-2up's 8-character big-number
// budget as "9 chars" — an arithmetic contradiction the author could only
// escape by trial and error, and the reason a whole deck of European figures
// silently degraded to bullet lists (go-slide-creator-5ok4).
func TestMaxLengthBudgetsCountRunes(t *testing.T) {
	reg := Default()

	cases := []struct {
		name    string
		pattern string
		values  string
		wantErr bool
	}{
		{
			name:    "euro amount inside the 8-character big budget",
			pattern: "kpi-2up",
			values:  `[{"big":"€980.2M","small":"Umsatz"},{"big":"117%","small":"NRR"}]`,
		},
		{
			name:    "German caption inside the 40-character small budget",
			pattern: "kpi-2up",
			values:  `[{"big":"117%","small":"Geschäftsbereich Süddeutschland"},{"big":"6.2%","small":"Churn"}]`,
		},
		{
			name:    "an en-dash and umlauts inside an agenda item budget",
			pattern: "agenda",
			values:  `{"items":["Märkte – Wachstum und Größenordnung","Strategie","Umsetzung"]}`,
		},
		{
			name:    "a genuinely over-long value is still rejected",
			pattern: "kpi-2up",
			values:  `[{"big":"EUR 1,186.42 million","small":"ARR"},{"big":"117%","small":"NRR"}]`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pat, ok := reg.Get(tc.pattern)
			if !ok {
				t.Fatalf("pattern %s not registered", tc.pattern)
			}
			values := pat.NewValues()
			if err := json.Unmarshal([]byte(tc.values), values); err != nil {
				t.Fatalf("unmarshal values: %v", err)
			}
			err := pat.Validate(values, nil, nil)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected a max_length error, got none")
				}
				// The count in the message must be the character count, which is
				// what the budget means.
				if !strings.Contains(err.Error(), "(20 chars)") {
					t.Errorf("message should report 20 characters, got %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("content inside the character budget was rejected: %v", err)
			}
		})
	}
}

// runeLen is the whole rule; pin it directly so a future refactor cannot
// quietly reintroduce byte counting.
func TestRuneLenCountsCharacters(t *testing.T) {
	cases := map[string]int{
		"€186.4M":          7,
		"Geschäftsbereich": 16,
		"plain":            5,
		"":                 0,
	}
	for in, want := range cases {
		if got := runeLen(in); got != want {
			t.Errorf("runeLen(%q) = %d, want %d (len() = %d)", in, got, want, len(in))
		}
	}
}
