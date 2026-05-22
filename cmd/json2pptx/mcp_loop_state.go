// mcp_loop_state.go implements resumable per-pass state for the convergence
// facades (auto_repair, make_deck).
//
// Why this exists: auto_repair and make_deck hide a multi-pass
// generate→inspect→repair loop behind one long-running call and only return a
// trace after the whole loop finishes. If the gate is not met within the pass
// budget — or a render stalls late — the agent has no way to pick up where the
// loop left off; calling the tool again restarts from scratch and re-runs every
// completed pass.
//
// This file adds a per-process, TTL'd session store that snapshots the loop
// after it stops (the post-repair deck, the accumulated trace, the next pass
// index, and the resume parameters). The facade returns a caller-visible
// next_state block — completion status, a resume token, the remaining findings,
// and a suggested next action — so the agent can inspect the partial result and,
// when more work is warranted, call the same tool again with resume_token to
// continue from the saved deck WITHOUT repeating the passes already done.
//
// Scope mirrors mcp_idempotency.go: the store is per-process and drops on
// restart. It is a convergence checkpoint, not durable persistence.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// loopSessionTTL bounds how long a resumable checkpoint stays loadable. One
// hour comfortably covers an interactive repair session while keeping the store
// from holding deck JSON indefinitely.
const loopSessionTTL = time.Hour

// maxNextStateFindings caps how many remaining findings the next_state block
// echoes. The full set is recoverable by re-running the facade; this keeps the
// response bounded when a deck has many open findings.
const maxNextStateFindings = 25

// Loop completion statuses. completion is the single explicit label for how the
// last call terminated, so a partial or degraded result is never mistaken for a
// converged one.
const (
	// loopCompletionConverged: the gate was met on complete evidence. Nothing
	// more to do; not resumable.
	loopCompletionConverged = "converged"
	// loopCompletionConvergedDegraded: the gate was met but on degraded /
	// static-only evidence (allow_degraded_scoring). Resumable to retry for
	// complete evidence once render tooling is available.
	loopCompletionConvergedDegraded = "converged_degraded"
	// loopCompletionExhausted: the per-call pass budget ran out with the gate
	// unmet. Resumable: raise max_passes or relax the gate and continue.
	loopCompletionExhausted = "max_passes_exhausted"
	// loopCompletionNoProgress: a pass applied no repairs (the loop stalled)
	// with the gate unmet. Resumable, but likely needs a manual edit or a
	// relaxed gate to make further progress.
	loopCompletionNoProgress = "no_progress"
	// loopCompletionRenderIncomplete: the render that backs the score did not
	// complete. Resumable once render tooling is present, or set
	// allow_degraded_scoring.
	loopCompletionRenderIncomplete = "render_incomplete"
)

// loopCheckpoint is the server-side resumable snapshot of a convergence loop.
// It is held only in the session store and is never serialized into a response;
// the caller-visible projection is loopNextState.
type loopCheckpoint struct {
	// Tool names the facade that produced the checkpoint ("auto_repair" or
	// "make_deck"). Resume refuses a token issued by a different tool so a
	// make_deck session is never continued as an auto_repair (its provenance and
	// plan would be wrong).
	Tool string

	// Input is the post-repair deck after the last completed pass: relative
	// asset paths already resolved to absolute, canonical layout IDs already
	// resolved, and every repair from the prior call baked in. Resuming from it
	// is exactly "continue from where we stopped" — no completed pass repeats.
	Input *PresentationInput
	// Trace is the accumulated per-pass record across the whole session. The
	// resumed loop appends to it with continuous (global) pass numbering.
	Trace []autoRepairTraceEntry
	// NextPass is the 1-based global pass index the resumed loop starts at
	// (passes already run + 1).
	NextPass int

	// Terminal state of the call that wrote the checkpoint, used to seed the
	// next_state projection and (for the gate fields) survive a resume that runs
	// no additional passes.
	LastScore       int
	LastGateReasons []string
	GatePassed      bool
	Completion      string

	// Resume parameters inherited by default when the caller resumes. gate and
	// max_passes may be overridden on the resume call (the documented
	// "relaxed bounds" path); the rest are fixed for the session.
	Gate           autoRepairGate
	MaxPasses      int
	VQA            visualQAConfig
	OutputFilename string
	BaseDir        string
	AllowDegraded  bool
	Provenance     contentProvenance

	// Plan carries the make_deck plan summary so a resumed make_deck call can
	// repopulate its required `plan` field without re-planning. Nil for
	// auto_repair.
	Plan *makeDeckPlanSummary
}

// loopResumeState is the slice of a checkpoint the convergence loop itself needs
// to continue: where to start and the trace to extend. The deck is passed
// separately as the loop's input argument.
type loopResumeState struct {
	StartPass int
	Trace     []autoRepairTraceEntry
}

