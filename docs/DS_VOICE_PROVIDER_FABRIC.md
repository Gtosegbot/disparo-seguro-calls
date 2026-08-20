# DS Voice 2.0 — Provider Fabric & OmniRoute

Este documento detalha o funcionamento técnico do **DS Voice Provider Fabric (OmniRoute)**, a camada de voz proprietária e agnóstica de provedores de Speech-to-Speech (S2S) do ecossistema **Disparo Seguro Calls**.

---

## 1. O Conceito DS Voice

O cliente final não escolhe de forma obrigatória as APIs do Grok, Gemini, OpenAI ou ElevenLabs. O produto comercializa a experiência de voz abstrata **DS Voice**. O motor OmniRoute cuida de selecionar e chavear dinamicamente os provedores de rede baseando-se em:
* Custo estimado por minuto.
* Latência de resposta (Time to First Byte - TTFB).
* Requisitos de recursos (VAD, Tool Calling, etc.).
* Idioma e gênero de voz.
* Saúde atual (taxa de erro e timeouts).

---

## 2. Capability Matrix

Cada provedor do pool é catalogado na struct `ProviderCatalogItem` contendo:
* **Capabilities**: Flags indicando se o provedor faz `realtime_audio`, `stt`, `tts`, `vad`, ou `tool_calling` de forma nativa.
* **Languages**: Lista de locais suportados (ex.: `pt-BR`, `en-US`).
* **Health / Health Score**: Um float dinâmico de `0.0` a `1.0` medido pela taxa de sucesso histórica daquela sessão de chamadas.

Durante a resolução do OmniRoute, provedores que não possuem as capabilities solicitadas pelo `VoiceProfile` (ex.: ferramenta precisa de `tool_calling` ativa mas o provedor não suporta) são eliminados ativamente da seleção antes da avaliação de custos.

---

## 3. Dynamic Cost-Aware Routing (OmniRoute Score)

O OmniRoute resolve o provedor ativo calculando o score efetivo de cada provedor elegível através da fórmula:
```go
effective_score = cost_score + latency_score + quality_score + availability_score
```
Onde:
* **Cost Score**: Inversamente proporcional ao custo em cents de dólar por minuto (menor custo = maior pontuação).
* **Latency Score**: Inversamente proporcional à latência de handshake WebSocket/dial (menor latência = maior pontuação).
* **Quality Score**: Peso atribuído à classe de qualidade do modelo (`premium`, `balanced`, `economy`).
* **Availability Score**: Peso derivado diretamente do Health Score operacional.

Políticas Mapeadas:
* `Economy`: Maximiza o peso do `Cost Score` (direciona para o Internal Free Pool).
* `LowLatency`: Prioriza o `Latency Score` (provedor mais rápido).
* `Premium`: Maximiza o `Quality Score` (modelos com maior coerência e modulação de fala).
* `Balanced`: Distribuição equilibrada de todos os vetores.

---

## 4. Fallback Fabric (Cadeias de Fallback)

Quando o gateway detecta uma falha crítica de conexão ou streaming de áudio no provedor ativo, o runtime aciona a cadeia de fallback de forma atômica:
1. Marca o provedor primário como instável (reduzindo sua prioridade e saúde no catálogo).
2. Obtém o próximo item na fila de fallbacks cadastrados (ex.: `grok_realtime ➔ gemini_realtime ➔ loopback`).
3. Chaveia o fluxo de áudio e restabelece a conexão WebSocket com o provedor secundário de forma silenciosa para a ligação em andamento, registrando `fallback_started_at`, `primary_provider`, `fallback_provider` e `failure_reason`.

---

## 5. Tiers Comerciais

A infraestrutura está preparada para segregar os clientes em 4 níveis de consumo:
* `internal`: Acesso apenas ao pool econômico (loopback/local) para validações gratuitas.
* `basic`: Acesso a provedores econômicos com caminhos de fallback restritos.
* `premium`: Acesso total aos provedores realtime premium.
* `enterprise`: Políticas de roteamento e SLAs de latência personalizáveis.
