package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"wacalls/internal/platform/instance"
)

type instancesRouter struct {
	manager  *instance.Manager
	sessions *SessionManager
	log      *slog.Logger
}

func newInstancesRouter(mgr *instance.Manager, sessions *SessionManager, log *slog.Logger) *instancesRouter {
	return &instancesRouter{manager: mgr, sessions: sessions, log: log}
}

func (r *instancesRouter) mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/instances", r.create)
	mux.HandleFunc("GET /api/instances", r.list)
	mux.HandleFunc("GET /api/instances/{id}", r.get)
	mux.HandleFunc("POST /api/instances/{id}/pair", r.pair)
	mux.HandleFunc("GET /api/instances/{id}/pairing", r.pairingStatus)
	mux.HandleFunc("POST /api/instances/{id}/logout", r.logout)
	mux.HandleFunc("DELETE /api/instances/{id}", r.deleteInstance)

	// ChatSeguro/Chatwoot mappings per line
	mux.HandleFunc("GET /api/instances/{id}/chatseguro", r.getChatSeguro)
	mux.HandleFunc("POST /api/instances/{id}/chatseguro", r.setChatSeguro)
	mux.HandleFunc("DELETE /api/instances/{id}/chatseguro", r.deleteChatSeguro)
	mux.HandleFunc("POST /api/instances/{id}/chatseguro/test", r.testChatSeguro)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func getAuthenticatedTenantID(req *http.Request) (string, error) {
	apiKey := req.Header.Get("X-API-Key")
	if apiKey == "" {
		apiKey = req.URL.Query().Get("apiKey")
	}

	masterKey := os.Getenv("WACALLS_API_KEY")
	if masterKey != "" && apiKey == masterKey {
		tenantID := req.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "admin-tenant"
		}
		return tenantID, nil
	}

	if strings.HasPrefix(apiKey, "tenant-") {
		resolvedTenant := strings.TrimPrefix(apiKey, "tenant-")
		
		clientTenant := req.Header.Get("X-Tenant-ID")
		if clientTenant != "" && clientTenant != resolvedTenant {
			return "", instance.ErrForbidden
		}
		return resolvedTenant, nil
	}

	return "", errors.New("unauthorized: invalid tenant credentials")
}

