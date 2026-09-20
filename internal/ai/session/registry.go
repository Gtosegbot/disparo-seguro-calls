// Package session - registry stores all active AISessions, keyed by id.
// All lookups enforce tenant isolation.
package session

import (
	"errors"
	"sync"
)

// ErrNotFound is returned when the requested session does not exist.
var ErrNotFound = errors.New("ai session not found")

// ErrForbidden is returned when a tenant tries to access another tenant's session.
var ErrForbidden = errors.New("forbidden: tenant mismatch")

// Registry is a thread-safe in-process store of active AISessions.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*AISession
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{sessions: make(map[string]*AISession)}
}

// Register adds a session to the registry.
func (r *Registry) Register(s *AISession) {
	r.mu.Lock()
	r.sessions[s.ID] = s
	r.mu.Unlock()
}

// Get returns the session for (id, tenantID), enforcing tenant isolation.
func (r *Registry) Get(id, tenantID string) (*AISession, error) {
	r.mu.RLock()
	s, ok := r.sessions[id]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	if s.TenantID != tenantID {
		return nil, ErrForbidden
	}
	return s, nil
}

// Remove deletes a session from the registry.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	delete(r.sessions, id)
	r.mu.Unlock()
}

// ListByTenant returns snapshots of all sessions owned by tenantID.
func (r *Registry) ListByTenant(tenantID string) []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []map[string]any
	for _, s := range r.sessions {
		if s.TenantID == tenantID {
			out = append(out, s.Snapshot())
		}
	}
	return out
}

// ReapStaleSessions scans for active sessions exceeding their max duration or timeout and transitions them to StateEnded.
// Prevents the system from getting stuck in CALLING, PROCESSING, or AUDIO_STREAMING when the peer vanishes.
func (r *Registry) ReapStaleSessions(defaultMaxAge time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	reaped := 0

	for _, s := range r.sessions {
		st := s.State()
		if st == StateEnded || st == StateError {
			continue
		}

		limit := defaultMaxAge
		if s.Profile.MaxDuration > 0 {
			limit = s.Profile.MaxDuration
		}

		refTime := s.CreatedAt
		if s.StartedAt != nil {
			refTime = *s.StartedAt
		}

		if now.Sub(refTime) > limit {
			s.SetState(StateEnded)
			s.MarkEnded(&Outcome{
				Reason:   "watchdog_timeout_recovery",
				Duration: now.Sub(refTime),
				Metadata: map[string]any{
					"timeout_limit_seconds": limit.Seconds(),
					"recovered_at":           now,
				},
			})
			reaped++
		}
	}

	return reaped
}
