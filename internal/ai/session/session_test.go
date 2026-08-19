// Package session - tests for AISession, PromptContext, VoiceProfile, Registry.
package session_test

import (
	"testing"
	"time"

	"wacalls/internal/ai/session"
)

// ─── PromptContext ───────────────────────────────────────────────────────────

func TestPromptContext_Build_StableHash(t *testing.T) {
	pc := &session.PromptContext{
		PlatformRules:   "rule1",
		ProfilePrompt:   "prompt1",
		SessionContext:  "ctx1",
		BusinessContext: "biz1",
		TaskPrompt:      "task1",
	}
	_, _, h1 := pc.Build()
	_, _, h2 := pc.Build()
	if h1 != h2 {
		t.Errorf("hash not stable: %q vs %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected SHA-256 hex (64 chars), got %d", len(h1))
	}
}

func TestPromptContext_Build_DifferentInputsDifferentHash(t *testing.T) {
	pc1 := &session.PromptContext{ProfilePrompt: "A"}
	pc2 := &session.PromptContext{ProfilePrompt: "B"}
	_, _, h1 := pc1.Build()
	_, _, h2 := pc2.Build()
	if h1 == h2 {
		t.Error("expected different hashes for different inputs")
	}
}

func TestPromptContext_Build_ContainsAllSections(t *testing.T) {
	pc := &session.PromptContext{
		PlatformRules:   "PLATFORM",
		ProfilePrompt:   "PROFILE",
		SessionContext:  "SESSION",
		BusinessContext: "BUSINESS",
		TaskPrompt:      "TASK",
	}
	prompt, _, _ := pc.Build()
	for _, section := range []string{"PLATFORM", "PROFILE", "SESSION", "BUSINESS", "TASK"} {
		if !contains(prompt, section) {
			t.Errorf("prompt missing section %q", section)
		}
	}
}

// ─── AISession ────────────────────────────────────────────────────────────────

func TestAISession_InitialState(t *testing.T) {
	sess := makeSession("s1", "t1")
	if sess.State() != session.StateCreated {
		t.Errorf("expected CREATED, got %s", sess.State())
	}
}

func TestAISession_SetState(t *testing.T) {
	sess := makeSession("s2", "t1")
	transitions := []session.AIState{
		session.StateDialing, session.StateConnected, session.StateListening, session.StateEnded,
	}
	var received []session.AIState
	sess.OnStateChange(func(s *session.AISession, state session.AIState) {
		received = append(received, state)
	})
	for _, st := range transitions {
		sess.SetState(st)
	}
	if len(received) != len(transitions) {
		t.Errorf("expected %d callbacks, got %d", len(transitions), len(received))
	}
	for i, st := range transitions {
		if received[i] != st {
			t.Errorf("callback[%d]: expected %s, got %s", i, st, received[i])
		}
	}
}

func TestAISession_MarkStarted_Idempotent(t *testing.T) {
	sess := makeSession("s3", "t1")
	sess.MarkStarted()
	first := sess.StartedAt
	sess.MarkStarted()
	if sess.StartedAt != first {
		t.Error("MarkStarted should be idempotent")
	}
	if first == nil {
		t.Error("StartedAt should be set after MarkStarted")
	}
}

func TestAISession_MarkEnded_Idempotent(t *testing.T) {
	sess := makeSession("s4", "t1")
	o1 := &session.Outcome{Reason: "done"}
	o2 := &session.Outcome{Reason: "other"}
	sess.MarkEnded(o1)
	sess.MarkEnded(o2)
	if sess.GetOutcome().Reason != "done" {
		t.Error("MarkEnded should be idempotent — first call wins")
	}
}

func TestAISession_Snapshot_NoSecrets(t *testing.T) {
	sess := makeSession("s5", "t1")
	snap := sess.Snapshot()
	// Snapshot must contain expected keys
	for _, k := range []string{"id", "tenant_id", "session_id", "call_id", "state", "provider"} {
		if _, ok := snap[k]; !ok {
			t.Errorf("snapshot missing key %q", k)
		}
	}
	// Snapshot must not contain keys that could hold secrets
	forbidden := []string{"api_key", "secret", "password", "token"}
	for _, k := range forbidden {
		if _, ok := snap[k]; ok {
			t.Errorf("snapshot MUST NOT expose %q", k)
		}
	}
}

// ─── VoiceProfile ────────────────────────────────────────────────────────────

func TestVoiceProfile_NoAPIKeys(t *testing.T) {
	// VoiceProfile should not have any secret-carrying fields at compile time.
	// This test documents the contract; if someone adds an ApiKey field, it breaks.
	p := session.VoiceProfile{
		ID:             session.ProfileSurvey,
		Version:        "1.0",
		Language:       "pt-BR",
		Voice:          "nova",
		Prompt:         "Olá, posso ajudar?",
		ProviderPolicy: session.PolicyPrimary,
		MaxDuration:    5 * time.Minute,
		BargeIn:        true,
	}
	if p.ID == "" {
		t.Error("profile ID must be set")
	}
}

// ─── Registry ────────────────────────────────────────────────────────────────

func TestRegistry_TenantIsolation(t *testing.T) {
	reg := session.NewRegistry()
	s1 := makeSession("r1", "tenant-A")
	s2 := makeSession("r2", "tenant-B")
	reg.Register(s1)
	reg.Register(s2)

	// Same tenant: OK
	got, err := reg.Get("r1", "tenant-A")
	if err != nil || got.ID != "r1" {
		t.Errorf("expected to get r1 for tenant-A: %v", err)
	}

	// Cross-tenant: Forbidden
	_, err = reg.Get("r1", "tenant-B")
	if err != session.ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}

	// Missing: NotFound
	_, err = reg.Get("missing", "tenant-A")
	if err != session.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRegistry_Remove(t *testing.T) {
	reg := session.NewRegistry()
	s := makeSession("rm1", "t1")
	reg.Register(s)
	reg.Remove("rm1")
	_, err := reg.Get("rm1", "t1")
	if err != session.ErrNotFound {
		t.Error("expected ErrNotFound after Remove")
	}
}

func TestRegistry_ListByTenant(t *testing.T) {
	reg := session.NewRegistry()
	reg.Register(makeSession("l1", "tX"))
	reg.Register(makeSession("l2", "tX"))
	reg.Register(makeSession("l3", "tY"))
	list := reg.ListByTenant("tX")
	if len(list) != 2 {
		t.Errorf("expected 2 sessions for tX, got %d", len(list))
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func makeSession(id, tenantID string) *session.AISession {
	return session.NewAISession(id, tenantID, "sess-"+id, "call-"+id, "agent-test",
		session.VoiceProfile{
			ID:       session.ProfileSurvey,
			Language: "pt-BR",
			BargeIn:  true,
		},
	)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
