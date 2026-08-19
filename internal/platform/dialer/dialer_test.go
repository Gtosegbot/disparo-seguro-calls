package dialer_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"wacalls/internal/platform/dialer"
)

// MockInstanceProvider implements dialer.InstanceProvider
type mockInstanceProvider struct {
	mu        sync.Mutex
	instances map[string]*dialer.InstanceInfo
	dials     map[string]int
}

func newMockInstanceProvider() *mockInstanceProvider {
	return &mockInstanceProvider{
		instances: make(map[string]*dialer.InstanceInfo),
		dials:     make(map[string]int),
	}
}

func (m *mockInstanceProvider) GetInstancesPool(ctx context.Context, ids []string) ([]dialer.InstanceInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []dialer.InstanceInfo
	for _, id := range ids {
		if inst, ok := m.instances[id]; ok {
			out = append(out, *inst)
		}
	}
	return out, nil
}

func (m *mockInstanceProvider) StartOutgoingCall(ctx context.Context, instanceID, phone string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dials[instanceID]++
	if inst, ok := m.instances[instanceID]; ok {
		inst.ActiveCalls++
	}

	return fmt.Sprintf("mock-call-id-%s-%s-%d", instanceID, phone, time.Now().UnixNano()), nil
}

func TestDialer_SchedulerAndQueueFlow(t *testing.T) {
	resEngine := dialer.NewInMemReservationEngine()
	queue := dialer.NewQueue(resEngine)
	ip := newMockInstanceProvider()
	
	logger := slog.New(slog.NewTextHandler(&nullWriter{}, nil))
	scheduler := dialer.NewScheduler(queue, ip, logger)

	// 1. Criar e enfileirar Campanha
	camp := &dialer.DialerCampaign{
		ID:                  "camp-123",
		TenantID:            "tenant-A",
		Name:                "Campaign 1",
		Mode:                dialer.ModeH2H,
		Status:              dialer.StatusDraft,
		MaxConcurrentCalls:  12, // limite global
		DialIntervalSeconds: 1,  // intervalo baixo para acelerar testes
		MaxAttempts:         3,
		RetryDelaySeconds:   2,
		Strategy:            "round-robin",
		InstancePool:        []string{"inst-1", "inst-2", "inst-3"}, // 3 instâncias
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	queue.SaveCampaign(camp)

	// Configurar estado das instâncias no mock
	// inst-1: CONNECTED e saudável (pode discar)
	ip.instances["inst-1"] = &dialer.InstanceInfo{
		ID:                 "inst-1",
		Status:             "CONNECTED",
		ActiveCalls:        0,
		MaxConcurrentCalls: 4,
	}
	// inst-2: CONNECTED e saudável
	ip.instances["inst-2"] = &dialer.InstanceInfo{
		ID:                 "inst-2",
		Status:             "CONNECTED",
		ActiveCalls:        0,
		MaxConcurrentCalls: 4,
	}
	// inst-3: OFFLINE (deve ser ignorada pelo Round-Robin)
	ip.instances["inst-3"] = &dialer.InstanceInfo{
		ID:                 "inst-3",
		Status:             "OFFLINE",
		ActiveCalls:        0,
		MaxConcurrentCalls: 4,
	}

	// Enfileirar 5 leads (FIFO)
	var jobs []*dialer.DialerJob
	for i := 1; i <= 5; i++ {
		jobs = append(jobs, &dialer.DialerJob{
			ID:            fmt.Sprintf("job-%d", i),
			CampaignID:    camp.ID,
			TenantID:      camp.TenantID,
			LeadID:        fmt.Sprintf("lead-%d", i),
			Phone:         fmt.Sprintf("55119999000%d", i),
			Name:          fmt.Sprintf("Lead %d", i),
			Position:      i,
			Status:        dialer.JobQueued,
			Attempt:       1,
			NextAttemptAt: time.Now().Add(-1 * time.Minute),
			CreatedAt:     time.Now(),
		})
	}
	queue.Enqueue(jobs)

	// 2. Iniciar campanha no Scheduler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler.StartCampaign(ctx, camp.ID)

	// Aguarda processamento de alguns ciclos do scheduler
	time.Sleep(1500 * time.Millisecond)

	// Parar scheduler
	scheduler.StopCampaign(camp.ID)

	// 3. Validações
	// Deve ter processado ligações através de inst-1 e inst-2 (RR alternado), e ignorado inst-3
	ip.mu.Lock()
	dialsInst1 := ip.dials["inst-1"]
	dialsInst2 := ip.dials["inst-2"]
	dialsInst3 := ip.dials["inst-3"]
	ip.mu.Unlock()

	if dialsInst3 > 0 {
		t.Errorf("expected inst-3 (OFFLINE) to have 0 dials, got %d", dialsInst3)
	}

	if dialsInst1 == 0 && dialsInst2 == 0 {
		t.Errorf("expected healthy instances (inst-1/inst-2) to have processed dials, got 0")
	}

	// Valida concorrência respeitada
	totalActiveCalls := ip.instances["inst-1"].ActiveCalls + ip.instances["inst-2"].ActiveCalls
	if totalActiveCalls > 12 {
		t.Errorf("concurrency limit exceeded: expected <= 12, got %d", totalActiveCalls)
	}

	// Verificar se os jobs processados transicionaram para RINGING/DIALING/CONNECTED
	campaignJobs := queue.GetJobsForCampaign(camp.ID)
	processedCount := 0
	for _, j := range campaignJobs {
		if j.Status == dialer.JobRinging || j.Status == dialer.JobDialing || j.Status == dialer.JobCompleted {
			processedCount++
		}
	}

	if processedCount == 0 {
		t.Error("expected at least one job to be processed")
	}
}

func TestDialer_FifoOrder(t *testing.T) {
	resEngine := dialer.NewInMemReservationEngine()
	queue := dialer.NewQueue(resEngine)
	ctx := context.Background()

	campID := "camp-1"
	// Cadastrar jobs com posições 2 (primeiro cadastrado) e 1 (segundo cadastrado)
	job1 := &dialer.DialerJob{ID: "j1", CampaignID: campID, Position: 2, Status: dialer.JobQueued, NextAttemptAt: time.Now().Add(-1 * time.Second)}
	job2 := &dialer.DialerJob{ID: "j2", CampaignID: campID, Position: 1, Status: dialer.JobQueued, NextAttemptAt: time.Now().Add(-1 * time.Second)}

	queue.Enqueue([]*dialer.DialerJob{job1, job2})

	// O NextJob deve retornar o job2 (Position 1) primeiro por conta da ordenação FIFO
	next, err := queue.NextJob(ctx, campID)
	if err != nil {
		t.Fatalf("failed to get next job: %v", err)
	}
	if next == nil {
		t.Fatal("expected a job, got nil")
	}
	if next.ID != "j2" {
		t.Errorf("expected FIFO prioritizing position 1 (j2), got %s", next.ID)
	}
}

func TestDialer_RetryPacing(t *testing.T) {
	resEngine := dialer.NewInMemReservationEngine()
	queue := dialer.NewQueue(resEngine)
	ctx := context.Background()

	camp := &dialer.DialerCampaign{
		ID:                "camp-retry",
		MaxAttempts:       3,
		RetryDelaySeconds: 10,
	}
	queue.SaveCampaign(camp)

	job := &dialer.DialerJob{
		ID:         "job-1",
		CampaignID: camp.ID,
		Attempt:    1,
		Status:     dialer.JobReserved,
	}
	queue.Enqueue([]*dialer.DialerJob{job})

	// 1. Simular falha e disparar retentativa
	err := queue.HandleRetry(ctx, job.ID)
	if err != nil {
		t.Fatalf("HandleRetry failed: %v", err)
	}

	jobs := queue.GetJobsForCampaign(camp.ID)
	j := jobs[0]

	if j.Status != dialer.JobRetryPending {
		t.Errorf("expected state JobRetryPending, got %s", j.Status)
	}
	if j.Attempt != 2 {
		t.Errorf("expected attempt 2, got %d", j.Attempt)
	}

	// 2. Simular estouro de limite de tentativas
	j.Attempt = 3
	j.Status = dialer.JobReserved
	err = queue.HandleRetry(ctx, job.ID)
	if err != nil {
		t.Fatalf("HandleRetry failed: %v", err)
	}

	if j.Status != dialer.JobFailed {
		t.Errorf("expected final state JobFailed, got %s", j.Status)
	}
}

func TestDialer_AISessionAutomaticStart(t *testing.T) {
	resEngine := dialer.NewInMemReservationEngine()
	queue := dialer.NewQueue(resEngine)
	ip := newMockInstanceProvider()
	logger := slog.New(slog.NewTextHandler(&nullWriter{}, nil))
	scheduler := dialer.NewScheduler(queue, ip, logger)

	camp := &dialer.DialerCampaign{
		ID:                  "camp-ai",
		TenantID:            "tenant-A",
		Name:                "AI Campaign",
		Mode:                dialer.ModeAI, // Modo IA!
		Status:              dialer.StatusDraft,
		MaxConcurrentCalls:  8,
		DialIntervalSeconds: 1,
		MaxAttempts:         3,
		RetryDelaySeconds:   2,
		Strategy:            "round-robin",
		InstancePool:        []string{"inst-1"},
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	queue.SaveCampaign(camp)

	ip.instances["inst-1"] = &dialer.InstanceInfo{
		ID:                 "inst-1",
		Status:             "CONNECTED",
		ActiveCalls:        0,
		MaxConcurrentCalls: 4,
	}

	job := &dialer.DialerJob{
		ID:            "job-ai-1",
		CampaignID:    camp.ID,
		TenantID:      camp.TenantID,
		LeadID:        "lead-ai-1",
		Phone:         "551199990000",
		Name:          "AI Lead 1",
		Position:      1,
		Status:        dialer.JobQueued,
		Attempt:       1,
		NextAttemptAt: time.Now().Add(-1 * time.Minute),
		CreatedAt:     time.Now(),
	}
	queue.Enqueue([]*dialer.DialerJob{job})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler.StartCampaign(ctx, camp.ID)
	time.Sleep(1200 * time.Millisecond)
	scheduler.StopCampaign(camp.ID)

	jobs := queue.GetJobsForCampaign(camp.ID)
	j := jobs[0]

	// Valida se o status transicionou corretamente para GREETING de forma automatizada
	if j.Status != dialer.JobGreeting {
		t.Errorf("expected job state JobGreeting, got %s", j.Status)
	}

	if j.AISessionID == "" || j.ProviderSessionID == "" {
		t.Errorf("expected AI session context IDs to be populated, got ai=%q, prov=%q", j.AISessionID, j.ProviderSessionID)
	}

	if j.Provider != "grok_realtime" || j.VoiceProfile != "sales" {
		t.Errorf("expected provider and profile to be resolved, got prov=%q, prof=%q", j.Provider, j.VoiceProfile)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

type nullWriter struct{}

func (n *nullWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
