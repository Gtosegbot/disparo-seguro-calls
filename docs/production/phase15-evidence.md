# DS Voice — Production E2E Evidence (Fase 15)

Este documento registra as evidências locais e preparações operacionais para ativação real da infraestrutura física na VPS.

---

## 1. Evidências Físicas Simuladas (MOCK_VALIDATED)

* **STATUS**: `MOCK_VALIDATED`
* **TIMESTAMP**: `2026-08-19T23:04:00Z`
* **JOB_ID**: `job-physical-e2e-pending`
* **CALL_ID**: `call-physical-e2e-pending`
* **RESULT**: Execuções de chamadas de voz reais suspensas localmente devido à falta de credenciais do provedor de IA e chip físico conectado no terminal de desenvolvimento local.
* **AUDIT LOG SELECTIONS**:
  ```json
  {"event_id":"59df04a2-9a09-411a-96e0-282c0ad727bc","timestamp":"2026-08-19T23:04:00Z","tenant_id":"tenant-A","campaign_id":"camp-123","job_id":"job-999","call_id":"call-888","event_type":"JOB_STARTED","source":"harness"}
  {"event_id":"f0f9b6e2-e1a1-432a-bc91-23091e0a293c","timestamp":"2026-08-19T23:04:02Z","tenant_id":"tenant-A","campaign_id":"camp-123","job_id":"job-999","call_id":"call-888","event_type":"CALL_CONNECTED","source":"gemini_realtime"}
  {"event_id":"40fa91be-01a2-4a2a-b09e-01a0cb8b9cda","timestamp":"2026-08-19T23:04:04Z","tenant_id":"tenant-A","campaign_id":"camp-123","job_id":"job-999","call_id":"call-888","event_type":"COST_RECORDED","source":"harness","metadata":{"platform_cost":0.15,"provider_cost":0.35,"total":0.5}}
  ```

---

## 2. Scripts Criados para Execução na VPS
* **Preflight Check**: `scripts/production/preflight.sh`
* **Idempotent Deploy**: `scripts/production/deploy.sh`
* **Smoke Test**: `scripts/production/smoke-test.sh`
