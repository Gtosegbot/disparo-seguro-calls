package instance_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"wacalls/internal/platform/instance"
)

// Teste de criação e isolamento de Tenant com token
func TestManager_CreateAndGet_TenantIsolation(t *testing.T) {
	db := newMockDB()
	defer db.Close()
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
	defer db.Close()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	// Cria primeira instância
	inst1, err := mgr.Create(ctx, "tenant-A", "Line A")
	if err != nil {
		t.Fatalf("failed to create inst1: %v", err)
	}

	// Tenta criar segunda instância com a mesma sessão Whatsmeow (violação unique constraint)
	sc.fixedID = inst1.SessionID
	_, err = mgr.Create(ctx, "tenant-A", "Line B")
	if err != instance.ErrDuplicate {
		t.Errorf("expected ErrDuplicate, got %v", err)
	}
}

// Teste de ciclo de vida, capacity e transições para BUSY
func TestManager_LifecycleAndCapacity(t *testing.T) {
	db := newMockDB()
	defer db.Close()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	inst, err := mgr.Create(ctx, "tenant-A", "Line A")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if inst.Status != instance.StatePairing {
		t.Errorf("expected initial state PAIRING, got %s", inst.Status)
	}

	// Mudar para CONNECTED
	_ = mgr.UpdateStatus(ctx, inst.SessionID, instance.StateConnected, "5511999999999")

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
	defer db.Close()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	// 1. Criar instância e salvar estado CONNECTED
	inst, _ := mgr.Create(ctx, "tenant-A", "Line A")
	_ = mgr.UpdateStatus(ctx, inst.SessionID, instance.StateConnected, "5511999999999")

	// 2. Simular reinício do processo.
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

// Teste de vinculação/desvinculação de ChatSeguro por linha
func TestManager_ChatSeguroIntegration(t *testing.T) {
	db := newMockDB()
	defer db.Close()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	inst, _ := mgr.Create(ctx, "tenant-A", "Line A")

	// 1. Vincular ChatSeguro
	config := `{"url":"http://chatwoot","account_id":1,"account_token":"tok","inbox_id":123}`
	err := mgr.SetChatSeguro(ctx, "tenant-A", inst.ID, 123, config)
	if err != nil {
		t.Fatalf("failed to set chatseguro: %v", err)
	}

	if sc.chatwootConfig[inst.SessionID] != config {
		t.Errorf("expected chatwoot config to be set, got %s", sc.chatwootConfig[inst.SessionID])
	}

	// 2. Tentar vincular com Tenant B (deve dar erro ErrForbidden)
	err = mgr.SetChatSeguro(ctx, "tenant-B", inst.ID, 123, config)
	if err != instance.ErrForbidden {
		t.Errorf("expected ErrForbidden for tenant-B, got %v", err)
	}

	// 3. Remover ChatSeguro
	err = mgr.DeleteChatSeguro(ctx, "tenant-A", inst.ID)
	if err != nil {
		t.Fatalf("failed to delete chatseguro: %v", err)
	}

	if sc.chatwootConfig[inst.SessionID] != "" {
		t.Errorf("expected chatwoot config to be cleared, got %s", sc.chatwootConfig[inst.SessionID])
	}
}

// Teste de deleção e limpeza de recursos
func TestManager_DeleteClean(t *testing.T) {
	db := newMockDB()
	defer db.Close()
	sc := newMockSessionController()
	mgr := instance.NewManager(db, sc)
	ctx := context.Background()

	inst, _ := mgr.Create(ctx, "tenant-A", "Line A")

	err := mgr.Delete(ctx, "tenant-A", inst.ID)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if !sc.deleted[inst.SessionID] {
		t.Error("expected whatsmeow session to be deleted")
	}
}

// ─── Mocks & Helpers ──────────────────────────────────────────────────────────

func newMockDB() *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS instances (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		session_id TEXT NOT NULL UNIQUE,
		phone TEXT,
		display_name TEXT,
		status TEXT,
		proxy_id TEXT,
		chatseguro_inbox_id INTEGER,
		max_concurrent_calls INTEGER,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		metadata BLOB
	);
	CREATE INDEX IF NOT EXISTS idx_instances_tenant ON instances(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_instances_session ON instances(session_id);`)
	if err != nil {
		panic(err)
	}
	return db
}

type mockSessionController struct {
	fixedID        string
	sessions       map[string]string
	activeCalls    map[string]int
	deleted        map[string]bool
	loggedOut      map[string]bool
	chatwootConfig map[string]string
}

func newMockSessionController() *mockSessionController {
	return &mockSessionController{
		sessions:       make(map[string]string),
		activeCalls:    make(map[string]int),
		deleted:        make(map[string]bool),
		loggedOut:      make(map[string]bool),
		chatwootConfig: make(map[string]string),
	}
}

func (m *mockSessionController) CreateSession(name string) (string, error) {
	if m.fixedID != "" {
		return m.fixedID, nil
	}
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

func (m *mockSessionController) SetChatwootConfig(ctx context.Context, sessionID string, configJSON string) error {
	m.chatwootConfig[sessionID] = configJSON
	return nil
}

func (m *mockSessionController) DeleteChatwootConfig(ctx context.Context, sessionID string) error {
	m.chatwootConfig[sessionID] = ""
	return nil
}

func uuid() string {
	return "mock-session-uuid-" + time.Now().Format("20060102150405.000000000")
}
