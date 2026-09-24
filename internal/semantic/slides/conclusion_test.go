package slides

import "testing"

func TestFoldConclusion(t *testing.T) {
	tests := []struct{ name, primary, takeaway, want string }{
		{"primary only", "Fund the pod.", "", "Fund the pod."},
		{"takeaway only", "", "Fund the pod.", "Fund the pod."},
		{"same meaning with punctuation", "Fund the pod in Q3.", "FUND THE POD IN Q3!", "Fund the pod in Q3."},
		{"near duplicate", "Fund the retention pod in Q3.", "Fund the retention pod in Q3, now.", "Fund the retention pod in Q3."},
		{"distinct", "Fund the pod in Q3.", "Begin hiring next month.", "Fund the pod in Q3. — Begin hiring next month."},
		{"short phrase not suppressed", "Fund the pod", "Fund the pod later", "Fund the pod — Fund the pod later"},
		{"negation is distinct", "Fund the retention pod in Q3.", "Do not fund the retention pod in Q3.", "Fund the retention pod in Q3. — Do not fund the retention pod in Q3."},
		{"number change is distinct", "Increase enterprise revenue by 15 percent.", "Increase enterprise revenue by 150 percent.", "Increase enterprise revenue by 15 percent. — Increase enterprise revenue by 150 percent."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FoldConclusion(tt.primary, tt.takeaway); got != tt.want {
				t.Errorf("FoldConclusion(%q, %q) = %q, want %q", tt.primary, tt.takeaway, got, tt.want)
			}
		})
	}
}
