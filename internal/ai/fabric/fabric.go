package fabric

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"wacalls/internal/ai/provider"
	"wacalls/internal/ai/session"
)

// ProviderStats keeps metadata regarding cost, latency, capacity and quality.
type ProviderStats struct {
	Name        string
	License     string  // e.g. "proprietary", "apache-2.0"
	CostPerMin  float64 // USD cents
	LatencyMs   int     // Target average latency
	Quality     string  // "premium", "standard", "economy"
	CapacityTps int     // Transactions per second maximum capacity
	Healthy     bool
}

// Fabric manages the pool of AI speech/live providers, resolving and fallback routing.
type Fabric struct {
	log       *slog.Logger
	registry  *provider.Registry
	mu        sync.RWMutex
	stats     map[string]*ProviderStats
	fallbacks map[string]string // primary -> secondary fallback
}

// NewFabric creates a new Provider Fabric.
func NewFabric(reg *provider.Registry, log *slog.Logger) *Fabric {
	if log == nil {
		log = slog.Default()
	}

	f := &Fabric{
		log:       log,
		registry:  reg,
		stats:     make(map[string]*ProviderStats),
		fallbacks: make(map[string]string),
	}

	// Bootstrap statistics for pool resolution
	f.stats["grok_realtime"] = &ProviderStats{
		Name:        "grok_realtime",
		License:     "proprietary",
		CostPerMin:  12.5,
		LatencyMs:   120,
		Quality:     "premium",
		CapacityTps: 50,
		Healthy:     true,
	}

	f.stats["gemini_realtime"] = &ProviderStats{
		Name:        "gemini_realtime",
		License:     "proprietary",
		CostPerMin:  5.0,
		LatencyMs:   150,
		Quality:     "excellent",
		CapacityTps: 100,
		Healthy:     true,
	}

	f.stats["loopback"] = &ProviderStats{
		Name:        "loopback",
		License:     "mit",
		CostPerMin:  0.0,
		LatencyMs:   10,
		Quality:     "economy",
		CapacityTps: 1000,
		Healthy:     true,
	}

	// Setup default fallbacks
	f.fallbacks["grok_realtime"] = "gemini_realtime"
	f.fallbacks["gemini_realtime"] = "loopback"

	return f
}

// ResolveProvider selects the best provider based on policy, budget and health.
func (f *Fabric) ResolveProvider(ctx context.Context, policy session.ProviderPolicy, targetQuality string) (provider.Provider, string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var selected string

	switch policy {
	case session.PolicyCostFirst:
		// Escolhe o de menor custo saudável
		var cheapest *ProviderStats
		for _, stat := range f.stats {
			if stat.Healthy {
				if cheapest == nil || stat.CostPerMin < cheapest.CostPerMin {
					cheapest = stat
				}
			}
		}
		if cheapest != nil {
			selected = cheapest.Name
		}

	case session.PolicyLatency:
		// Escolhe o de menor latência saudável
		var lowestLat *ProviderStats
		for _, stat := range f.stats {
			if stat.Healthy {
				if lowestLat == nil || stat.LatencyMs < lowestLat.LatencyMs {
					lowestLat = stat
				}
			}
		}
		if lowestLat != nil {
			selected = lowestLat.Name
		}

	default: // PolicyPrimary ou fallback padrão
		// Filtra por qualidade ou escolhe grok_realtime se saudável
		if stat, ok := f.stats["grok_realtime"]; ok && stat.Healthy {
			selected = "grok_realtime"
		} else if stat, ok := f.stats["gemini_realtime"]; ok && stat.Healthy {
			selected = "gemini_realtime"
		} else {
			selected = "loopback"
		}
	}

	if selected == "" {
		return nil, "", errors.New("fabric: no healthy providers available in pool")
	}

	prov, apiKey, ok := f.registry.Resolve(selected)
	if !ok {
		return nil, "", fmt.Errorf("fabric: resolved provider %q missing from registry", selected)
	}

	return prov, apiKey, nil
}

// HandleFallback routes dynamically to secondary provider if primary fails, logging metrics.
func (f *Fabric) HandleFallback(ctx context.Context, primaryName string, primaryErr error) (provider.Provider, string, error) {
	f.mu.Lock()
	// Marca o primário como temporariamente instável/quebrado
	if stat, ok := f.stats[primaryName]; ok {
		stat.Healthy = false
		f.log.Warn("fabric: marking provider unhealthy", "provider", primaryName, "err", primaryErr)
	}
	f.mu.Unlock()

	f.mu.RLock()
	fallbackName, hasFallback := f.fallbacks[primaryName]
	f.mu.RUnlock()

	if !hasFallback || fallbackName == "" {
		return nil, "", fmt.Errorf("fabric: no fallback configured for %q (original error: %w)", primaryName, primaryErr)
	}

	f.log.Info("fabric: triggering automatic fallback",
		"primary_provider", primaryName,
		"fallback_provider", fallbackName,
		"failure_reason", primaryErr.Error(),
		"fallback_started_at", time.Now().UTC(),
	)

	f.mu.RLock()
	stat, ok := f.stats[fallbackName]
	f.mu.RUnlock()

	if !ok || !stat.Healthy {
		return nil, "", fmt.Errorf("fabric: fallback provider %q is also unhealthy", fallbackName)
	}

	prov, apiKey, ok := f.registry.Resolve(fallbackName)
	if !ok {
		return nil, "", fmt.Errorf("fabric: fallback provider %q missing from registry", fallbackName)
	}

	return prov, apiKey, nil
}

// SetHealth status dynamically (e.g. from connection monitors)
func (f *Fabric) SetHealth(name string, healthy bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if stat, ok := f.stats[name]; ok {
		stat.Healthy = healthy
		f.log.Info("fabric: provider health status updated", "provider", name, "healthy", healthy)
	}
}
