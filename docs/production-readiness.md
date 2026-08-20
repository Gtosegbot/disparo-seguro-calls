# DS Voice — Production Readiness & Recovery (Fase 12)

Este documento detalha as políticas de estabilidade de software e disaster recovery consolidadas na **Fase 12** do **Disparo Seguro Calls**.

---

## 1. Políticas de Durabilidade e Consistência

* **Job Durability**: O `JobStateMachine` impede transições de estado impossíveis ou duplicadas. Em caso de crash do worker no estado `RUNNING`, os jobs são marcados e recuperados de forma atômica no banco de dados principal.
* **Idempotency Registry**: Chaves de idempotência únicas (`idempotency_key`) para toda transação previnem a duplicação acidental de disparos ou faturamentos sob condições de latência ou perda temporária de pacotes.

---

## 2. Disaster Recovery & Heartbeat

Caso um worker caia em execução, o heartbeat do lease expira e as seguintes ações automáticas ocorrem:
1. **Re-Queue**: Leads que estavam no estado `CLAIMED` mas sem progresso de áudio ou discagem são devolvidos à fila FIFO (`JobQueued`).
2. **Safe Terminate**: Conexões órfãs com provedores de IA são destruídas silenciosamente para evitar vazamento de tokens.
