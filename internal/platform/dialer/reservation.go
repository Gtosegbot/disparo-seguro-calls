package dialer

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrAlreadyReserved = errors.New("job already reserved")
)

// ReservationEngine manages distributed lock/lease state for queue jobs.
type ReservationEngine interface {
	Reserve(ctx context.Context, jobID string, leaseSeconds int) error
	Release(ctx context.Context, jobID string) error
	Commit(ctx context.Context, jobID string) error
}

// InMemReservationEngine implements ReservationEngine locally using memory locks.
type InMemReservationEngine struct {
	mu    sync.Mutex
	locks map[string]bool
}

// NewInMemReservationEngine creates an in-memory reservation engine.
func NewInMemReservationEngine() *InMemReservationEngine {
	return &InMemReservationEngine{locks: make(map[string]bool)}
}

func (r *InMemReservationEngine) Reserve(ctx context.Context, jobID string, _ int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locks[jobID] {
		return ErrAlreadyReserved
	}
	r.locks[jobID] = true
	return nil
}

func (r *InMemReservationEngine) Release(ctx context.Context, jobID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.locks, jobID)
	return nil
}

func (r *InMemReservationEngine) Commit(ctx context.Context, jobID string) error {
	// For in-memory, committing simply means releasing the lease lock since job status in DB moves away from QUEUED
	return r.Release(ctx, jobID)
}
