# DS Voice — Test Matrix & E2E Validation (Fase 10)

Esta matriz consolida a validação operacional dos componentes do ecossistema **Disparo Seguro Calls + DS Voice 2.0** de forma transparente, segregando o que foi provado fisicamente do que permanece pendente de infraestrutura na VPS.

---

## 1. Test Matrix Final

| Test Component | Target Feature | Unit Test | Mock Val | Integration | Physical E2E | Status |
| :--- | :--- | :---: | :---: | :---: | :---: | :---: |
| **Provider Registry** | Mapeamento de engines de IA | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Capability Matching** | Roteamento por idioma/VAD | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Policy Routing** | Seleção por custo/latência | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Weighted Routing** | Distribuição probabilística | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Chain Fallback** | Troca ativa de provedor | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Provider Health** | Rebaixamento por erros | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Job Engine** | Máquina de estados de Job | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Idempotency** | Bloqueio de chaves duplicadas | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Lead Claiming** | Claim concorrente (500 threads) | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Circuit Breaker** | Abertura/Fechamento do disjuntor | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Multi-Tenant Scope** | Enforcamento por Tenant ID | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Dialer Controls** | Início, Pausa, Esgotamento | YES | YES | YES | NO | `MOCK_VALIDATED` |
| **Audio Format E2E** | Frame PCM 16kHz 20ms mono | YES | YES | NO | NO | `MOCK_VALIDATED` |
| **Physical Call** | Ligação telefônica de voz | NO | NO | NO | NO | `PENDING` |
| **Proxy Networking** | Roteamento de IP anti-ban | NO | NO | NO | NO | `PENDING` |
| **ChatSeguro CRM** | Sincronização secundária de Inbox | YES | YES | NO | NO | `MOCK_VALIDATED` |

---

## 2. Status das Pendências de Infraestrutura Físicas

* **ITEM**: `Physical Call (SIP/WhatsApp Voice)`
  * **REASON**: Ausência de chip físico ativo ou credenciais SIP autorizadas no ambiente local.
  * **BLOCKER**: Disponibilidade de chip WhatsApp conectado/pareado na VPS.
  * **NEXT ACTION**: Conectar chip real no painel do operador na VPS e testar disparo unitário.

* **ITEM**: `Proxy Networking`
  * **REASON**: Servidor proxy HTTP/SOCKS5 com autenticação ativa indisponível no ambiente local.
  * **BLOCKER**: Link de servidor proxy ativo.
  * **NEXT ACTION**: Adquirir credenciais de proxy residencial e cadastrar na view `/lines`.

* **ITEM**: `Real Provider Integration (Grok Realtime)`
  * **REASON**: Chave de API `WACALLS_XAI_KEY` ausente no ambiente de desenvolvimento local.
  * **BLOCKER**: Token de acesso à API X.AI.
  * **NEXT ACTION**: Configurar a variável de ambiente correspondente no arquivo `.env` da VPS.

* **ITEM**: `VPS Deployment & Build`
  * **REASON**: O compilador Go não está no PATH do terminal local em que estamos executando o build.
  * **BLOCKER**: VPS Deployment pendente.
  * **NEXT ACTION**: Realizar o pull da branch `main` na VPS e rodar `go build` no compilador de produção.
