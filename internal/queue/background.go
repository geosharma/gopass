// Package queue implements an experimental background queue for cleanup jobs.
// Beware: It's likely broken.
// We can easily close a channel which might later be written to.
// The current locking is but a poor workaround.
// A better implementation would create a queue object in main, pass
// it through and wait for the channel to be empty before leaving main.
// Will do that later.
package queue

import (
	"context"

	"github.com/gopasspw/gopass/internal/debug"
)

type contextKey int

const (
	ctxKeyQueue contextKey = iota
)

// Queuer is a queue interface
type Queuer interface {
	Add(Task) Task
	Wait(context.Context) error
}

// WithQueue adds the given queue to the context
func WithQueue(ctx context.Context, q *Queue) context.Context {
	return context.WithValue(ctx, ctxKeyQueue, q)
}

// GetQueue returns an existing queue from the context or
// returns a noop one.
func GetQueue(ctx context.Context) Queuer {
	if q, ok := ctx.Value(ctxKeyQueue).(*Queue); ok {
		return q
	}
	return &noop{}
}

type noop struct{}

// Add always returns the task
func (n *noop) Add(t Task) Task {
	return t
}

// Wait always returns nil
func (n *noop) Wait(_ context.Context) error {
	return nil
}

// Task is a background task
type Task func(ctx context.Context) error

// Queue is a serialized background processing unit
type Queue struct {
	work chan Task
	done chan struct{}
}

// New creates a new queue
func New(ctx context.Context) *Queue {
	q := &Queue{
		work: make(chan Task, 1024),
		done: make(chan struct{}, 1),
	}
	go q.run(ctx)
	return q
}

func (q *Queue) run(ctx context.Context) {
	for t := range q.work {
		if err := t(ctx); err != nil {
			debug.Log("Task failed: %s", err)
		}
		debug.Log("Task done")
	}
	debug.Log("all tasks done")
	q.done <- struct{}{}
}

// Add enqueues a new task
func (q *Queue) Add(t Task) Task {
	q.work <- t
	debug.Log("enqueued task")
	return func(_ context.Context) error { return nil }
}

// Wait waits for all tasks to be processed
func (q *Queue) Wait(ctx context.Context) error {
	close(q.work)
	select {
	case <-q.done:
		return nil
	case <-ctx.Done():
		debug.Log("context canceled")
		return ctx.Err()
	}
}