// loopNextState is the caller-visible per-pass state block returned by the
// facades. It exposes enough to inspect a partial result and resume it: the
// completion label, whether resuming can help, a human/agent next action, the
// pass accounting, the artifact path, the resume token, and the findings still
// open after the last pass. The full post-repair deck JSON lives in the
// response's final_presentation field (not duplicated here), and the per-pass
// attempted fixes live in trace[].repairs_applied.
type loopNextState struct {
	// Completion is one of the loopCompletion* constants.
	Completion string `json:"completion"`
	// Resumable is true when calling the tool again with ResumeToken can make
	// further progress (every status except a clean converged run).
	Resumable bool `json:"resumable"`
	// ResumeToken is the opaque handle to pass back as resume_token to continue
	// this session. Present whenever a checkpoint was stored.
	ResumeToken string `json:"resume_token,omitempty"`
	// NextAction is a one-line, machine-parseable-ish instruction for what to do
	// next given Completion.
	NextAction string `json:"next_action"`
	// PassesRun is the total number of passes run across the whole session
	// (matches len(trace)).
	PassesRun int `json:"passes_run"`
	// NextPass is the global pass index a resume would start at. Present only
	// when Resumable.
	NextPass int `json:"next_pass,omitempty"`
	// MaxPasses echoes the per-call pass budget that produced this result.
	MaxPasses int `json:"max_passes"`
	// ArtifactPath is the on-disk PPTX written for this checkpoint (mirrors the
	// response's top-level path).
	ArtifactPath string `json:"artifact_path,omitempty"`
	// RemainingFindings summarizes the findings still open after the last pass,
	// capped at maxNextStateFindings. Empty on a converged run.
	RemainingFindings []nextStateFinding `json:"remaining_findings,omitempty"`
}

// nextStateFinding is the compact per-finding shape echoed in next_state. The
// full finding objects (with measured/allowed extents and fix params) are
// re-derivable by re-running the facade; this is enough to decide the next move.
type nextStateFinding struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
	Action  string `json:"action,omitempty"`
}

// loopSessionStore is a per-process, TTL'd store of resumable loop checkpoints
// keyed by an opaque resume token. Safe for concurrent use across MCP handler
// goroutines. A nil receiver behaves as "no store" so tests that don't exercise
// resume can leave the field unset.
type loopSessionStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]loopSessionEntry
	now     func() time.Time
}

type loopSessionEntry struct {
	checkpoint *loopCheckpoint
	expiresAt  time.Time
}

func newLoopSessionStore(ttl time.Duration) *loopSessionStore {
	return &loopSessionStore{
		ttl:     ttl,
		entries: make(map[string]loopSessionEntry),
		now:     time.Now,
	}
}

// Save stores the checkpoint under a freshly minted token and returns it. A nil
// receiver or checkpoint yields an empty token (resume simply won't be offered).
func (s *loopSessionStore) Save(cp *loopCheckpoint) string {
	if s == nil || cp == nil {
		return ""
	}
	token := newResumeToken()
	if token == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[token] = loopSessionEntry{
		checkpoint: cp,
		expiresAt:  s.now().Add(s.ttl),
	}
	return token
}

// Load returns the live checkpoint for a token. Expired or unknown tokens (and a
// nil receiver) return ok=false.
func (s *loopSessionStore) Load(token string) (*loopCheckpoint, bool) {
	if s == nil || token == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[token]
	if !ok {
		return nil, false
	}
	if s.now().After(entry.expiresAt) {
		delete(s.entries, token)
		return nil, false
	}
	return entry.checkpoint, true
}

// newResumeToken mints a 128-bit random hex token. Returns the empty string on
// the (practically impossible) RNG failure, which the caller treats as "no
// resume offered" rather than crashing a successful generation.
func newResumeToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return "rs_" + hex.EncodeToString(b[:])
}

// --- Tool parameter ---

const resumeTokenParamDescription = "Optional resume token returned in a previous auto_repair/make_deck response's next_state.resume_token. When set, the server reloads the saved post-repair deck and continues the convergence loop from where it stopped, WITHOUT repeating completed passes. On resume the deck, accumulated trace, and content provenance come from the saved session; gate and max_passes may be overridden on this call (e.g. to relax bounds or grant more passes) while base_dir, visual_qa, allow_degraded_scoring, and output_filename are inherited. presentation (auto_repair) / outline (make_deck) are ignored when resume_token is supplied. Tokens are per-process and expire after 1 hour."

// resumeTokenToolParam declares the resume_token parameter on the facades.
func resumeTokenToolParam() mcp.ToolOption {
	return mcp.WithString("resume_token", mcp.Description(resumeTokenParamDescription))
}

// resumeToken reads the resume_token argument, returning "" when absent or
// non-string.
func resumeToken(request mcp.CallToolRequest) string {
	raw, ok := request.GetArguments()["resume_token"]
	if !ok {
		return ""
	}
	s, _ := raw.(string)
	return s
}

