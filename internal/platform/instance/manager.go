package instance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrForbidden   = errors.New("forbidden: tenant access denied")
	ErrNotFound    = errors.New("instance not found")
	ErrDuplicate   = errors.New("instance already exists for this session")
)

// SessionController abstracts whatsmeow session controls to prevent circular package imports.
type SessionController interface {
	CreateSession(name string) (string, error)
	DeleteSession(ctx context.Context, id string) error
	LogoutSession(ctx context.Context, id string) error
	GetActiveCallsCount(id string) int
}

// Manager orchestrates tenant-isolated instances and QR/pairing state.
type Manager struct {
	db sqlDB
	sc SessionController
	mu sync.RWMutex
}

// sqlDB matches a subset of sql.DB methods.
type sqlDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// NewManager creates an Instance Manager.
func NewManager(db sqlDB, sc SessionController) *Manager {
	return &Manager{db: db, sc: sc}
}

// Create creates a brand new Instance linked to a Tenant and generates the backend session.
func (m *Manager) Create(ctx context.Context, tenantID, displayName string) (*Instance, error) {
	// 1. Generate AstraCalls Session
	sessionID, err := m.sc.CreateSession(displayName)
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()
	now := time.Now().UTC()
	inst := &Instance{
		ID:                 id,
		TenantID:           tenantID,
		SessionID:          sessionID,
		DisplayName:        displayName,
		Status:             StatePairing,
		MaxConcurrentCalls: 8, // Concorrência máxima default
		CreatedAt:          now,
		UpdatedAt:          now,
		Metadata:           make(map[string]any),
	}

	metaJSON, _ := json.Marshal(inst.Metadata)

	// 2. Persist in DB
	query := `INSERT INTO instances (id, tenant_id, session_id, phone, display_name, status, proxy_id, chatseguro_inbox_id, max_concurrent_calls, created_at, updated_at, metadata)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	_, err = m.db.ExecContext(ctx, query,
		inst.ID, inst.TenantID, inst.SessionID, inst.Phone, inst.DisplayName,
		string(inst.Status), inst.ProxyID, inst.ChatSeguroInboxID, inst.MaxConcurrentCalls,
		inst.CreatedAt, inst.UpdatedAt, metaJSON)

	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "unique") || strings.Contains(errStr, "23505") || strings.Contains(errStr, "UNIQUE") {
			return nil, ErrDuplicate
		}
		return nil, err
	}

	return inst, nil
}

// Get returns the instance, verifying tenant ownership.
func (m *Manager) Get(ctx context.Context, tenantID, id string) (*Instance, error) {
	query := `SELECT id, tenant_id, session_id, phone, display_name, status, proxy_id, chatseguro_inbox_id, max_concurrent_calls, created_at, updated_at, metadata
	          FROM instances WHERE id = $1`
	
	row := m.db.QueryRowContext(ctx, query, id)
	inst, err := m.scanInstance(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	if inst.TenantID != tenantID {
		return nil, ErrForbidden
	}

	// Update active calls dynamic count from AstraCalls registry
	inst.ActiveCalls = m.sc.GetActiveCallsCount(inst.SessionID)
	if inst.ActiveCalls > 0 && inst.Status == StateConnected {
		inst.Status = StateBusy
	}

	return inst, nil
}

// List returns all instances owned by the tenant.
func (m *Manager) List(ctx context.Context, tenantID string) ([]*Instance, error) {
	query := `SELECT id, tenant_id, session_id, phone, display_name, status, proxy_id, chatseguro_inbox_id, max_concurrent_calls, created_at, updated_at, metadata
	          FROM instances WHERE tenant_id = $1 ORDER BY created_at DESC`
	
	rows, err := m.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Instance
	for rows.Next() {
		inst, scanErr := m.scanInstance(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		inst.ActiveCalls = m.sc.GetActiveCallsCount(inst.SessionID)
		if inst.ActiveCalls > 0 && inst.Status == StateConnected {
			inst.Status = StateBusy
		}
		out = append(out, inst)
	}

	return out, nil
}

// UpdateStatus changes the state of an instance.
func (m *Manager) UpdateStatus(ctx context.Context, sessionID string, status InstanceState, phone string) error {
	query := `UPDATE instances SET status = $1, phone = $2, updated_at = $3 WHERE session_id = $4`
	_, err := m.db.ExecContext(ctx, query, string(status), phone, time.Now().UTC(), sessionID)
	return err
}

// Logout terminates the whatsmeow connection but preserves configuration.
func (m *Manager) Logout(ctx context.Context, tenantID, id string) error {
	inst, err := m.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}

	// Terminate active whatsmeow connection
	if err := m.sc.LogoutSession(ctx, inst.SessionID); err != nil {
		return err
	}

	// Set status to OFFLINE
	return m.UpdateStatus(ctx, inst.SessionID, StateOffline, inst.Phone)
}

// Delete removes the instance, kills the whatsmeow session, and cleans resources.
func (m *Manager) Delete(ctx context.Context, tenantID, id string) error {
	inst, err := m.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}

	// 1. Delete whatsmeow session and DB
	if err := m.sc.DeleteSession(ctx, inst.SessionID); err != nil {
		return err
	}

	// 2. Remove from instances table
	query := `DELETE FROM instances WHERE id = $1`
	_, err = m.db.ExecContext(ctx, query, id)
	return err
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

type scannable interface {
	Scan(dest ...any) error
}

func (m *Manager) scanInstance(row scannable) (*Instance, error) {
	var (
		inst     Instance
		status   string
		metaBytes []byte
	)
	err := row.Scan(
		&inst.ID, &inst.TenantID, &inst.SessionID, &inst.Phone, &inst.DisplayName,
		&status, &inst.ProxyID, &inst.ChatSeguroInboxID, &inst.MaxConcurrentCalls,
		&inst.CreatedAt, &inst.UpdatedAt, &metaBytes,
	)
	if err != nil {
		return nil, err
	}
	inst.Status = InstanceState(status)
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &inst.Metadata)
	}
	return &inst, nil
}
