package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/svggen"
)

// mockDiagram is a test diagram that always succeeds.
type mockDiagram struct {
	typeID string
}

func (m *mockDiagram) Type() string { return m.typeID }

func (m *mockDiagram) Render(_ *svggen.RequestEnvelope) (*svggen.SVGDocument, error) {
	return &svggen.SVGDocument{
		Content: []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect width="100" height="100"/></svg>`),
		Width:   100,
		Height:  100,
		ViewBox: "0 0 100 100",
	}, nil
}

func (m *mockDiagram) Validate(_ *svggen.RequestEnvelope) error {
	return nil
}

// newTestServer creates a server with a mock registry for testing.
func newTestServer() (*Server, *svggen.Registry) {
	registry := svggen.NewRegistry()
	registry.Register(&mockDiagram{typeID: "test_chart"})
	registry.Register(&mockDiagram{typeID: "bar_chart"})

	cfg := DefaultConfig()
	server := NewServer(cfg, registry)

	return server, registry
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Port != 3001 {
		t.Errorf("Port = %d, want 3001", cfg.Port)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 60*time.Second {
		t.Errorf("WriteTimeout = %v, want 60s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want 120s", cfg.IdleTimeout)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", cfg.ShutdownTimeout)
	}
	if cfg.MaxRequestSize != 10*1024*1024 {
		t.Errorf("MaxRequestSize = %d, want 10MB", cfg.MaxRequestSize)
	}
}

func TestNewServer(t *testing.T) {
	t.Run("with default registry", func(t *testing.T) {
		cfg := DefaultConfig()
		server := NewServer(cfg, nil)

		if server == nil {
			t.Fatal("NewServer returned nil")
		}
		if server.registry != svggen.DefaultRegistry() {
			t.Error("expected default registry when nil passed")
		}
	})

	t.Run("with custom registry", func(t *testing.T) {
		registry := svggen.NewRegistry()
		cfg := DefaultConfig()
		server := NewServer(cfg, registry)

		if server == nil {
			t.Fatal("NewServer returned nil")
		}
		if server.registry != registry {
			t.Error("expected custom registry")
		}
	})
}

func TestHandleHealth(t *testing.T) {
	server, _ := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if status, ok := resp["status"].(string); !ok || status != "healthy" {
		t.Errorf("status = %v, want 'healthy'", resp["status"])
	}
	if _, ok := resp["timestamp"]; !ok {
		t.Error("response missing timestamp")
	}
}

func TestHandleTypes(t *testing.T) {
	server, _ := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/types", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	types, ok := resp["types"].([]any)
	if !ok {
		t.Fatal("response missing types array")
	}
	if len(types) != 2 {
		t.Errorf("types count = %d, want 2", len(types))
	}

	count, ok := resp["count"].(float64)
	if !ok || count != 2 {
		t.Errorf("count = %v, want 2", resp["count"])
	}
}

func TestHandleRender(t *testing.T) {
	server, _ := newTestServer()

	tests := []struct {
		name           string
		body           string
		contentType    string
		wantStatus     int
		wantSVG        bool
		wantErrorField bool
	}{
		{
			name:        "valid JSON request",
			body:        `{"type":"test_chart","data":{"values":[1,2,3]}}`,
			contentType: "application/json",
			wantStatus:  http.StatusOK,
			wantSVG:     true,
		},
		{
			name:        "valid YAML request",
			body:        "type: test_chart\ndata:\n  values: [1, 2, 3]",
			contentType: "application/x-yaml",
			wantStatus:  http.StatusOK,
			wantSVG:     true,
		},
		{
			name:        "YAML with text/yaml content type",
			body:        "type: bar_chart\ndata:\n  values: [4, 5, 6]",
			contentType: "text/yaml",
			wantStatus:  http.StatusOK,
			wantSVG:     true,
		},
		{
			name:           "missing type field",
			body:           `{"data":{"values":[1,2,3]}}`,
			contentType:    "application/json",
			wantStatus:     http.StatusBadRequest,
			wantErrorField: true,
		},
		{
			name:           "missing data field",
			body:           `{"type":"test_chart"}`,
			contentType:    "application/json",
			wantStatus:     http.StatusBadRequest,
			wantErrorField: true,
		},
		{
			name:           "unknown diagram type",
			body:           `{"type":"unknown_type","data":{"values":[1,2,3]}}`,
			contentType:    "application/json",
			wantStatus:     http.StatusBadRequest,
			wantErrorField: true,
		},
		{
			name:           "invalid JSON",
			body:           `{invalid json`,
			contentType:    "application/json",
			wantStatus:     http.StatusBadRequest,
			wantErrorField: true,
		},
		{
			name:           "invalid YAML",
			body:           "type: [invalid",
			contentType:    "application/x-yaml",
			wantStatus:     http.StatusBadRequest,
			wantErrorField: true,
		},
		{
			name:        "empty content type defaults to JSON",
			body:        `{"type":"test_chart","data":{"values":[1,2,3]}}`,
			contentType: "",
			wantStatus:  http.StatusOK,
			wantSVG:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			if tc.wantSVG {
				var resp RenderResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.SVG == "" {
					t.Error("expected SVG in response")
				}
				if !strings.Contains(resp.SVG, "<svg") {
					t.Error("SVG does not contain <svg tag")
				}
				if resp.Width <= 0 {
					t.Error("expected positive width")
				}
				if resp.Height <= 0 {
					t.Error("expected positive height")
				}
			}

			if tc.wantErrorField {
				var errResp errorResponse
				if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
					t.Fatalf("failed to decode error response: %v", err)
				}
				if errResp.Error.Message == "" {
					t.Error("expected error message in response")
				}
			}
		})
	}
}

func TestHandleRenderBatch(t *testing.T) {
	server, _ := newTestServer()

	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
		wantTotal   int
		wantSuccess int
		wantFailed  int
	}{
		{
			name: "multiple valid requests",
			body: `{
				"requests": [
					{"type":"test_chart","data":{"values":[1,2,3]}},
					{"type":"bar_chart","data":{"values":[4,5,6]}}
				]
			}`,
			contentType: "application/json",
			wantStatus:  http.StatusOK,
			wantTotal:   2,
			wantSuccess: 2,
			wantFailed:  0,
		},
		{
			name: "mixed valid and invalid requests",
			body: `{
				"requests": [
					{"type":"test_chart","data":{"values":[1,2,3]}},
					{"type":"unknown_type","data":{"values":[4,5,6]}}
				]
			}`,
			contentType: "application/json",
			wantStatus:  http.StatusOK,
			wantTotal:   2,
			wantSuccess: 1,
			wantFailed:  1,
		},
		{
			name: "YAML batch request",
			body: `requests:
  - type: test_chart
    data:
      values: [1, 2, 3]
  - type: bar_chart
    data:
      values: [4, 5, 6]`,
			contentType: "application/x-yaml",
			wantStatus:  http.StatusOK,
			wantTotal:   2,
			wantSuccess: 2,
			wantFailed:  0,
		},
		{
			name:        "empty requests array",
			body:        `{"requests":[]}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "invalid JSON",
			body:        `{invalid`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/render/batch", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			if tc.wantStatus != http.StatusOK {
				return
			}

			var resp BatchResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Total != tc.wantTotal {
				t.Errorf("total = %d, want %d", resp.Total, tc.wantTotal)
			}
			if resp.Success != tc.wantSuccess {
				t.Errorf("success = %d, want %d", resp.Success, tc.wantSuccess)
			}
			if resp.Failed != tc.wantFailed {
				t.Errorf("failed = %d, want %d", resp.Failed, tc.wantFailed)
			}
			if len(resp.Results) != tc.wantTotal {
				t.Errorf("results length = %d, want %d", len(resp.Results), tc.wantTotal)
			}
		})
	}
}

