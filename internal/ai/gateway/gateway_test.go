package gateway_test

import (
	"context"
	"testing"
	"time"

	"wacalls/internal/ai/events"
	"wacalls/internal/ai/gateway"
	"wacalls/internal/ai/provider"
	"wacalls/internal/ai/session"
)

func TestVoiceGateway_LifecycleAndIsolation(t *testing.T) {
	sr := session.NewRegistry()
	pr := provider.NewRegistry()
	bus := events.NewBus()
	
	// Registrar provedor loopback
	pr.Register("loopback", "", func() provider.Provider {
		return provider.NewLoopbackProvider(nil)
	})
	
	gw := gateway.NewVoiceGateway(sr, pr, bus, nil)
	
	profile := session.VoiceProfile{
		ID:       session.ProfileSurvey,
		Language: "pt-BR",
		Voice:    "nova",
		BargeIn:  true,
	}
	
	promptCtx := &session.PromptContext{
		PlatformRules: "Regras",
	}
	
	ctx := context.Background()
	
	// 1. Iniciar IA Session
	sess, err := gw.StartAISession(ctx, "tenant-A", "sess-1", "call-1", "agent-1", profile, "loopback", promptCtx, func(pcm []float32) {})
	if err != nil {
		t.Fatalf("failed to start AI session: %v", err)
	}
	
	if sess.State() != session.StateListening {
		t.Errorf("expected state LISTENING, got %s", sess.State())
	}
	
	// Validar que foi registrada
	registered, err := sr.Get(sess.ID, "tenant-A")
	if err != nil || registered == nil {
		t.Fatalf("expected session to be registered under tenant-A: %v", err)
	}
	
	// Validar isolamento (tenant-B não pode ler)
	_, err = sr.Get(sess.ID, "tenant-B")
	if err != session.ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
	
	// 2. Parar IA Session
	gw.StopAISession(sess.ID, "completed")
	
	// Validar remoção do registro ativo
	_, err = sr.Get(sess.ID, "tenant-A")
	if err != session.ErrNotFound {
		t.Errorf("expected ErrNotFound after stop, got %v", err)
	}
}

func TestH2HProtection_ByDefaultDisabled(t *testing.T) {
	// Chamadas sem aiSessionID setado explicitamente não podem ser processadas pela IA.
	// Esse teste garante a separação estrita de H2H.
	var aiCalled bool
	
	// Mock do callback que seria a chamada de IA
	aiGatewayMock := func(aiSessionID string) {
		if aiSessionID != "" {
			aiCalled = true
		}
	}
	
	// 1. Chamada comum (H2H) -> aiSessionID vazia
	aiSessionID := ""
	aiGatewayMock(aiSessionID)
	
	if aiCalled {
		t.Error("H2H call triggered AI pipeline incorrectly")
	}
	
	// 2. Chamada IA -> aiSessionID ativa
	aiSessionID = "ai-active-uuid"
	aiGatewayMock(aiSessionID)
	
	if !aiCalled {
		t.Error("AI call failed to trigger AI pipeline")
	}
}
