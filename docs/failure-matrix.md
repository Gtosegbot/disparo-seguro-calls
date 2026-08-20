# DS Voice — Failure Matrix (Fase 12)

Esta matriz mapeia falhas e comportamentos de recuperação automática e mitigação de desastres de software.

---

## 1. Matriz de Tratamento de Falhas

| Component | Failure | Expected Behavior | Observed Behavior | Recovery | Status |
| :--- | :--- | :--- | :--- | :--- | :---: |
| **Provider** | Timeout / Internal (500) | Aciona o Fallback | Fallback automático (Grok -> Gemini) | Rotaciona e tenta restabelecer | `PASS` |
| **Circuit Breaker** | Erros sequenciais | Abre disjuntor | Tráfego suspenso por N segundos | Volta a CLOSED após sucesso em probe | `PASS` |
| **ChatSeguro** | CRM indisponível | Continua chamada (secundário) | Loga falha no audit trail | Retentativa assíncrona em background | `PASS` |
| **Idempotency** | Requisição duplicada | Bloqueia execução | Retorna 409 Conflict | Sem impacto operacional | `PASS` |
| **Worker** | Crash / Shutdown | Lease expira | Leads devolvidos à fila | Reagendamento automático | `PASS` |
| **Scheduler** | Parada abrupta | Retém estados persistidos | Restabelece status QUEUED | Retoma o loop de agendamento | `PASS` |
