package visualqa

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultAPIURL   = "https://api.anthropic.com/v1/messages"
	defaultModel    = "claude-haiku-4-5-20251001"
	apiVersion      = "2023-06-01"
	defaultMaxToks  = 2048
	defaultParallel = 4

	// defaultHTTPTimeout bounds a single vision request (connect + send +
	// receive). Without it http.DefaultClient would wait forever on a stalled
	// API call, blocking the inspection goroutine — and InspectAll, which waits
	// for every goroutine — indefinitely.
	defaultHTTPTimeout = 60 * time.Second
	// defaultTotalTimeout bounds the whole InspectAll fan-out so a deck with many
	// slides (or several simultaneously stalled calls) still returns in bounded
	// time even if individual per-request timeouts stack up.
	defaultTotalTimeout = 5 * time.Minute
)

// VisionTimeoutCode is the diagnostic code carried by TimeoutError. It mirrors
// diagnostics.CodeVisionTimeout, declared here so the visualqa package stays free
// of that dependency.
const VisionTimeoutCode = "VISION_TIMEOUT"

// TimeoutError is returned by InspectSlide when a vision API call exceeds its
// deadline. It carries the slide index, elapsed time, and recovery guidance so
// callers can surface a structured VISION_TIMEOUT diagnostic.
type TimeoutError struct {
	Code       string        // always VisionTimeoutCode
	SlideIndex int           // slide whose inspection timed out
	Elapsed    time.Duration // wall time before the call was abandoned
	Timeout    time.Duration // the per-request deadline that was exceeded
}

// Error implements error. It bundles slide, elapsed, deadline, and recovery
// action so surfaces that only forward err.Error() stay actionable.
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s: vision inspection of slide %d timed out after %s (deadline %s). %s",
		e.Code, e.SlideIndex, e.Elapsed.Round(time.Millisecond), e.Timeout, e.Action())
}

// Action returns the suggested retry/degrade guidance for this timeout.
func (e *TimeoutError) Action() string {
	return "Retry the inspection; if it recurs, reduce parallelism or fall back to the " +
		"heuristic inspector (unset ANTHROPIC_API_KEY) to degrade gracefully."
}

// isTimeoutErr reports whether err (or ctx) represents a deadline/timeout, so a
// stalled vision call is attributed to a VISION_TIMEOUT rather than a generic
// API error. It catches both the per-request context deadline and the
// http.Client.Timeout (a net.Error with Timeout()==true).
func isTimeoutErr(ctx context.Context, err error) bool {
	if ctx != nil && ctx.Err() == context.DeadlineExceeded {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return false
}

// Agent performs visual QA on slide images using Claude Haiku's vision API.
type Agent struct {
	apiKey       string
	apiURL       string
	model        string
	maxTokens    int
	parallelism  int
	httpClient   *http.Client
	httpTimeout  time.Duration
	totalTimeout time.Duration
}

// DefaultModel returns the Claude model the visual-QA agent uses when no model
// override is supplied. Exposed so callers (e.g. the auto_repair visual-QA mode)
// can report the default model in their cost/requirements summary without
// hard-coding the version string.
func DefaultModel() string { return defaultModel }

// Option configures the Agent.
type Option func(*Agent)

// WithModel sets the Claude model to use.
func WithModel(model string) Option {
	return func(a *Agent) { a.model = model }
}

// WithParallelism sets the number of concurrent API calls.
func WithParallelism(n int) Option {
	return func(a *Agent) { a.parallelism = n }
}

// WithAPIURL overrides the Anthropic API endpoint.
func WithAPIURL(url string) Option {
	return func(a *Agent) { a.apiURL = url }
}

// WithHTTPTimeout sets the per-request deadline for a single vision call. It
// caps both the HTTP client and the per-slide request context.
func WithHTTPTimeout(d time.Duration) Option {
	return func(a *Agent) { a.httpTimeout = d }
}

// WithTotalTimeout sets the overall deadline for an InspectAll fan-out. Zero
// disables the total cap (per-request timeouts still apply).
func WithTotalTimeout(d time.Duration) Option {
	return func(a *Agent) { a.totalTimeout = d }
}

// NewAgent creates a new visual QA agent.
// The API key is read from the ANTHROPIC_API_KEY environment variable.
func NewAgent(opts ...Option) (*Agent, error) {
	a := &Agent{
		apiKey:       os.Getenv("ANTHROPIC_API_KEY"),
		apiURL:       defaultAPIURL,
		model:        defaultModel,
		maxTokens:    defaultMaxToks,
		parallelism:  defaultParallel,
		httpTimeout:  defaultHTTPTimeout,
		totalTimeout: defaultTotalTimeout,
	}
	for _, o := range opts {
		o(a)
	}
	if a.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is required")
	}
	// Bind the HTTP client to the resolved per-request timeout so a stalled API
	// call cannot block an inspection goroutine forever (http.DefaultClient has
	// no timeout).
	a.httpClient = &http.Client{Timeout: a.httpTimeout}
	return a, nil
}

