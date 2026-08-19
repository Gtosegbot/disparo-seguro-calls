package instance_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"wacalls/internal/platform/instance"
)

// Teste de criação e isolamento de Tenant
func TestManager_CreateAndGet_TenantIsolation(t *testing.T) {
	db := newMockDB()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	// 1. Criar instância para Tenant A
	inst, err := mgr.Create(ctx, "tenant-A", "Line A")
	if err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	if inst.TenantID != "tenant-A" {
		t.Errorf("expected tenant-A, got %s", inst.TenantID)
	}

	// Salva no banco mockado para leitura subsequente
	db.store[inst.ID] = inst

	// 2. Leitura com o mesmo Tenant: OK
	got, err := mgr.Get(ctx, "tenant-A", inst.ID)
	if err != nil {
		t.Errorf("failed to get instance: %v", err)
	}
	if got.ID != inst.ID {
		t.Errorf("id mismatch: %s vs %s", got.ID, inst.ID)
	}

	// 3. Leitura com Tenant diferente: Erro de Permissão (Forbidden)
	_, err = mgr.Get(ctx, "tenant-B", inst.ID)
	if err != instance.ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// Teste de ciclo de vida e concorrência máxima (capacity)
func TestManager_LifecycleAndCapacity(t *testing.T) {
	db := newMockDB()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	inst, _ := mgr.Create(ctx, "tenant-A", "Line A")
	db.store[inst.ID] = inst

	// Teste de status inicial
	if inst.Status != instance.StatePairing {
		t.Errorf("expected initial state PAIRING, got %s", inst.Status)
	}

	// Mudar para CONNECTED
	_ = mgr.UpdateStatus(ctx, inst.SessionID, instance.StateConnected, "5511999999999")
	
	// Atualiza o estado simulando gravação do whatsmeow
	inst.Status = instance.StateConnected
	inst.Phone = "5511999999999"

	// Mocking active calls count to 3
	sc.activeCalls[inst.SessionID] = 3

	// O Get() deve transicionar dinamicamente o status para BUSY se ActiveCalls > 0
	got, _ := mgr.Get(ctx, "tenant-A", inst.ID)
	if got.Status != instance.StateBusy {
		t.Errorf("expected dynamic state BUSY, got %s", got.Status)
	}
	if got.ActiveCalls != 3 {
		t.Errorf("expected 3 active calls, got %d", got.ActiveCalls)
	}
}

// Teste de deleção e limpeza de recursos
func TestManager_DeleteClean(t *testing.T) {
	db := newMockDB()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	inst, _ := mgr.Create(ctx, "tenant-A", "Line A")
	db.store[inst.ID] = inst

	err := mgr.Delete(ctx, "tenant-A", inst.ID)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if !sc.deleted[inst.SessionID] {
		t.Error("expected whatsmeow session to be deleted")
	}
}

// ─── Mocks ───────────────────────────────────────────────────────────────────

type mockDB struct {
	store map[string]*instance.Instance
}

func newMockDB() *mockDB {
	return &mockDB{store: make(map[string]*instance.Instance)}
}

func (m *mockDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}

func (m *mockDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	// Dummy sql.Row return. Para testes mais precisos, usamos o Get do Manager direto
	// simulando injeção no store.
	return nil
}

// QueryRow e QueryContext mockados para simulação
func (m *mockDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

type mockSessionController struct {
	sessions    map[string]string
	activeCalls map[string]int
	deleted     map[string]bool
	loggedOut   map[string]bool
}

func newMockSessionController() *mockSessionController {
	return &mockSessionController{
		sessions:    make(map[string]string),
		activeCalls: make(map[string]int),
		deleted:     make(map[string]bool),
		loggedOut:   make(map[string]bool),
	}
}

func (m *mockSessionController) CreateSession(name string) (string, error) {
	id := uuid()
	m.sessions[id] = name
	return id, nil
}

func (m *mockSessionController) DeleteSession(ctx context.Context, id string) error {
	m.deleted[id] = true
	return nil
}

func (m *mockSessionController) LogoutSession(ctx context.Context, id string) error {
	m.loggedOut[id] = true
	return nil
}

func (m *mockSessionController) GetActiveCallsCount(id string) int {
	return m.activeCalls[id]
}

// scanInstance mock override for mockDB query row
func (m *mockDB) GetInstanceMock(id string) (*instance.Instance, error) {
	inst, ok := m.store[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return inst, nil
}

func uuid() string {
	// Simple mock UUID generator
	return "mock-session-uuid-" + time.Now().String()
}
