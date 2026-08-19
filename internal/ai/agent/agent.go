// Package agent defines the Agent interface and initial concrete implementations.
// Agents orchestrate the conversation turn loop on top of the AI provider.
package agent

import (
	"context"

	"wacalls/internal/ai/session"
)

// Input is one turn of user input.
type Input struct {
	Text       string
	AudioFrame []float32 // optional raw audio if agent needs it
}

// TurnResult is what an agent returns after processing a turn.
type TurnResult struct {
	Reply   string        // text to speak (passed to provider)
	Outcome *session.Outcome // non-nil signals the session should end
}

// Agent is the core conversation controller.
// Start/Stop lifecycle is managed by AIMediaAdapter.
type Agent interface {
	// ID returns the stable agent identifier.
	ID() string

	// Start initialises the agent for the given session.
	Start(ctx context.Context, sess *session.AISession) error

	// HandleInput processes one user turn and returns a reply.
	// The reply is sent to the AI provider as a text injection.
	HandleInput(ctx context.Context, in Input) (*TurnResult, error)

	// Interrupt is called when barge-in occurs (agent may reset state).
	Interrupt()

	// Stop gracefully finalises the agent and produces the outcome.
	Stop(ctx context.Context) (*session.Outcome, error)
}

// ─────────────────────────────────────────────
// SurveyAgent
// ─────────────────────────────────────────────

// SurveyAgent conducts structured surveys via voice.
type SurveyAgent struct {
	id   string
	sess *session.AISession
}

func NewSurveyAgent() *SurveyAgent {
	return &SurveyAgent{id: "survey_agent_v1"}
}

func (a *SurveyAgent) ID() string { return a.id }

func (a *SurveyAgent) Start(ctx context.Context, sess *session.AISession) error {
	a.sess = sess
	return nil
}

func (a *SurveyAgent) HandleInput(_ context.Context, in Input) (*TurnResult, error) {
	// In Grok Realtime mode, the provider handles full turn detection.
	// SurveyAgent can inject follow-up logic here when needed.
	return &TurnResult{Reply: in.Text}, nil
}

func (a *SurveyAgent) Interrupt() {
	// Reset any in-progress turn state if the agent tracks it.
}

func (a *SurveyAgent) Stop(_ context.Context) (*session.Outcome, error) {
	return &session.Outcome{Reason: "survey_completed"}, nil
}

// ─────────────────────────────────────────────
// SalesAgent
// ─────────────────────────────────────────────

// SalesAgent drives outbound sales conversations via voice.
type SalesAgent struct {
	id   string
	sess *session.AISession
}

func NewSalesAgent() *SalesAgent {
	return &SalesAgent{id: "sales_agent_v1"}
}

func (a *SalesAgent) ID() string { return a.id }

func (a *SalesAgent) Start(ctx context.Context, sess *session.AISession) error {
	a.sess = sess
	return nil
}

func (a *SalesAgent) HandleInput(_ context.Context, in Input) (*TurnResult, error) {
	return &TurnResult{Reply: in.Text}, nil
}

func (a *SalesAgent) Interrupt() {}

func (a *SalesAgent) Stop(_ context.Context) (*session.Outcome, error) {
	return &session.Outcome{Reason: "sales_call_ended"}, nil
}
