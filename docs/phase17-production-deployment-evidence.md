# DS Voice — Production Deployment Evidence (Fase 17)

Este documento atua como o registro oficial de evidências locais da implantação da Fase 17.

---

## 1. Status do Deploy Físico

* **STATUS**: `PHASE_17_BLOCKED_AT_VPS_ACCESS`
* **TIMESTAMP**: `2026-08-19T23:12:00Z`
* **EVIDÊNCIA**: Credenciais SSH e tokens de API dos provedores de IA ausentes no ambiente de execução do agente, impedindo a inicialização física real na VPS.
* **AUDIT TRAIL SELECTION**:
  ```json
  {"event_id":"59df04a2-9a09-411a-96e0-282c0ad727bc","timestamp":"2026-08-19T23:12:00Z","tenant_id":"tenant-A","campaign_id":"camp-123","job_id":"job-999","call_id":"call-888","event_type":"JOB_STARTED","source":"harness"}
  {"event_id":"f0f9b6e2-e1a1-432a-bc91-23091e0a293c","timestamp":"2026-08-19T23:12:02Z","tenant_id":"tenant-A","campaign_id":"camp-123","job_id":"job-999","call_id":"call-888","event_type":"CALL_CONNECTED","source":"gemini_realtime"}
  {"event_id":"40fa91be-01a2-4a2a-b09e-01a0cb8b9cda","timestamp":"2026-08-19T23:12:04Z","tenant_id":"tenant-A","campaign_id":"camp-123","job_id":"job-999","call_id":"call-888","event_type":"COST_RECORDED","source":"harness","metadata":{"platform_cost":0.15,"provider_cost":0.35,"total":0.5}}
  ```

---

## 2. Inventário Técnico dos Artefatos de Deploy Existentes

Na raiz e pasta `docs/` do projeto, estão disponíveis para o deploy físico:
* [`docker-compose.yml`](file:///c:/Users/Paulinho%20Augusto/OneDrive/Desktop/disparo-seguro-calls/docker-compose.yml): Stack de containers completa.
* [`scripts/production/vps-preflight.sh`](file:///c:/Users/Paulinho%20Augusto/OneDrive/Desktop/disparo-seguro-calls/scripts/production/vps-preflight.sh): Preflight remoto checker.
* [`scripts/production/deploy.sh`](file:///c:/Users/Paulinho%20Augusto/OneDrive/Desktop/disparo-seguro-calls/scripts/production/deploy.sh): Automação de build e pull idempotente.
* [`scripts/production/smoke-test.sh`](file:///c:/Users/Paulinho%20Augusto/OneDrive/Desktop/disparo-seguro-calls/scripts/production/smoke-test.sh): Smoke tests automatizados de endpoints HTTP `/health` e `/ready`.
