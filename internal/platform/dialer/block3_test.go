package dialer_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"wacalls/internal/platform/dialer"
)

// ============================================================================
// 9. MULTI-TENANT + RBAC TESTS
// ============================================================================

func TestBlock3_RBAC_RolePermissions(t *testing.T) {
	// 1. VIEWER: Read only
	viewer := dialer.RoleViewer
	if !viewer.HasPermission(dialer.PermRead) {
		t.Error("viewer must have PermRead")
	}
	if viewer.HasPermission(dialer.PermOperate) {
		t.Error("viewer must NOT have PermOperate")
	}
	if viewer.HasPermission(dialer.PermAdmin) {
		t.Error("viewer must NOT have PermAdmin")
	}

	// 2. OPERATOR: Read + Operate, no Admin
	operator := dialer.RoleOperator
	if !operator.HasPermission(dialer.PermRead) || !operator.HasPermission(dialer.PermOperate) {
		t.Error("operator must have PermRead and PermOperate")
	}
	if operator.HasPermission(dialer.PermAdmin) {
		t.Error("operator must NOT have PermAdmin")
	}

	// 3. ADMIN: Full access
	admin := dialer.RoleAdmin
	if !admin.HasPermission(dialer.PermRead) || !admin.HasPermission(dialer.PermOperate) || !admin.HasPermission(dialer.PermAdmin) {
		t.Error("admin must have all permissions")
	}
}

func TestBlock3_RBAC_MultiTenantIsolation(t *testing.T) {
	tenantA := &dialer.TenantIdentity{TenantID: "tenant-A", Role: dialer.RoleOperator}
	tenantB := &dialer.TenantIdentity{TenantID: "tenant-B", Role: dialer.RoleOperator}
	adminGlobal := &dialer.TenantIdentity{TenantID: "admin-tenant", Role: dialer.RoleAdmin}

	// Tenant A acessando Tenant A com PermOperate -> Permitido
	err := dialer.Authorize(tenantA, "tenant-A", dialer.PermOperate)
	if err != nil {
		t.Errorf("tenant A accessing own resources failed: %v", err)
	}

	// Tenant A tentando acessar Tenant B -> Bloqueado com ErrCrossTenantForbidden
	err = dialer.Authorize(tenantA, "tenant-B", dialer.PermOperate)
	if !errors.Is(err, dialer.ErrCrossTenantForbidden) {
		t.Errorf("expected ErrCrossTenantForbidden for cross-tenant access, got %v", err)
	}

	// Tenant B tentando acessar Tenant A -> Bloqueado
	err = dialer.Authorize(tenantB, "tenant-A", dialer.PermRead)
	if !errors.Is(err, dialer.ErrCrossTenantForbidden) {
		t.Errorf("expected ErrCrossTenantForbidden, got %v", err)
	}

	// Admin Global gerenciando Tenant A ou B -> Permitido
	err = dialer.Authorize(adminGlobal, "tenant-A", dialer.PermAdmin)
	if err != nil {
		t.Errorf("admin global must be able to manage any tenant, got %v", err)
	}
}

func TestBlock3_RBAC_AuthenticateRequest(t *testing.T) {
	// 1. Tenant autenticado com role explícita na chave
	req := httptest.NewRequest(http.MethodGet, "/api/campaigns", nil)
	req.Header.Set("X-API-Key", "tenant-client123-OPERATOR")

	id, err := dialer.AuthenticateRequest(req)
	if err != nil {
		t.Fatalf("auth failed: %v", err)
	}
	if id.TenantID != "client123" || id.Role != dialer.RoleOperator {
		t.Errorf("expected tenant client123 and role OPERATOR, got %s / %s", id.TenantID, id.Role)
	}

	// 2. Tentativa de forjar X-Tenant-ID diferente da chave
	reqForged := httptest.NewRequest(http.MethodGet, "/api/campaigns", nil)
	reqForged.Header.Set("X-API-Key", "tenant-client123-OPERATOR")
	reqForged.Header.Set("X-Tenant-ID", "attacker-tenant")

	_, err = dialer.AuthenticateRequest(reqForged)
	if !errors.Is(err, dialer.ErrCrossTenantForbidden) {
		t.Errorf("expected ErrCrossTenantForbidden for header forgery, got %v", err)
	}
}

// ============================================================================
// 10. WEBHOOKS + AUTH + ANTI-REPLAY TESTS
// ============================================================================