// loadResumeCheckpoint resolves a resume_token for the given tool, returning an
// MCP error result the caller passes straight through when the token is unknown,
// expired, or was issued by a different tool.
func (mc *mcpConfig) loadResumeCheckpoint(tool, token string) (*loopCheckpoint, *mcp.CallToolResult) {
	cp, ok := mc.loopSessions.Load(token)
	if !ok {
		return nil, mcpErrorWithNext(
			"RESUME_TOKEN_NOT_FOUND",
			fmt.Sprintf("resume_token %q is unknown or has expired (resumable sessions live %s). Start a fresh run without resume_token.", token, loopSessionTTL),
			nil,
		)
	}
	if cp.Tool != tool {
		return nil, mcpErrorWithNext(
			"RESUME_TOKEN_MISMATCH",
			fmt.Sprintf("resume_token %q was issued by %s, not %s; resume it with the tool that created it.", token, cp.Tool, tool),
			nil,
		)
	}
	return cp, nil
}

// --- next_state / checkpoint derivation ---

// deriveLoopCompletion classifies how the loop terminated. The order matters:
// a met gate is reported first (clean vs degraded), then the failure modes from
// most to least diagnostic.
func deriveLoopCompletion(gatePassed, evidenceComplete, renderComplete, stalled bool) string {
	if gatePassed {
		if evidenceComplete {
			return loopCompletionConverged
		}
		return loopCompletionConvergedDegraded
	}
	if !renderComplete {
		return loopCompletionRenderIncomplete
	}
	if stalled {
		return loopCompletionNoProgress
	}
	return loopCompletionExhausted
}

// nextActionForCompletion maps a completion status to a one-line instruction.
func nextActionForCompletion(completion string) string {
	switch completion {
	case loopCompletionConverged:
		return "Deck converged on complete evidence. No further passes needed; resume with a stricter gate only if you want to refine further."
	case loopCompletionConvergedDegraded:
		return "Gate met on degraded/static-only evidence. Resume with resume_token once render tooling (libreoffice + magick) is available to confirm on complete evidence."
	case loopCompletionExhausted:
		return "Pass budget exhausted with the gate unmet. Resume with resume_token and a higher max_passes or a relaxed gate to continue from the current deck without repeating completed passes."
	case loopCompletionNoProgress:
		return "Repairs stopped making progress with the gate unmet. Resume with resume_token after editing the deck (repair_slide) or relaxing the gate; another automatic pass alone is unlikely to help."
	case loopCompletionRenderIncomplete:
		return "Render evidence was incomplete. Resume with resume_token once render tooling is available, or pass allow_degraded_scoring to converge on static analysis."
	default:
		return "Resume with resume_token to continue the convergence loop from the current deck state."
	}
}

// summarizeRemainingFindings projects the open findings into the compact
// next_state shape, capped at maxNextStateFindings.
func summarizeRemainingFindings(findings []patterns.FitFinding) []nextStateFinding {
	if len(findings) == 0 {
		return nil
	}
	n := len(findings)
	if n > maxNextStateFindings {
		n = maxNextStateFindings
	}
	out := make([]nextStateFinding, 0, n)
	for i := 0; i < n; i++ {
		f := findings[i]
		out = append(out, nextStateFinding{
			Code:    f.Code,
			Path:    f.Path,
			Message: f.Message,
			Action:  f.Action,
		})
	}
	return out
}

// extractResumeMaxPasses reads the per-call pass budget on a resume, honoring
// both auto_repair's max_passes and make_deck's max_repair_passes. When neither
// is supplied it inherits fallback (the session's budget) instead of silently
// resetting to the engine default. The result is clamped to [1, 10].
func extractResumeMaxPasses(request mcp.CallToolRequest, fallback int) int {
	args := request.GetArguments()
	if raw, ok := args["max_repair_passes"]; ok {
		return clampMaxPasses(anyToInt(raw, fallback))
	}
	if _, ok := args["max_passes"]; ok {
		return extractMaxPasses(request)
	}
	return clampMaxPasses(fallback)
}

// extractAutoRepairGateOver merges the request's gate fields over a base gate
// (rather than over the engine defaults). Used on resume so omitted gate fields
// inherit the session's gate while supplied ones override it. Mirrors
// extractAutoRepairGate's json round-trip so it tolerates any concrete arg type.
func extractAutoRepairGateOver(request mcp.CallToolRequest, base autoRepairGate) autoRepairGate {
	gate := base
	raw, ok := request.GetArguments()["gate"]
	if !ok || raw == nil {
		return gate
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return gate
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return gate
	}
	if v, ok := parsed["min_score"]; ok {
		gate.MinScore = anyToInt(v, gate.MinScore)
	}
	if v, ok := parsed["max_p0_findings"]; ok {
		gate.MaxP0Findings = anyToInt(v, gate.MaxP0Findings)
	}
	if v, ok := parsed["max_p1_findings"]; ok {
		gate.MaxP1Findings = anyToInt(v, gate.MaxP1Findings)
	}
	if v, ok := parsed["require_takeaway_on_charts"]; ok {
		if b, ok := v.(bool); ok {
			gate.RequireTakeawayOnCharts = b
		}
	}
	return gate
}