func writeJSONInst(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErrorInst(w http.ResponseWriter, code int, msg string) {
	writeJSONInst(w, code, map[string]string{"error": msg})
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func (r *instancesRouter) create(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err == instance.ErrForbidden {
		writeErrorInst(w, http.StatusForbidden, "forbidden: tenant access denied")
		return
	} else if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	var body struct {
		DisplayName string `json:"display_name"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)
	name := strings.TrimSpace(body.DisplayName)
	if name == "" {
		name = "WhatsApp Line"
	}

	inst, err := r.manager.Create(req.Context(), tenantID, name)
	if err != nil {
		writeErrorInst(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSONInst(w, http.StatusCreated, inst)
}

func (r *instancesRouter) list(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err == instance.ErrForbidden {
		writeErrorInst(w, http.StatusForbidden, "forbidden: tenant access denied")
		return
	} else if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	list, err := r.manager.List(req.Context(), tenantID)
	if err != nil {
		writeErrorInst(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSONInst(w, http.StatusOK, map[string]any{
		"instances": list,
		"timestamp": time.Now().UTC(),
	})
}

func (r *instancesRouter) get(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err == instance.ErrForbidden {
		writeErrorInst(w, http.StatusForbidden, "forbidden: tenant access denied")
		return
	} else if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	id := req.PathValue("id")
	inst, err := r.manager.Get(req.Context(), tenantID, id)
	if err == instance.ErrNotFound {
		writeErrorInst(w, http.StatusNotFound, err.Error())
		return
	} else if err == instance.ErrForbidden {
		writeErrorInst(w, http.StatusForbidden, err.Error())
		return
	} else if err != nil {
		writeErrorInst(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSONInst(w, http.StatusOK, inst)
}

func (r *instancesRouter) pair(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err == instance.ErrForbidden {
		writeErrorInst(w, http.StatusForbidden, "forbidden: tenant access denied")
		return
	} else if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	id := req.PathValue("id")
	inst, err := r.manager.Get(req.Context(), tenantID, id)
	if err != nil {
		writeErrorInst(w, http.StatusNotFound, err.Error())
		return
	}

	var body struct {
		Phone string `json:"phone,omitempty"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)
	phone := strings.TrimSpace(body.Phone)

	if phone != "" {
		code, err := r.sessions.PairPhone(inst.SessionID, phone)
		if err != nil {
			writeErrorInst(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONInst(w, http.StatusOK, map[string]any{
			"pairing_session_id": inst.ID,
			"status":             "PAIRING",
			"code":               code,
			"expires_at":         time.Now().Add(2 * time.Minute).UTC(),
		})
	} else {
		err := r.sessions.Pair(inst.SessionID)
		if err != nil {
			writeErrorInst(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONInst(w, http.StatusOK, map[string]any{
			"pairing_session_id": inst.ID,
			"status":             "PAIRING",
			"expires_at":         time.Now().Add(2 * time.Minute).UTC(),
		})
	}
}

func (r *instancesRouter) pairingStatus(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err == instance.ErrForbidden {
		writeErrorInst(w, http.StatusForbidden, "forbidden: tenant access denied")
		return
	} else if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	id := req.PathValue("id")
	inst, err := r.manager.Get(req.Context(), tenantID, id)
	if err != nil {
		writeErrorInst(w, http.StatusNotFound, err.Error())
		return
	}

	waSess, ok := r.sessions.Get(inst.SessionID)
	if !ok {
		writeErrorInst(w, http.StatusNotFound, "whatsapp session missing")
		return
	}

	waSess.mu.Lock()
	auth := waSess.auth
	waSess.mu.Unlock()

	resp := instance.PairingResponse{
		PairingSessionID: inst.ID,
		Status:           inst.Status,
		ExpiresAt:        time.Now().Add(1 * time.Minute).UTC(),
	}

	if auth.State == "qr" && auth.QR != "" {
		resp.QR = auth.QR
	}

	writeJSONInst(w, http.StatusOK, resp)
}

func (r *instancesRouter) logout(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err == instance.ErrForbidden {
		writeErrorInst(w, http.StatusForbidden, "forbidden: tenant access denied")
		return
	} else if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	id := req.PathValue("id")
	err = r.manager.Logout(req.Context(), tenantID, id)
	if err == instance.ErrNotFound {
		writeErrorInst(w, http.StatusNotFound, err.Error())
		return
	} else if err == instance.ErrForbidden {
		writeErrorInst(w, http.StatusForbidden, err.Error())
		return
	} else if err != nil {
		writeErrorInst(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSONInst(w, http.StatusOK, map[string]string{"status": "logged_out", "instance_id": id})
}

func (r *instancesRouter) deleteInstance(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err == instance.ErrForbidden {
		writeErrorInst(w, http.StatusForbidden, "forbidden: tenant access denied")
		return
	} else if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	id := req.PathValue("id")
	err = r.manager.Delete(req.Context(), tenantID, id)
	if err == instance.ErrNotFound {
		writeErrorInst(w, http.StatusNotFound, err.Error())
		return
	} else if err == instance.ErrForbidden {
		writeErrorInst(w, http.StatusForbidden, err.Error())
		return
	} else if err != nil {
		writeErrorInst(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/instances/{id}/chatseguro
func (r *instancesRouter) getChatSeguro(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	id := req.PathValue("id")
	inst, err := r.manager.Get(req.Context(), tenantID, id)
	if err != nil {
		writeErrorInst(w, http.StatusNotFound, err.Error())
		return
	}

	waSess, ok := r.sessions.Get(inst.SessionID)
	if !ok {
		writeErrorInst(w, http.StatusNotFound, "whatsapp session not found")
		return
	}

	cw := waSess.getChatwoot()
	status := "DISCONNECTED"
	if cw.valid() {
		status = "CONNECTED"
	}

	writeJSONInst(w, http.StatusOK, map[string]any{
		"inbox_id":     inst.ChatSeguroInboxID,
		"status":       status,
		"channel":      "whatsapp",
		"connected_at": time.Now().UTC(),
	})
}

// POST /api/instances/{id}/chatseguro
func (r *instancesRouter) setChatSeguro(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	id := req.PathValue("id")
	var body struct {
		URL          string `json:"url"`
		AccountID    int    `json:"account_id"`
		AccountToken string `json:"account_token"`
		InboxID      int    `json:"inbox_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeErrorInst(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if body.URL == "" || body.AccountID == 0 || body.AccountToken == "" || body.InboxID == 0 {
		writeErrorInst(w, http.StatusBadRequest, "missing required chatwoot parameters")
		return
	}

	cwConfig := ChatwootConfig{
		URL:                body.URL,
		AccountID:          body.AccountID,
		AccountToken:       body.AccountToken,
		InboxID:            body.InboxID,
		Groups:             true,
		Channels:           true,
		GroupsSkipIncoming: false,
		SignMsg:            false,
		AlwaysOnline:       true,
		ReadMessages:       true,
	}

	configBytes, _ := json.Marshal(cwConfig)

	err = r.manager.SetChatSeguro(req.Context(), tenantID, id, body.InboxID, string(configBytes))
	if err != nil {
		writeErrorInst(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSONInst(w, http.StatusOK, map[string]any{
		"inbox_id":      body.InboxID,
		"status":        "CONNECTED",
		"instance_id":   id,
		"configured_at": time.Now().UTC(),
	})
}

// DELETE /api/instances/{id}/chatseguro
func (r *instancesRouter) deleteChatSeguro(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	id := req.PathValue("id")
	err = r.manager.DeleteChatSeguro(req.Context(), tenantID, id)
	if err != nil {
		writeErrorInst(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/instances/{id}/chatseguro/test
func (r *instancesRouter) testChatSeguro(w http.ResponseWriter, req *http.Request) {
	tenantID, err := getAuthenticatedTenantID(req)
	if err != nil {
		writeErrorInst(w, http.StatusUnauthorized, err.Error())
		return
	}

	id := req.PathValue("id")
	inst, err := r.manager.Get(req.Context(), tenantID, id)
	if err != nil {
		writeErrorInst(w, http.StatusNotFound, err.Error())
		return
	}

	waSess, ok := r.sessions.Get(inst.SessionID)
	if !ok {
		writeErrorInst(w, http.StatusNotFound, "whatsapp session not found")
		return
	}

	cw := waSess.getChatwoot()
	if !cw.valid() {
		writeErrorInst(w, http.StatusBadRequest, "chatseguro not configured on this instance")
		return
	}

	_, _, err = cw.req("GET", "/profile", nil)
	if err != nil {
		writeErrorInst(w, http.StatusBadGateway, "failed to contact chatseguro server: "+err.Error())
		return
	}

	writeJSONInst(w, http.StatusOK, map[string]string{
		"status":  "HEALTHY",
		"message": "connection to chatseguro api verified successfully",
	})
}