func TestBlock3_Webhook_SecurityAndReplay(t *testing.T) {
	secret := "secret-phase-production-key-2026"
	rotatedOldSecret := "previous-key-rotation-2025"

	cfg := dialer.DefaultWebhookConfig(secret)
	cfg.RotatedSecrets = []string{rotatedOldSecret}
	cfg.ReplayWindow = 2 * time.Second

	wsm := dialer.NewWebhookSecurityManager(cfg)

	payload := []byte(`{"event":"call_completed","job_id":"job-100","outcome":"answered"}`)
	nowTs := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "nonce-uuid-1"
	eventID := "evt-101"

	sig := dialer.ComputeSignature(secret, nowTs, nonce, payload)

	// 1. Assinatura válida no prazo -> Aceita
	err := wsm.Verify(nowTs, nonce, eventID, sig, payload)
	if err != nil {
		t.Fatalf("valid webhook failed: %v", err)
	}

	// 2. Replay do mesmo nonce ou eventID -> Bloqueado
	err = wsm.Verify(nowTs, nonce, eventID, sig, payload)
	if !errors.Is(err, dialer.ErrWebhookReplayDetected) {
		t.Errorf("expected ErrWebhookReplayDetected for duplicate, got %v", err)
	}

	// 3. Assinatura com chave antiga rotacionada (Secret Rotation) -> Aceita com nonce novo
	nonceRotated := "nonce-uuid-2"
	eventIDRotated := "evt-102"
	sigRotated := dialer.ComputeSignature(rotatedOldSecret, nowTs, nonceRotated, payload)

	err = wsm.Verify(nowTs, nonceRotated, eventIDRotated, sigRotated, payload)
	if err != nil {
		t.Errorf("rotated secret signature verification failed: %v", err)
	}

	// 4. Assinatura adulterada / chave inválida -> Rejeitado
	invalidSig := "deadbeef00112233445566778899aabbccddeeff"
	err = wsm.Verify(nowTs, "nonce-uuid-3", "evt-103", invalidSig, payload)
	if !errors.Is(err, dialer.ErrWebhookInvalidSignature) {
		t.Errorf("expected ErrWebhookInvalidSignature, got %v", err)
	}

	// 5. Timestamp expirado (> ReplayWindow) -> Rejeitado
	oldTs := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	oldSig := dialer.ComputeSignature(secret, oldTs, "nonce-uuid-4", payload)
	err = wsm.Verify(oldTs, "nonce-uuid-4", "evt-104", oldSig, payload)
	if !errors.Is(err, dialer.ErrWebhookTimestampExpired) {
		t.Errorf("expected ErrWebhookTimestampExpired for old event, got %v", err)
	}
}

// ============================================================================
// 11. COST INTEGRITY TESTS
// ============================================================================

func TestBlock3_CostIntegrity_NoDoubleCharging(t *testing.T) {
	engine := dialer.NewOperationCostEngine()

	tenantID := "tenant-alpha"
	jobID := "job-cost-1"
	callID := "call-guid-123"
	costEventID := "event-charge-123"

	tx := dialer.CostTransaction{
		CostEventID:  costEventID,
		TenantID:     tenantID,
		JobID:        jobID,
		CallID:       callID,
		Attempt:      1,
		Provider:     "gemini_realtime",
		PlatformCost: 0.15,
		ProviderCost: 0.35,
		Billable:     true,
	}

	// 1. Primeira gravação da transação
	engine.RecordTransaction(tx)

	if engine.GetTotalCost(jobID) != 0.50 {
		t.Errorf("expected 0.50, got %.2f", engine.GetTotalCost(jobID))
	}
	if engine.GetTenantCost(tenantID) != 0.50 {
		t.Errorf("expected tenant cost 0.50, got %.2f", engine.GetTenantCost(tenantID))
	}

	// 2. Simulação de Webhook Duplicado com o mesmo costEventID
	engine.RecordTransaction(tx)
	if engine.GetTotalCost(jobID) != 0.50 {
		t.Errorf("duplicate webhook doubled cost! Expected 0.50, got %.2f", engine.GetTotalCost(jobID))
	}

	// 3. Simulação de Retry que falhou e NÃO é cobrável (Billable = false)
	failedRetryTx := dialer.CostTransaction{
		CostEventID:  "event-charge-retry-failed",
		TenantID:     tenantID,
		JobID:        jobID,
		CallID:       "call-guid-456",
		Attempt:      2,
		Provider:     "gemini_realtime",
		PlatformCost: 0.15,
		ProviderCost: 0.00,
		Billable:     false, // attempt != billable event
	}
	engine.RecordTransaction(failedRetryTx)

	// O custo cobrável permanece inalterado em 0.50
	if engine.GetTotalCost(jobID) != 0.50 {
		t.Errorf("non-billable attempt added to cost! Expected 0.50, got %.2f", engine.GetTotalCost(jobID))
	}

	// 4. Isolamento multi-tenant de custos
	engine.RecordTransaction(dialer.CostTransaction{
		CostEventID:  "event-charge-tenant-b",
		TenantID:     "tenant-beta",
		JobID:        "job-cost-2",
		PlatformCost: 1.00,
		ProviderCost: 2.00,
		Billable:     true,
	})

	if engine.GetTenantCost(tenantID) != 0.50 {
		t.Errorf("tenant A cost contaminated: expected 0.50, got %.2f", engine.GetTenantCost(tenantID))
	}
	if engine.GetTenantCost("tenant-beta") != 3.00 {
		t.Errorf("tenant B cost incorrect: expected 3.00, got %.2f", engine.GetTenantCost("tenant-beta"))
	}
}