func TestHandleRenderBatchMaxSize(t *testing.T) {
	registry := svggen.NewRegistry()
	registry.Register(&mockDiagram{typeID: "test_chart"})

	cfg := DefaultConfig()
	cfg.MaxBatchSize = 3
	server := NewServer(cfg, registry)

	t.Run("within limit", func(t *testing.T) {
		body := `{"requests":[
			{"type":"test_chart","data":{"values":[1]}},
			{"type":"test_chart","data":{"values":[2]}},
			{"type":"test_chart","data":{"values":[3]}}
		]}`
		req := httptest.NewRequest(http.MethodPost, "/render/batch", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		body := `{"requests":[
			{"type":"test_chart","data":{"values":[1]}},
			{"type":"test_chart","data":{"values":[2]}},
			{"type":"test_chart","data":{"values":[3]}},
			{"type":"test_chart","data":{"values":[4]}}
		]}`
		req := httptest.NewRequest(http.MethodPost, "/render/batch", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}

		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
			t.Fatalf("failed to decode error response: %v", err)
		}
		if !strings.Contains(errResp.Error.Message, "exceeds maximum") {
			t.Errorf("error message = %q, want to contain 'exceeds maximum'", errResp.Error.Message)
		}
	})

	t.Run("default limit is 100", func(t *testing.T) {
		cfgDefault := DefaultConfig()
		srv := NewServer(cfgDefault, registry)

		if cfgDefault.MaxBatchSize != 0 {
			t.Fatalf("default MaxBatchSize = %d, want 0 (uses default of 100)", cfgDefault.MaxBatchSize)
		}

		// Verify the server uses 100 as default by checking a batch of 101
		// would be rejected. Build 101 requests.
		var reqs []string
		for i := 0; i < 101; i++ {
			reqs = append(reqs, `{"type":"test_chart","data":{"values":[1]}}`)
		}
		body := `{"requests":[` + strings.Join(reqs, ",") + `]}`
		req := httptest.NewRequest(http.MethodPost, "/render/batch", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d for 101 items with default limit", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestServerRun(t *testing.T) {
	t.Run("starts and stops gracefully", func(t *testing.T) {
		registry := svggen.NewRegistry()
		registry.Register(&mockDiagram{typeID: "test"})

		cfg := DefaultConfig()
		cfg.Port = 0 // Use a random available port
		server := NewServer(cfg, registry)

		ctx, cancel := context.WithCancel(context.Background())

		errChan := make(chan error, 1)
		go func() {
			errChan <- server.Run(ctx)
		}()

		// Give server time to start
		time.Sleep(100 * time.Millisecond)

		// Cancel context to trigger shutdown
		cancel()

		select {
		case err := <-errChan:
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("server did not shut down in time")
		}
	})
}

func TestRequestLogging(t *testing.T) {
	server, _ := newTestServer()

	// Make a request and verify logging doesn't break
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestLoggingResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		lrw := &loggingResponseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		lrw.WriteHeader(http.StatusNotFound)

		if lrw.statusCode != http.StatusNotFound {
			t.Errorf("statusCode = %d, want %d", lrw.statusCode, http.StatusNotFound)
		}
	})

	t.Run("default status code is 200", func(t *testing.T) {
		rec := httptest.NewRecorder()
		lrw := &loggingResponseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		if lrw.statusCode != http.StatusOK {
			t.Errorf("statusCode = %d, want %d", lrw.statusCode, http.StatusOK)
		}
	})
}

func TestWriteError(t *testing.T) {
	server, _ := newTestServer()

	rec := httptest.NewRecorder()
	server.writeError(rec, http.StatusBadRequest, CodeInvalidRequest, "test error message")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("success = true, want false")
	}
	if resp.Error.Message != "test error message" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "test error message")
	}
	if resp.Error.Code != CodeInvalidRequest {
		t.Errorf("error code = %q, want %q", resp.Error.Code, CodeInvalidRequest)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	server, _ := newTestServer()

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/healthz"},
		{http.MethodPost, "/types"},
		{http.MethodGet, "/render"},
		{http.MethodGet, "/render/batch"},
		{http.MethodPut, "/render"},
		{http.MethodDelete, "/render"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			// Go 1.22+ returns 405 for method not allowed
			if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 405 or 404", rec.Code)
			}
		})
	}
}

