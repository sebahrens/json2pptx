package core

import "testing"

func TestCheckCapacity_NoFindings(t *testing.T) {
	findings := CheckCapacity(5, 10, 50)
	if len(findings) != 0 {
		t.Errorf("expected no findings within limits, got %d", len(findings))
	}
}

func TestCheckCapacity_ExceedsSeries(t *testing.T) {
	findings := CheckCapacity(MaxSeries+1, 10, 100)
	if len(findings) == 0 {
		t.Fatal("expected capacity exceeded finding for series")
	}
	if findings[0].Code != FindingCapacityExceeded {
		t.Errorf("Code = %q, want %q", findings[0].Code, FindingCapacityExceeded)
	}
}

func TestCheckCapacity_ExceedsCategories(t *testing.T) {
	findings := CheckCapacity(5, MaxCategories+1, 100)
	if len(findings) == 0 {
		t.Fatal("expected capacity exceeded finding for categories")
	}
}

func TestCheckCapacity_ExceedsPoints(t *testing.T) {
	findings := CheckCapacity(5, 10, MaxPoints+1)
	if len(findings) == 0 {
		t.Fatal("expected capacity exceeded finding for points")
	}
}

func TestCheckCapacity_MultipleExceeded(t *testing.T) {
	findings := CheckCapacity(MaxSeries+1, MaxCategories+1, MaxPoints+1)
	if len(findings) != 3 {
		t.Errorf("expected 3 findings, got %d", len(findings))
	}
}
