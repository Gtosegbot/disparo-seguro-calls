# UPSTREAM BASELINE — DISPARO SEGURO CALLS

Este documento registra a linha de corte e equiparação da paridade de paridade de núcleo com o upstream AstraCalls.

## Detalhes do Baseline

- **Status**: UPSTREAM PARITY BASELINE
- **Upstream Utilizado**: `https://github.com/AstraOnlineWeb/AstraCalls.git`
- **Commit Exato Upstream**: `507a977a1a2924a875ab35bff30afd8553077488`
- **Nosso Commit Baseline**: `e11031c3ddd2debcfc714fc263294c2d54c70c50`
- **Tag Criada**: `astracalls-baseline-2026-08-19`
- **Data da Sincronização**: 19 de Agosto de 2026

## Testes Realizados

- **Frontend Compilation (`npm run build` em client/)**: **Passou com sucesso** (bundle gerado com Vite, tsc compilou limpo).

## Diferenças Intencionais Mantidas (Nosso Fork)

1. **Roteamento por `callId`**: O mapeamento e roteamento de chamadas concorrentes por ID único sob o mesmo contêiner de sessão.
2. **Registro de Chamadas Múltiplas**: Suporte para múltiplas chamadas concorrentes (`-max-calls-per-session`).
3. **Eventos ICE Customizados (`OnTerminalICE`)**: Utilizado para o teardown do canal na nuvem de mídia.

## Evolução Própria de IA

A partir deste baseline, os seguintes módulos serão acoplados como evolução proprietária:
- Integração de Sinalização com o Voice Gateway
- OmniRoute / Provedores de Voz (Grok Realtime / Gemini Live)
- Processadores de Mídia e transcoding em tempo real na nuvem de mídia