func TestMaxRequestSize(t *testing.T) {
	registry := svggen.NewRegistry()
	registry.Register(&mockDiagram{typeID: "test"})

	cfg := DefaultConfig()
	cfg.MaxRequestSize = 100 // Very small limit for testing
	server := NewServer(cfg, registry)

	// Create a request body larger than the limit
	largeBody := make([]byte, 200)
	for i := range largeBody {
		largeBody[i] = 'a'
	}

	req := httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestContentNegotiation(t *testing.T) {
	server, _ := newTestServer()

	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{
			name:        "application/json",
			contentType: "application/json",
			body:        `{"type":"test_chart","data":{"values":[1,2,3]}}`,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "application/json; charset=utf-8",
			contentType: "application/json; charset=utf-8",
			body:        `{"type":"test_chart","data":{"values":[1,2,3]}}`,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "application/x-yaml",
			contentType: "application/x-yaml",
			body:        "type: test_chart\ndata:\n  values: [1,2,3]",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "text/yaml",
			contentType: "text/yaml",
			body:        "type: test_chart\ndata:\n  values: [1,2,3]",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "no content type defaults to JSON",
			contentType: "",
			body:        `{"type":"test_chart","data":{"values":[1,2,3]}}`,
			wantStatus:  http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				body, _ := io.ReadAll(rec.Body)
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, string(body))
			}
		})
	}
}

