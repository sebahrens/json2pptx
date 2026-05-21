package diagnostics

// FindingEnvelope is the single agent-facing response shape for every
// diagnostic-bearing surface in json2pptx: validate, validate-template,
// template-check, generate -dry-run, repair, inspect, the forthcoming
// preflight / examine-template subcommands, and their MCP and HTTP serve-mode
// equivalents. It wraps a slice of Findings with the run-level metadata an
// agent needs to correlate the result with its request (which tool ran, the
// SHA-256 of the input it sent, the template it targeted) and a single OK flag
// it can branch on.
//
// The envelope is built from transport-neutral Diagnostic values via
// BuildEnvelope, so the existing Diagnostic type and its From* converters
// remain the adapter input — callers do not construct Findings by hand.
//
// The wire contract is documented in docs/AGENT_DIAGNOSTICS.md section 2 and
// validated by docs/api/finding-envelope.schema.json.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// SchemaVersion identifies the wire version of the FindingEnvelope contract.
// Bump it only on a backward-incompatible change to the envelope or Finding
// shape; additive optional fields keep the same version.
const SchemaVersion = "1.0"

// DefaultTool is the tool identifier reported in the envelope when a caller
// does not supply one.
const DefaultTool = "json2pptx"

// Namespace is the uppercase dotted prefix that classifies a finding's origin.
// Every Finding.Code begins with one of these prefixes.
type Namespace = string

// Finding-code namespace prefixes. A finding's Category is one of these and its
// Code is "<Namespace>.<legacy-code>".
const (
	NamespaceTemplate Namespace = "TPL"    // template structure / metadata problems
	NamespaceFit      Namespace = "FIT"    // content overflow / density diagnostics
	NamespaceGrid     Namespace = "GRID"   // shape-grid / pattern layout problems
	NamespaceRender   Namespace = "RENDER" // generation / rendering / media failures
	NamespacePolicy   Namespace = "POLICY" // content-policy violations (e.g. emoji)
	NamespaceInput    Namespace = "INPUT"  // request / JSON-payload problems
)

// AllNamespaces returns every declared namespace prefix, in catalog order.
func AllNamespaces() []Namespace {
	return []Namespace{
		NamespaceTemplate,
		NamespaceFit,
		NamespaceGrid,
		NamespaceRender,
		NamespacePolicy,
		NamespaceInput,
	}
}

// Action is a member of the remediation action vocabulary. Every
// RemediationAction.Action is one of the declared Action constants.
type Action = string

// Remediation action vocabulary. These name the machine-actionable repair an
// agent can take; the kind-specific arguments travel in RemediationAction.Params.
const (
	ActionShortenText       Action = "shorten_text"
	ActionReplaceValue      Action = "replace_value"
	ActionApplyPatch        Action = "apply_patch"
	ActionSwitchLayout      Action = "switch_layout"
	ActionSplitSlide        Action = "split_slide"
	ActionMoveToPlaceholder Action = "move_to_placeholder"
	ActionRemoveEmoji       Action = "remove_emoji"
	ActionRegeneratePattern Action = "regenerate_pattern"
)

// AllActions returns every declared remediation action, in catalog order.
func AllActions() []Action {
	return []Action{
		ActionShortenText,
		ActionReplaceValue,
		ActionApplyPatch,
		ActionSwitchLayout,
		ActionSplitSlide,
		ActionMoveToPlaceholder,
		ActionRemoveEmoji,
		ActionRegeneratePattern,
	}
}

// IsValidAction reports whether a is a declared remediation action.
func IsValidAction(a Action) bool {
	for _, v := range AllActions() {
		if v == a {
			return true
		}
	}
	return false
}

