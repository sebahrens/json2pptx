package api

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Modern sessions keep a small content synopsis; older sessions retain the
// complete JSON fallback for clients without structuredContent support.

// fakeSession is a ClientSession with a fixed ID, enough for the per-session
// protocol registry.
type fakeSession struct{ id string }

func (f *fakeSession) SessionID() string                                   { return f.id }
func (f *fakeSession) NotificationChannel() chan<- mcp.JSONRPCNotification { return nil }
func (f *fakeSession) Initialize()                                         {}
func (f *fakeSession) Initialized() bool                                   { return true }

// testServer builds the context carrier: mcp-go stores the session on the
// context via (*MCPServer).WithContext.
var testServer = server.NewMCPServer("text-fallback-test", "0.0.0")

// ctxForSession returns a context carrying a session with the given id.
func ctxForSession(id string) context.Context {
	return testServer.WithContext(context.Background(), &fakeSession{id: id})
}

func withMode(t *testing.T, mode TextFallbackMode) {
	t.Helper()
	SetTextFallbackMode(mode)
	t.Cleanup(func() { SetTextFallbackMode(TextFallbackAuto) })
}

func TestTextSummaryOnModernProtocol(t *testing.T) {
	withMode(t, TextFallbackAuto)
	ctx := ctxForSession("modern")
	RecordProtocolVersion(ctx, "2025-06-18")
	t.Cleanup(func() { ForgetProtocolVersion(ctx) })

	res, err := MCPSuccessResult(ctx, map[string]string{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("modern result content blocks = %d, want one summary", len(res.Content))
	}
	if text := res.Content[0].(mcp.TextContent).Text; text != `{"key":"value"}` || len(text) > maxMCPTextSummaryBytes {
		t.Errorf("modern text synopsis = %q", text)
	}
	if res.StructuredContent == nil {
		t.Error("structuredContent must always be present")
	}
}

func TestTextFallbackKeptForOlderProtocol(t *testing.T) {
	withMode(t, TextFallbackAuto)
	for _, version := range []string{"2024-11-05", "2025-03-26"} {
		ctx := ctxForSession("old-" + version)
		RecordProtocolVersion(ctx, version)
		res, err := MCPSuccessResult(ctx, map[string]string{"key": "value"})
		ForgetProtocolVersion(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Content) == 0 {
			t.Errorf("protocol %s lost its text fallback — that client cannot read structuredContent", version)
		}
	}
}

// TestTextFallbackKeptWhenProtocolUnknown is the safe direction: a handler
// called directly (every unit test, and any transport that did not record an
// initialize) keeps the copy.
func TestTextFallbackKeptWhenProtocolUnknown(t *testing.T) {
	withMode(t, TextFallbackAuto)
	res, err := MCPSuccessResult(context.Background(), map[string]string{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) == 0 {
		t.Error("an unknown protocol must keep the text copy")
	}
}

func TestTextFallbackModes(t *testing.T) {
	ctx := ctxForSession("modes")
	RecordProtocolVersion(ctx, "2025-06-18")
	t.Cleanup(func() { ForgetProtocolVersion(ctx) })

	withMode(t, TextFallbackAlways)
	if res, err := MCPSuccessResult(ctx, map[string]string{"k": "v"}); err != nil {
		t.Fatal(err)
	} else if len(res.Content) != 1 || res.Content[0].(mcp.TextContent).Text != `{"k":"v"}` {
		t.Errorf("always: want complete JSON copy, got %+v", res.Content)
	}

	SetTextFallbackMode(TextFallbackNever)
	if res, _ := MCPSuccessResult(context.Background(), map[string]string{"k": "v"}); len(res.Content) != 1 {
		t.Error("never: a bounded summary should remain even for an unknown protocol")
	}

	// An unrecognised mode leaves the current one in place.
	SetTextFallbackMode("nonsense")
	if res, _ := MCPSuccessResult(context.Background(), map[string]string{"k": "v"}); len(res.Content) != 1 {
		t.Error("an unrecognised mode changed the behaviour")
	}
	SetTextFallbackMode(TextFallbackAuto)
}

func TestProtocolVersionIsPerSession(t *testing.T) {
	a, b := ctxForSession("a"), ctxForSession("b")
	RecordProtocolVersion(a, "2025-06-18")
	RecordProtocolVersion(b, "2024-11-05")
	t.Cleanup(func() { ForgetProtocolVersion(a); ForgetProtocolVersion(b) })

	if got := negotiatedProtocolVersion(a); got != "2025-06-18" {
		t.Errorf("session a = %q", got)
	}
	if got := negotiatedProtocolVersion(b); got != "2024-11-05" {
		t.Errorf("session b = %q", got)
	}
	ForgetProtocolVersion(a)
	if got := negotiatedProtocolVersion(a); got != "" {
		t.Errorf("session a survived being forgotten: %q", got)
	}
	if got := negotiatedProtocolVersion(b); got != "2024-11-05" {
		t.Errorf("forgetting a disturbed b: %q", got)
	}
}
