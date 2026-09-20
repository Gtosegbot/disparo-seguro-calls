package dialer

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

var (
	ErrWebhookMissingHeader   = errors.New("webhook: missing required signature or timestamp header")
	ErrWebhookTimestampExpired = errors.New("webhook: timestamp outside replay window")
	ErrWebhookInvalidSignature = errors.New("webhook: signature verification failed")
	ErrWebhookReplayDetected  = errors.New("webhook: replay detected, event or nonce already processed")
)

// WebhookConfig defines security policies for external inbound webhooks.
type WebhookConfig struct {
	PrimarySecret   string        `json:"-"` // Never serialized in logs
	RotatedSecrets  []string      `json:"-"` // Previous valid secrets during rotation
	ReplayWindow    time.Duration `json:"replay_window"`
	MaxFutureDrift  time.Duration `json:"max_future_drift"`
}

// DefaultWebhookConfig provides sensible enterprise defaults.
func DefaultWebhookConfig(primarySecret string) WebhookConfig {
	return WebhookConfig{
		PrimarySecret:  primarySecret,
		RotatedSecrets: nil,
		ReplayWindow:   5 * time.Minute,
		MaxFutureDrift: 1 * time.Minute,
	}
}

// WebhookSecurityManager validates inbound webhooks against replay, forgery, and tampering.
type WebhookSecurityManager struct {
	mu           sync.Mutex
	cfg          WebhookConfig
	seenNonces   map[string]time.Time // nonce -> seenAt
	seenEvents   map[string]time.Time // event_id -> seenAt
	cleanupTicker *time.Ticker
}

// NewWebhookSecurityManager creates a thread-safe webhook verifier.
func NewWebhookSecurityManager(cfg WebhookConfig) *WebhookSecurityManager {
	if cfg.ReplayWindow <= 0 {
		cfg.ReplayWindow = 5 * time.Minute
	}
	if cfg.MaxFutureDrift <= 0 {
		cfg.MaxFutureDrift = 1 * time.Minute
	}

	return &WebhookSecurityManager{
		cfg:        cfg,
		seenNonces: make(map[string]time.Time),
		seenEvents: make(map[string]time.Time),
	}
}

// ComputeSignature calculates the expected hex-encoded HMAC-SHA256 of the payload.
// Message format: fmt.Sprintf("t=%s,n=%s,b=%s", timestamp, nonce, payload)
func ComputeSignature(secret, timestamp, nonce string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("t=%s,n=%s,b=%s", timestamp, nonce, string(payload))))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify verifies payload authenticity, timestamp fresh check, and anti-replay nonce/event_id cache.
func (wsm *WebhookSecurityManager) Verify(timestampStr, nonce, eventID, providedSig string, payload []byte) error {
	if providedSig == "" || timestampStr == "" {
		return ErrWebhookMissingHeader
	}

	// 1. Timestamp Freshness Check
	tsInt, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: invalid timestamp format", ErrWebhookTimestampExpired)
	}
	eventTime := time.Unix(tsInt, 0)
	now := time.Now()

	if now.Sub(eventTime) > wsm.cfg.ReplayWindow {
		return fmt.Errorf("%w: event is too old (delta=%v, max=%v)", ErrWebhookTimestampExpired, now.Sub(eventTime), wsm.cfg.ReplayWindow)
	}
	if eventTime.Sub(now) > wsm.cfg.MaxFutureDrift {
		return fmt.Errorf("%w: event timestamp in the future (delta=%v)", ErrWebhookTimestampExpired, eventTime.Sub(now))
	}

	// 2. Signature verification with Constant-Time Comparison and Secret Rotation support
	allSecrets := append([]string{wsm.cfg.PrimarySecret}, wsm.cfg.RotatedSecrets...)
	signatureMatched := false

	for _, secret := range allSecrets {
		if secret == "" {
			continue
		}
		expectedSig := ComputeSignature(secret, timestampStr, nonce, payload)
		if subtle.ConstantTimeCompare([]byte(expectedSig), []byte(providedSig)) == 1 {
			signatureMatched = true
			break
		}
	}

	if !signatureMatched {
		return ErrWebhookInvalidSignature
	}

	// 3. Anti-Replay Nonce and EventID Check (Atomic)
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	wsm.cleanupExpired(now)

	if nonce != "" {
		if _, exists := wsm.seenNonces[nonce]; exists {
			return fmt.Errorf("%w: duplicate nonce %s", ErrWebhookReplayDetected, nonce)
		}
		wsm.seenNonces[nonce] = now
	}

	if eventID != "" {
		if _, exists := wsm.seenEvents[eventID]; exists {
			return fmt.Errorf("%w: duplicate event_id %s", ErrWebhookReplayDetected, eventID)
		}
		wsm.seenEvents[eventID] = now
	}

	return nil
}

// cleanupExpired purges entries older than replay window.
func (wsm *WebhookSecurityManager) cleanupExpired(now time.Time) {
	threshold := now.Add(-wsm.cfg.ReplayWindow)
	for nonce, seenAt := range wsm.seenNonces {
		if seenAt.Before(threshold) {
			delete(wsm.seenNonces, nonce)
		}
	}
	for eventID, seenAt := range wsm.seenEvents {
		if seenAt.Before(threshold) {
			delete(wsm.seenEvents, eventID)
		}
	}
}
