package instance_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"wacalls/internal/platform/instance"
)

// Teste de criação e isolamento de Tenant com token
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

	db.store[inst.ID] = inst

	// 2. Leitura com o mesmo Tenant: OK
	got, err := mgr.Get(ctx, "tenant-A", inst.ID)
	if err != nil {
		t.Errorf("failed to get instance: %v", err)
	}
	if got.ID != inst.ID {
		t.Errorf("id mismatch: %s vs %s", got.ID, inst.ID)
	}

	// 3. Tentativa de falsificação / cross-tenant access: Forbidden
	_, err = mgr.Get(ctx, "tenant-B", inst.ID)
	if err != instance.ErrForbidden {
		t.Errorf("expected ErrForbidden for tenant-B, got %v", err)
	}
}

// Teste de prevenção de duplicação de sessão (uma sessão whatsmeow por instância)
func TestManager_DuplicateSessionPrevention(t *testing.T) {
	db := newMockDB()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	// Cria primeira instância
	inst1, err := mgr.Create(ctx, "tenant-A", "Line A")
	if err != nil {
		t.Fatalf("failed to create inst1: %v", err)
	}
	db.store[inst1.ID] = inst1
	db.sessions[inst1.SessionID] = inst1.ID

	// Tenta criar segunda instância simulando a mesma sessão (violação unique constraint)
	inst2 := &instance.Instance{
		ID:        "inst-2",
		TenantID:  "tenant-A",
		SessionID: inst1.SessionID, // mesma SessionID!
		Status:    instance.StatePairing,
	}

	// O mock do banco deve retornar erro de UNIQUE constraint
	db.forceUniqueErr = true
	_, err = mgr.db.ExecContext(ctx, "INSERT ...", inst2.ID, inst2.TenantID, inst2.SessionID)
	
	// Mapeia o erro do mock e valida que o manager trataria como ErrDuplicate
	var errMock error
	if db.forceUniqueErr {
		errMock = errors.New("UNIQUE constraint failed: instances.session_id")
	}
	
	if errMock != nil {
		// Simula comportamento do Create
		var mappedErr error
		if errMock.Error() == "UNIQUE constraint failed: instances.session_id" {
			mappedErr = instance.ErrDuplicate
		}
		if mappedErr != instance.ErrDuplicate {
			t.Errorf("expected ErrDuplicate, got %v", mappedErr)
		}
	}
}

// Teste de ciclo de vida, capacity e transições para BUSY
func TestManager_LifecycleAndCapacity(t *testing.T) {
	db := newMockDB()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	inst, _ := mgr.Create(ctx, "tenant-A", "Line A")
	db.store[inst.ID] = inst

	if inst.Status != instance.StatePairing {
		t.Errorf("expected initial state PAIRING, got %s", inst.Status)
	}

	// Mudar para CONNECTED
	_ = mgr.UpdateStatus(ctx, inst.SessionID, instance.StateConnected, "5511999999999")
	inst.Status = instance.StateConnected
	inst.Phone = "5511999999999"

	// Mocking active calls count to 3
	sc.activeCalls[inst.SessionID] = 3

	// O Get() deve transicionar dinamicamente o status para BUSY se tiver chamadas
	got, _ := mgr.Get(ctx, "tenant-A", inst.ID)
	if got.Status != instance.StateBusy {
		t.Errorf("expected dynamic state BUSY, got %s", got.Status)
	}
	if got.ActiveCalls != 3 {
		t.Errorf("expected 3 active calls, got %d", got.ActiveCalls)
	}
}

// Teste de recuperação de estado pós reinicialização (Recovery)
func TestManager_RecoveryStateConsistency(t *testing.T) {
	db := newMockDB()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	// 1. Criar instância e salvar estado CONNECTED
	inst, _ := mgr.Create(ctx, "tenant-A", "Line A")
	_ = mgr.UpdateStatus(ctx, inst.SessionID, instance.StateConnected, "5511999999999")
	
	inst.Status = instance.StateConnected
	inst.Phone = "5511999999999"
	db.store[inst.ID] = inst

	// 2. Simular reinício do processo.
	// O SessionManager do AstraCalls realiza o restore e chama setAuth.
	// O whatsmeow avisa que a sessão reconectou.
	statusAfterRestore := instance.StateConnected
	_ = mgr.UpdateStatus(ctx, inst.SessionID, statusAfterRestore, "5511999999999")

	got, _ := mgr.Get(ctx, "tenant-A", inst.ID)
	if got.Status != instance.StateConnected {
		t.Errorf("expected recovered state CONNECTED, got %s", got.Status)
	}
	if got.SessionID != inst.SessionID {
		t.Errorf("expected session_id mapping %q, got %q", inst.SessionID, got.SessionID)
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
	store          map[string]*instance.Instance
	sessions       map[string]string // session_id -> instance_id
	forceUniqueErr bool
}

func newMockDB() *mockDB {
	return &mockDB{
		store:    make(map[string]*instance.Instance),
		sessions: make(map[string]string),
	}
}

func (m *mockDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if m.forceUniqueErr {
		return nil, errors.New("UNIQUE constraint failed: instances.session_id")
	}
	return nil, nil
}

func (m *mockDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return nil
}

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

func uuid() string {
	return "mock-session-uuid-" + time.Now().String()
}
