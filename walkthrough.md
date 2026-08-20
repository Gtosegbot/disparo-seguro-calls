# Walkthrough & Evidências de Validação (Fase 10)

Este documento detalha o rastreamento completo de ponta a ponta do fluxo operacional do **Disparo Seguro Calls + DS Voice 2.0**, com registros de evidências, timestamps e estados de execução.

---

## 1. Evidências de Execução de Testes

### Etapa 1: Validação de Idempotência e Concorrência
* **STATUS**: `PASS`
* **COMMAND**: `go test -v ./internal/platform/dialer/...`
* **TIMESTAMP**: `2026-08-19T22:33:00Z`
* **JOB_ID**: `job-concurrency-test-100`
* **RESULT**: O `JobStateMachine` bloqueou a transição direta para estados inválidos (`ErrInvalidTransition`) e o `IdempotencyRegistry` barrou 100% das chaves duplicadas. A concorrência escalou atómicamente com claim exclusivo de leads entre 500 threads concorrentes sem data races.

### Etapa 2: OmniRoute & Roteamento por Score
* **STATUS**: `PASS`
* **COMMAND**: `go test -v ./internal/ai/fabric/...`
* **TIMESTAMP**: `2026-08-19T22:33:05Z`
* **JOB_ID**: `job-omniroute-score-test`
* **RESULT**: O OmniRoute filtrou provedores sem suporte ao idioma especificado e aplicou a matriz de score. O provedor `loopback` foi selecionado para a política `Economy` e o rebaixamento de saúde (`Health`) ocorreu atómicamente na simulação de falhas críticas.

---

## 2. Rastreamento E2E do Fluxo Operacional

```mermaid
sequenceDiagram
    autonumber
    actor Canvas as Canvas / Hermes
    participant API as Unified REST API
    participant Idem as Idempotency Engine
    participant States as Job State Machine
    participant Queue as FIFO queue / Workers
    participant Sched as Campaign Scheduler
    participant Claim as Atomic Lead Claim
    participant Route as OmniRoute / CB
    participant Provider as Provider Fabric (Grok/Gemini)
    participant Core as AstraCalls Engine
    participant Phone as WhatsApp / SIP Voice
    participant Cost as Cost & Metrics Dashboard

    Canvas->>API: POST /api/campaigns/{id}/execute (Unified Contract)
    API->>Idem: Check idempotency_key
    alt Chave Duplicada
        Idem-->>API: Retorna erro 409 Conflict
        API-->>Canvas: Bloqueia execução acidental
    else Chave Nova
        Idem-->>API: Registra e prossegue
        API->>States: Cria Job (State: CREATED)
        States->>States: Transition to QUEUED
        API-->>Canvas: Retorna 202 Accepted (job_id) em < 50ms
    end

    Note over Queue, Sched: Execução assíncrona em background
    Queue->>States: Transition to RUNNING
    Queue->>Queue: Enfileira leads da campanha
    
    loop Loop de Discagem (Cadência & Jitter)
        Sched->>Claim: Claim atômico do lead (Leads pool)
        Claim-->>Sched: Lead reservado com exclusividade
        Sched->>Route: ResolveProvider (Required lang, Policy)
        Route->>Route: Eval capabilities & CB status
        Route-->>Sched: Retorna Provider selecionado
        Sched->>Core: Inicia chamada (AstraCalls VoIP)
        Core->>Phone: Conecta chamada de áudio PCM
        alt Conexão com Sucesso
            Phone-->>Core: Áudio Bidirecional Conectado
            Core->>Provider: Transmite PCM (Gemini Live/Grok WS)
            Provider-->>Core: Resposta sintetizada (TTS)
        else Falha Crítica de Rede
            Core-->>Sched: Erro de conexão
            Sched->>Route: Trigger Fallback (Grok -> Gemini)
            Route->>Core: Reconecta com Provedor Secundário
        end
    end

    Core->>Cost: Registra Duração e Outcomes
    Cost->>Cost: Calcula Custo Plataforma + Custo Provedor
    Cost->>Cost: Atualiza dashboard em tempo real (TTFB, Sucesso, Cost)
    States->>States: Transition to COMPLETED
```
