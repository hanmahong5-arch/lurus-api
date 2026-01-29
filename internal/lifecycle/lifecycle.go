// Package lifecycle provides utilities for graceful shutdown and
// lifecycle management of background tasks.
package lifecycle

import (
	"context"
	"sync"
	"time"
)

// Task represents a background task that can be run and stopped.
type Task interface {
	Run(ctx context.Context) error
	Name() string
}

// Manager manages the lifecycle of background tasks.
type Manager struct {
	mu       sync.Mutex
	tasks    []Task
	shutdown chan struct{}
	done     chan struct{}
}

// NewManager creates a new lifecycle manager.
func NewManager() *Manager {
	return &Manager{
		shutdown: make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Register adds a task to be managed.
func (m *Manager) Register(t Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks = append(m.tasks, t)
}

// Start begins all registered tasks.
func (m *Manager) Start(ctx context.Context) {
	var wg sync.WaitGroup

	for _, t := range m.tasks {
		wg.Add(1)
		go func(task Task) {
			defer wg.Done()
			_ = task.Run(ctx)
		}(t)
	}

	go func() {
		wg.Wait()
		close(m.done)
	}()
}

// Shutdown signals all tasks to stop and waits for completion.
func (m *Manager) Shutdown(timeout time.Duration) error {
	close(m.shutdown)

	select {
	case <-m.done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}

// Done returns a channel that is closed when all tasks have completed.
func (m *Manager) Done() <-chan struct{} {
	return m.done
}

// TickerTask is a helper for creating ticker-based background tasks.
type TickerTask struct {
	name     string
	interval time.Duration
	fn       func(ctx context.Context) error
}

// NewTickerTask creates a new ticker-based task.
func NewTickerTask(name string, interval time.Duration, fn func(ctx context.Context) error) *TickerTask {
	return &TickerTask{
		name:     name,
		interval: interval,
		fn:       fn,
	}
}

// Name returns the task name.
func (t *TickerTask) Name() string {
	return t.name
}

// Run executes the task on a ticker until context is cancelled.
func (t *TickerTask) Run(ctx context.Context) error {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := t.fn(ctx); err != nil {
				// Log error but continue
				continue
			}
		}
	}
}
