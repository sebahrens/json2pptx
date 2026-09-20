package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type loggingTestSession struct {
	id      string
	notices chan mcp.JSONRPCNotification
	mu      sync.Mutex
	level   mcp.LoggingLevel
}

func newLoggingTestSession(id string) *loggingTestSession {
	return &loggingTestSession{id: id, notices: make(chan mcp.JSONRPCNotification, 16), level: mcp.LoggingLevelError}
}

func (s *loggingTestSession) SessionID() string { return s.id }
func (s *loggingTestSession) Initialize()       {}
func (s *loggingTestSession) Initialized() bool { return true }
func (s *loggingTestSession) NotificationChannel() chan<- mcp.JSONRPCNotification {
	return s.notices
}
func (s *loggingTestSession) SetLogLevel(level mcp.LoggingLevel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.level = level
}
func (s *loggingTestSession) GetLogLevel() mcp.LoggingLevel {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.level
}

func loggingRPC(t *testing.T, srv *server.MCPServer, ctx context.Context, method string, params map[string]any) mcp.JSONRPCMessage {
	t.Helper()
	request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	return srv.HandleMessage(ctx, request)
}

func loggingNoticeData(t *testing.T, session *loggingTestSession) map[string]any {
	t.Helper()
	select {
	case notice := <-session.notices:
		if notice.Method != "notifications/message" {
			t.Fatalf("unexpected notification method %q", notice.Method)
		}
		data, ok := notice.Params.AdditionalFields["data"].(map[string]any)
		if !ok {
			t.Fatalf("unexpected log data: %+v", notice.Params.AdditionalFields["data"])
		}
		return data
	default:
		t.Fatal("expected MCP log notification")
		return nil
	}
}

func TestMCPLoggingSetLevelAndRender(t *testing.T) {
	mc := semanticTestConfig(t)
	srv := newMCPServer(mc)
	initialized := loggingRPC(t, srv, context.Background(), "initialize", map[string]any{
		"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "logging-test", "version": "0"},
	})
	initResponse, ok := initialized.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("initialize failed: %T %+v", initialized, initialized)
	}
	initJSON, err := json.Marshal(initResponse.Result)
	if err != nil {
		t.Fatal(err)
	}
	var init struct {
		Capabilities struct {
			Logging *struct{} `json:"logging"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(initJSON, &init); err != nil || init.Capabilities.Logging == nil {
		t.Fatalf("initialize did not advertise logging: %s (%v)", initJSON, err)
	}
	session := newLoggingTestSession("render")
	if err := srv.RegisterSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	ctx := srv.WithContext(context.Background(), session)
	if _, ok := loggingRPC(t, srv, ctx, "logging/setLevel", map[string]any{"level": "info"}).(mcp.JSONRPCResponse); !ok {
		t.Fatal("logging/setLevel did not succeed")
	}
	if session.GetLogLevel() != mcp.LoggingLevelInfo {
		t.Fatal("session did not adopt info level")
	}
	response := loggingRPC(t, srv, ctx, "tools/call", map[string]any{
		"name": "render_deck_spec", "arguments": map[string]any{"spec": validSemanticSpec},
	})
	if _, ok := response.(mcp.JSONRPCResponse); !ok {
		t.Fatalf("render_deck_spec protocol call failed: %T %+v", response, response)
	}
	start := loggingNoticeData(t, session)
	if start["message"] != "deck render started" || start["tool"] != "render_deck_spec" || start["slide_count"] != 2 {
		t.Fatalf("unexpected render start log: %+v", start)
	}
	finish := loggingNoticeData(t, session)
	if finish["message"] != "deck render finished" || finish["pptx_path"] == "" {
		t.Fatalf("unexpected render finish log: %+v", finish)
	}
	loggingRPC(t, srv, ctx, "logging/setLevel", map[string]any{"level": "warning"})
	loggingRPC(t, srv, ctx, "tools/call", map[string]any{
		"name": "render_deck_spec", "arguments": map[string]any{"spec": "meta: [broken: yaml"},
	})
	failed := loggingNoticeData(t, session)
	if failed["message"] != "deck spec parse failed" || failed["tool"] != "render_deck_spec" {
		t.Fatalf("unexpected render failure log: %+v", failed)
	}
}

func TestMCPLoggingSessionIsolationAndFiltering(t *testing.T) {
	mc := semanticTestConfig(t)
	srv := newMCPServer(mc)
	first := newLoggingTestSession("first")
	second := newLoggingTestSession("second")
	for _, session := range []*loggingTestSession{first, second} {
		if err := srv.RegisterSession(context.Background(), session); err != nil {
			t.Fatal(err)
		}
	}
	firstCtx := srv.WithContext(context.Background(), first)
	secondCtx := srv.WithContext(context.Background(), second)
	// Before setLevel, the SDK's default error threshold suppresses INFO/WARN.
	mc.logRenderEvent(firstCtx, mcp.LoggingLevelInfo, "before opt-in", map[string]any{"session": "first"})
	if len(first.notices) != 0 || len(second.notices) != 0 {
		t.Fatal("log notification sent before client opt-in")
	}
	loggingRPC(t, srv, firstCtx, "logging/setLevel", map[string]any{"level": "info"})
	loggingRPC(t, srv, secondCtx, "logging/setLevel", map[string]any{"level": "warning"})
	var wg sync.WaitGroup
	for _, item := range []struct {
		ctx context.Context
		id  string
	}{{firstCtx, "first"}, {secondCtx, "second"}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mc.logRenderEvent(item.ctx, mcp.LoggingLevelInfo, "info", map[string]any{"session": item.id})
			mc.logRenderEvent(item.ctx, mcp.LoggingLevelWarning, "warning", map[string]any{"session": item.id})
		}()
	}
	wg.Wait()
	if len(first.notices) != 2 || len(second.notices) != 1 {
		t.Fatalf("unexpected filtered counts: first=%d second=%d", len(first.notices), len(second.notices))
	}
	for range 2 {
		if got := loggingNoticeData(t, first); got["session"] != "first" {
			t.Fatalf("cross-session log in first: %+v", got)
		}
	}
	if got := loggingNoticeData(t, second); got["session"] != "second" || got["message"] != "warning" {
		t.Fatalf("cross-session or unfiltered log in second: %+v", got)
	}
}