// InspectSlide analyzes a single slide image and returns findings.
func (a *Agent) InspectSlide(ctx context.Context, imgData []byte, info SlideInfo) (*SlideResult, error) {
	prompt := PromptForSlideType(info.Type)
	if info.Title != "" {
		prompt += fmt.Sprintf("\n\nSlide title: %q", info.Title)
	}

	b64 := base64.StdEncoding.EncodeToString(imgData)
	mediaType := "image/jpeg"
	if len(imgData) > 8 && string(imgData[:8]) == "\x89PNG\r\n\x1a\n" {
		mediaType = "image/png"
	}

	reqBody := apiRequest{
		Model:     a.model,
		MaxTokens: a.maxTokens,
		System:    systemPrompt,
		Messages: []apiMessage{
			{
				Role: "user",
				Content: []apiContent{
					{
						Type: "image",
						Source: &apiImageSource{
							Type:      "base64",
							MediaType: mediaType,
							Data:      b64,
						},
					},
					{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Derive a per-slide deadline so a stalled call is bounded and attributable
	// to this slide as a VISION_TIMEOUT, independent of the HTTP client's own cap.
	reqCtx := ctx
	if a.httpTimeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, a.httpTimeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, "POST", a.apiURL, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	start := time.Now()
	resp, err := a.httpClient.Do(req)
	if err != nil {
		if isTimeoutErr(reqCtx, err) {
			return nil, &TimeoutError{Code: VisionTimeoutCode, SlideIndex: info.Index, Elapsed: time.Since(start), Timeout: a.httpTimeout}
		}
		return nil, fmt.Errorf("API call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if isTimeoutErr(reqCtx, err) {
			return nil, &TimeoutError{Code: VisionTimeoutCode, SlideIndex: info.Index, Elapsed: time.Since(start), Timeout: a.httpTimeout}
		}
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// Extract text content from response.
	var text string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}

	result := &SlideResult{
		SlideIndex: info.Index,
		SlideType:  info.Type,
		RawOutput:  text,
	}

	findings, err := parseFindings(text, info)
	if err != nil {
		result.Error = fmt.Sprintf("parse findings: %v", err)
	}
	// Always store findings — SchemaError returns partial results alongside
	// the error so callers can inspect what the model produced.
	result.Findings = findings

	return result, nil
}

// InspectAll analyzes multiple slides concurrently. The whole fan-out is bounded
// by the total inspection deadline (a.totalTimeout): once it elapses, every
// in-flight and not-yet-started per-slide call observes the cancelled context and
// returns a VISION_TIMEOUT promptly, so InspectAll never blocks indefinitely on a
// stalled API even across many slides.
func (a *Agent) InspectAll(ctx context.Context, slides []SlideImage) *Report {
	if ctx == nil {
		ctx = context.Background()
	}
	if a.totalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.totalTimeout)
		defer cancel()
	}

	report := &Report{
		SlideCount: len(slides),
		Results:    make([]SlideResult, len(slides)),
	}

	sem := make(chan struct{}, a.parallelism)
	var wg sync.WaitGroup

	for i, s := range slides {
		wg.Add(1)
		go func(idx int, slide SlideImage) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := a.InspectSlide(ctx, slide.Data, slide.Info)
			if err != nil {
				report.Results[idx] = SlideResult{
					SlideIndex: slide.Info.Index,
					SlideType:  slide.Info.Type,
					Error:      err.Error(),
				}
				return
			}
			report.Results[idx] = *result
		}(i, s)
	}

	wg.Wait()
	report.Summarize()
	return report
}

// SlideImage pairs image data with slide metadata.
type SlideImage struct {
	Info SlideInfo
	Data []byte // JPEG or PNG image data
}

// parseFindings extracts structured findings from the model's JSON response.
func parseFindings(text string, info SlideInfo) ([]Finding, error) {
	// Strip markdown code fences if present.
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		// Remove opening fence line.
		if idx := strings.Index(text, "\n"); idx >= 0 {
			text = text[idx+1:]
		}
		// Remove closing fence.
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
		text = strings.TrimSpace(text)
	}

	var raw []struct {
		Severity    string `json:"severity"`
		Category    string `json:"category"`
		Description string `json:"description"`
		Location    string `json:"location"`
	}

	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w (raw: %s)", err, truncate(text, 200))
	}

	findings := make([]Finding, 0, len(raw))
	var violations []string
	for i, r := range raw {
		sev := Severity(r.Severity)
		if !ValidSeverity(sev) {
			violations = append(violations, fmt.Sprintf("finding[%d]: unknown severity %q", i, r.Severity))
		}
		if !ValidCategory(r.Category) {
			violations = append(violations, fmt.Sprintf("finding[%d]: unknown category %q", i, r.Category))
		}
		f := Finding{
			SlideIndex:     info.Index,
			SlideType:      info.Type,
			Severity:       sev,
			Category:       r.Category,
			Description:    r.Description,
			Location:       r.Location,
			SuggestedFixes: SuggestedFixesForCategory(r.Category),
		}
		findings = append(findings, f)
	}
	if len(violations) > 0 {
		return findings, &SchemaError{Violations: violations}
	}
	return findings, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// API request/response types for the Anthropic Messages API.

type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system"`
	Messages  []apiMessage `json:"messages"`
}

type apiMessage struct {
	Role    string       `json:"role"`
	Content []apiContent `json:"content"`
}

type apiContent struct {
	Type   string          `json:"type"`
	Text   string          `json:"text,omitempty"`
	Source *apiImageSource `json:"source,omitempty"`
}

type apiImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type apiResponse struct {
	Content []apiContentBlock `json:"content"`
}

type apiContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
