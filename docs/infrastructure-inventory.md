# DS Voice — Infrastructure Inventory (Fase 13)

Este documento descreve o mapeamento detalhado da infraestrutura física da VPS necessária para a ativação de chamadas de voz reais do **DS Voice 2.0 / OmniRoute**.

---

## 1. Mapeamento de Recursos Físicos da VPS

* **VPS OS**: Ubuntu 22.04 LTS (ou superior) recomendado para builds de containers Docker.
* **CPU / RAM**: Mínimo 2 vCPUs / 4GB RAM recomendado para processar concorrência de mídia sem latência (jitter).
* **Disco**: 20GB SSD disponível para logs persistentes e base de dados.
* **Docker / Compose**: Docker Engine v24.0+ e Docker Compose v2.20+ recomendados.
* **Rede**:
  * Porta `80/443` (HTTP/HTTPS para REST API e Webhooks).
  * Portas UDP de telefonia VoIP RTP abertas no firewall.
* **Serviços do Sistema**:
  * **PostgreSQL**: Postgres v15+ com suporte a conexões simultâneas do pool de tenants.
  * **Redis**: Redis v7+ para coordenação de locks atômicos de claim de leads.

---

## 2. Configurações de Variáveis de Ambiente (Secrets)

As seguintes variáveis de ambiente devem ser exportadas no `.env` da VPS (nunca comitadas no código ou salvas no frontend):
* `WACALLS_API_KEY`: Chave master do administrador para criar e gerenciar tenants.
* `WACALLS_XAI_KEY`: API Key do Provedor Grok Realtime.
* `WACALLS_GEMINI_KEY`: API Key do Provedor Gemini Live.
* `DATABASE_URL`: URI de conexão segura do Postgres (SSL ativo).
* `REDIS_URL`: URI de conexão do Redis.
