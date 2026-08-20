# DS Voice — Production E2E Evidence (Fase 18)

Este documento registra as evidências locais e preparações da Fase 18 para a ativação física na VPS.

---

## 1. Status do Deploy Físico

* **STATUS**: `PHASE_18_BLOCKED_AT_VPS_ACCESS`
* **TIMESTAMP**: `2026-08-19T23:14:00Z`
* **EVIDÊNCIA**: Credenciais SSH e tokens de API dos provedores de IA ausentes no ambiente de execução do agente, impedindo o deploy físico real.

---

## 2. Inventário de Recursos Físicos Mapeados para Execução Futura

1. **VPS**:
   * Arquitetura recomendada: Ubuntu 22.04 LTS, Docker v24+, Compose v2+.
2. **PostgreSQL / Supabase**:
   * Schema e migrations integrados ao docker-compose local.
3. **Redis**:
   * Lock de claiming concorrente pronto em `orchestration.go`.
4. **Secrets do `.env`**:
   * `WACALLS_XAI_KEY` e `WACALLS_GEMINI_KEY` (IA Realtime).
   * `CHATSEGURO_TOKEN` e `CHATSEGURO_URL` (CRM).
   * `PROXY_HOST` / `PROXY_PORT` / `PROXY_PASSWORD` (Proxy).
5. **Canais Físicos**:
   * whatsmeow e SIP VoIP (telefonia real).
