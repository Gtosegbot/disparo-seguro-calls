package fabric_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"wacalls/internal/ai/fabric"
	"wacalls/internal/ai/provider"
	"wacalls/internal/ai/session"
)

func TestFabric_ResolveProviderPolicies(t *testing.T) {
	registry := provider.NewRegistry()
	
	// Registrar mock do Grok e Gemini no registry
	registry.Register("grok_realtime", "grok-key", func() provider.Provider {
		return &mockRealtimeProvider{name: "grok_realtime"}
	})
	registry.Register("gemini_realtime", "gemini-key", func() provider.Provider {
		return &mockRealtimeProvider{name: "gemini_realtime"}
	})
	registry.Register("loopback", "", func() provider.Provider {
		return &mockRealtimeProvider{name: "loopback"}
	})

	logger := slog.New(slog.NewTextHandler(&nullWriter{}, nil))
	fab := fabric.NewFabric(registry, logger)
	ctx := context.Background()

	// 1. Validar política padrão (Primary) -> Deve retornar grok_realtime se saudável
	prov, _, err := fab.ResolveProvider(ctx, session.ProviderPolicy("primary"), "premium")
	if err != nil {
		t.Fatalf("failed to resolve standard provider: %v", err)
	}
	if prov.Name() != "grok_realtime" {
		t.Errorf("expected grok_realtime, got %s", prov.Name())
	}

	// 2. Validar política CostFirst -> Deve retornar loopback (custo 0.0) ou gemini_realtime se loopback não estiver no registry
	provCost, _, err := fab.ResolveProvider(ctx, session.PolicyCostFirst, "economy")
	if err != nil {
		t.Fatalf("failed to resolve cheapest provider: %v", err)
	}
	if provCost.Name() != "loopback" {
		t.Errorf("expected loopback as cheapest, got %s", provCost.Name())
	}
}

func TestFabric_AutomaticFallback(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register("grok_realtime", "grok-key", func() provider.Provider {
		return &mockRealtimeProvider{name: "grok_realtime"}
	})
	registry.Register("gemini_realtime", "gemini-key", func() provider.Provider {
		return &mockRealtimeProvider{name: "gemini_realtime"}
	})

	logger := slog.New(slog.NewTextHandler(&nullWriter{}, nil))
	fab := fabric.NewFabric(registry, logger)
	ctx := context.Background()

	// 1. Simula erro no Grok
	originalErr := errors.New("websocket handshake timeout")
	
	// 2. Executa fallback
	prov, _, err := fab.HandleFallback(ctx, "grok_realtime", originalErr)
	if err != nil {
		t.Fatalf("fallback execution failed: %v", err)
	}

	if prov.Name() != "gemini_realtime" {
		t.Errorf("expected fallback to route to gemini_realtime, got %s", prov.Name())
	}
}

// ─── Mocks ───────────────────────────────────────────────────────────────────

type mockRealtimeProvider struct {
	name string
}

func (m *mockRealtimeProvider) Name() string { return m.name }
func (m *mockRealtimeProvider) Start(ctx context.Context, cfg provider.Config) (<-chan provider.PCMChunk, error) {
	out := make(chan provider.PCMChunk)
	close(out)
	return out, nil
}
func (m *mockRealtimeProvider) SendAudio(chunk provider.PCMChunk) error { return nil }
func (m *mockRealtimeProvider) Interrupt() error                        { return nil }
func (m *mockRealtimeProvider) Stop() error                             { return nil }

type nullWriter struct{}

func (n *nullWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
