package core

import "testing"

func TestPromoteFindings_Off(t *testing.T) {
	findings := []Finding{
		{Code: FindingCapacityExceeded, Severity: SeverityWarning},
		{Code: FindingInvalidNumeric, Severity: SeverityWarning},
	}

	// "off" should not modify severities.
	got := PromoteFindings(findings, "off")
	for _, f := range got {
		if f.Severity != SeverityWarning {
			t.Errorf("code %s: severity = %q, want %q", f.Code, f.Severity, SeverityWarning)
		}
	}
}

func TestPromoteFindings_Empty(t *testing.T) {
	got := PromoteFindings(nil, "strict")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestPromoteFindings_Warn(t *testing.T) {
	findings := []Finding{
		{Code: FindingCapacityExceeded, Severity: SeverityWarning},
		{Code: FindingLegendOverflowDropped, Severity: SeverityWarning},
		{Code: FindingOverflowSuppressed, Severity: SeverityWarning},
		{Code: FindingInvalidNumeric, Severity: SeverityWarning},      // not promoted under warn
		{Code: FindingAutoLogScaleApplied, Severity: SeverityInfo},     // advisory, stays
		{Code: FindingLabelTruncated, Severity: SeverityInfo},          // advisory, stays
	}

	got := PromoteFindings(findings, "warn")

	want := map[string]string{
		FindingCapacityExceeded:      SeverityShrinkOrSplit,
		FindingLegendOverflowDropped: SeverityShrinkOrSplit,
		FindingOverflowSuppressed:    SeverityShrinkOrSplit,
		FindingInvalidNumeric:        SeverityWarning,       // no warn-level rule
		FindingAutoLogScaleApplied:   SeverityInfo,          // advisory
		FindingLabelTruncated:        SeverityInfo,          // advisory
	}

	for _, f := range got {
		if exp, ok := want[f.Code]; ok && f.Severity != exp {
			t.Errorf("code %s: severity = %q, want %q", f.Code, f.Severity, exp)
		}
	}
}

func TestPromoteFindings_Strict(t *testing.T) {
	findings := []Finding{
		{Code: FindingCapacityExceeded, Severity: SeverityWarning},
		{Code: FindingInvalidNumeric, Severity: SeverityWarning},
		{Code: FindingZeroSumPie, Severity: SeverityWarning},
		{Code: FindingNegativeOnLog, Severity: SeverityWarning},
		{Code: FindingInvalidTimeFormat, Severity: SeverityWarning},
		{Code: FindingAllZeroSeries, Severity: SeverityWarning},
		{Code: FindingLegendOverflowDropped, Severity: SeverityWarning},
		{Code: FindingOverflowSuppressed, Severity: SeverityWarning},
		{Code: FindingTickThinned, Severity: SeverityInfo},             // advisory
		{Code: FindingAutoLogScaleApplied, Severity: SeverityInfo},     // advisory
		{Code: FindingLabelEllipsized, Severity: SeverityInfo},         // advisory
	}

	got := PromoteFindings(findings, "strict")

	want := map[string]string{
		FindingCapacityExceeded:      SeverityRefuse,
		FindingInvalidNumeric:        SeverityRefuse,
		FindingZeroSumPie:            SeverityRefuse,
		FindingNegativeOnLog:         SeverityRefuse,
		FindingInvalidTimeFormat:     SeverityRefuse,
		FindingAllZeroSeries:         SeverityRefuse,
		FindingLegendOverflowDropped: SeverityShrinkOrSplit,
		FindingOverflowSuppressed:    SeverityShrinkOrSplit,
		FindingTickThinned:           SeverityInfo,           // advisory
		FindingAutoLogScaleApplied:   SeverityInfo,           // advisory
		FindingLabelEllipsized:       SeverityInfo,           // advisory
	}

	for _, f := range got {
		if exp, ok := want[f.Code]; ok && f.Severity != exp {
			t.Errorf("code %s: severity = %q, want %q", f.Code, f.Severity, exp)
		}
	}
}

func TestPromoteFindings_DoesNotMutateOriginal(t *testing.T) {
	findings := []Finding{
		{Code: FindingCapacityExceeded, Severity: SeverityWarning},
	}

	_ = PromoteFindings(findings, "strict")

	if findings[0].Severity != SeverityWarning {
		t.Error("original slice was mutated")
	}
}

func TestHasRefuseFindings(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		want     bool
	}{
		{"empty", nil, false},
		{"no refuse", []Finding{{Severity: SeverityWarning}}, false},
		{"has refuse", []Finding{{Severity: SeverityRefuse}}, true},
		{"mixed", []Finding{{Severity: SeverityWarning}, {Severity: SeverityRefuse}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasRefuseFindings(tt.findings); got != tt.want {
				t.Errorf("HasRefuseFindings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPromoteFindings_UnknownCode(t *testing.T) {
	findings := []Finding{
		{Code: "chart.unknown_future_code", Severity: SeverityWarning},
	}

	got := PromoteFindings(findings, "strict")
	if got[0].Severity != SeverityWarning {
		t.Errorf("unknown code was promoted: severity = %q, want %q", got[0].Severity, SeverityWarning)
	}
}
