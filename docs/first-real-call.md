# DS Voice — First Real Call Checklist & Guide (Fase 14/15)

Este documento atua como o checklist operacional obrigatório para a execução do Golden Path físico.

---

## 1. Passo a Passo da Primeira Chamada Real

1. **Precheck de Linha**:
   * Acesse a tela `/lines` no painel.
   * Conecte um chip de WhatsApp de teste escaneando o QR Code.
   * Assegure que o status da linha mude para `CONNECTED`.

2. **Preflight & Dry Run**:
   * Configure uma campanha de teste contendo apenas **1 lead** (número autorizado de testes).
   * Execute um Dry Run para validar a comunicação de ponta a ponta sem discagem física:
     ```bash
     POST /api/campaigns/{id}/execute (com a flag dry_run=true)
     ```

3. **Disparo Físico (First Call)**:
   * Execute a chamada real.
   * Acompanhe os logs em background:
     ```bash
     docker compose logs -f wacalls
     ```
   * Monitore outcomes, tempos de resposta e a correlação no Dashboard de custos.
