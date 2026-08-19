// Package provider defines the provider-agnostic interfaces for AI voice.
// No "if grok / if gemini" logic belongs outside this package.
package provider

import "context"

// PCMChunk carries raw PCM_S16LE audio in the canonical internal format:
//   - 16 kHz, mono, 20 ms frames → 640 bytes per frame
type PCMChunk struct {
	Data []byte // always 640 bytes for a full frame; may be shorter at end
}

// Provider is the interface every AI voice backend must implement.
// The core never imports a concrete provider; it talks only to this interface.
type Provider interface {
	// Name returns a stable identifier, e.g. "grok_realtime".
	Name() string

	// Start initialises the provider session and returns a channel that
	// emits synthesized PCM audio back to the caller.
	// ctx cancellation triggers orderly shutdown.
	Start(ctx context.Context, cfg Config) (<-chan PCMChunk, error)

	// SendAudio delivers one PCM frame from the caller to the provider.
	SendAudio(chunk PCMChunk) error

	// Interrupt signals barge-in: discard current TTS output immediately.
	Interrupt() error

	// Stop cleanly terminates the provider session.
	Stop() error
}

// Config carries per-session provider settings. No raw API keys — those
// are resolved server-side from the ProviderRegistry.
type Config struct {
	Model         string         // e.g. "grok-2-voice-preview"
	Voice         string         // provider-specific voice name
	SystemPrompt  string         // final assembled prompt
	Language      string         // e.g. "pt-BR"
	MaxDuration   int            // seconds; 0 = unlimited
	BargeIn       bool
	Extra         map[string]any // provider-specific overrides
}

// Registry maps provider names to their factories.
// API keys are stored here, not in VoiceProfile.
type Registry struct {
	factories map[string]Factory
	secrets   map[string]string // providerName -> API key (never exposed to client)
}

// Factory creates a Provider instance.
type Factory func() Provider

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
		secrets:   make(map[string]string),
	}
}

// Register adds a provider factory and its secret.
func (r *Registry) Register(name, apiKey string, f Factory) {
	r.factories[name] = f
	r.secrets[name] = apiKey
}

// Resolve returns a configured Provider for the given name.
// It injects the API key internally; callers never see it.
func (r *Registry) Resolve(name string) (Provider, string, bool) {
	f, ok := r.factories[name]
	if !ok {
		return nil, "", false
	}
	secret := r.secrets[name]
	return f(), secret, true
}
