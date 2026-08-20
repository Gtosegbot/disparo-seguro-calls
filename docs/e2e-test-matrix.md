# DS Voice — E2E Test Matrix (Fase 12)

Esta matriz mapeia os testes determinísticos cobertos pela suíte do `harness_test.go` simulando a infraestrutura de rede e telefonia real de produção.

---

## 1. Matriz de Testes Automatizados

| Test Name | Component | Status | Verification Point |
| :--- | :--- | :---: | :--- |
| `TestHarness_PreflightValidation` | Preflight Checks | `PASS` | Impede campanhas sem ID, Tenant ou concorrência. |
| `TestHarness_GoldenPathExecution` | Execution Fabric | `PASS` | Simula fluxo completo com sucesso e atesta custos. |
| `TestHarness_DryRun` | Pre-flight Run | `PASS` | Confirma o simulado sem acionar tráfego de rede. |
| `TestHarness_FailureInjection` | Error Handling | `PASS` | Injeta falhas de timeout de IA e offline de CRM ChatSeguro. |
| `TestOrchestration_CircuitBreaker`| CB Engine | `PASS` | Transita estados CLOSED/OPEN de provedores degradados. |
| `TestOrchestration_LeadClaiming` | Leads Pool | `PASS` | Garante claim atômico e exclusivo sob concorrência de 500 threads. |