// ============================================================================
// 12. AUDIT + CORRELATION + OBSERVABILITY TESTS
// ============================================================================

func TestBlock3_Observability_CorrelationAndSanitization(t *testing.T) {
	audit := dialer.NewAuditTrail()
	metrics := dialer.NewMetricsRegistry()

	ctx := dialer.CorrelationContext{
		TenantID:    "tenant-1",
		CampaignID:  "camp-1",
		JobID:       "job-1",
		LeadID:      "lead-1",
		CallID:      "call-1",
		Attempt:     1,
		Provider:    "grok_realtime",
		CostEventID: "cost-evt-1",
	}

	// 1. Audit Trail com Correlation Context e Chaves Sensíveis
	sensitiveMeta := map[string]any{
		"phone":          "+5511999998888",
		"provider_token": "secret-token-xyz-12345",
		"api_key":        "top-secret-api-key",
		"duration_sec":   45,
	}

	audit.LogWithCorrelation(ctx, dialer.EventCallConnected, "worker-1", sensitiveMeta)

	logs := audit.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(logs))
	}

	entry := logs[0]
	if entry.TenantID != ctx.TenantID || entry.JobID != ctx.JobID || entry.LeadID != ctx.LeadID || entry.CostEventID != ctx.CostEventID {
		t.Errorf("mismatched correlation context in log entry: %+v", entry)
	}

	// Valida sanitização de chaves sensíveis
	if entry.Metadata["api_key"] != "[REDACTED]" || entry.Metadata["provider_token"] != "[REDACTED]" {
		t.Errorf("sensitive credential was not redacted in audit log: %+v", entry.Metadata)
	}
	if entry.Metadata["phone"] != "+5511999998888" {
		t.Errorf("non-sensitive field was wrongly altered: %v", entry.Metadata["phone"])
	}

	// 2. Metrics Registry Completo (15 métricas mínimas)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.RecordJobCreated()
			metrics.RecordLeadClaimed()
			metrics.RecordCallStart()
			metrics.RecordCallCompleted()
			metrics.RecordRetry()
			metrics.RecordCosts(0.15, 0.35)
		}()
	}
	wg.Wait()

	metrics.RecordLatency(120*time.Millisecond, 350*time.Millisecond)

	snap := metrics.Snapshot()
	if snap["jobs_total"].(int64) != 50 {
		t.Errorf("expected 50 jobs_total, got %v", snap["jobs_total"])
	}
	if snap["calls_completed"].(int64) != 50 {
		t.Errorf("expected 50 calls_completed, got %v", snap["calls_completed"])
	}
	if snap["provider_latency"].(int64) != 120 {
		t.Errorf("expected 120ms provider_latency, got %v", snap["provider_latency"])
	}
	if snap["ttfb"].(int64) != 350 {
		t.Errorf("expected 350ms ttfb, got %v", snap["ttfb"])
	}
	if snap["cost_total"].(float64) != 25.00 { // 50 * 0.50 = 25.00
		t.Errorf("expected 25.00 cost_total, got %v", snap["cost_total"])
	}
}
