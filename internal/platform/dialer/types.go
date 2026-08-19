package dialer

import (
	"time"
)

// CampaignMode defines the type of dialer behavior.
type CampaignMode string

const (
	ModeH2H CampaignMode = "h2h" // Human-to-Human: calls go direct to queue of agents
	ModeAI  CampaignMode = "ai"  // AI session: calls start a Voice Agent Session
)

// CampaignStatus is the current execution state of a campaign.
type CampaignStatus string

const (
	StatusDraft     CampaignStatus = "DRAFT"
	StatusReady     CampaignStatus = "READY"
	StatusRunning   CampaignStatus = "RUNNING"
	StatusPaused    CampaignStatus = "PAUSED"
	StatusDraining  CampaignStatus = "DRAINING"
	StatusCompleted CampaignStatus = "COMPLETED"
	StatusStopped   CampaignStatus = "STOPPED"
	StatusError     CampaignStatus = "ERROR"
)

// DialerCampaign represents a mass calling campaign owned by a tenant.
type DialerCampaign struct {
	ID                  string         `json:"campaign_id"`
	TenantID            string         `json:"tenant_id"`
	Name                string         `json:"name"`
	Mode                CampaignMode   `json:"mode"`
	Status              CampaignStatus `json:"status"`
	MaxConcurrentCalls  int            `json:"max_concurrent_calls"`
	DialIntervalSeconds int            `json:"dial_interval_seconds"` // Intervalo entre disparos por instância
	MaxAttempts         int            `json:"max_attempts"`
	RetryDelaySeconds   int            `json:"retry_delay_seconds"`
	Strategy            string         `json:"strategy"` // "round-robin", etc.
	InstancePool        []string       `json:"instance_pool"` // IDs das instâncias autorizadas
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

// JobStatus represents the state of a single dialer lead job.
type JobStatus string

const (
	JobQueued       JobStatus = "QUEUED"
	JobReserved     JobStatus = "RESERVED"
	JobDialing      JobStatus = "DIALING"
	JobRinging      JobStatus = "RINGING"
	JobConnected    JobStatus = "CONNECTED"
	JobAIStarting   JobStatus = "AI_STARTING"
	JobGreeting     JobStatus = "GREETING"
	JobListening    JobStatus = "LISTENING"
	JobThinking     JobStatus = "THINKING"
	JobSpeaking     JobStatus = "SPEAKING"
	JobEnded        JobStatus = "ENDED"
	JobCompleted    JobStatus = "COMPLETED"
	JobBusy         JobStatus = "BUSY"
	JobNoAnswer     JobStatus = "NO_ANSWER"
	JobFailed       JobStatus = "FAILED"
	JobRetryPending JobStatus = "RETRY_PENDING"
	JobSkipped      JobStatus = "SKIPPED"
)

// DialerJob represents a single lead queue target to dial in a campaign.
type DialerJob struct {
	ID                string    `json:"job_id"`
	CampaignID        string    `json:"campaign_id"`
	TenantID          string    `json:"tenant_id"`
	LeadID            string    `json:"lead_id"`
	Phone             string    `json:"phone"`
	Name              string    `json:"name"`
	Position          int       `json:"position"`
	Status            JobStatus `json:"status"`
	Attempt           int       `json:"attempt"`
	InstanceID        string    `json:"instance_id,omitempty"` // Qual linha disparou
	CallID            string    `json:"call_id,omitempty"`
	AISessionID       string    `json:"ai_session_id,omitempty"`
	ProviderSessionID string    `json:"provider_session_id,omitempty"`
	VoiceProfile      string    `json:"voice_profile,omitempty"`
	AgentID           string    `json:"agent_id,omitempty"`
	Provider          string    `json:"provider,omitempty"`
	Outcome           string    `json:"outcome,omitempty"`
	NextAttemptAt     time.Time `json:"next_attempt_at"`
	CreatedAt         time.Time `json:"created_at"`
	StartedAt         time.Time `json:"started_at,omitempty"`
	EndedAt           time.Time `json:"ended_at,omitempty"`
}

// DialerMetrics contains metrics aggregated per campaign or per instance.
type DialerMetrics struct {
	ActiveCalls     int           `json:"active_calls"`
	QueueSize       int           `json:"queue_size"`
	AvailableSlots  int           `json:"available_slots"`
	CallsStarted    int           `json:"calls_started"`
	CallsCompleted  int           `json:"calls_completed"`
	Busy            int           `json:"busy"`
	NoAnswer        int           `json:"no_answer"`
	Failed          int           `json:"failed"`
	Retries         int           `json:"retries"`
	CallsPerMinute  float64       `json:"calls_per_minute"`
	AverageDuration time.Duration `json:"average_duration"`
}
