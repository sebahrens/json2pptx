package workerpool

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// filterErrors returns only the results that have errors.
func filterErrors[T any](results []Result[T]) []Result[T] {
	var errs []Result[T]
	for _, r := range results {
		if r.Err != nil {
			errs = append(errs, r)
		}
	}
	return errs
}

// filterSuccess returns only the results that succeeded.
func filterSuccess[T any](results []Result[T]) []Result[T] {
	var success []Result[T]
	for _, r := range results {
		if r.Err == nil {
			success = append(success, r)
		}
	}
	return success
}

func TestNew(t *testing.T) {
	t.Run("positive workers", func(t *testing.T) {
		pool := New[int](4)
		if pool.workers != 4 {
			t.Errorf("expected 4 workers, got %d", pool.workers)
		}
	})

	t.Run("zero workers defaults to NumCPU", func(t *testing.T) {
		pool := New[int](0)
		if pool.workers <= 0 {
			t.Errorf("expected positive workers, got %d", pool.workers)
		}
	})

	t.Run("negative workers defaults to NumCPU", func(t *testing.T) {
		pool := New[int](-1)
		if pool.workers <= 0 {
			t.Errorf("expected positive workers, got %d", pool.workers)
		}
	})
}

func TestPool_Run_Empty(t *testing.T) {
	pool := New[int](2)
	results := pool.Run(context.Background(), nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestPool_Run_SingleTask(t *testing.T) {
	pool := New[string](2)
	tasks := []Task[string]{
		{
			ID: "task-1",
			Execute: func(ctx context.Context) (string, error) {
				return "hello", nil
			},
		},
	}

	results := pool.Run(context.Background(), tasks)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ID != "task-1" {
		t.Errorf("expected ID 'task-1', got '%s'", r.ID)
	}
	if r.Value != "hello" {
		t.Errorf("expected value 'hello', got '%s'", r.Value)
	}
	if r.Err != nil {
		t.Errorf("expected no error, got %v", r.Err)
	}
}

func TestPool_Run_MultipleTasks(t *testing.T) {
	pool := New[int](3)
	tasks := make([]Task[int], 10)
	for i := 0; i < 10; i++ {
		i := i // Capture for closure
		tasks[i] = Task[int]{
			ID: fmt.Sprintf("task-%d", i),
			Execute: func(ctx context.Context) (int, error) {
				return i * 2, nil
			},
		}
	}

	results := pool.Run(context.Background(), tasks)
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}

	resultMap := MapResults(results)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("task-%d", i)
		r, ok := resultMap[id]
		if !ok {
			t.Errorf("missing result for %s", id)
			continue
		}
		if r.Value != i*2 {
			t.Errorf("task %s: expected %d, got %d", id, i*2, r.Value)
		}
		if r.Err != nil {
			t.Errorf("task %s: unexpected error %v", id, r.Err)
		}
	}
}

func TestPool_Run_WithErrors(t *testing.T) {
	pool := New[string](2)
	errFail := errors.New("intentional failure")

	tasks := []Task[string]{
		{
			ID: "success-1",
			Execute: func(ctx context.Context) (string, error) {
				return "ok", nil
			},
		},
		{
			ID: "fail-1",
			Execute: func(ctx context.Context) (string, error) {
				return "", errFail
			},
		},
		{
			ID: "success-2",
			Execute: func(ctx context.Context) (string, error) {
				return "ok2", nil
			},
		},
	}

	results := pool.Run(context.Background(), tasks)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	errs := filterErrors(results)
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
	if errs[0].ID != "fail-1" {
		t.Errorf("expected fail-1 to have error, got %s", errs[0].ID)
	}

	success := filterSuccess(results)
	if len(success) != 2 {
		t.Errorf("expected 2 successes, got %d", len(success))
	}
}

