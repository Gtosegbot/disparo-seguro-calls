package main

import (
	"context"
	"log/slog"
	"os"

	waLog "go.mau.fi/whatsmeow/util/log"

	"wacalls/internal/ai/events"
	"wacalls/internal/ai/gateway"
	"wacalls/internal/ai/provider"
	"wacalls/internal/ai/session"
	"wacalls/internal/platform/instance"
)

type server struct {
	broker      *Broker
	sessions    *SessionManager
	log         *slog.Logger
	staticDir   string

	// Camada proprietária de IA
	aiSessions  *session.Registry
	aiProviders *provider.Registry
	aiEvents    *events.Bus
	aiGateway   *gateway.VoiceGateway

	// Camada de produto White-Label
	instanceMgr *instance.Manager
}

// newServer monta o provedor de banco (Postgres, 1 banco por sessão no estilo
// WAHA), abre o banco principal e inicializa o gerenciador de sessões.
func newServer(ctx context.Context, pgURL, pgNamespace, staticDir string, maxCalls int, log *slog.Logger) (*server, error) {
	waLogger := waLog.Noop
	if log.Enabled(ctx, slog.LevelDebug) {
		waLogger = waLog.Stdout("WA", "DEBUG", true)
	}

	dbProv, err := newDBProvider(ctx, pgURL, pgNamespace, waLogger, log)
	if err != nil {
		return nil, err
	}

	mainDB, err := dbProv.openMainDB(ctx)
	if err != nil {
		return nil, err
	}
	store, err := newSessionStore(ctx, mainDB)
	if err != nil {
		return nil, err
	}

	// Inicialização da Camada de IA
	aiReg := session.NewRegistry()
	aiProvs := provider.NewRegistry()
	aiBus := events.NewBus()

	xaiKey := os.Getenv("WACALLS_XAI_KEY")
	aiProvs.Register("grok_realtime", xaiKey, func() provider.Provider {
		return provider.NewGrokRealtime(xaiKey, log)
	})
	geminiKey := os.Getenv("WACALLS_GEMINI_KEY")
	aiProvs.Register("gemini_realtime", geminiKey, func() provider.Provider {
		return provider.NewGeminiRealtime(geminiKey, log)
	})
	aiProvs.Register("loopback", "", func() provider.Provider {
		return provider.NewLoopbackProvider(log)
	})

	aiGW := gateway.NewVoiceGateway(aiReg, aiProvs, aiBus, log)

	broker := NewBroker()
	mgr := newSessionManager(ctx, dbProv, broker, store, waLogger, log, maxCalls)
	mgr.aiGateway = aiGW // injeta o gateway na gerência de sessões

	// Inicialização do InstanceManager
	instMgr := instance.NewManager(mainDB, mgr)
	mgr.instanceMgr = instMgr

	broker.SnapshotFn = mgr.snapshotEvents
	broker.AccountForSession = mgr.accountIDForSession

	return &server{
		broker:      broker,
		sessions:    mgr,
		log:         log,
		staticDir:   staticDir,
		aiSessions:  aiReg,
		aiProviders: aiProvs,
		aiEvents:    aiBus,
		aiGateway:   aiGW,
		instanceMgr: instMgr,
	}, nil
}
