package fabric

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"wacalls/internal/ai/provider"
	"wacalls/internal/ai/session"
)

// ProviderType categorises the integration mechanism.
type ProviderType string

const (
	TypeRealtimeSpeech ProviderType = "realtime"
	TypeSttLlmTts      ProviderType = "stt_llm_tts"
)

// ProviderCapabilities describes features enabled in the underlying provider model.
type ProviderCapabilities struct {
	RealtimeAudio bool `json:"realtime_audio"`
	STT           bool `json:"stt"`
	TTS           bool `json:"tts"`
	VAD           bool `json:"vad"`
	ToolCalling   bool `json:"tool_calling"`
}

// ProviderCatalogItem represents a registered AI voice engine in the DS Voice ecosystem.
type ProviderCatalogItem struct {
	Name          string               `json:"name"`
	Model         string               `json:"model"`
	Type          ProviderType         `json:"provider_type"`
	Capabilities  ProviderCapabilities `json:"capabilities"`
	VoiceSupport  []string             `json:"voice_support"`
	Languages     []string             `json:"languages"`
	EstimatedCost float64              `json:"estimated_cost"` // USD cents per minute
	LatencyTarget int                  `json:"latency_target"` // ms
	QualityClass  string               `json:"quality_class"`  // "premium", "balanced", "economy"
	License       string               `json:"license"`        // "proprietary", "apache-2.0", etc.
	Enabled       bool                 `json:"enabled"`
	Health        float64              `json:"health"` // 0.0 to 1.0
	Priority      int                  `json:"priority"`
	Weight        int                  `json:"weight"` // Weighted routing percentage (0-100)

	// Live operational metrics
	ActiveSessions int           `json:"active_sessions"`
	SuccessCount   int64         `json:"success_count"`
	FailureCount   int64         `json:"failure_count"`
	AvgTTFBMs      int64         `json:"avg_ttfb_ms"`
	Timeouts       int64         `json:"timeouts"`
	FallbackCount  int64         `json:"fallback_count"`
	SuccessRate    float64       `json:"success_rate"`
	TotalSessions  int64         `json:"total_sessions"`
	TotalDuration  time.Duration `json:"total_duration"`
}

// Fabric orchestrates multiple AI speech providers, routing, cost optimizations and fallback.
type Fabric struct {
	log       *slog.Logger
	registry  *provider.Registry
	mu        sync.RWMutex
	catalog   map[string]*ProviderCatalogItem
	fallbacks map[string][]string // Primary -> list of secondary fallback chain
}

// NewFabric creates a production-ready Fabric 2.0.
func NewFabric(reg *provider.Registry, log *slog.Logger) *Fabric {
	if log == nil {
		log = slog.Default()
	}

	f := &Fabric{
		log:       log,
		registry:  reg,
		catalog:   make(map[string]*ProviderCatalogItem),
		fallbacks: make(map[string][]string),
	}

	f.bootstrapCatalog()
	return f
}

