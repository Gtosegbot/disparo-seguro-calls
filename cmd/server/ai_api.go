// ai_api.go — HTTP handlers for the AI voice session API.
// Mounts under /api/ai/. Tenant isolation is enforced on every request.
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"wacalls/internal/ai/events"
	"wacalls/internal/ai/gateway"
	"wacalls/internal/ai/provider"
	"wacalls/internal/ai/session"
)

// aiRouter holds the dependencies for AI API handlers.
type aiRouter struct {
	gateway     *gateway.VoiceGateway
	sessionReg  *session.Registry
	providerReg *provider.Registry
	bus         *events.Bus
	log         *slog.Logger
}

// newAIRouter creates a fully wired aiRouter.
func newAIRouter(
	gw *gateway.VoiceGateway,
	sr *session.Registry,
	pr *provider.Registry,
	bus *events.Bus,
	log *slog.Logger,
) *aiRouter {
	return &aiRouter{
		gateway:     gw,
		sessionReg:  sr,
		providerReg: pr,
		bus:         bus,
		log:         log,
	}
}

// mount registers all /api/ai/* routes on mux.
func (r *aiRouter) mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/ai/sessions", r.createSession)
	mux.HandleFunc("POST /api/ai/sessions/{id}/start", r.startSession)
	mux.HandleFunc("POST /api/ai/sessions/{id}/stop", r.stopSession)
	mux.HandleFunc("GET /api/ai/sessions/{id}", r.getSession)
	mux.HandleFunc("GET /api/ai/sessions", r.listSessions)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func tenantFromRequest(req *http.Request) string {
	// TODO: replace with real JWT/API-key extraction
	return req.Header.Get("X-Tenant-ID")
}

func writeJSONAI(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErrorAI(w http.ResponseWriter, status int, msg string) {
	writeJSONAI(w, status, map[string]string{"error": msg})
}

// ─── handlers ───────────────────────────────────────────────────────────────

// POST /api/ai/sessions
func (r *aiRouter) createSession(w http.ResponseWriter, req *http.Request) {
	tenantID := tenantFromRequest(req)
	if tenantID == "" {
		writeErrorAI(w, http.StatusUnauthorized, "missing X-Tenant-ID")
		return
	}

	var body struct {
		SessionID    string               `json:"session_id"`
		CallID       string               `json:"call_id"`
		AgentID      string               `json:"agent_id"`
		ProviderName string               `json:"provider"`
		Profile      session.VoiceProfile `json:"profile"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeErrorAI(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.SessionID == "" || body.CallID == "" {
		writeErrorAI(w, http.StatusBadRequest, "session_id and call_id are required")
		return
	}
	if body.ProviderName == "" {
		body.ProviderName = "grok_realtime"
	}
	if _, _, ok := r.providerReg.Resolve(body.ProviderName); !ok {
		writeErrorAI(w, http.StatusBadRequest, "unknown provider: "+body.ProviderName)
		return
	}

	id := uuid.New().String()
	sess := session.NewAISession(id, tenantID, body.SessionID, body.CallID, body.AgentID, body.Profile)
	sess.Provider = body.ProviderName
	r.sessionReg.Register(sess)

	r.log.Info("ai_api: session created", "id", id, "tenant", tenantID)
	writeJSONAI(w, http.StatusCreated, sess.Snapshot())
}

// POST /api/ai/sessions/{id}/start
func (r *aiRouter) startSession(w http.ResponseWriter, req *http.Request) {
	tenantID := tenantFromRequest(req)
	id := req.PathValue("id")

	sess, err := r.sessionReg.Get(id, tenantID)
	if err != nil {
		writeErrorAI(w, http.StatusNotFound, err.Error())
		return
	}
	if sess.State() != session.StateCreated {
		writeErrorAI(w, http.StatusConflict, "session already started")
		return
	}

	waSess, ok := r.sessions.Get(sess.SessionID)
	if !ok {
		writeErrorAI(w, http.StatusNotFound, "whatsapp session not found")
		return
	}

	ac, ok := waSess.reg.get(sess.CallID)
	if !ok {
		writeErrorAI(w, http.StatusNotFound, "call not found")
		return
	}

	promptCtx := &session.PromptContext{
		PlatformRules:  "Você é um agente de voz do Disparo Seguro. Seja conciso, profissional e humanizado.",
		ProfilePrompt:  sess.Profile.Prompt,
		SessionContext: "call_id=" + sess.CallID,
	}

	// writeFn routes synthesised audio back into the AstraCalls call leg.
	writeFn := func(samples []float32) {
		ac, ok := waSess.reg.get(sess.CallID)
		if ok {
			ac.cm.FeedCapturedPCM(samples)
		}
	}

	started, err := r.gateway.StartAISession(
		req.Context(),
		tenantID,
		sess.SessionID,
		sess.CallID,
		sess.AgentID,
		sess.Profile,
		sess.Provider,
		promptCtx,
		writeFn,
	)
	if err != nil {
		writeErrorAI(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Ativa a interceptação de áudio setando o ID de IA na chamada ativa
	ac.aiSessionID = started.ID

	r.log.Info("ai_api: session started", "id", started.ID)
	writeJSONAI(w, http.StatusOK, started.Snapshot())
}

// POST /api/ai/sessions/{id}/stop
func (r *aiRouter) stopSession(w http.ResponseWriter, req *http.Request) {
	tenantID := tenantFromRequest(req)
	id := req.PathValue("id")

	sess, err := r.sessionReg.Get(id, tenantID)
	if err != nil {
		writeErrorAI(w, http.StatusNotFound, err.Error())
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)
	if body.Reason == "" {
		body.Reason = "api_stop"
	}

	// Limpa o vinculo de IA na chamada ativa
	if waSess, ok := r.sessions.Get(sess.SessionID); ok {
		if ac, ok := waSess.reg.get(sess.CallID); ok {
			ac.aiSessionID = ""
		}
	}

	r.gateway.StopAISession(id, body.Reason)
	writeJSONAI(w, http.StatusOK, map[string]string{"status": "stopped", "id": id})
}

// GET /api/ai/sessions/{id}
func (r *aiRouter) getSession(w http.ResponseWriter, req *http.Request) {
	tenantID := tenantFromRequest(req)
	id := req.PathValue("id")

	sess, err := r.sessionReg.Get(id, tenantID)
	if err != nil {
		writeErrorAI(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONAI(w, http.StatusOK, sess.Snapshot())
}

// GET /api/ai/sessions
func (r *aiRouter) listSessions(w http.ResponseWriter, req *http.Request) {
	tenantID := tenantFromRequest(req)
	if tenantID == "" {
		writeErrorAI(w, http.StatusUnauthorized, "missing X-Tenant-ID")
		return
	}
	snaps := r.sessionReg.ListByTenant(tenantID)
	if snaps == nil {
		snaps = []map[string]any{}
	}
	writeJSONAI(w, http.StatusOK, map[string]any{
		"sessions":  snaps,
		"timestamp": time.Now().UTC(),
	})
}
