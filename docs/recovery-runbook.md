# DS Voice — Recovery Runbook (Fase 12)

Manual operacional de resposta a incidentes críticos e disaster recovery.

---

## 1. Falha Crítica do Banco de Dados / Redis
1. **Sintoma**: Scheduler reporta erros consecutivos de conexão no pool.
2. **Ação**:
   * Verificar status do Redis: `redis-cli ping`.
   * Verificar conexões abertas no Postgres: `SELECT * FROM pg_stat_activity`.
   * Se necessário, reiniciar os containers: `docker-compose restart db redis`.

---

## 2. Inconsistência de Estado de Job
1. **Sintoma**: Um lead permanece como `RUNNING` indefinidamente mesmo após a chamada terminar.
2. **Ação**:
   * O lease do reservation engine expira automaticamente após 30 segundos.
   * Caso persistir, reiniciar o worker para disparar a rotina de re-queue automático de transações órfãs.
