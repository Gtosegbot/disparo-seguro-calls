# DS Voice — Real Network Matrix (Fase 11)

Esta matriz mapeia o status de homologação de rede e infraestrutura de hardware física real do **DS Voice 2.0 / OmniRoute / Execution Fabric**.

---

## 1. Etapa 25 — Real Network Matrix

| Component | Mock | Integration | Network E2E | Physical E2E | Status |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **VPS** | YES | YES | NO | NO | `PENDING` |
| **Proxy** | YES | YES | NO | NO | `PENDING` |
| **WhatsApp** | YES | YES | NO | NO | `PENDING` |
| **SIP** | YES | YES | NO | NO | `PENDING` |
| **Provider** | YES | YES | NO | NO | `PENDING` |
| **DS Voice** | YES | YES | NO | NO | `PENDING` |
| **Audio** | YES | YES | NO | NO | `PENDING` |
| **Dialer** | YES | YES | NO | NO | `PENDING` |
| **ChatSeguro**| YES | YES | NO | NO | `PENDING` |
| **Dashboard** | YES | YES | NO | NO | `PENDING` |
| **Cost** | YES | YES | NO | NO | `PENDING` |

---

## 2. Rastreamento e Validação por Prova Real

* **ETAPA 1: VPS DISCOVERY**
  * **STATUS**: `PENDING`
  * **COMMAND**: `docker ps`, `uname -a`, `df -h`
  * **EVIDÊNCIA**: VPS de produção externa não conectada ao terminal de desenvolvimento local.
  * **PRÓXIMA AÇÃO**: Executar o checkout e deploy da branch `main` na VPS de homologação.

* **ETAPA 3: GO BUILD REAL**
  * **STATUS**: `GO_LOCAL_BUILD_UNAVAILABLE`
  * **COMMAND**: `go build ./...`
  * **EVIDÊNCIA**: `CommandNotFoundException` no terminal local do Windows.
  * **PRÓXIMA AÇÃO**: Compilar e gerar o binário estático via CI/CD Dockerfile.

* **ETAPA 7: REAL PROXY**
  * **STATUS**: `PENDING`
  * **COMMAND**: `curl -x socks5://user:pass@proxy_host:port http://ifconfig.me`
  * **EVIDÊNCIA**: Credenciais de túnel residencial indisponíveis localmente.
  * **PRÓXIMA AÇÃO**: Cadastrar token de proxy ativo no cockpit.

* **ETAPA 9: WHATSAPP VOICE REAL**
  * **STATUS**: `PENDING`
  * **COMMAND**: Disparo ativo via `/api/campaigns/{id}/execute`
  * **EVIDÊNCIA**: Conexão com dispositivo celular real pendente na VPS.
  * **PRÓXIMA AÇÃO**: Realizar o scanner de QR Code no painel operacional `/lines` usando chip físico.
