# Walkthrough & Evidências Físicas E2E (Fase 11)

Este documento atua como o registro oficial de homologação do **DS Voice 2.0 / OmniRoute / Dialer**.

---

## 1. Rastreamento e Validação E2E

### Etapa 1: Validação de Chamada Física Controlada
* **STATUS**: `PENDING`
* **COMMAND**: `POST /api/campaigns/{id}/execute`
* **TIMESTAMP**: `2026-08-19T22:38:00Z`
* **JOB_ID**: `job-physical-e2e-pending`
* **CALL_ID**: `call-physical-e2e-pending`
* **RESULT**: Chamada de áudio PCM real suspensa devido à indisponibilidade de dispositivo WhatsApp/SIP conectado fisicamente no terminal do agente local. O fluxo de sinalização de dados simulado no unit_test passou com sucesso (`MOCK_VALIDATED`).

---

## 2. Diagrama de Integração do Pipeline Real

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
