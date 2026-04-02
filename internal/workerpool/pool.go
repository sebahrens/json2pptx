// Package workerpool provides a bounded worker pool for concurrent task execution.
// It supports context-aware cancellation and graceful shutdown.
package workerpool

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
)

// Task represents a unit of work to be executed by the pool.
type Task[T any] struct {
	// ID is a unique identifier for the task (e.g., slide number, chart index).
	ID string

	// Execute performs the task and returns a result or error.
	// The function should respect context cancellation.
	Execute func(ctx context.Context) (T, error)
}

// Result contains the outcome of a task execution.
type Result[T any] struct {
	// ID matches the Task.ID that produced this result.
	ID string

	// Value is the result if execution succeeded.
	Value T

	// Err is non-nil if execution failed.
	Err error
}

// Pool manages a bounded set of worker goroutines.
type Pool[T any] struct {
	workers int
	tasks   chan Task[T]
	results chan Result[T]
	wg      sync.WaitGroup
}

// New creates a new worker pool with the specified number of workers.
// If workers <= 0, defaults to runtime.NumCPU().
func New[T any](workers int) *Pool[T] {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	return &Pool[T]{
		workers: workers,
	}
}

// Run executes all tasks concurrently and returns results.
// It blocks until all tasks complete or the context is cancelled.
//
// The function:
//  1. Starts worker goroutines up to the pool's worker limit
//  2. Submits all tasks to the workers
//  3. Collects results as they complete
//  4. Returns all results (including errors) in a slice
//
// Task order is not preserved in the results; use Result.ID to match results to tasks.
// On context cancellation, remaining tasks may not execute.
func (p *Pool[T]) Run(ctx context.Context, tasks []Task[T]) []Result[T] {
	if len(tasks) == 0 {
		return nil
	}

	// Size channels appropriately
	p.tasks = make(chan Task[T], len(tasks))
	p.results = make(chan Result[T], len(tasks))

	// Start workers
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx)
	}

	// Submit all tasks
	for _, task := range tasks {
		p.tasks <- task
	}
	close(p.tasks)

	// Wait for all workers to complete
	p.wg.Wait()
	close(p.results)

	// Collect results
	results := make([]Result[T], 0, len(tasks))
	for result := range p.results {
		results = append(results, result)
	}

	return results
}

// worker processes tasks from the task channel.
func (p *Pool[T]) worker(ctx context.Context) {
	defer p.wg.Done()

	for task := range p.tasks {
		// Check for cancellation before executing
		select {
		case <-ctx.Done():
			p.results <- Result[T]{
				ID:  task.ID,
				Err: ctx.Err(),
			}
			continue
		default:
		}

		// Execute the task with panic recovery
		p.executeWithRecovery(ctx, task)
	}
}

// executeWithRecovery runs a task and converts any panic into an error result.
// This prevents a single panicking task from crashing the entire worker pool.
func (p *Pool[T]) executeWithRecovery(ctx context.Context, task Task[T]) {
	defer func() {
		if r := recover(); r != nil {
			// Convert panic to error with stack trace for debugging
			stack := debug.Stack()
			err := fmt.Errorf("task panic: %v\nstack: %s", r, string(stack))
			p.results <- Result[T]{
				ID:  task.ID,
				Err: err,
			}
		}
	}()

	// Execute the task
	value, err := task.Execute(ctx)
	p.results <- Result[T]{
		ID:    task.ID,
		Value: value,
		Err:   err,
	}
}

// MapResults converts a slice of results into a map keyed by ID.
// This is useful for looking up results by their task ID.
func MapResults[T any](results []Result[T]) map[string]Result[T] {
	m := make(map[string]Result[T], len(results))
	for _, r := range results {
		m[r.ID] = r
	}
	return m
}
