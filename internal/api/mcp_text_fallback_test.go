package api

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// go-slide-creator-vxre. MCPSuccessResult put the whole payload in
// Content[0].text AND in StructuredContent. One pass over the 18 callable core
// tools weighed 389 KB on the wire for 168 KB of information, 53,134 B of it
// pure two-space indentation.

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

func TestTextFallbackOmittedOnModernProtocol(t *testing.T) {
	withMode(t, TextFallbackAuto)
	ctx := ctxForSession("modern")
	RecordProtocolVersion(ctx, "2025-06-18")
	t.Cleanup(func() { ForgetProtocolVersion(ctx) })

	res, err := MCPSuccessResult(ctx, map[string]string{"key": "value"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) != 0 {
		t.Errorf("a client on 2025-06-18 still received the duplicate text copy: %+v", res.Content)
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
	if res, _ := MCPSuccessResult(ctx, map[string]string{"k": "v"}); len(res.Content) == 0 {
		t.Error("always: the text copy should be present even on a modern protocol")
	}

	SetTextFallbackMode(TextFallbackNever)
	if res, _ := MCPSuccessResult(context.Background(), map[string]string{"k": "v"}); len(res.Content) != 0 {
		t.Error("never: the text copy should be absent even for an unknown protocol")
	}

	// An unrecognised mode leaves the current one in place.
	SetTextFallbackMode("nonsense")
	if res, _ := MCPSuccessResult(context.Background(), map[string]string{"k": "v"}); len(res.Content) != 0 {
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
