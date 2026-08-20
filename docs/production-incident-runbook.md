# DS Voice — Production Incident Runbook (Fase 12/14)

Este documento descreve os procedimentos operacionais para mitigação de falhas em produção do **Disparo Seguro Calls**.

---

## 1. Falha Consecutiva de Provedores de IA
* **Sintoma**: Ligações caem imediatamente ou retornam erro de timeout.
* **Causa**: Provedor principal degradado ou limites de cota da API estourados.
* **Mitigação**:
  1. O Circuit Breaker abrirá o circuito automaticamente.
  2. O OmniRoute migrará o tráfego de voz para o provedor secundário (Gemini Realtime) via Fallback Chain.
  3. Ajuste os pesos percentuais (Weighted Routing) via endpoint `/api/admin/providers/{name}` reduzindo a carga do provedor com problemas.

---

## 2. Inconsistência de Estado do Database
* **Sintoma**: Um lead fica travado em processamento no Dialer.
* **Mitigação**:
  1. Os leases de claiming de leads possuem expiração de 30 segundos.
  2. Caso o travamento persistir no banco físico, execute o reset manual da reserva daquele ID ou reinicie o container do worker.