func TestPool_Run_ContextCancellation(t *testing.T) {
	pool := New[int](2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure context is cancelled on all paths

	var executed atomic.Int32

	// Create tasks that block until context is cancelled
	tasks := make([]Task[int], 5)
	for i := 0; i < 5; i++ {
		i := i
		tasks[i] = Task[int]{
			ID: fmt.Sprintf("task-%d", i),
			Execute: func(ctx context.Context) (int, error) {
				executed.Add(1)
				// First task cancels context before completing
				if i == 0 {
					cancel()
				}
				select {
				case <-ctx.Done():
					return 0, ctx.Err()
				case <-time.After(100 * time.Millisecond):
					return i, nil
				}
			},
		}
	}

	results := pool.Run(ctx, tasks)
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// At least some tasks should have context.Canceled error
	cancelledCount := 0
	for _, r := range results {
		if errors.Is(r.Err, context.Canceled) {
			cancelledCount++
		}
	}
	if cancelledCount == 0 {
		t.Error("expected at least one task to be cancelled")
	}
}

func TestPool_Run_ConcurrentExecution(t *testing.T) {
	pool := New[int](4)
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	tasks := make([]Task[int], 20)
	for i := 0; i < 20; i++ {
		i := i
		tasks[i] = Task[int]{
			ID: fmt.Sprintf("task-%d", i),
			Execute: func(ctx context.Context) (int, error) {
				// Track concurrent executions
				c := concurrent.Add(1)
				for {
					old := maxConcurrent.Load()
					if c <= old {
						break
					}
					if maxConcurrent.CompareAndSwap(old, c) {
						break
					}
				}

				// Small sleep to allow overlap
				time.Sleep(10 * time.Millisecond)

				concurrent.Add(-1)
				return i, nil
			},
		}
	}

	results := pool.Run(context.Background(), tasks)
	if len(results) != 20 {
		t.Fatalf("expected 20 results, got %d", len(results))
	}

	// With 4 workers and 20 tasks, we should see concurrency
	max := maxConcurrent.Load()
	if max < 2 {
		t.Errorf("expected concurrent execution, max concurrent was %d", max)
	}
	if max > 4 {
		t.Errorf("concurrency exceeded worker count: max was %d", max)
	}

	t.Logf("max concurrent executions: %d", max)
}

func TestMapResults(t *testing.T) {
	results := []Result[int]{
		{ID: "a", Value: 1},
		{ID: "b", Value: 2},
		{ID: "c", Value: 3},
	}

	m := MapResults(results)
	if len(m) != 3 {
		t.Errorf("expected 3 entries, got %d", len(m))
	}
	if m["a"].Value != 1 {
		t.Errorf("expected a=1, got %d", m["a"].Value)
	}
	if m["b"].Value != 2 {
		t.Errorf("expected b=2, got %d", m["b"].Value)
	}
	if m["c"].Value != 3 {
		t.Errorf("expected c=3, got %d", m["c"].Value)
	}
}

func TestPool_Run_PanicRecovery(t *testing.T) {
	pool := New[string](2)

	tasks := []Task[string]{
		{
			ID: "success-1",
			Execute: func(ctx context.Context) (string, error) {
				return "ok", nil
			},
		},
		{
			ID: "panic-1",
			Execute: func(ctx context.Context) (string, error) {
				panic("intentional panic in task")
			},
		},
		{
			ID: "success-2",
			Execute: func(ctx context.Context) (string, error) {
				return "ok2", nil
			},
		},
	}

	// This should NOT panic, but all tasks should complete (including panicking one as error)
	results := pool.Run(context.Background(), tasks)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	resultMap := MapResults(results)

	// Check that successful tasks completed
	if r, ok := resultMap["success-1"]; !ok {
		t.Error("missing result for success-1")
	} else if r.Err != nil {
		t.Errorf("success-1 should not have error, got: %v", r.Err)
	} else if r.Value != "ok" {
		t.Errorf("success-1 expected 'ok', got '%s'", r.Value)
	}

	if r, ok := resultMap["success-2"]; !ok {
		t.Error("missing result for success-2")
	} else if r.Err != nil {
		t.Errorf("success-2 should not have error, got: %v", r.Err)
	} else if r.Value != "ok2" {
		t.Errorf("success-2 expected 'ok2', got '%s'", r.Value)
	}

	// Check that panicking task was captured as an error
	if r, ok := resultMap["panic-1"]; !ok {
		t.Error("missing result for panic-1")
	} else if r.Err == nil {
		t.Error("panic-1 should have an error from recovered panic")
	} else {
		// Verify the error message contains panic info
		errMsg := r.Err.Error()
		if !strings.Contains(errMsg, "task panic") {
			t.Errorf("error should contain 'task panic', got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "intentional panic in task") {
			t.Errorf("error should contain original panic message, got: %s", errMsg)
		}
	}
}

func BenchmarkPool_Run(b *testing.B) {
	pool := New[int](4)

	tasks := make([]Task[int], 100)
	for i := 0; i < 100; i++ {
		i := i
		tasks[i] = Task[int]{
			ID: fmt.Sprintf("task-%d", i),
			Execute: func(ctx context.Context) (int, error) {
				// Simulate some work
				result := 0
				for j := 0; j < 1000; j++ {
					result += j
				}
				return result, nil
			},
		}
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		pool.Run(context.Background(), tasks)
	}
}