func (f *Fabric) bootstrapCatalog() {
	// 1. Grok Realtime Catalog
	f.catalog["grok_realtime"] = &ProviderCatalogItem{
		Name:  "grok_realtime",
		Model: "grok-2-voice-preview",
		Type:  TypeRealtimeSpeech,
		Capabilities: ProviderCapabilities{
			RealtimeAudio: true,
			STT:           true,
			TTS:           true,
			VAD:           true,
			ToolCalling:   true,
		},
		VoiceSupport:  []string{"aura", "charon", "puck"},
		Languages:     []string{"pt-BR", "en-US", "es-ES"},
		EstimatedCost: 12.5,
		LatencyTarget: 120,
		QualityClass:  "premium",
		License:       "proprietary",
		Enabled:       true,
		Health:        1.0,
		Priority:      1,
		Weight:        70,
		SuccessRate:   1.0,
	}

	// 2. Gemini Realtime Catalog
	f.catalog["gemini_realtime"] = &ProviderCatalogItem{
		Name:  "gemini_realtime",
		Model: "gemini-2.0-flash-exp",
		Type:  TypeRealtimeSpeech,
		Capabilities: ProviderCapabilities{
			RealtimeAudio: true,
			STT:           true,
			TTS:           true,
			VAD:           true,
			ToolCalling:   true,
		},
		VoiceSupport:  []string{"Puck", "Charon", "Kore"},
		Languages:     []string{"pt-BR", "en-US", "es-ES", "fr-FR"},
		EstimatedCost: 5.0,
		LatencyTarget: 150,
		QualityClass:  "balanced",
		License:       "proprietary",
		Enabled:       true,
		Health:        1.0,
		Priority:      2,
		Weight:        30,
		SuccessRate:   1.0,
	}

	// 3. Economy Loopback / Internal Free Pool
	f.catalog["loopback"] = &ProviderCatalogItem{
		Name:  "loopback",
		Model: "internal-economic-loopback",
		Type:  TypeRealtimeSpeech,
		Capabilities: ProviderCapabilities{
			RealtimeAudio: true,
			STT:           true,
			TTS:           true,
			VAD:           true,
			ToolCalling:   false,
		},
		VoiceSupport:  []string{"internal-female", "internal-male"},
		Languages:     []string{"pt-BR", "en-US"},
		EstimatedCost: 0.0,
		LatencyTarget: 10,
		QualityClass:  "economy",
		License:       "mit",
		Enabled:       true,
		Health:        1.0,
		Priority:      3,
		Weight:        0,
		SuccessRate:   1.0,
	}

	// Fallback Chain Registration: Grok -> Gemini -> Loopback -> Terminal
	f.fallbacks["grok_realtime"] = []string{"gemini_realtime", "loopback"}
	f.fallbacks["gemini_realtime"] = []string{"loopback"}
}

// ResolveProvider evaluates constraints (languages, features) and selects a provider using policy score or weighted distribution.
func (f *Fabric) ResolveProvider(ctx context.Context, policy session.ProviderPolicy, requiredLang string) (provider.Provider, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var eligible []*ProviderCatalogItem
	for _, item := range f.catalog {
		if !item.Enabled || item.Health < 0.2 {
			continue // Ignora desabilitados ou gravemente degradados
		}

		// Filtra por idioma
		langMatch := false
		for _, l := range item.Languages {
			if l == requiredLang {
				langMatch = true
				break
			}
		}
		if !langMatch && requiredLang != "" {
			continue
		}

		eligible = append(eligible, item)
	}

	if len(eligible) == 0 {
		return nil, "", errors.New("omniroute: no healthy providers matches capability matrix constraints")
	}

	var selected *ProviderCatalogItem

	// 1. Política baseada em pesos (Weighted Routing)
	if policy == "Weighted" {
		totalWeight := 0
		for _, item := range eligible {
			totalWeight += item.Weight
		}
		if totalWeight > 0 {
			r := rand.Intn(totalWeight)
			sum := 0
			for _, item := range eligible {
				sum += item.Weight
				if r < sum {
					selected = item
					break
				}
			}
		}
	}

	// 2. Score-based routing (Economy, Latency, Premium) se não resolvido por pesos
	if selected == nil {
		var bestScore float64 = -999999.0
		for _, item := range eligible {
			score := f.calculateScore(item, policy)
			if score > bestScore {
				bestScore = score
				selected = item
			}
		}
	}

	if selected == nil {
		selected = eligible[0] // fallback de segurança
	}

	prov, apiKey, ok := f.registry.Resolve(selected.Name)
	if !ok {
		return nil, "", fmt.Errorf("fabric: resolved provider %q is missing from active registry", selected.Name)
	}

	selected.ActiveSessions++
	selected.TotalSessions++

	f.log.Info("omniroute: provider resolved successfully",
		"resolved_provider", selected.Name,
		"policy", string(policy),
		"cost_estimate", selected.EstimatedCost,
		"latency_target", selected.LatencyTarget,
	)

	return prov, apiKey, nil
}