// Where locates a finding within a deck or template. All fields are optional;
// an empty Where (IsZero) is omitted from the envelope.
type Where struct {
	// Slide is the 0-based index of the offending slide, when known.
	Slide *int `json:"slide,omitempty"`
	// SlideID is the stable slide identifier, when the deck supplies one.
	SlideID string `json:"slide_id,omitempty"`
	// LayoutID is the template layout the slide resolved to.
	LayoutID string `json:"layout_id,omitempty"`
	// LayoutRole is the canonical role of that layout (e.g. "title", "content").
	LayoutRole string `json:"layout_role,omitempty"`
	// PlaceholderID is the offending placeholder's portable id.
	PlaceholderID string `json:"placeholder_id,omitempty"`
	// PlaceholderRole is the canonical role of that placeholder.
	PlaceholderRole string `json:"placeholder_role,omitempty"`
}

// IsZero reports whether the Where carries no location information.
func (w Where) IsZero() bool {
	return w.Slide == nil &&
		w.SlideID == "" &&
		w.LayoutID == "" &&
		w.LayoutRole == "" &&
		w.PlaceholderID == "" &&
		w.PlaceholderRole == ""
}

// RemediationAction is one machine-actionable repair step. Action is a member
// of the action vocabulary; Params carries the kind-specific arguments (which
// row to split at, the replacement value, the target placeholder, etc.).
type RemediationAction struct {
	Action Action         `json:"action"`
	Params map[string]any `json:"params,omitempty"`
}

// Remediation carries a primary repair plus ranked alternatives, so an agent
// can pick a path rather than being handed a single take-it-or-leave-it fix.
type Remediation struct {
	Primary      *RemediationAction  `json:"primary,omitempty"`
	Alternatives []RemediationAction `json:"alternatives,omitempty"`
}

// Finding is one machine-readable, agent-fixable issue. It is produced from a
// transport-neutral Diagnostic by FindingFromDiagnostic; callers do not build
// it field by field.
type Finding struct {
	// ID is unique within an envelope so an agent can refer back to a specific
	// finding (e.g. "fit-1").
	ID string `json:"id"`
	// Code is the dotted, namespaced code, e.g. "FIT.placeholder_overflow".
	Code string `json:"code"`
	// Severity is "error", "warning", or "info".
	Severity Severity `json:"severity"`
	// Category is the namespace prefix of Code (e.g. "FIT").
	Category Namespace `json:"category"`
	// Where locates the issue in the deck/template; omitted when unknown.
	Where *Where `json:"where,omitempty"`
	// Message is the human-readable description.
	Message string `json:"message"`
	// Evidence carries numeric/enum facts only (measured vs. allowed extents,
	// overflow ratios, the offending JSON path, expected types). Never prose.
	Evidence map[string]any `json:"evidence,omitempty"`
	// Remediation is the structured repair suggestion, when one exists.
	Remediation *Remediation `json:"remediation,omitempty"`
	// DocURL points at the human documentation for the code, when available.
	DocURL string `json:"doc_url,omitempty"`
	// DescribeCommand is an executable command that explains the code, e.g.
	// "json2pptx describe-finding placeholder_overflow".
	DescribeCommand string `json:"describe_command,omitempty"`
}

// FindingEnvelope is the single response shape for every diagnostic-bearing
// surface. See the package-level doc on this type for the full contract.
type FindingEnvelope struct {
	// SchemaVersion is the wire version of this contract.
	SchemaVersion string `json:"schema_version"`
	// Tool is the producing tool, e.g. "json2pptx".
	Tool string `json:"tool"`
	// Subcommand names the surface that produced the envelope, e.g. "validate".
	Subcommand string `json:"subcommand"`
	// InputSHA256 is the hex SHA-256 of the request payload, for correlation.
	InputSHA256 string `json:"input_sha256,omitempty"`
	// Template is the template the run targeted, when applicable.
	Template string `json:"template,omitempty"`
	// OK is true when no finding has error severity.
	OK bool `json:"ok"`
	// Summary is a short human-readable roll-up, e.g. "2 errors, 1 warning".
	Summary string `json:"summary"`
	// Findings is the list of issues; always non-nil (may be empty).
	Findings []Finding `json:"findings"`
}

