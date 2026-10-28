package livekitbridge

import (
	"log/slog"
	"sync"
	"wacalls/internal/voip/call"
)

// BridgeManager gerencia o pool de pontes ativas entre linhas de WhatsApp (BitBrowser/WWS) e salas LiveKit
type BridgeManager struct {
	cfg      Config
	log      *slog.Logger
	mu       sync.RWMutex
	sessions map[string]*BridgeSession
}

var (
	defaultManager *BridgeManager
	once           sync.Once
)

// Default retorna a instância singleton do BridgeManager
func Default() *BridgeManager {
	once.Do(func() {
		cfg := LoadConfigFromEnv()
		defaultManager = NewBridgeManager(cfg, slog.Default())
	})
	return defaultManager
}

func NewBridgeManager(cfg Config, log *slog.Logger) *BridgeManager {
	if log == nil {
		log = slog.Default()
	}
	return &BridgeManager{
		cfg:      cfg,
		log:      log.With("component", "livekit_bridge_manager"),
		sessions: make(map[string]*BridgeSession),
	}
}

// AttachCall vincula uma chamada WhatsApp do CallManager a uma sala do LiveKit Local
func (m *BridgeManager) AttachCall(cm *call.CallManager, callID, peerPhone string) *BridgeSession {
	if !m.cfg.Enabled {
		m.log.Debug("LiveKit Bridge desabilitada via env", "callID", callID)
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Se já existir sessão ativa para este callID, reutiliza
	if sess, exists := m.sessions[callID]; exists {
		return sess
	}

	sess := NewBridgeSession(m.cfg, cm, callID, peerPhone, m.log)
	if err := sess.Start(); err != nil {
		m.log.Error("Falha ao iniciar bridge de voz com o LiveKit", "err", err, "callID", callID)
		return nil
	}

	m.sessions[callID] = sess
	m.log.Info("Chamada WhatsApp conectada à sala LiveKit com sucesso", "callID", callID, "sessionsCount", len(m.sessions))
	return sess
}

// GetSession busca a sessão ativa da bridge para uma chamada
func (m *BridgeManager) GetSession(callID string) *BridgeSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[callID]
}

// DetachCall remove e encerra a ponte para uma chamada terminada
func (m *BridgeManager) DetachCall(callID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, exists := m.sessions[callID]
	if !exists {
		return
	}

	sess.Stop()
	delete(m.sessions, callID)
	m.log.Info("Chamada WhatsApp desconectada da sala LiveKit", "callID", callID, "remainingSessions", len(m.sessions))
}

// ActiveSessionsCount retorna a contagem de pontes simultâneas ativas
func (m *BridgeManager) ActiveSessionsCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
