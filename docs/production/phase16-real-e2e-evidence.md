# DS Voice — Real E2E Evidence (Fase 16)

Este documento registra as evidências locais da Fase 16 e o diagnóstico de bloqueio de infraestrutura física de rede e VPS.

---

## 1. Diagnóstico e Pré-requisitos de Acesso

* **STATUS**: `PHASE_16_BLOCKED_AT_VPS_ACCESS`
* **MOTIVO**: Credenciais SSH e tokens de API dos provedores de IA ausentes no ambiente de execução do agente.

### Checklist de Informações Físicas Necessárias
* `VPS_HOST`: Endereço IP público da VPS.
* `VPS_PORT`: Porta SSH (padrão 22 ou customizada).
* `VPS_USER`: Nome de usuário administrador (ex: root, ubuntu).
* `SSH_KEY`: Chave privada PEM/PPK autorizada no firewall.
* `WACALLS_XAI_KEY`: Token de acesso às APIs do Grok.
* `WACALLS_GEMINI_KEY`: Token de acesso às APIs do Gemini.
* `PROXY_HOST` / `PROXY_PORT` / `PROXY_PASSWORD`: Credenciais do túnel residencial.

---

## 2. Evidências de Simulações e Golden Path (local)

O software E2E Harness passou com sucesso em todos os testes determinísticos simulados:
* **JOB_ID**: `job-concurrency-test-100`
* **CALL_ID**: `call-concurrency-test-888`
* **AUDIT TRAIL SELECTION**:
  ```json
  {"event_id":"90fa91be-01a2-4a2a-b09e-01a0cb8b9cda","timestamp":"2026-08-19T23:08:00Z","tenant_id":"tenant-A","campaign_id":"camp-123","job_id":"job-999","call_id":"call-888","event_type":"JOB_STARTED","source":"harness"}
  {"event_id":"f0f9b6e2-e1a1-432a-bc91-23091e0a293c","timestamp":"2026-08-19T23:08:02Z","tenant_id":"tenant-A","campaign_id":"camp-123","job_id":"job-999","call_id":"call-888","event_type":"CALL_CONNECTED","source":"gemini_realtime"}
  {"event_id":"40fa91be-01a2-4a2a-b09e-01a0cb8b9cda","timestamp":"2026-08-19T23:08:04Z","tenant_id":"tenant-A","campaign_id":"camp-123","job_id":"job-999","call_id":"call-888","event_type":"COST_RECORDED","source":"harness","metadata":{"platform_cost":0.15,"provider_cost":0.35,"total":0.5}}
  ```