// EnvelopeOptions carries the run-level metadata stamped onto an envelope.
type EnvelopeOptions struct {
	// Tool is the producing tool; defaults to DefaultTool when empty.
	Tool string
	// Subcommand names the surface (e.g. "validate", "generate -dry-run").
	Subcommand string
	// InputSHA256 is the hex SHA-256 of the request payload (optional).
	InputSHA256 string
	// Template is the targeted template (optional).
	Template string
}

// BuildEnvelope assembles a FindingEnvelope from transport-neutral diagnostics.
// OK is true when no diagnostic has error severity. Findings is always non-nil
// and each Finding receives an id unique within the envelope.
func BuildEnvelope(opts EnvelopeOptions, ds []Diagnostic) FindingEnvelope {
	tool := opts.Tool
	if tool == "" {
		tool = DefaultTool
	}
	findings := make([]Finding, 0, len(ds))
	for i, d := range ds {
		f := FindingFromDiagnostic(d)
		f.ID = fmt.Sprintf("%s-%d", strings.ToLower(f.Category), i+1)
		findings = append(findings, f)
	}
	return FindingEnvelope{
		SchemaVersion: SchemaVersion,
		Tool:          tool,
		Subcommand:    opts.Subcommand,
		InputSHA256:   opts.InputSHA256,
		Template:      opts.Template,
		OK:            !HasErrors(ds),
		Summary:       Summary(ds),
		Findings:      findings,
	}
}

// FindingFromDiagnostic adapts a single transport-neutral Diagnostic into a
// Finding: it namespaces the code, classifies the category, lifts numeric/enum
// facts into Evidence, maps the Fix into a Remediation, and points
// DescribeCommand at the executable describe-finding lookup. The legacy
// (un-prefixed) code is used in DescribeCommand so the command stays runnable
// against the existing describe-finding registry.
func FindingFromDiagnostic(d Diagnostic) Finding {
	legacy := d.Code
	ns := ClassifyCode(legacy)
	sev := d.Severity
	if sev == "" {
		sev = SeverityError
	}
	f := Finding{
		ID:              DottedCode(ns, legacy),
		Code:            DottedCode(ns, legacy),
		Severity:        sev,
		Category:        ns,
		Message:         d.Message,
		Evidence:        evidenceFromDiagnostic(d),
		DescribeCommand: describeCommand(legacy),
	}
	if w := whereFromPath(d.Path); !w.IsZero() {
		f.Where = &w
	}
	if d.Fix != nil {
		f.Remediation = &Remediation{
			Primary: remediationFromFix(d.Fix),
		}
	}
	return f
}

// FindingsFromDiagnostics adapts a slice of Diagnostics into Findings, assigning
// each an id unique within the slice. Returns nil for an empty input.
func FindingsFromDiagnostics(ds []Diagnostic) []Finding {
	if len(ds) == 0 {
		return nil
	}
	out := make([]Finding, len(ds))
	for i, d := range ds {
		f := FindingFromDiagnostic(d)
		f.ID = fmt.Sprintf("%s-%d", strings.ToLower(f.Category), i+1)
		out[i] = f
	}
	return out
}

// ComputeInputSHA256 returns the hex-encoded SHA-256 of the input bytes, for
// use as FindingEnvelope.InputSHA256.
func ComputeInputSHA256(input []byte) string {
	sum := sha256.Sum256(input)
	return hex.EncodeToString(sum[:])
}

// DottedCode returns the namespaced code "<ns>.<legacy>". If legacy already
// begins with a known namespace prefix (e.g. it was already namespaced), it is
// returned unchanged.
func DottedCode(ns Namespace, legacy string) string {
	if legacy == "" {
		return ns
	}
	for _, known := range AllNamespaces() {
		if strings.HasPrefix(legacy, known+".") {
			return legacy
		}
	}
	return ns + "." + legacy
}

