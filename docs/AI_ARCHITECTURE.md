# AI Architecture — Disparo Seguro Calls

> Este documento inicia a linha de evolução proprietária do Disparo Seguro
> sobre o baseline AstraCalls (`astracalls-baseline-2026-08-19`).

---

## 1. Visão Geral

```
┌──────────────────────────────────────────────────────┐
│                  AstraCalls Core                     │
│  (internal/voip, signaling, transport, WhatsApp)     │
│                NOT MODIFIED                          │
└─────────────────────┬────────────────────────────────┘
                      │ float32 audio frames (OnPeerAudio)
                      ▼
┌──────────────────────────────────────────────────────┐
│                 AI Layer (internal/ai/)              │
│                                                      │
│  ┌─────────────┐        ┌──────────────────────┐     │
│  │  Media      │        │  AI Session          │     │
│  │  Adapter    │◄──────►│  (session.Registry)  │     │
│  └──────┬──────┘        └──────────────────────┘     │
│         │                                            │
│         ▼                                            │
│  ┌─────────────┐        ┌──────────────────────┐     │
│  │  Voice      │        │  Agent Runtime       │     │
│  │  Gateway    │        │  (Survey, Sales...)  │     │
│  └──────┬──────┘        └──────────────────────┘     │
│         │                                            │
│         ▼                                            │
│  ┌─────────────────────────────────────────────┐     │
│  │         Provider Registry                   │     │
│  │  (grok_realtime | future: gemini, elabs...) │     │
│  └─────────────────────────────────────────────┘     │
│                                                      │
│  ┌─────────────────────────────────────────────┐     │
│  │                 Event Bus                   │     │
│  │  (analytics, CRM, billing, audit)           │     │
│  └─────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────┘
                      │ float32 audio frames (synthesised)
                      ▼
┌──────────────────────────────────────────────────────┐
│                  AstraCalls Core                     │
│              (DataChannel → peer)                    │
└──────────────────────────────────────────────────────┘
```

---

## 2. Pacotes

| Pacote | Caminho | Responsabilidade |
|--------|---------|-----------------|
| `session` | `internal/ai/session` | `AISession`, `VoiceProfile`, `PromptContext`, `Registry` |
| `provider` | `internal/ai/provider` | `Provider` interface, `ProviderRegistry`, `GrokRealtime` |
| `media` | `internal/ai/media` | `AIMediaAdapter` — pont AstraCalls ↔ Provider |
| `agent` | `internal/ai/agent` | `Agent` interface, `SurveyAgent`, `SalesAgent` |
| `gateway` | `internal/ai/gateway` | `VoiceGateway` — orquestra tudo |
| `events` | `internal/ai/events` | Bus de eventos interno (não-blocking) |

---

## 3. AISession — Ciclo de Vida

```
CREATED → DIALING → RINGING → CONNECTED → LISTENING
                                              │
                              ┌───────────────┤
                              │               │
                           THINKING       SPEAKING
                              │               │
                              └───────────────┤
                                          INTERRUPTED
                                              │
                                          LISTENING
                                              │
                                          ENDING → ENDED
                                              │
                                           ERROR
```

Campos principais:

- `id` — UUID da sessão AI
- `tenant_id` — isolamento multi-tenant
- `session_id` — ID da sessão AstraCalls
- `call_id` — ID da chamada AstraCalls
- `agent_id` — agente responsável
- `voice_profile` — perfil sem API keys
- `provider` — nome do provider resolvido
- `state` — estado atual (thread-safe)

---

## 4. VoiceProfile

Configuração visível ao tenant. **Nunca contém API keys.**

```json
{
  "id": "survey",
  "version": "1.0",
  "language": "pt-BR",
  "voice": "nova",
  "prompt": "Você é um pesquisador...",
  "provider_policy": "primary",
  "max_duration": 300000000000,
  "barge_in": true
}
```

---

## 5. PromptContext

Construção em camadas com hash SHA-256 para auditoria:

```
platform_rules     → regras globais da plataforma
profile_prompt     → prompt do perfil de voz
session_context    → contexto da chamada atual
business_context   → contexto do negócio do tenant
task_prompt        → instrução específica da tarefa
                   ↓
                Prompt Final + Version + SHA-256 Hash
```

---

## 6. Provider — Design Sem Lock-in

```go
type Provider interface {
    Name() string
    Start(ctx context.Context, cfg Config) (<-chan PCMChunk, error)
    SendAudio(chunk PCMChunk) error
    Interrupt() error
    Stop() error
}
```

Trocar de Grok para Gemini = registrar novo provider. Zero mudança no core.

### Formato de Áudio Interno Canônico

```
PCM_S16LE | 16kHz | mono | 20ms | 640 bytes/frame
```

---

## 7. Barge-In

```
SPEAKING
  → VAD detecta fala do usuário
  → NotifyBargeIn()
  → prov.Interrupt() (cancela TTS)
  → flush output buffer
  → INTERRUPTED
  → LISTENING
```

Implementado em `AIMediaAdapter.handleBargeIn()` — reutilizável por qualquer provider.

---

## 8. Event Bus

Eventos internos não-bloqueantes:

| Evento | Quando |
|--------|--------|
| `call.created` | nova sessão criada |
| `call.ringing` | chamada tocando |
| `call.connected` | chamada atendida |
| `ai.started` | adapter conectado |
| `ai.listening` | aguardando fala |
| `ai.thinking` | processando |
| `ai.speaking` | TTS ativo |
| `ai.interrupted` | barge-in detectado |
| `ai.ended` | sessão encerrada |
| `ai.error` | erro interno |

---

## 9. API HTTP

```
POST   /api/ai/sessions              → criar sessão (CREATED)
POST   /api/ai/sessions/{id}/start   → conectar provider (LISTENING)
POST   /api/ai/sessions/{id}/stop    → encerrar sessão
GET    /api/ai/sessions/{id}         → snapshot da sessão
GET    /api/ai/sessions              → listar sessões do tenant
```

Autenticação: `X-Tenant-ID` header (substituir por JWT na próxima fase).
Secrets (API keys) nunca aparecem em resposta alguma.

---

## 10. Segurança

- API keys ficam exclusivamente no `ProviderRegistry` server-side
- Nenhum endpoint aceita ou retorna credentials de provider
- Toda query respeita `tenant_id`
- Tenant A não pode acessar sessão do Tenant B (`ErrForbidden`)

---

## 11. Observabilidade

Campos registrados por sessão:

`session_id`, `call_id`, `tenant`, `agent`, `profile`, `provider`, `model`,
`state`, `ttfb_ms`, `audio_frames`, `duration`, `reason`

**Nunca registrado:** secrets, API keys, tokens.

---

## 12. Próximas Fases

| Fase | Capacidade |
|------|-----------|
| AI-002 | Integração real com DataChannel do AstraCalls |
| AI-003 | Gemini Live API (segundo provider) |
| AI-004 | CRM handoff via Event Bus |
| AI-005 | Billing por sessão AI |
| AI-006 | Dashboard multi-tenant |

---

*Baseline: `astracalls-baseline-2026-08-19` @ `e11031c`*
*AI Foundation: feat commit imediatamente acima do baseline*
