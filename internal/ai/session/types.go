// Package session defines the core abstractions of the Disparo Seguro AI voice layer.
// It sits ABOVE the AstraCalls core (internal/voip) without modifying it.
package session

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// ─────────────────────────────────────────────
// State machine
// ─────────────────────────────────────────────

// AIState represents the lifecycle of one AI-driven call leg.
type AIState string

const (
	StateCreated     AIState = "CREATED"
	StateDialing     AIState = "DIALING"
	StateRinging     AIState = "RINGING"
	StateConnected   AIState = "CONNECTED"
	StateListening   AIState = "LISTENING"
	StateThinking    AIState = "THINKING"
	StateSpeaking    AIState = "SPEAKING"
	StateInterrupted AIState = "INTERRUPTED"
	StateEnding      AIState = "ENDING"
	StateEnded       AIState = "ENDED"
	StateError       AIState = "ERROR"
)

// ─────────────────────────────────────────────
// VoiceProfile
// ─────────────────────────────────────────────

// ProfileID identifies a named voice behaviour configuration.
type ProfileID string

const (
	ProfileSurvey           ProfileID = "survey"
	ProfileSales            ProfileID = "sales"
	ProfileSupport          ProfileID = "support"
	ProfileTechnical        ProfileID = "technical_support"
	ProfileDynamicInterview ProfileID = "dynamic_interviewer"
)

// ProviderPolicy enumerates how the AI layer selects a provider.
type ProviderPolicy string

const (
	PolicyPrimary   ProviderPolicy = "primary"
	PolicyCostFirst ProviderPolicy = "cost_first"
	PolicyLatency   ProviderPolicy = "latency"
)

// VoiceProfile is a tenant-visible, key-free configuration blob.
// It never stores API keys; those live in the backend ProviderRegistry.
type VoiceProfile struct {
	ID             ProfileID      `json:"id"`
	Version        string         `json:"version"`
	Language       string         `json:"language"`
	Voice          string         `json:"voice"`
	Prompt         string         `json:"prompt"`
	Tools          []string       `json:"tools,omitempty"`
	ProviderPolicy ProviderPolicy `json:"provider_policy"`
	MaxDuration    time.Duration  `json:"max_duration"`
	BargeIn        bool           `json:"barge_in"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// ─────────────────────────────────────────────
// PromptContext
// ─────────────────────────────────────────────

// PromptContext assembles the final prompt from layered sources.
type PromptContext struct {
	PlatformRules   string
	ProfilePrompt   string
	SessionContext  string
	BusinessContext string
	TaskPrompt      string
}

// Build returns the assembled prompt, its version string, and its SHA-256 hash.
func (pc *PromptContext) Build() (prompt, version, hash string) {
	prompt = fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s",
		pc.PlatformRules,
		pc.ProfilePrompt,
		pc.SessionContext,
		pc.BusinessContext,
		pc.TaskPrompt,
	)
	version = "1.0"
	sum := sha256.Sum256([]byte(prompt))
	hash = fmt.Sprintf("%x", sum)
	return
}

// ─────────────────────────────────────────────
// TurnEntry / Outcome
// ─────────────────────────────────────────────

// TurnEntry is one exchange in the session transcript.
type TurnEntry struct {
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

// Outcome summarises what happened after the session ends.
type Outcome struct {
	Reason      string         `json:"reason"`
	Transcript  []TurnEntry    `json:"transcript"`
	Duration    time.Duration  `json:"duration"`
	AudioFrames int            `json:"audio_frames"`
	TTFBms      int64          `json:"ttfb_ms"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ─────────────────────────────────────────────
// AISession
// ─────────────────────────────────────────────

// AISession represents one AstraCalls call leg associated with an AI agent.
// Thread-safe. It knows nothing about Grok or Gemini directly.
type AISession struct {
	mu sync.RWMutex

	ID        string
	TenantID  string
	SessionID string // AstraCalls session id
	CallID    string // AstraCalls call id
	AgentID   string
	Profile   VoiceProfile
	Provider  string // e.g. "grok_realtime"
	Model     string // e.g. "grok-2-voice-preview"

	state     AIState
	CreatedAt time.Time
	StartedAt *time.Time
	EndedAt   *time.Time

	Metadata map[string]any

	startedOnce sync.Once
	stoppedOnce sync.Once

	outcome       *Outcome
	onStateChange func(*AISession, AIState)
}

// NewAISession creates a session in StateCreated.
func NewAISession(id, tenantID, sessionID, callID, agentID string, profile VoiceProfile) *AISession {
	return &AISession{
		ID:        id,
		TenantID:  tenantID,
		SessionID: sessionID,
		CallID:    callID,
		AgentID:   agentID,
		Profile:   profile,
		state:     StateCreated,
		CreatedAt: time.Now().UTC(),
		Metadata:  make(map[string]any),
	}
}

// State returns the current state (thread-safe read).
func (s *AISession) State() AIState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetState transitions the state and fires the optional callback.
func (s *AISession) SetState(next AIState) {
	s.mu.Lock()
	s.state = next
	cb := s.onStateChange
	s.mu.Unlock()
	if cb != nil {
		cb(s, next)
	}
}

// OnStateChange registers a callback invoked on every state transition.
func (s *AISession) OnStateChange(fn func(*AISession, AIState)) {
	s.mu.Lock()
	s.onStateChange = fn
	s.mu.Unlock()
}

// MarkStarted records the start timestamp (idempotent).
func (s *AISession) MarkStarted() {
	s.startedOnce.Do(func() {
		s.mu.Lock()
		t := time.Now().UTC()
		s.StartedAt = &t
		s.mu.Unlock()
	})
}

// MarkEnded records the end timestamp (idempotent) and sets the outcome.
func (s *AISession) MarkEnded(o *Outcome) {
	s.stoppedOnce.Do(func() {
		s.mu.Lock()
		t := time.Now().UTC()
		s.EndedAt = &t
		s.outcome = o
		s.mu.Unlock()
	})
}

// GetOutcome returns the outcome (nil if session has not ended).
func (s *AISession) GetOutcome() *Outcome {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.outcome
}

// Snapshot returns a serializable view (never contains secrets).
func (s *AISession) Snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{
		"id":         s.ID,
		"tenant_id":  s.TenantID,
		"session_id": s.SessionID,
		"call_id":    s.CallID,
		"agent_id":   s.AgentID,
		"provider":   s.Provider,
		"model":      s.Model,
		"state":      string(s.state),
		"created_at": s.CreatedAt,
		"started_at": s.StartedAt,
		"ended_at":   s.EndedAt,
		"profile_id": string(s.Profile.ID),
	}
}
