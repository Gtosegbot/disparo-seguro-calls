# DS Voice — Operator Handoff & VPS Access Bootstrap (Fase 19)

Este documento atua como o manual oficial para o operador de infraestrutura realizar a ativação física do sistema de chamadas em produção.

---

## 1. Current Status

* **Status**: `PHASE_19_BLOCKED_AT_OPERATOR_INPUT`
* **Motivo**: Credenciais SSH da VPS e chaves dos provedores de telefonia e IA não configuradas no terminal do agente.

---

## 2. Required VPS Access

Para que o deploy possa ser ativado, o operador de infraestrutura precisa prover os seguintes parâmetros:
* **VPS_HOST**: Endereço IP público da VPS.
* **VPS_PORT**: Porta SSH (padrão `22`).
* **VPS_USER**: Nome do usuário administrativo (`root` ou com privilégios de sudo).
* **SSH_KEY**: Chave privada PEM autorizada para login.

---

## 3. Required Environment Variables

O operador deve criar o arquivo `.env` na raiz do repositório clonado na VPS contendo os segredos reais:
* `WACALLS_XAI_KEY`: API Key do Grok Realtime.
* `WACALLS_GEMINI_KEY`: API Key do Gemini Live.
* `DATABASE_URL`: URI de conexão segura do Postgres (PostgreSQL v15+).
* `REDIS_URL`: URI de conexão do Redis (Redis v7+).
* `CHATSEGURO_TOKEN`: Token de autenticação do CRM.
* `PROXY_HOST` / `PROXY_PORT` / `PROXY_PASSWORD`: Credenciais do túnel anti-ban.

---

## 4. Next Physical Action

1. **Acesso SSH**: O operador deve acessar a VPS:
   ```bash
   ssh -i /path/to/key.pem user@vps_ip
   ```
2. **Clonar Repositório**:
   ```bash
   git clone https://github.com/Gtosegbot/disparo-seguro-calls.git
   cd disparo-seguro-calls
   ```
3. **Provisionar o .env**: Criar o arquivo `.env` preenchido com as chaves reais.
4. **Executar o Deploy**:
   ```bash
   bash ./scripts/production/deploy.sh
   ```
5. **Pareamento de Linha**: Acessar o painel no navegador na porta `8080/lines`, escanear o QR Code de teste e iniciar a chamada controlada.
