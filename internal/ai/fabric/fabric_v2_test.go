package fabric_test

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"sync"
	"testing"
	"time"

	"wacalls/internal/ai/fabric"
	"wacalls/internal/ai/provider"
	"wacalls/internal/ai/session"
)

func TestFabricV2_CapabilityMatching(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register("grok_realtime", "key", func() provider.Provider { return &mockRealtimeProvider{name: "grok_realtime"} })
	registry.Register("gemini_realtime", "key", func() provider.Provider { return &mockRealtimeProvider{name: "gemini_realtime"} })
	registry.Register("loopback", "", func() provider.Provider { return &mockRealtimeProvider{name: "loopback"} })

	logger := slog.New(slog.NewTextHandler(&nullWriter{}, nil))
	fab := fabric.NewFabric(registry, logger)
	ctx := context.Background()

	// 1. Solicita idioma não suportado (ex.: "ru-RU") -> Deve falhar ou não achar elegíveis
	_, _, err := fab.ResolveProvider(ctx, session.ProviderPolicy("primary"), "ru-RU")
	if err == nil {
		t.Error("expected error for unsupported language, got nil")
	}

	// 2. Solicita idioma suportado (pt-BR) -> Deve retornar com sucesso
	prov, _, err := fab.ResolveProvider(ctx, session.ProviderPolicy("primary"), "pt-BR")
	if err != nil {
		t.Fatalf("failed to resolve supported lang: %v", err)
	}
	if prov == nil {
		t.Fatal("resolved provider is nil")
	}
}

func TestFabricV2_CostAndPolicyRouting(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register("grok_realtime", "key", func() provider.Provider { return &mockRealtimeProvider{name: "grok_realtime"} })
	registry.Register("gemini_realtime", "key", func() provider.Provider { return &mockRealtimeProvider{name: "gemini_realtime"} })
	registry.Register("loopback", "", func() provider.Provider { return &mockRealtimeProvider{name: "loopback"} })

	logger := slog.New(slog.NewTextHandler(&nullWriter{}, nil))
	fab := fabric.NewFabric(registry, logger)
	ctx := context.Background()

	// 1. Testar política Economy -> Deve priorizar loopback (custo 0) ou gemini_realtime (custo 5)
	provEco, _, err := fab.ResolveProvider(ctx, session.PolicyCostFirst, "pt-BR")
	if err != nil {
		t.Fatalf("failed Resolve: %v", err)
	}
	if provEco.Name() != "loopback" {
		t.Errorf("expected cheapest provider 'loopback', got %s", provEco.Name())
	}

	// Desabilita loopback para testar o próximo mais barato
	_ = fab.SetProviderStatus("loopback", false, 3, 0)
	
	provEco2, _, _ := fab.ResolveProvider(ctx, session.PolicyCostFirst, "pt-BR")
	if provEco2.Name() != "gemini_realtime" {
		t.Errorf("expected gemini_realtime as cheapest after loopback disabled, got %s", provEco2.Name())
	}
}

func TestFabricV2_WeightedRoutingDistribution(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register("grok_realtime", "key", func() provider.Provider { return &mockRealtimeProvider{name: "grok_realtime"} })
	registry.Register("gemini_realtime", "key", func() provider.Provider { return &mockRealtimeProvider{name: "gemini_realtime"} })
	registry.Register("loopback", "", func() provider.Provider { return &mockRealtimeProvider{name: "loopback"} })

	logger := slog.New(slog.NewTextHandler(&nullWriter{}, nil))
	fab := fabric.NewFabric(registry, logger)
	ctx := context.Background()

	// Desabilita loopback para focar a distribuição em Grok e Gemini
	_ = fab.SetProviderStatus("loopback", false, 3, 0)
	
	// Define pesos: Grok = 80%, Gemini = 20%
	_ = fab.SetProviderStatus("grok_realtime", true, 1, 80)
	_ = fab.SetProviderStatus("gemini_realtime", true, 2, 20)

	grokCount := 0
	geminiCount := 0

	for i := 0; i < 100; i++ {
		prov, _, err := fab.ResolveProvider(ctx, session.ProviderPolicy("Weighted"), "pt-BR")
		if err != nil {
			t.Fatalf("failed Resolve: %v", err)
		}
		if prov.Name() == "grok_realtime" {
			grokCount++
		} else if prov.Name() == "gemini_realtime" {
			geminiCount++
		}
	}

	if grokCount == 0 || geminiCount == 0 {
		t.Errorf("expected weighted distribution across both providers, got grok=%d, gemini=%d", grokCount, geminiCount)
	}
}

func TestFabricV2_ChainFallbackAndHealth(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register("grok_realtime", "key", func() provider.Provider { return &mockRealtimeProvider{name: "grok_realtime"} })
	registry.Register("gemini_realtime", "key", func() provider.Provider { return &mockRealtimeProvider{name: "gemini_realtime"} })
	registry.Register("loopback", "", func() provider.Provider { return &mockRealtimeProvider{name: "loopback"} })

	logger := slog.New(slog.NewTextHandler(&nullWriter{}, nil))
	fab := fabric.NewFabric(registry, logger)
	ctx := context.Background()

	// 1. Simular erro no Grok -> Deve rotacionar para Gemini
	prov, _, err := fab.HandleFallback(ctx, "grok_realtime", errors.New("WS error"))
	if err != nil {
		t.Fatalf("fallback failed: %v", err)
	}
	if prov.Name() != "gemini_realtime" {
		t.Errorf("expected fallback gemini_realtime, got %s", prov.Name())
	}

	// 2. Simular erro no Gemini -> Deve rotacionar para Loopback
	prov2, _, err := fab.HandleFallback(ctx, "gemini_realtime", errors.New("API limit"))
	if err != nil {
		t.Fatalf("fallback failed: %v", err)
	}
	if prov2.Name() != "loopback" {
		t.Errorf("expected fallback loopback, got %s", prov2.Name())
	}
}

func TestFabricV2_ConcurrencyScale(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register("grok_realtime", "key", func() provider.Provider { return &mockRealtimeProvider{name: "grok_realtime"} })
	registry.Register("gemini_realtime", "key", func() provider.Provider { return &mockRealtimeProvider{name: "gemini_realtime"} })
	registry.Register("loopback", "", func() provider.Provider { return &mockRealtimeProvider{name: "loopback"} })

	logger := slog.New(slog.NewTextHandler(&nullWriter{}, nil))
	fab := fabric.NewFabric(registry, logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(1000)

	// Simula 1000 goroutines acessando concorrentemente o Fabric
	for i := 0; i < 1000; i++ {
		go func(idx int) {
			defer wg.Done()
			
			// Seleciona política aleatoriamente
			policies := []session.ProviderPolicy{session.PolicyCostFirst, session.PolicyLatency, "Premium"}
			policy := policies[rand.Intn(len(policies))]

			prov, _, err := fab.ResolveProvider(ctx, policy, "pt-BR")
			if err == nil && prov != nil {
				// Simula finalização de sessão aleatória logando métricas
				duration := time.Duration(rand.Intn(60)) * time.Second
				success := rand.Float32() > 0.1
				ttfb := int64(rand.Intn(200) + 50)
				fab.CompleteSession(prov.Name(), duration, success, ttfb)
			}
		}(i)
	}

	wg.Wait()
	
	// Confirma que o catálogo de observabilidade compilou as métricas sem quebrar ou dar data races
	catalog := fab.GetCatalog()
	if len(catalog) == 0 {
		t.Error("expected catalog statistics to be populated")
	}
}
