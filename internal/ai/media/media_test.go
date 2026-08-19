package media_test

import (
	"context"
	"math"
	"testing"
	"time"

	"wacalls/internal/ai/events"
	"wacalls/internal/ai/media"
	"wacalls/internal/ai/provider"
	"wacalls/internal/ai/session"
)

// Teste de conversão float32 ↔ PCM e ausência de clipping.
func TestPCMConversion_ClippingAndFormat(t *testing.T) {
	// 1. Amostras normais e extremas
	input := []float32{0.0, 0.5, -0.5, 1.0, -1.0, 2.0, -2.0} // 2.0 e -2.0 devem clipar para 1.0 e -1.0
	
	// Convert float32 to PCM
	pcm := provider.Float32ToPCM(input)
	if len(pcm) != len(input)*2 {
		t.Fatalf("expected PCM length %d, got %d", len(input)*2, len(pcm))
	}
	
	// Convert PCM back to float32
	back := provider.PCMToFloat32(pcm)
	if len(back) != len(input) {
		t.Fatalf("expected back samples length %d, got %d", len(input), len(back))
	}
	
	// Valida limites (clipping)
	for i, orig := range input {
		expected := orig
		if orig > 1.0 {
			expected = 1.0
		} else if orig < -1.0 {
			expected = -1.0
		}
		
		diff := math.Abs(float64(back[i] - expected))
		if diff > 0.001 {
			t.Errorf("sample[%d]: expected ~%f, got %f (orig %f)", i, expected, back[i], orig)
		}
	}
}

// Teste de loopback real
func TestAIMediaAdapter_Loopback(t *testing.T) {
	sess := session.NewAISession("s1", "t1", "ws-sess", "call-id-1", "agent-1", session.VoiceProfile{
		ID:       session.ProfileSurvey,
		Language: "pt-BR",
		BargeIn:  true,
	})
	bus := events.NewBus()
	log := session.NewRegistry() // reaproveita slog default indiretamente ou nil
	_ = log
	
	prov := provider.NewLoopbackProvider(nil)
	cfg := provider.Config{
		Model: "loopback",
	}
	
	var (
		receivedSamples []float32
		writeCount      int
	)
	writeFn := func(pcm []float32) {
		writeCount++
		receivedSamples = append(receivedSamples, pcm...)
	}
	
	adapter := media.NewAIMediaAdapter(sess, prov, cfg, bus, writeFn, nil)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("failed to start adapter: %v", err)
	}
	
	// Envia áudio senoidal
	testAudio := make([]float32, 320) // 1 frame
	for i := range testAudio {
		testAudio[i] = float32(math.Sin(float64(i) * 0.1))
	}
	
	adapter.OnPeerAudio(testAudio)
	
	// Aguarda processamento assíncrono do loopback
	time.Sleep(100 * time.Millisecond)
	
	adapter.Stop("test_completed")
	
	if writeCount == 0 {
		t.Error("expected writeFn to be called at least once")
	}
	if len(receivedSamples) < 320 {
		t.Errorf("expected at least 320 samples back, got %d", len(receivedSamples))
	}
}
