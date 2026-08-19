package instance

import (
	"time"
)

// InstanceState represents the lifecycle status of a WhatsApp line connection.
type InstanceState string

const (
	StatePairing        InstanceState = "PAIRING"
	StateAvailable      InstanceState = "AVAILABLE"
	StateConnected      InstanceState = "CONNECTED"
	StateBusy           InstanceState = "BUSY"
	StateDegraded       InstanceState = "DEGRADED"
	StateReconnecting   InstanceState = "RECONNECTING"
	StateReauthRequired InstanceState = "REAUTH_REQUIRED"
	StateComplianceHold InstanceState = "COMPLIANCE_HOLD"
	StateDraining       InstanceState = "DRAINING"
	StateOffline        InstanceState = "OFFLINE"
)

// Instance is the product-level abstraction representing one WhatsApp connection/account.
// Handled multi-tenant and secure.
type Instance struct {
	ID                 string         `json:"instance_id"`
	TenantID           string         `json:"tenant_id"`
	SessionID          string         `json:"session_id"` // Mapeia para a Session do AstraCalls
	Phone              string         `json:"phone"`
	DisplayName        string         `json:"display_name"`
	Status             InstanceState  `json:"status"`
	ProxyID            string         `json:"proxy_id"`            // Preparação para Proxy Manager
	ChatSeguroInboxID  string         `json:"chatseguro_inbox_id"` // Preparação para ChatSeguro integration
	MaxConcurrentCalls int            `json:"max_concurrent_calls"`
	ActiveCalls        int            `json:"active_calls"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

// PairingResponse is returned by pairing endpoints, isolating client from raw whatsmeow tokens.
type PairingResponse struct {
	PairingSessionID string        `json:"pairing_session_id"`
	Status           InstanceState `json:"status"`
	QR               string        `json:"qr,omitempty"`         // base64 encoded PNG
	ExpiresAt        time.Time     `json:"expires_at"`
}
