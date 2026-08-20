package dialer_test

import (
	"context"
	"errors"
	"testing"

	"wacalls/internal/platform/dialer"
)

func TestHarness_PreflightValidation(t *testing.T) {
	// 1. Campanha inválida
	invalidCamp := &dialer.DialerCampaign{
		ID: "",
	}
	err := dialer.ValidateCampaignPreflight(invalidCamp)
	if err == nil {
		t.Error("expected preflight error for empty ID, got nil")
	}

	// 2. Campanha válida
	validCamp := &dialer.DialerCampaign{
		ID:                 "camp-123",
		TenantID:           "tenant-A",
		MaxConcurrentCalls: 8,
		MaxAttempts:        3,
	}
	err = dialer.ValidateCampaignPreflight(validCamp)
	if err != nil {
		t.Errorf("unexpected preflight error: %v", err)
	}
}

func TestHarness_GoldenPathExecution(t *testing.T) {
	h := dialer.NewE2EHarness()
	ctx := context.Background()

	tenantID := "tenant-A"
	campaignID := "camp-123"
	jobID := "job-999"

	// 1. Executa chamada simulada (Golden Path - tudo com sucesso)
	status, err := h.SimulateCall(ctx, tenantID, campaignID, jobID, "gemini_realtime", false)
	if err != nil {
		t.Fatalf("golden path failed: %v", err)
	}
	if status != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %s", status)
	}

	// 2. Valida se os custos foram atribuídos corretamente
	totalCost := h.Costs.GetTotalCost(jobID)
	expectedCost := 0.15 + 0.35 // platform + provider
	if totalCost != expectedCost {
		t.Errorf("expected cost %.2f, got %.2f", expectedCost, totalCost)
	}

	// 3. Valida se os logs de auditoria e correlation ID estão corretos
	logs := h.Audit.GetLogs()
	if len(logs) == 0 {
		t.Fatal("expected audit logs, got 0")
	}

	hasCSDelivery := false
	for _, entry := range logs {
		if entry.JobID != jobID || entry.TenantID != tenantID || entry.CampaignID != campaignID {
			t.Errorf("mismatched correlation keys in entry: %+v", entry)
		}
		if entry.EventType == dialer.EventCSDelivery {
			hasCSDelivery = true
		}
	}

	if !hasCSDelivery {
		t.Error("expected EventCSDelivery log entry in audit trail")
	}
}

func TestHarness_DryRun(t *testing.T) {
	h := dialer.NewE2EHarness()
	ctx := context.Background()

	// Dry run deve retornar imediatamente sem computar custos de operadora ou ping
	status, err := h.SimulateCall(ctx, "tenant-A", "camp-123", "job-999", "gemini_realtime", true)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if status != "DRY_RUN_OK" {
		t.Errorf("expected DRY_RUN_OK, got %s", status)
	}

	// Custo deve ser 0
	totalCost := h.Costs.GetTotalCost("job-999")
	if totalCost != 0.0 {
		t.Errorf("expected 0 cost for dry run, got %.2f", totalCost)
	}
}

func TestHarness_FailureInjection(t *testing.T) {
	h := dialer.NewE2EHarness()
	ctx := context.Background()

	tenantID := "tenant-A"
	campaignID := "camp-123"
	jobID := "job-999"

	// 1. Injeta falha de timeout do Provedor de IA
	h.SetProviderBehavior("grok_realtime", &dialer.SimulatedProviderBehavior{
		InjectTimeout: true,
	})

	_, err := h.SimulateCall(ctx, tenantID, campaignID, jobID, "grok_realtime", false)
	if err == nil {
		t.Error("expected error from injected provider timeout, got nil")
	}

	// 2. Injeta falha secundária no ChatSeguro (chamada deve continuar com SUCESSO)
	h.InjectCSFailure = true
	h.SetProviderBehavior("grok_realtime", &dialer.SimulatedProviderBehavior{
		InjectTimeout: false, // desliga erro
	})

	status, err := h.SimulateCall(ctx, tenantID, campaignID, jobID, "grok_realtime", false)
	if err != nil {
		t.Fatalf("call should succeed despite CS failure: %v", err)
	}
	if status != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %s", status)
	}

	// Valida que o erro de CRM foi logado no audit trail para retentativa assíncrona
	logs := h.Audit.GetLogs()
	hasCSFailure := false
	for _, entry := range logs {
		if entry.EventType == dialer.EventCSFailure {
			hasCSFailure = true
		}
	}
	if !hasCSFailure {
		t.Error("expected EventCSFailure log entry in audit trail")
	}
}