// describeCommand returns the executable describe-finding command for a legacy
// code, or "" when there is no code to describe.
func describeCommand(legacy string) string {
	if legacy == "" {
		return ""
	}
	return "json2pptx describe-finding " + legacy
}

// remediationFromFix maps a Diagnostic Fix into a primary RemediationAction,
// translating the fix kind into the action vocabulary while preserving the
// original kind and params.
func remediationFromFix(fix *Fix) *RemediationAction {
	params := fix.Params
	if !IsValidAction(fix.Kind) {
		// Preserve the original kind so nothing is lost on the wire.
		merged := make(map[string]any, len(fix.Params)+1)
		for k, v := range fix.Params {
			merged[k] = v
		}
		if fix.Kind != "" {
			merged["kind"] = fix.Kind
		}
		if len(merged) == 0 {
			merged = nil
		}
		return &RemediationAction{Action: mapFixKindToAction(fix.Kind), Params: merged}
	}
	return &RemediationAction{Action: fix.Kind, Params: params}
}

// mapFixKindToAction translates a legacy Fix.Kind into the action vocabulary,
// defaulting to ActionApplyPatch for unrecognized kinds.
func mapFixKindToAction(kind string) Action {
	k := strings.ToLower(kind)
	switch {
	case strings.Contains(k, "emoji"):
		return ActionRemoveEmoji
	case strings.Contains(k, "split"):
		return ActionSplitSlide
	case strings.Contains(k, "shrink"), strings.Contains(k, "shorten"),
		strings.Contains(k, "reduce"), strings.Contains(k, "truncate"):
		return ActionShortenText
	case strings.Contains(k, "layout"):
		return ActionSwitchLayout
	case strings.Contains(k, "move"), strings.Contains(k, "placeholder"):
		return ActionMoveToPlaceholder
	case strings.Contains(k, "regenerate"), strings.Contains(k, "pattern"):
		return ActionRegeneratePattern
	case strings.Contains(k, "provide"), strings.Contains(k, "replace"),
		strings.Contains(k, "use_one_of"), strings.Contains(k, "value"),
		strings.Contains(k, "enum"):
		return ActionReplaceValue
	default:
		return ActionApplyPatch
	}
}

// evidenceFromDiagnostic lifts numeric/enum facts from a Diagnostic into the
// Finding evidence map: scalar Details entries, known structured extents
// (measured/allowed/overflow_ratio), the offending JSON path, and the expected
// type. Free-form prose and arbitrary nested objects are dropped.
func evidenceFromDiagnostic(d Diagnostic) map[string]any {
	ev := map[string]any{}
	for k, v := range d.Details {
		switch k {
		case "measured", "allowed", "overflow_ratio", "action", "pattern", "segment_index":
			ev[k] = v
		default:
			if isScalarFact(v) {
				ev[k] = v
			}
		}
	}
	if d.Path != "" {
		ev["path"] = d.Path
	}
	if d.ExpectedType != "" {
		ev["expected_type"] = d.ExpectedType
	}
	if len(ev) == 0 {
		return nil
	}
	return ev
}

// isScalarFact reports whether v is a numeric, boolean, or string fact suitable
// for the evidence map.
func isScalarFact(v any) bool {
	switch v.(type) {
	case string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	default:
		return false
	}
}

// whereFromPath does a best-effort extraction of a slide index from a JSON path
// like "slides[0].content.body". Only the slide index is derivable from a JSON
// path; richer location fields are populated by callers that know the layout
// and placeholder roles.
func whereFromPath(path string) Where {
	var w Where
	const marker = "slides["
	i := strings.Index(path, marker)
	if i < 0 {
		return w
	}
	rest := path[i+len(marker):]
	j := strings.IndexByte(rest, ']')
	if j <= 0 {
		return w
	}
	n := 0
	for _, r := range rest[:j] {
		if r < '0' || r > '9' {
			return w
		}
		n = n*10 + int(r-'0')
	}
	w.Slide = &n
	return w
}

