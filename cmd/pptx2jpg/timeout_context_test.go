package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRealCommandRunnerDefaultTimeoutContext(t *testing.T) {
	runner := &RealCommandRunner{}
	ctx, cancel := runner.newTimeoutContext(20 * time.Millisecond)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 20*time.Millisecond {
		t.Fatal("default runner omitted requested deadline")
	}
	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("expired context error=%v", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("default deadline did not expire")
	}
	ctx, cancel = runner.newTimeoutContext(time.Hour)
	cancel()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("cancel error=%v", ctx.Err())
	}
}
