# UPSTREAM SYNC AUDIT

Auditoria de equiparação de paridade entre o fork **Disparo Seguro Calls** e o repositório upstream **AstraCalls** realizada em 19 de Agosto de 2026.

## Commits de Paridade Upstream

| Commit | Componente / Arquivos | Classificação | Motivo / Risco | Ação |
| :--- | :--- | :--- | :--- | :--- |
| `507a977` | docs | A | Changelog v0.0.6. Sem risco. | Incorporar diretamente. |
| `a38c2a6` | signaling / calls | A | Para de tocar nos outros dispositivos quando um destino atende. Sem risco. | Incorporar diretamente. |
| `d0d32de` | docs / openapi | A | Documentação de rotas. Sem risco. | Incorporar diretamente. |
| `9b17978` | auth / session | A | Conectar sessão por código de pareamento. Baixo risco. | Incorporar diretamente. |
| `d472456` | transport / audio | A | Transporte de áudio por WebSocket (fallback proxy UDP). Baixo risco. | Incorporar diretamente. |
| `a693eb0` | chatwoot / messaging | A | Envia legenda com documento. Sem risco. | Incorporar diretamente. |
| `dec1b60` | chatwoot / messaging | A | Corrige legenda perdida. Sem risco. | Incorporar diretamente. |
| `0884093` | chatwoot / queue | A | Fila de reentrega durável do Chatwoot. Baixo risco. | Incorporar diretamente. |
| `3b77c32` | bugfixes | A | Correções gerais de gravação, chamada e Chatwoot. Baixo risco. | Incorporar diretamente. |
| `e3bc6ca` | docker | A | Build ARM64 do codec MLow. Sem risco. | Incorporar diretamente. |
| `8a40f16` | webrtc / video | A | Vídeo H264 via WebCodecs sobre datachannel. Médio risco. | Incorporar diretamente. |
| `6f86240` | calls / control | B | Transferência cega entre atendentes. Médio risco (toca controle de chamadas). | Mesclar com atenção às rotas personalizadas. |
| `cd11a44` | calls / control | B | Retenção (Hold/Resume) com música de espera. Médio risco. | Mesclar com atenção às rotas personalizadas. |
| `fbba1cb` | signaling / calls | B | Corrige sinalização para o WhatsApp atual. Alto risco se quebrar compatibilidade. | Mesclar e validar sinalização. |
| `09d33ea` | transport / webrtc | A | Desativa mDNS e limita relays por chamada. Baixo risco (melhora performance). | Incorporar diretamente. |
| `a8641dd` | codecs / mlow | A | MLow portátil baseline SSE2 contra crash. Baixo risco. | Incorporar diretamente. |
| `5011b34` | proxy | A | Proxy de saída HTTP/Socks5 por sessão. Baixo risco. | Incorporar diretamente. |
| `c5feb24` | recording | B | Gravação de chamada opt-in por sessão. Médio risco. | Mesclar e validar hooks de áudio. |
| `654c946` | chatwoot | A | Integração abre conversas de grupos e canais. Baixo risco. | Incorporar diretamente. |

## Modificações Próprias do Fork (A Serem Preservadas)

- **Multichamadas Simultâneas por Sessão** (`feat: route calls per callId so one session runs N concurrent calls`): O mapeamento e roteamento de chamadas por ID único para permitir chamadas concorrentes sob a mesma sessão.
- **Exposição de hooks de sinalização** (`OnTerminalICE`): Necessário para a sinalização WebRTC customizada na nuvem.
- **Registry de chamadas concorrentes por sessão**: Necessário para controle de concorrência.