// TestPNGOutput tests PNG output format rendering.
// These tests use the default registry with real diagram implementations.
func TestPNGOutput(t *testing.T) {
	// Use default registry which has real diagram implementations with PNG support
	cfg := DefaultConfig()
	server := NewServer(cfg, nil)

	t.Run("PNG output for bar_chart", func(t *testing.T) {
		body := `{
			"type": "bar_chart",
			"data": {
				"categories": ["A", "B", "C"],
				"series": [{"name": "S1", "values": [10, 20, 30]}]
			},
			"output": {"format": "png"}
		}`

		req := httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			body, _ := io.ReadAll(rec.Body)
			t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, string(body))
		}

		var resp RenderResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Format != "png" {
			t.Errorf("format = %q, want 'png'", resp.Format)
		}
		if resp.PNG == "" {
			t.Error("expected PNG data in response")
		}
		if resp.SVG != "" {
			t.Error("expected no SVG in PNG response")
		}
		if resp.Width <= 0 || resp.Height <= 0 {
			t.Error("expected positive dimensions")
		}
	})

	t.Run("SVG output is default", func(t *testing.T) {
		body := `{
			"type": "bar_chart",
			"data": {
				"categories": ["A", "B"],
				"series": [{"name": "S1", "values": [10, 20]}]
			}
		}`

		req := httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			body, _ := io.ReadAll(rec.Body)
			t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, string(body))
		}

		var resp RenderResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Format != "svg" {
			t.Errorf("format = %q, want 'svg'", resp.Format)
		}
		if resp.SVG == "" {
			t.Error("expected SVG in response")
		}
		if resp.PNG != "" {
			t.Error("expected no PNG in SVG response")
		}
	})

	t.Run("unsupported format returns error", func(t *testing.T) {
		body := `{
			"type": "bar_chart",
			"data": {
				"categories": ["A", "B"],
				"series": [{"name": "S1", "values": [10, 20]}]
			},
			"output": {"format": "pdf"}
		}`

		req := httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("batch with mixed formats", func(t *testing.T) {
		body := `{
			"requests": [
				{
					"type": "bar_chart",
					"data": {
						"categories": ["A", "B"],
						"series": [{"name": "S1", "values": [10, 20]}]
					},
					"output": {"format": "svg"}
				},
				{
					"type": "bar_chart",
					"data": {
						"categories": ["C", "D"],
						"series": [{"name": "S2", "values": [30, 40]}]
					},
					"output": {"format": "png"}
				}
			]
		}`

		req := httptest.NewRequest(http.MethodPost, "/render/batch", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			body, _ := io.ReadAll(rec.Body)
			t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, string(body))
		}

		var resp BatchResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 2 || resp.Success != 2 || resp.Failed != 0 {
			t.Errorf("batch stats: total=%d success=%d failed=%d, want 2/2/0",
				resp.Total, resp.Success, resp.Failed)
		}

		if len(resp.Results) != 2 {
			t.Fatalf("results length = %d, want 2", len(resp.Results))
		}

		// First result should be SVG
		if resp.Results[0].Format != "svg" {
			t.Errorf("first result format = %q, want 'svg'", resp.Results[0].Format)
		}
		if resp.Results[0].SVG == "" {
			t.Error("first result should have SVG")
		}

		// Second result should be PNG
		if resp.Results[1].Format != "png" {
			t.Errorf("second result format = %q, want 'png'", resp.Results[1].Format)
		}
		if resp.Results[1].PNG == "" {
			t.Error("second result should have PNG")
		}
	})

	t.Run("PNG output is deterministic", func(t *testing.T) {
		body := `{
			"type": "bar_chart",
			"data": {
				"categories": ["X", "Y"],
				"series": [{"name": "Test", "values": [100, 200]}]
			},
			"output": {"format": "png", "width": 400, "height": 300}
		}`

		// Render twice
		var pngOutputs []string
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("render %d: status = %d, want %d", i, rec.Code, http.StatusOK)
			}

			var resp RenderResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("render %d: failed to decode response: %v", i, err)
			}

			pngOutputs = append(pngOutputs, resp.PNG)
		}

		// Compare outputs - they should be identical
		if pngOutputs[0] != pngOutputs[1] {
			t.Error("PNG output is not deterministic: identical inputs produced different outputs")
		}
	})
}
