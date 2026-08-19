# DS Voice Architecture

Este documento descreve a arquitetura de orquestração de voz e o ecossistema proprietário de IA em tempo real do **Disparo Seguro Calls**.

---

## 1. Visão Geral da Arquitetura

O ecossistema DS Voice separa estritamente a sinalização e mídia do WhatsApp (transporte do AstraCalls) da orquestração inteligente de inteligência artificial em tempo real (DS Voice Runtime + OmniRoute).

```
                      [ WhatsApp VoIP Network ]
                                 │
                                 ▼
                         [ AstraCalls Core ]
             (whatsmeow Signaling & WebRTC Loopback Audio)
                                 │
                                 ▼
                        [ AI Media Adapter ]
             (PCM 16kHz Mono 20ms Downlink / Uplink Wiring)
                                 │
                                 ▼
                     [ DS Voice Runtime (Fabric) ]
             (VAD, Barge-in, Interruption & Tool Execution)
                                 │
                                 ▼
                       [ OmniRoute / Router ]
           (Dynamic Cost, Latency & Quality Based Routing)
                                 │
                   ┌─────────────┴─────────────┐
                   ▼                           ▼
          [ Grok Realtime API ]      [ Gemini Live API ]
             (xAI wss protocol)       (Google Live wss)
```

---

## 2. Diferença entre Conceitos

### DS Voice (Nosso Runtime)
O **DS Voice** é o motor de orquestração de diálogos da nossa plataforma. Suas responsabilidades incluem:
* Capturar áudio e rodar o Voice Profile de negócio.
* Gerenciar estados da conversa (`LISTENING`, `THINKING`, `SPEAKING`).
* Controlar interrupções do usuário de forma imediata (**Barge-in**).
* Executar as chamadas de funções (**Tools**) autorizadas como `qualify_lead`, `move_kanban` ou `handoff_human` sem expor dados confidenciais aos provedores externos.

### Grok & Gemini (Provedores Físicos)
* **Grok Realtime** e **Gemini Live** são os motores de Speech-to-Speech (S2S) de terceiros.
* Eles recebem frames de áudio e fornecem o streaming de retorno (TTS) e transcrição.
* O **DS Voice Fabric** atua como uma casca isoladora (OmniRoute), permitindo que mudemos de provedor ou façamos fallback dinâmico instantaneamente sem que a sinalização do WhatsApp precise ser reiniciada ou que o AstraCalls core seja alterado.

---

## 3. Fluxo de Sinalização e Mídia (AI Dialer)

1. O **Dialer Scheduler** dispara uma chamada de campanha do tipo `ModeAI`.
2. A linha atende e a sinalização do AstraCalls aciona o `OnPeerAudio()`.
3. O `VoiceGateway` inicia a sessão de IA (`StartAISession()`) e acopla a chamada ao `AIMediaAdapter`.
4. O `Provider Fabric` resolve o provedor ativo (ex.: `grok_realtime`).
5. A conversa flui de forma bidirecional.
6. Se o provedor falhar ou retornar erro de rede, o Fabric aciona o **Fallback dinâmico** para o provedor secundário (ex.: `gemini_realtime`) em tempo de execução, mantendo a chamada ativa e documentando o motivo da falha.
7. Ao término, os dados de desfecho (`outcome`, transcrição e duração) são consolidados e retornados ao `DialerJob` na fila.
