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
