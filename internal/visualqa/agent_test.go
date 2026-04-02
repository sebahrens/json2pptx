package visualqa

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseFindings(t *testing.T) {
	info := SlideInfo{Index: 0, Type: "content", Title: "Test Slide"}

	tests := []struct {
		name    string
		text    string
		want    int
		wantErr bool
	}{
		{
			name: "empty array",
			text: `[]`,
			want: 0,
		},
		{
			name: "single finding",
			text: `[{"severity":"P1","category":"text_overflow","description":"Title extends beyond boundary","location":"top-right"}]`,
			want: 1,
		},
		{
			name: "multiple findings",
			text: `[
				{"severity":"P0","category":"contrast","description":"Unreadable text","location":"center"},
				{"severity":"P2","category":"spacing","description":"Tight margins","location":"bottom"}
			]`,
			want: 2,
		},
		{
			name: "with code fences",
			text: "```json\n[{\"severity\":\"P1\",\"category\":\"alignment\",\"description\":\"Off center\",\"location\":\"title\"}]\n```",
			want: 1,
		},
		{
			name: "invalid json",
			text: `not json at all`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := parseFindings(tt.text, info)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(findings) != tt.want {
				t.Errorf("got %d findings, want %d", len(findings), tt.want)
			}
			for _, f := range findings {
				if f.SlideIndex != info.Index {
					t.Errorf("finding slide index = %d, want %d", f.SlideIndex, info.Index)
				}
				if f.SlideType != info.Type {
					t.Errorf("finding slide type = %q, want %q", f.SlideType, info.Type)
				}
			}
		})
	}
}

func TestPromptForSlideType(t *testing.T) {
	// Known types should return specific prompts.
	for _, st := range []string{"title", "section", "content", "chart", "table", "diagram", "image", "two-column", "comparison", "blank"} {
		p := PromptForSlideType(st)
		// Note: blank and comparison have entries in the map, so they won't match defaultPrompt.
		// We only verify all types return a non-empty prompt.
		if p == "" {
			t.Errorf("PromptForSlideType(%q) returned empty string", st)
		}
	}

	// Unknown type should return default prompt.
	p := PromptForSlideType("unknown_type_xyz")
	if p != defaultPrompt {
		t.Error("unknown slide type should return defaultPrompt")
	}
}

func TestReportSummarize(t *testing.T) {
	r := &Report{
		Results: []SlideResult{
			{
				Findings: []Finding{
					{Severity: SeverityP0},
					{Severity: SeverityP1},
				},
			},
			{
				Findings: []Finding{
					{Severity: SeverityP2},
					{Severity: SeverityP2},
					{Severity: SeverityP3},
				},
			},
		},
	}
	r.Summarize()
	if r.TotalIssues != 5 {
		t.Errorf("total issues = %d, want 5", r.TotalIssues)
	}
	if r.TotalByP0 != 1 {
		t.Errorf("P0 = %d, want 1", r.TotalByP0)
	}
	if r.TotalByP1 != 1 {
		t.Errorf("P1 = %d, want 1", r.TotalByP1)
	}
	if r.TotalByP2 != 2 {
		t.Errorf("P2 = %d, want 2", r.TotalByP2)
	}
	if r.TotalByP3 != 1 {
		t.Errorf("P3 = %d, want 1", r.TotalByP3)
	}
}

func TestNewAgentRequiresAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := NewAgent()
	if err == nil {
		t.Fatal("expected error when API key is missing")
	}
}

func TestInspectSlideWithMockServer(t *testing.T) {
	findings := []struct {
		Severity    string `json:"severity"`
		Category    string `json:"category"`
		Description string `json:"description"`
		Location    string `json:"location"`
	}{
		{Severity: "P1", Category: "contrast", Description: "Low contrast text", Location: "center"},
	}
	findingsJSON, _ := json.Marshal(findings)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request structure.
		body, _ := io.ReadAll(r.Body)
		var req apiRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("invalid request body: %v", err)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing or wrong API key header")
		}
		if r.Header.Get("anthropic-version") != apiVersion {
			t.Errorf("missing anthropic-version header")
		}
		if len(req.Messages) != 1 || len(req.Messages[0].Content) != 2 {
			t.Errorf("expected 1 message with 2 content blocks (image + text)")
		}

		resp := apiResponse{
			Content: []apiContentBlock{
				{Type: "text", Text: string(findingsJSON)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	agent, err := NewAgent(WithAPIURL(srv.URL), WithParallelism(1))
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	// Create a minimal JPEG-like image (just needs to be non-empty bytes).
	fakeImg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10} // JPEG magic bytes

	result, err := agent.InspectSlide(context.Background(), fakeImg, SlideInfo{
		Index: 0,
		Type:  "content",
		Title: "Test Slide",
	})
	if err != nil {
		t.Fatalf("InspectSlide: %v", err)
	}

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].Severity != SeverityP1 {
		t.Errorf("severity = %q, want P1", result.Findings[0].Severity)
	}
	if result.Findings[0].Category != "contrast" {
		t.Errorf("category = %q, want contrast", result.Findings[0].Category)
	}
}

func TestInspectAllConcurrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Content: []apiContentBlock{
				{Type: "text", Text: "[]"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	agent, err := NewAgent(WithAPIURL(srv.URL), WithParallelism(2))
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	slides := []SlideImage{
		{Info: SlideInfo{Index: 0, Type: "title"}, Data: []byte{0xFF, 0xD8}},
		{Info: SlideInfo{Index: 1, Type: "content"}, Data: []byte{0xFF, 0xD8}},
		{Info: SlideInfo{Index: 2, Type: "chart"}, Data: []byte{0xFF, 0xD8}},
	}

	report := agent.InspectAll(context.Background(), slides)
	if report.SlideCount != 3 {
		t.Errorf("slide count = %d, want 3", report.SlideCount)
	}
	if len(report.Results) != 3 {
		t.Errorf("results count = %d, want 3", len(report.Results))
	}
	if report.TotalIssues != 0 {
		t.Errorf("total issues = %d, want 0", report.TotalIssues)
	}
}

func TestFindingString(t *testing.T) {
	f := Finding{
		SlideIndex:  2,
		SlideType:   "chart",
		Severity:    SeverityP1,
		Category:    "text_overflow",
		Description: "X-axis labels overlap",
		Location:    "bottom",
	}
	s := f.String()
	if s == "" {
		t.Fatal("Finding.String() returned empty")
	}
	// Check key parts are present.
	for _, want := range []string{"P1", "2", "chart", "text_overflow", "X-axis labels overlap", "bottom"} {
		if !contains(s, want) {
			t.Errorf("Finding.String() = %q, missing %q", s, want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
