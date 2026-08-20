package dialer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditEventType identifies operational log entries.
type AuditEventType string

const (
	EventCampaignCreated AuditEventType = "CAMPAIGN_CREATED"
	EventCampaignStarted AuditEventType = "CAMPAIGN_STARTED"
	EventCampaignPaused  AuditEventType = "CAMPAIGN_PAUSED"
	EventJobCreated      AuditEventType = "JOB_CREATED"
	EventJobQueued       AuditEventType = "JOB_QUEUED"
	EventJobStarted      AuditEventType = "JOB_STARTED"
	EventCallConnected   AuditEventType = "CALL_CONNECTED"
	EventCallEnded       AuditEventType = "CALL_ENDED"
	EventProviderFailed  AuditEventType = "PROVIDER_FAILED"
	EventCostRecorded    AuditEventType = "COST_RECORDED"
	EventCSDelivery      AuditEventType = "CHATSEGURO_DELIVERY"
	EventCSFailure       AuditEventType = "CHATSEGURO_FAILURE"
)

// AuditLogEntry is a structured record of an operational event.
type AuditLogEntry struct {
	EventID    string         `json:"event_id"`
	Timestamp  time.Time      `json:"timestamp"`
	TenantID   string         `json:"tenant_id"`
	CampaignID string         `json:"campaign_id"`
	JobID      string         `json:"job_id"`
	CallID     string         `json:"call_id"`
	EventType  AuditEventType `json:"event_type"`
	Source     string         `json:"source"`
	Metadata   map[string]any `json:"metadata"`
}

// AuditTrail logs all critical actions for audit compliance.
type AuditTrail struct {
	mu   sync.Mutex
	logs []AuditLogEntry
}

func NewAuditTrail() *AuditTrail {
	return &AuditTrail{logs: make([]AuditLogEntry, 0)}
}

func (at *AuditTrail) Log(tenantID, campaignID, jobID, callID string, eventType AuditEventType, source string, meta map[string]any) {
	at.mu.Lock()
	defer at.mu.Unlock()

	entry := AuditLogEntry{
		EventID:    uuid.New().String(),
		Timestamp:  time.Now().UTC(),
		TenantID:   tenantID,
		CampaignID: campaignID,
		JobID:      jobID,
		CallID:     callID,
		EventType:  eventType,
		Source:     source,
		Metadata:   meta,
	}
	at.logs = append(at.logs, entry)
}

func (at *AuditTrail) GetLogs() []AuditLogEntry {
	at.mu.Lock()
	defer at.mu.Unlock()
	out := make([]AuditLogEntry, len(at.logs))
	copy(out, at.logs)
	return out
}

// PreflightError represents a validation gap.
type PreflightError struct {
	Field  string
	Reason string
}

func (pe *PreflightError) Error() string {
	return fmt.Sprintf("preflight validation failed on %s: %s", pe.Field, pe.Reason)
}

// PreflightValidator checks if campaign holds complete operational parameters.
func ValidateCampaignPreflight(camp *DialerCampaign) error {
	if camp.ID == "" {
		return &PreflightError{Field: "ID", Reason: "campaign ID cannot be empty"}
	}
	if camp.TenantID == "" {
		return &PreflightError{Field: "TenantID", Reason: "tenant ID must be valid"}
	}
	if camp.MaxConcurrentCalls <= 0 {
		return &PreflightError{Field: "MaxConcurrentCalls", Reason: "must allow at least 1 concurrent call"}
	}
	if camp.DialIntervalSeconds < 0 {
		return &PreflightError{Field: "DialIntervalSeconds", Reason: "interval cannot be negative"}
	}
	if camp.MaxAttempts <= 0 {
		return &PreflightError{Field: "MaxAttempts", Reason: "max attempts must be greater than zero"}
	}
	return nil
}

// SimulatedProviderBehavior models test latency and errors.
type SimulatedProviderBehavior struct {
	InjectTimeout  bool
	Inject500Error bool
	LatencyMs      int
}

// E2EHarness simulates the complete physical infrastructure.
type E2EHarness struct {
	Audit            *AuditTrail
	Costs            *OperationCostEngine
	ProviderBehavior map[string]*SimulatedProviderBehavior
	InjectCSFailure  bool
	mu               sync.Mutex
}

func NewE2EHarness() *E2EHarness {
	return &E2EHarness{
		Audit:            NewAuditTrail(),
		Costs:            NewOperationCostEngine(),
		ProviderBehavior: make(map[string]*SimulatedProviderBehavior),
	}
}

func (h *E2EHarness) SetProviderBehavior(name string, b *SimulatedProviderBehavior) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ProviderBehavior[name] = b
}

// SimulateCall runs the E2E simulation.
func (h *E2EHarness) SimulateCall(ctx context.Context, tenantID, campaignID, jobID, providerName string, dryRun bool) (string, error) {
	callID := uuid.New().String()

	// 1. Audit Job Started
	h.Audit.Log(tenantID, campaignID, jobID, callID, EventJobStarted, "harness", nil)

	if dryRun {
		h.Audit.Log(tenantID, campaignID, jobID, callID, EventJobCreated, "harness", map[string]any{"dry_run": true})
		return "DRY_RUN_OK", nil
	}

	h.mu.Lock()
	behavior, exists := h.ProviderBehavior[providerName]
	h.mu.Unlock()

	// 2. Simulated Latency
	if exists && behavior.LatencyMs > 0 {
		select {
		case <-time.After(time.Duration(behavior.LatencyMs) * time.Millisecond):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	// 3. Simulated Provider Errors
	if exists {
		if behavior.InjectTimeout {
			h.Audit.Log(tenantID, campaignID, jobID, callID, EventProviderFailed, providerName, map[string]any{"error": "timeout"})
			return "", errors.New("harness: provider connection timeout")
		}
		if behavior.Inject500Error {
			h.Audit.Log(tenantID, campaignID, jobID, callID, EventProviderFailed, providerName, map[string]any{"error": "500 Internal Error"})
			return "", errors.New("harness: provider internal API error (500)")
		}
	}

	// 4. Simulate Audio Connection Success
	h.Audit.Log(tenantID, campaignID, jobID, callID, EventCallConnected, providerName, nil)

	// Calculate and record platform and API provider cost
	platformCost := 0.15 // USD Cents
	providerCost := 0.35
	if providerName == "grok_realtime" {
		providerCost = 1.25
	}
	h.Costs.RecordCost(jobID, platformCost, providerCost)
	h.Audit.Log(tenantID, campaignID, jobID, callID, EventCostRecorded, "harness", map[string]any{
		"platform_cost": platformCost,
		"provider_cost": providerCost,
		"total":         platformCost + providerCost,
	})

	// 5. ChatSeguro sync simulation
	if h.InjectCSFailure {
		h.Audit.Log(tenantID, campaignID, jobID, callID, EventCSFailure, "chatseguro", map[string]any{"error": "CRM offline"})
		// Sincronização secundária falhou mas a chamada continua com SUCESSO (não-bloqueante)
	} else {
		h.Audit.Log(tenantID, campaignID, jobID, callID, EventCSDelivery, "chatseguro", map[string]any{"status": "delivered"})
	}

	h.Audit.Log(tenantID, campaignID, jobID, callID, EventCallEnded, providerName, map[string]any{"outcome": "ANSWERED"})

	return "COMPLETED", nil
}
