# DS Voice — VPS Deployment Guide (Fase 14)

Este documento atua como o manual oficial para o provisionamento e deploy em produção da aplicação **Disparo Seguro Calls + DS Voice**.

---

## 1. Passo a Passo do Deploy Real

1. **Configuração de Segredos**:
   * Copie o arquivo template de ambiente:
     ```bash
     cp .env.example .env
     ```
   * Preencha as chaves reais de provedores (`WACALLS_XAI_KEY`, `WACALLS_GEMINI_KEY`) no arquivo `.env`.

2. **Orquestração via Docker Compose**:
   * Certifique-se de que o Docker daemon esteja ativo na VPS.
   * Inicialize a stack de containers em background:
     ```bash
     docker compose up -d --build
     ```
   * Verifique o status das rotas e serviços:
     ```bash
     docker compose ps
     docker compose logs -f wacalls
     ```

3. **Verificação de Health Checks**:
   * Verifique o endpoint de liveness:
     ```bash
     curl -f http://localhost:8080/health
     ```
   * Verifique o endpoint de prontidão (readiness) que valida a comunicação atômica com o Postgres:
     ```bash
     curl -f http://localhost:8080/ready
     ```
