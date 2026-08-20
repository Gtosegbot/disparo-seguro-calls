package dialer_test

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	"wacalls/internal/platform/dialer"
)

func TestOrchestration_JobStateMachine(t *testing.T) {
	sm := dialer.NewJobStateMachine()

	// 1. Cria job
	j := sm.CreateJob("job-1", "tenant-A", "idem-1")
	if j.State != dialer.StateCreated {
		t.Errorf("expected state CREATED, got %s", j.State)
	}

	// 2. Transição válida: CREATED -> QUEUED
	err := sm.Transition("job-1", dialer.StateQueued)
	if err != nil {
		t.Fatalf("valid transition failed: %v", err)
	}

	// 3. Transição inválida: QUEUED -> COMPLETED (deve pular para RUNNING antes)
	err = sm.Transition("job-1", dialer.StateCompleted)
	if !errors.Is(err, dialer.ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}

	// 4. Transição válida: QUEUED -> RUNNING -> PAUSED -> RUNNING -> COMPLETED
	_ = sm.Transition("job-1", dialer.StateRunning)
	_ = sm.Transition("job-1", dialer.StatePaused)
	_ = sm.Transition("job-1", dialer.StateRunning)
	err = sm.Transition("job-1", dialer.StateCompleted)
	if err != nil {
		t.Fatalf("sequence of valid transitions failed: %v", err)
	}
}

func TestOrchestration_IdempotencyRegistry(t *testing.T) {
	ir := dialer.NewIdempotencyRegistry()

	// 1. Primeira requisição com idempotency_key
	jobID, err := ir.CheckOrRegister("key-unique-123", "job-999")
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	if jobID != "job-999" {
		t.Errorf("expected returned jobID job-999, got %s", jobID)
	}

	// 2. Requisição repetida com a mesma chave
	dupID, err := ir.CheckOrRegister("key-unique-123", "job-888")
	if !errors.Is(err, dialer.ErrIdempotencyBlock) {
		t.Errorf("expected ErrIdempotencyBlock, got %v", err)
	}
	if dupID != "job-999" {
		t.Errorf("expected original jobID job-999, got %s", dupID)
	}
}

func TestOrchestration_CircuitBreaker(t *testing.T) {
	cb := dialer.NewCircuitBreaker(3, 1) // threshold = 3, cooldown = 1s

	// 1. Circuito fechado (CLOSED) - Tudo OK
	err := cb.Check("grok_realtime")
	if err != nil {
		t.Errorf("expected closed circuit to allow request, got: %v", err)
	}

	// 2. Simula 3 falhas consecutivas
	cb.RecordFailure("grok_realtime")
	cb.RecordFailure("grok_realtime")
	cb.RecordFailure("grok_realtime")

	// 3. Circuito deve abrir (OPEN)
	err = cb.Check("grok_realtime")
	if !errors.Is(err, dialer.ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}

	// 4. Aguarda cooldown (1.1s)
	time.Sleep(1100 * time.Millisecond)

	// 5. Deve passar para HALF-OPEN (Check retorna nil para testar recuperação)
	err = cb.Check("grok_realtime")
	if err != nil {
		t.Errorf("expected half-open circuit to allow recovery test request, got: %v", err)
	}

	// 6. Sucesso no teste de recuperação -> Fecha o circuito
	cb.RecordSuccess("grok_realtime")
	err = cb.Check("grok_realtime")
	if err != nil {
		t.Errorf("expected recovered closed circuit to allow request, got: %v", err)
	}
}

func TestOrchestration_AtomicLeadClaimingScale(t *testing.T) {
	// Simula enfileiramento e claim atômico concorrente de leads por workers
	engine := dialer.NewInMemReservationEngine()
	queue := dialer.NewQueue(engine)
	ctx := context.Background()

	campaignID := "camp-X"
	queue.SaveCampaign(&dialer.DialerCampaign{
		ID:           campaignID,
		MaxAttempts:  3,
		RetryDelaySeconds: 5,
	})

	// Enfileira 100 leads
	var jobs []*dialer.DialerJob
	for i := 1; i <= 100; i++ {
		jobs = append(jobs, &dialer.DialerJob{
			ID:         fmt.Sprintf("lead-%d", i),
			CampaignID: campaignID,
			Status:     dialer.JobQueued,
			Position:   i,
		})
	}
	queue.Enqueue(jobs)

	// Simula 500 workers concorrentes disputando os 100 leads
	var wg sync.WaitGroup
	wg.Add(500)

	claimedMap := make(map[string]int)
	var mapMu sync.Mutex

	for i := 0; i < 500; i++ {
		go func() {
			defer wg.Done()
			
			// Pequeno jitter aleatório de início
			time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)

			job, err := queue.NextJob(ctx, campaignID)
			if err == nil && job != nil {
				mapMu.Lock()
				claimedMap[job.ID]++
				mapMu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Assegura que nenhum lead foi claimado mais de uma vez
	for id, count := range claimedMap {
		if count > 1 {
			t.Errorf("lead %s claimed %d times (expected max 1)", id, count)
		}
	}
}

func TestOrchestration_MultiTenantEnforcement(t *testing.T) {
	sm := dialer.NewJobStateMachine()

	// Cria jobs em tenants separados
	_ = sm.CreateJob("job-A", "tenant-A", "idem-A")
	_ = sm.CreateJob("job-B", "tenant-B", "idem-B")

	// Simula validação de tenant
	validateTenant := func(jobID, requesterTenant string) error {
		j, ok := sm.GetJob(jobID)
		if !ok {
			return errors.New("not found")
		}
		if j.TenantID != requesterTenant {
			return errors.New("unauthorized: cross tenant access blocked")
		}
		return nil
	}

	// 1. Acesso legítimo
	err := validateTenant("job-A", "tenant-A")
	if err != nil {
		t.Errorf("authorized access failed: %v", err)
	}

	// 2. Acesso cruzado ilícito (forged tenant)
	err = validateTenant("job-A", "tenant-B")
	if err == nil {
		t.Error("expected error for cross tenant access, got nil")
	}
}

// Mock de Provider para teste
type mockRealtimeProvider struct {
	name string
}

func (m *mockRealtimeProvider) Name() string { return m.name }
func (m *mockRealtimeProvider) Connect(ctx context.Context) error { return nil }
func (m *mockRealtimeProvider) SendAudio(ctx context.Context, pcm []byte) error { return nil }
func (m *mockRealtimeProvider) ReceiveAudio(ctx context.Context) ([]byte, error) { return nil }
func (m *mockRealtimeProvider) Close() error { return nil }

type nullWriter struct{}
func (nw *nullWriter) Write(p []byte) (n int, err error) { return len(p), nil }