// HandleFallback manages dynamic chain routing in case of provider failure.
func (f *Fabric) HandleFallback(ctx context.Context, primaryName string, primaryErr error) (provider.Provider, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	primary, ok := f.catalog[primaryName]
	if ok {
		primary.FailureCount++
		primary.ActiveSessions--
		primary.FallbackCount++
		// Degrada a saúde do provedor proporcionalmente
		primary.Health = float64(primary.SuccessCount) / float64(primary.SuccessCount+primary.FailureCount)
		if primary.Health < 0.1 {
			primary.Health = 0.1 // piso
		}
	}

	chain, hasChain := f.fallbacks[primaryName]
	if !hasChain || len(chain) == 0 {
		return nil, "", fmt.Errorf("fabric: no fallback path configured for %q (original error: %w)", primaryName, primaryErr)
	}

	var resolvedFallback *ProviderCatalogItem
	for _, fbName := range chain {
		fbItem, exists := f.catalog[fbName]
		if exists && fbItem.Enabled && fbItem.Health >= 0.5 {
			resolvedFallback = fbItem
			break
		}
	}

	if resolvedFallback == nil {
		return nil, "", fmt.Errorf("fabric: all fallback providers in chain for %q are offline/unhealthy", primaryName)
	}

	f.log.Info("fabric: automatic fallback routed successfully",
		"primary_provider", primaryName,
		"fallback_provider", resolvedFallback.Name,
		"failure_reason", primaryErr.Error(),
		"fallback_started_at", time.Now().UTC(),
	)

	prov, apiKey, ok := f.registry.Resolve(resolvedFallback.Name)
	if !ok {
		return nil, "", fmt.Errorf("fabric: fallback provider %q is missing from active registry", resolvedFallback.Name)
	}

	resolvedFallback.ActiveSessions++
	resolvedFallback.TotalSessions++

	return prov, apiKey, nil
}

// CompleteSession logs metrics on session end.
func (f *Fabric) CompleteSession(name string, duration time.Duration, success bool, ttfbMs int64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.catalog[name]
	if !ok {
		return
	}

	item.ActiveSessions--
	if item.ActiveSessions < 0 {
		item.ActiveSessions = 0
	}

	item.TotalDuration += duration

	if success {
		item.SuccessCount++
	} else {
		item.FailureCount++
	}

	if ttfbMs > 0 {
		// Média móvel ponderada simples de TTFB
		if item.AvgTTFBMs == 0 {
			item.AvgTTFBMs = ttfbMs
		} else {
			item.AvgTTFBMs = (item.AvgTTFBMs*9 + ttfbMs) / 10
		}
	}

	totalAttempts := item.SuccessCount + item.FailureCount
	if totalAttempts > 0 {
		item.SuccessRate = float64(item.SuccessCount) / float64(totalAttempts)
		item.Health = item.SuccessRate
	}
}

// GetCatalog returns a copy of the catalog for observation (invisível de keys).
func (f *Fabric) GetCatalog() []ProviderCatalogItem {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var out []ProviderCatalogItem
	for _, v := range f.catalog {
		out = append(out, *v)
	}
	return out
}

// SetProviderStatus toggles administrative enablement and prioritisation.
func (f *Fabric) SetProviderStatus(name string, enabled bool, priority int, weight int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.catalog[name]
	if !ok {
		return errors.New("provider not found in catalog")
	}

	item.Enabled = enabled
	item.Priority = priority
	item.Weight = weight
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (f *Fabric) calculateScore(item *ProviderCatalogItem, policy session.ProviderPolicy) float64 {
	// Normalização de Custo (inverte para quanto menor custo, melhor score)
	costScore := 100.0 - item.EstimatedCost

	// Normalização de Latência (inverte para menor latência, melhor score)
	latencyScore := 200.0 - float64(item.LatencyTarget)

	// Qualidade
	qualityScore := 50.0
	if item.QualityClass == "premium" {
		qualityScore = 100.0
	} else if item.QualityClass == "economy" {
		qualityScore = 10.0
	}

	// Disponibilidade / Health operacional
	availabilityScore := item.Health * 100.0

	// Fórmula configurável por políticas OmniRoute
	switch policy {
	case session.PolicyCostFirst, "Economy":
		return costScore*0.7 + availabilityScore*0.3
	case session.PolicyLatency, "LowLatency":
		return latencyScore*0.7 + availabilityScore*0.3
	case "Premium":
		return qualityScore*0.6 + latencyScore*0.2 + availabilityScore*0.2
	default: // Balanced
		return costScore*0.3 + latencyScore*0.3 + qualityScore*0.2 + availabilityScore*0.2
	}
}
