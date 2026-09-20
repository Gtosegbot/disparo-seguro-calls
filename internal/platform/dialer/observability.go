package dialer

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CorrelationContext carries distributed tracing and operational correlation keys.
type CorrelationContext struct {
	TenantID    string `json:"tenant_id"`
	CampaignID  string `json:"campaign_id"`
	JobID       string `json:"job_id"`
	LeadID      string `json:"lead_id"`
	CallID      string `json:"call_id"`
	Attempt     int    `json:"attempt"`
	Provider    string `json:"provider"`
	CostEventID string `json:"cost_event_id"`
}

// SensitiveKeys is the blocklist of fields that must never be recorded in audit logs.
var SensitiveKeys = []string{
	"api_key", "apikey", "secret", "password", "token", "authorization",
	"private_key", "bearer", "cookie", "session_secret",
}

// SanitizeMetadata strips sensitive keys from metadata to prevent credential leakage.
func SanitizeMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}
	clean := make(map[string]any, len(meta))
	for k, v := range meta {
		lower := strings.ToLower(k)
		isSensitive := false
		for _, sens := range SensitiveKeys {
			if strings.Contains(lower, sens) {
				isSensitive = true
				break
			}
		}
		if isSensitive {
			clean[k] = "[REDACTED]"
		} else {
			clean[k] = v
		}
	}
	return clean
}

// MetricsRegistry provides atomic counters, gauges, and latency trackers for the dialer.
type MetricsRegistry struct {
	mu sync.RWMutex

	// Counters
	JobsTotal        atomic.Int64 `json:"jobs_total"`
	JobsFailed       atomic.Int64 `json:"jobs_failed"`
	LeadsClaimed     atomic.Int64 `json:"leads_claimed"`
	CallsStarted     atomic.Int64 `json:"calls_started"`
	CallsCompleted   atomic.Int64 `json:"calls_completed"`
	CallsFailed      atomic.Int64 `json:"calls_failed"`
	ProviderErrors   atomic.Int64 `json:"provider_errors"`
	RetryCount       atomic.Int64 `json:"retry_count"`
	FallbackCount    atomic.Int64 `json:"fallback_count"`
	CircuitOpenCount atomic.Int64 `json:"circuit_open_count"`

	// Gauges & Latency
	ProviderLatencyMs atomic.Int64 `json:"provider_latency_ms"`
	TTFBMs            atomic.Int64 `json:"ttfb_ms"`

	// Cost Aggregators (stored as cents / microcents scaled by 1000)
	CostTotalCents    atomic.Int64 `json:"cost_total_cents"`
	CostProviderCents atomic.Int64 `json:"cost_provider_cents"`
	CostPlatformCents atomic.Int64 `json:"cost_platform_cents"`
}

// Global default registry
var DefaultMetrics = NewMetricsRegistry()

func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{}
}

func (m *MetricsRegistry) RecordCallStart() {
	m.CallsStarted.Add(1)
}

func (m *MetricsRegistry) RecordCallCompleted() {
	m.CallsCompleted.Add(1)
}

func (m *MetricsRegistry) RecordCallFailed() {
	m.CallsFailed.Add(1)
}

func (m *MetricsRegistry) RecordJobCreated() {
	m.JobsTotal.Add(1)
}

func (m *MetricsRegistry) RecordJobFailed() {
	m.JobsFailed.Add(1)
}

func (m *MetricsRegistry) RecordLeadClaimed() {
	m.LeadsClaimed.Add(1)
}

func (m *MetricsRegistry) RecordProviderError() {
	m.ProviderErrors.Add(1)
}

func (m *MetricsRegistry) RecordRetry() {
	m.RetryCount.Add(1)
}

func (m *MetricsRegistry) RecordFallback() {
	m.FallbackCount.Add(1)
}

func (m *MetricsRegistry) RecordCircuitOpen() {
	m.CircuitOpenCount.Add(1)
}

func (m *MetricsRegistry) RecordLatency(providerLatency, ttfb time.Duration) {
	m.ProviderLatencyMs.Store(providerLatency.Milliseconds())
	m.TTFBMs.Store(ttfb.Milliseconds())
}

func (m *MetricsRegistry) RecordCosts(platformCost, providerCost float64) {
	totalCents := int64((platformCost + providerCost) * 100)
	platCents := int64(platformCost * 100)
	provCents := int64(providerCost * 100)

	m.CostTotalCents.Add(totalCents)
	m.CostPlatformCents.Add(platCents)
	m.CostProviderCents.Add(provCents)
}

// Snapshot returns a current snapshot map of all metrics for Prometheus / JSON export.
func (m *MetricsRegistry) Snapshot() map[string]any {
	return map[string]any{
		"jobs_total":         m.JobsTotal.Load(),
		"jobs_failed":        m.JobsFailed.Load(),
		"leads_claimed":      m.LeadsClaimed.Load(),
		"calls_started":      m.CallsStarted.Load(),
		"calls_completed":    m.CallsCompleted.Load(),
		"calls_failed":       m.CallsFailed.Load(),
		"provider_errors":    m.ProviderErrors.Load(),
		"provider_latency":   m.ProviderLatencyMs.Load(),
		"ttfb":               m.TTFBMs.Load(),
		"retry_count":        m.RetryCount.Load(),
		"fallback_count":     m.FallbackCount.Load(),
		"circuit_open_count": m.CircuitOpenCount.Load(),
		"cost_total":         float64(m.CostTotalCents.Load()) / 100.0,
		"cost_provider":      float64(m.CostProviderCents.Load()) / 100.0,
		"cost_platform":      float64(m.CostPlatformCents.Load()) / 100.0,
	}
}
