package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"wacalls/internal/platform/dialer"
)

type campaignsRouter struct {
	queue        *dialer.Queue
	scheduler    *dialer.Scheduler
	idemRegistry *dialer.IdempotencyRegistry
	states       *dialer.JobStateMachine
	costs        *dialer.OperationCostEngine
	log          *slog.Logger
}

func newCampaignsRouter(
	q *dialer.Queue,
	sched *dialer.Scheduler,
	idem *dialer.IdempotencyRegistry,
	states *dialer.JobStateMachine,
	costs *dialer.OperationCostEngine,
	log *slog.Logger,
) *campaignsRouter {
	return &campaignsRouter{
		queue:        q,
		scheduler:    sched,
		idemRegistry: idem,
		states:       states,
		costs:        costs,
		log:          log,
	}
}

func (r *campaignsRouter) mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/campaigns", r.listCampaigns)
	mux.HandleFunc("POST /api/campaigns", r.createCampaign)
	mux.HandleFunc("POST /api/campaigns/{id}/execute", r.executeCampaign)
	mux.HandleFunc("POST /api/campaigns/{id}/pause", r.pauseCampaign)
	mux.HandleFunc("POST /api/campaigns/{id}/stop", r.stopCampaign)
	mux.HandleFunc("GET /api/campaigns/{id}/metrics", r.getMetrics)
}

// GET /api/campaigns
func (r *campaignsRouter) listCampaigns(w http.ResponseWriter, req *http.Request) {
	tenantID := req.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		http.Error(w, "missing X-Tenant-ID", http.StatusUnauthorized)
		return
	}

	campaigns := r.queue.ListCampaigns(tenantID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"campaigns": campaigns})
}

// POST /api/campaigns
func (r *campaignsRouter) createCampaign(w http.ResponseWriter, req *http.Request) {
	tenantID := req.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		http.Error(w, "missing X-Tenant-ID", http.StatusUnauthorized)
		return
	}

	var body struct {
		Name                string `json:"name"`
		Mode                string `json:"mode"`
		MaxConcurrentCalls  int    `json:"max_concurrent_calls"`
		DialIntervalSeconds int    `json:"dial_interval_seconds"`
		MaxAttempts         int    `json:"max_attempts"`
		RetryDelaySeconds   int    `json:"retry_delay_seconds"`
	}

	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	camp := &dialer.DialerCampaign{
		ID:                  id,
		TenantID:            tenantID,
		Name:                body.Name,
		Mode:                dialer.CampaignMode(body.Mode),
		Status:              dialer.CampaignReady,
		MaxConcurrentCalls:  body.MaxConcurrentCalls,
		DialIntervalSeconds: body.DialIntervalSeconds,
		MaxAttempts:         body.MaxAttempts,
		RetryDelaySeconds:   body.RetryDelaySeconds,
	}

	r.queue.SaveCampaign(camp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(camp)
}

// POST /api/campaigns/{id}/execute
func (r *campaignsRouter) executeCampaign(w http.ResponseWriter, req *http.Request) {
	campaignID := req.PathValue("id")
	tenantID := req.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		http.Error(w, "missing X-Tenant-ID", http.StatusUnauthorized)
		return
	}

	var contract dialer.UnifiedContract
	if err := json.NewDecoder(req.Body).Decode(&contract); err != nil {
		http.Error(w, "invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Força o ID do tenant a partir do header de autenticação
	contract.TenantID = tenantID

	// 1. Validação de Idempotência
	jobID := contract.JobID
	if jobID == "" {
		jobID = uuid.New().String()
	}
	_, err := r.idemRegistry.CheckOrRegister(contract.IdempotencyKey, jobID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "duplicate",
			"error":   "duplicate request detected by idempotency key",
			"job_id":  jobID,
		})
		return
	}

	// 2. Máquina de Estados do Job Engine
	r.states.CreateJob(jobID, tenantID, contract.IdempotencyKey)
	_ = r.states.Transition(jobID, dialer.StateQueued)

	// 3. Processamento Assíncrono (Worker Model)
	go func() {
		r.log.Info("orchestration: starting job execution asynchronously", "job_id", jobID)
		_ = r.states.Transition(jobID, dialer.StateRunning)

		// Simula enfileiramento dos leads do payload no discador
		leads, ok := contract.Payload["leads"].([]any)
		if ok && len(leads) > 0 {
			var jobs []*dialer.DialerJob
			for idx, l := range leads {
				leadData, _ := l.(map[string]any)
				phone, _ := leadData["phone"].(string)
				
				jobs = append(jobs, &dialer.DialerJob{
					ID:         uuid.New().String(),
					CampaignID: campaignID,
					Phone:      phone,
					Status:     dialer.JobQueued,
					Position:   idx,
				})
			}
			r.queue.Enqueue(jobs)
		}

		// Atualiza para concluído no final do scheduler process
		_ = r.states.Transition(jobID, dialer.StateCompleted)
	}()

	// Retorna imediatamente em menos de 50ms (Conforme especificação assíncrona)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "accepted",
		"job_id":  jobID,
	})
}

// POST /api/campaigns/{id}/pause
func (r *campaignsRouter) pauseCampaign(w http.ResponseWriter, req *http.Request) {
	campaignID := req.PathValue("id")
	r.scheduler.PauseCampaign(campaignID)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

// POST /api/campaigns/{id}/stop
func (r *campaignsRouter) stopCampaign(w http.ResponseWriter, req *http.Request) {
	campaignID := req.PathValue("id")
	r.scheduler.StopCampaign(campaignID)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

// GET /api/campaigns/{id}/metrics
func (r *campaignsRouter) getMetrics(w http.ResponseWriter, req *http.Request) {
	campaignID := req.PathValue("id")
	metrics := r.scheduler.GetMetrics(campaignID)
	
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metrics)
}
