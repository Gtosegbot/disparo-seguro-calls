// Package provider - tests for provider registry and PCM format contract.
package provider_test

import (
	"context"
	"testing"

	"wacalls/internal/ai/provider"
)

func TestRegistry_RegisterAndResolve(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register("mock", "secret-key-mock", func() provider.Provider {
		return &mockProvider{}
	})
	prov, _, ok := reg.Resolve("mock")
	if !ok || prov == nil {
		t.Fatal("expected to resolve mock provider")
	}
	if prov.Name() != "mock" {
		t.Errorf("expected name 'mock', got %q", prov.Name())
	}
}

func TestRegistry_UnknownProvider(t *testing.T) {
	reg := provider.NewRegistry()
	_, _, ok := reg.Resolve("nonexistent")
	if ok {
		t.Error("expected false for unknown provider")
	}
}

func TestRegistry_SecretReturnedInternally(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register("safe", "super-secret-api-key", func() provider.Provider {
		return &mockProvider{}
	})
	_, apiKey, ok := reg.Resolve("safe")
	if !ok {
		t.Fatal("should resolve")
	}
	// The key must be returned to the internal gateway layer only.
	if apiKey == "" {
		t.Error("registry must return the key internally for gateway use")
	}
}

// Canonical PCM format contract.
func TestPCMFormat(t *testing.T) {
	const (
		sampleRate    = 16000
		frameDuration = 0.020
		bytesPerSample = 2
		channels       = 1
	)
	frameBytes := int(sampleRate * frameDuration * bytesPerSample * channels)
	if frameBytes != 640 {
		t.Errorf("canonical frame must be 640 bytes, got %d", frameBytes)
	}
}

func TestFloat32ToPCMRoundtrip(t *testing.T) {
	samples := []float32{0.0, 1.0, -1.0, 0.5, -0.5}
	pcm := provider.Float32ToPCM(samples)
	back := provider.PCMToFloat32(pcm)
	if len(back) != len(samples) {
		t.Fatalf("length mismatch: %d vs %d", len(back), len(samples))
	}
	// Allow 1-LSB rounding error
	for i, s := range samples {
		diff := float64(back[i] - s)
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("sample[%d]: expected ~%f, got %f", i, s, back[i])
		}
	}
}

// ─── mockProvider ─────────────────────────────────────────────────────────────

type mockProvider struct{}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Start(_ context.Context, _ provider.Config) (<-chan provider.PCMChunk, error) {
	ch := make(chan provider.PCMChunk)
	close(ch)
	return ch, nil
}
func (m *mockProvider) SendAudio(_ provider.PCMChunk) error { return nil }
func (m *mockProvider) Interrupt() error                    { return nil }
func (m *mockProvider) Stop() error                         { return nil }
