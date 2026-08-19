// Package gateway defines the VoiceGateway, the single integration point
// between AstraCalls and the AI provider stack.
// AstraCalls calls gateway.HandlePeerAudio(); everything else is internal.
package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"wacalls/internal/ai/agent"
	"wacalls/internal/ai/events"
	"wacalls/internal/ai/media"
	"wacalls/internal/ai/provider"
	"wacalls/internal/ai/session"
)

// VoiceGateway orchestrates the full AI voice lifecycle for one call leg.
//
// Responsibilities:
//   - Create and register AISession
//   - Resolve provider from Registry
//   - Instantiate and connect AIMediaAdapter
//   - Forward audio from AstraCalls to the adapter
//   - Route synthesised audio back to AstraCalls
type VoiceGateway struct {
	log      *slog.Logger
	registry *session.Registry
	providers *provider.Registry
	bus      *events.Bus

	mu       sync.Mutex
	adapters map[string]*media.AIMediaAdapter // sessionID -> adapter
}

// NewVoiceGateway creates a gateway.
func NewVoiceGateway(
	reg *session.Registry,
	provs *provider.Registry,
	bus *events.Bus,
	log *slog.Logger,
) *VoiceGateway {
	if log == nil {
		log = slog.Default()
	}
	return &VoiceGateway{
		log:      log,
		registry: reg,
		providers: provs,
		bus:      bus,
		adapters: make(map[string]*media.AIMediaAdapter),
	}
}

// StartAISession creates a new AISession, connects the provider, and starts audio flow.
// writeFn is called with synthesised audio frames to be fed back into AstraCalls.
func (g *VoiceGateway) StartAISession(
	ctx context.Context,
	tenantID, astracallsSessionID, callID, agentID string,
	profile session.VoiceProfile,
	providerName string,
	promptCtx *session.PromptContext,
	writeFn media.WriteFunc,
) (*session.AISession, error) {
	// Resolve provider
	prov, apiKey, ok := g.providers.Resolve(providerName)
	if !ok {
		return nil, fmt.Errorf("voice_gateway: unknown provider %q", providerName)
	}
	_ = apiKey // already embedded inside the concrete provider by the Registry

	// Build final prompt
	prompt, _, _ := promptCtx.Build()

	// Build provider config
	cfg := provider.Config{
		Model:        profile.Voice, // re-use voice field as model hint if not set
		Voice:        profile.Voice,
		SystemPrompt: prompt,
		Language:     profile.Language,
		MaxDuration:  int(profile.MaxDuration / time.Second),
		BargeIn:      profile.BargeIn,
	}

	// Create session
	sessID := uuid.New().String()
	sess := session.NewAISession(sessID, tenantID, astracallsSessionID, callID, agentID, profile)
	sess.Provider = providerName
	sess.Model = cfg.Model
	g.registry.Register(sess)

	// Wire state → events
	sess.OnStateChange(func(s *session.AISession, state session.AIState) {
		g.bus.Publish(events.Event{
			Kind:      stateToEventKind(state),
			SessionID: s.ID,
			TenantID:  s.TenantID,
			CallID:    s.CallID,
		})
	})

	// Create adapter
	adapter := media.NewAIMediaAdapter(sess, prov, cfg, g.bus, writeFn, g.log)

	g.mu.Lock()
	g.adapters[sessID] = adapter
	g.mu.Unlock()

	if err := adapter.Start(ctx); err != nil {
		g.registry.Remove(sessID)
		g.mu.Lock()
		delete(g.adapters, sessID)
		g.mu.Unlock()
		return nil, fmt.Errorf("voice_gateway: adapter start: %w", err)
	}

	g.log.Info("voice_gateway: ai session started",
		"session_id", sessID,
		"tenant", tenantID,
		"call_id", callID,
		"provider", providerName,
	)
	return sess, nil
}

// HandlePeerAudio is called by AstraCalls for every incoming audio frame.
// This is the ONLY call AstraCalls needs to make into the AI layer.
func (g *VoiceGateway) HandlePeerAudio(sessionID string, samples []float32) {
	g.mu.Lock()
	adapter, ok := g.adapters[sessionID]
	g.mu.Unlock()
	if !ok {
		return
	}
	adapter.OnPeerAudio(samples)
}

// StopAISession terminates the AI session.
func (g *VoiceGateway) StopAISession(sessionID, reason string) {
	g.mu.Lock()
	adapter, ok := g.adapters[sessionID]
	if ok {
		delete(g.adapters, sessionID)
	}
	g.mu.Unlock()

	if ok {
		adapter.Stop(reason)
		g.registry.Remove(sessionID)
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func stateToEventKind(state session.AIState) events.Kind {
	switch state {
	case session.StateListening:
		return events.KindAIListening
	case session.StateThinking:
		return events.KindAIThinking
	case session.StateSpeaking:
		return events.KindAISpeaking
	case session.StateInterrupted:
		return events.KindAIInterrupted
	case session.StateEnded:
		return events.KindAIEnded
	case session.StateError:
		return events.KindAIError
	default:
		return events.KindAIStarted
	}
}

// ─────────────────────────────────────────────
// AgentRegistry — maps agentID strings to Agent factories
// ─────────────────────────────────────────────

// AgentFactory creates an Agent.
type AgentFactory func() agent.Agent

// AgentRegistry maps stable agentID strings to factories.
type AgentRegistry struct {
	mu      sync.RWMutex
	entries map[string]AgentFactory
}

func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{entries: make(map[string]AgentFactory)}
}

func (r *AgentRegistry) Register(id string, f AgentFactory) {
	r.mu.Lock()
	r.entries[id] = f
	r.mu.Unlock()
}

func (r *AgentRegistry) Resolve(id string) (agent.Agent, bool) {
	r.mu.RLock()
	f, ok := r.entries[id]
	r.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return f(), true
}
