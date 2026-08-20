package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"wacalls/internal/ai/events"
	"wacalls/internal/ai/fabric"
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
	fabric      *fabric.Fabric
}

// newAIRouter creates a fully wired aiRouter.
func newAIRouter(
	gw *gateway.VoiceGateway,
	sr *session.Registry,
	pr *provider.Registry,
	bus *events.Bus,
	log *slog.Logger,
	fab *fabric.Fabric,
) *aiRouter {
	return &aiRouter{
		gateway:     gw,
		sessionReg:  sr,
		providerReg: pr,
		bus:         bus,
		log:         log,
		fabric:      fab,
	}
}

// mount registers all /api/ai/* and admin provider routes on mux.
func (r *aiRouter) mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/ai/sessions", r.createSession)
	mux.HandleFunc("POST /api/ai/sessions/{id}/start", r.startSession)
	mux.HandleFunc("POST /api/ai/sessions/{id}/stop", r.stopSession)
	mux.HandleFunc("GET /api/ai/sessions/{id}", r.getSession)
	mux.HandleFunc("GET /api/ai/sessions", r.listSessions)

	// Admin provider control routes
	mux.HandleFunc("GET /api/admin/providers", r.listProviders)
	mux.HandleFunc("POST /api/admin/providers/{name}", r.updateProvider)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func tenantFromRequest(req *http.Request) string {
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

// GET /api/admin/providers
func (r *aiRouter) listProviders(w http.ResponseWriter, req *http.Request) {
	catalog := r.fabric.GetCatalog()
	writeJSONAI(w, http.StatusOK, map[string]any{
		"providers": catalog,
		"timestamp": time.Now().UTC(),
	})
}

// POST /api/admin/providers/{name}
func (r *aiRouter) updateProvider(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")
	var body struct {
		Enabled  *bool `json:"enabled"`
		Priority *int  `json:"priority"`
		Weight   *int  `json:"weight"`
	}

	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeErrorAI(w, http.StatusBadRequest, "invalid payload: "+err.Error())
		return
	}

	// Recupera valores atuais para não sobrescrever com zero se omitidos
	catalog := r.fabric.GetCatalog()
	var current *fabric.ProviderCatalogItem
	for _, item := range catalog {
		if item.Name == name {
			current = &item
			break
		}
	}

	if current == nil {
		writeErrorAI(w, http.StatusNotFound, "provider not found in catalog")
		return
	}

	enabled := current.Enabled
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	priority := current.Priority
	if body.Priority != nil {
		priority = *body.Priority
	}

	weight := current.Weight
	if body.Weight != nil {
		weight = *body.Weight
	}

	err := r.fabric.SetProviderStatus(name, enabled, priority, weight)
	if err != nil {
		writeErrorAI(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSONAI(w, http.StatusOK, map[string]any{
		"status":   "updated",
		"provider": name,
		"enabled":  enabled,
		"priority": priority,
		"weight":   weight,
	})
}

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

	sess := session.NewAISession(body.SessionID, tenantID, "", body.CallID, body.AgentID, body.Profile)
	sess.Provider = body.ProviderName
	r.sessionReg.Register(sess)

	writeJSONAI(w, http.StatusCreated, sess)
}

// POST /api/ai/sessions/{id}/start
func (r *aiRouter) startSession(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	sess, ok := r.sessionReg.Get(id)
	if !ok {
		writeErrorAI(w, http.StatusNotFound, "session not found")
		return
	}

	var body struct {
		AstraCallsSessionID string `json:"astracalls_session_id"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)

	sess.AstraCallsSessionID = body.AstraCallsSessionID
	sess.SetState(session.StateConnected)

	writeJSONAI(w, http.StatusOK, sess)
}

// POST /api/ai/sessions/{id}/stop
func (r *aiRouter) stopSession(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	sess, ok := r.sessionReg.Get(id)
	if !ok {
		writeErrorAI(w, http.StatusNotFound, "session not found")
		return
	}

	r.gateway.StopAISession(sess.ID, "api_requested_stop")
	writeJSONAI(w, http.StatusOK, map[string]string{"status": "stopped", "session_id": id})
}

// GET /api/ai/sessions/{id}
func (r *aiRouter) getSession(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	sess, ok := r.sessionReg.Get(id)
	if !ok {
		writeErrorAI(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSONAI(w, http.StatusOK, sess)
}

// GET /api/ai/sessions
func (r *aiRouter) listSessions(w http.ResponseWriter, req *http.Request) {
	tenantID := tenantFromRequest(req)
	if tenantID == "" {
		writeErrorAI(w, http.StatusUnauthorized, "missing X-Tenant-ID")
		return
	}

	var match []*session.AISession
	for _, s := range r.sessionReg.List() {
		if s.TenantID == tenantID {
			match = append(match, s)
		}
	}
	writeJSONAI(w, http.StatusOK, match)
}
