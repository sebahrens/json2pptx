package patterns

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestKPISubLengthMatchesSchemaAcrossVariants(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
	}{
		{"kpi-inline", 2}, {"kpi-2up", 2}, {"kpi-3up", 3},
		{"kpi-4up", 4}, {"kpi-5up", 5}, {"kpi-6up", 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pat, ok := Default().Get(tc.name)
			if !ok {
				t.Fatal("pattern missing")
			}
			values := KPINupValues{}
			for i := 0; i < tc.count; i++ {
				values = append(values, KPICell{Big: "42%", Small: "Revenue"})
			}
			values[0].Sub = strings.Repeat("δ", kpiSubMaxChars)
			if err := pat.Validate(&values, nil, nil); err != nil {
				t.Fatalf("at 12 runes: %v", err)
			}
			values[0].Sub += "δ"
			var validation *ValidationError
			if err := pat.Validate(&values, nil, nil); !errors.As(err, &validation) || validation.Code != ErrCodeMaxLength || validation.Path != "values[0].sub" {
				t.Fatalf("13-rune sub: %v", err)
			}
		})
	}
}

func TestKPISubAliasAlsoRespectsLength(t *testing.T) {
	for _, name := range []string{"kpi-inline", "kpi-3up"} {
		t.Run(name, func(t *testing.T) {
			pat, _ := Default().Get(name)
			count := 2
			if name == "kpi-3up" {
				count = 3
			}
			rows := make([]map[string]string, count)
			for i := range rows {
				rows[i] = map[string]string{"big": "42%", "small": "Revenue"}
			}
			rows[0]["delta"] = strings.Repeat("+", 13)
			encoded, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			values := pat.NewValues()
			if err := json.Unmarshal(encoded, values); err != nil {
				t.Fatal(err)
			}
			if err := pat.Validate(values, nil, nil); err == nil || !strings.Contains(err.Error(), "values[0].sub exceeds maxLength 12") {
				t.Fatalf("13-character delta alias escaped validation: %v", err)
			}
		})
	}
}