// classifyMap maps legacy codes declared in codes.go to their namespace. Codes
// not present fall through to the heuristic in ClassifyCode.
var classifyMap = func() map[string]Namespace {
	m := map[string]Namespace{}
	add := func(ns Namespace, codes ...Code) {
		for _, c := range codes {
			m[c] = ns
		}
	}
	add(NamespaceInput,
		CodeMissingParameter, CodeInvalidParameter, CodeInvalidJSON, CodeInvalidKey,
		CodeInvalidGrid, CodeInvalidSlide, CodeInvalidSlideIndex, CodeInvalidPath,
		CodeAmbiguousInput, CodeUnsupported, CodeUnknownEnum, CodeUnknownTableStyleID,
		CodeUnknownThemeColor, CodeUnknownPattern, CodeStyleNotFound, CodeFileNotFound,
		CodeIconPath, CodeIconNotFound, CodeIconPathExtInvalid, CodeIconPathTraversal,
		CodeIconPathSymlinkEscape, CodeIconAmbiguous, CodeIconMissing, CodeIconList,
		CodeIconBundledNameUnknown, CodeIconFillIgnoredInline, CodeImagePath,
		CodeBackgroundImagePath, CodeAssetPathEnvUnset)
	add(NamespaceTemplate,
		CodeTemplateNotFound, CodeTemplateError, CodeTemplatesDir)
	add(NamespaceGrid, CodePatternError)
	add(NamespaceFit, CodeStrictFit)
	add(NamespaceRender,
		CodeGenerationFailed, CodeReadFailed, CodeRenderFailed, CodeLibreOfficeUnavailable,
		CodeImageMagickUnavailable, CodeOutputDir, CodeValidationFailed,
		CodeOutputValidationError, CodeOverlayFailed, CodeAssetTooLarge, CodeURLFetchFailed,
		CodeURLResolverInit, CodeSVGInvalidRoot, CodeSVGUnsafeXML, CodeSVGParseError,
		CodeSettingsError, CodeSettingsWriteDisabled, CodeInvalidImage, CodeInspectDisabled,
		CodeInternal)
	return m
}()

// ClassifyCode returns the namespace for a legacy finding code. Codes declared
// in codes.go are looked up directly; others are classified by substring
// heuristics over the code text, defaulting to NamespaceInput. The lowercase
// pattern/fit codes (e.g. "placeholder_overflow") and the dotted "chart.*"
// codes are handled by the heuristic.
func ClassifyCode(legacy string) Namespace {
	if legacy == "" {
		return NamespaceInput
	}
	// Already namespaced? Return the existing prefix.
	for _, ns := range AllNamespaces() {
		if strings.HasPrefix(legacy, ns+".") {
			return ns
		}
	}
	if ns, ok := classifyMap[legacy]; ok {
		return ns
	}
	l := strings.ToLower(legacy)
	switch {
	case strings.Contains(l, "emoji"), strings.Contains(l, "policy"):
		return NamespacePolicy
	case strings.HasPrefix(l, "chart."), strings.Contains(l, "render"),
		strings.Contains(l, "generation"), strings.Contains(l, "overlay"):
		return NamespaceRender
	case strings.Contains(l, "template"):
		return NamespaceTemplate
	case strings.Contains(l, "grid"), strings.Contains(l, "pattern"),
		strings.Contains(l, "cell"):
		return NamespaceGrid
	case strings.Contains(l, "overflow"), strings.Contains(l, "overset"),
		strings.Contains(l, "fit"), strings.Contains(l, "density"),
		strings.Contains(l, "accent"), strings.Contains(l, "too_long"),
		strings.Contains(l, "truncat"):
		return NamespaceFit
	default:
		return NamespaceInput
	}
}
