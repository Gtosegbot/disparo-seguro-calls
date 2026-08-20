package dialer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var (
	ErrInvalidTransition = errors.New("orchestration: invalid job state transition")
	ErrIdempotencyBlock  = errors.New("orchestration: duplicate execution request blocked by idempotency key")
	ErrCircuitOpen       = errors.New("orchestration: provider circuit breaker is open (halted)")
)

// UnifiedContract represents the unified operational contract format for Canvas/Hermes.
type UnifiedContract struct {
	Event          string         `json:"event"`
	Version        string         `json:"version"`
	JobID          string         `json:"job_id"`
	TenantID       string         `json:"tenant_id"`
	Source         string         `json:"source"` // "canvas", "hermes", "api", "system"
	Channel        string         `json:"channel"`
	Operation      string         `json:"operation"`
	Priority       string         `json:"priority"`
	IdempotencyKey string         `json:"idempotency_key"`
	CreatedAt      time.Time      `json:"created_at"`
	Payload        map[string]any `json:"payload"`
}

// JobState defines execution runs machine states.
type JobState string

const (
	StateCreated   JobState = "CREATED"
	StateQueued    JobState = "QUEUED"
	StateRunning   JobState = "RUNNING"
	StatePaused    JobState = "PAUSED"
	StateDraining  JobState = "DRAINING"
	StateCompleted JobState = "COMPLETED"
	StatePartial   JobState = "PARTIAL"
	StateFailed    JobState = "FAILED"
	StateCancelled JobState = "CANCELLED"
)

// ExecutionJob engine properties.
type ExecutionJob struct {
	ID             string    `json:"job_id"`
	TenantID       string    `json:"tenant_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	State          JobState  `json:"state"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// IdempotencyRegistry prevents double-spends of execution requests.
type IdempotencyRegistry struct {
	mu   sync.Mutex
	keys map[string]string // scopedKey (tenantID:key) -> jobID
}

func NewIdempotencyRegistry() *IdempotencyRegistry {
	return &IdempotencyRegistry{keys: make(map[string]string)}
}

// CheckOrRegister returns job ID if exists, otherwise registers the idempotency key atomically per tenant.
func (ir *IdempotencyRegistry) CheckOrRegister(tenantID, key, jobID string) (string, error) {
	if key == "" {
		return jobID, nil // Se for vazia e opcional em GET, permite prosseguir sem travar, mas operações mutáveis validam antes
	}
	ir.mu.Lock()
	defer ir.mu.Unlock()

	scopedKey := tenantID + ":" + key
	if existing, ok := ir.keys[scopedKey]; ok {
		return existing, ErrIdempotencyBlock
	}
	ir.keys[scopedKey] = jobID
	return jobID, nil
}

// ValidateUnifiedContract audits required parameters of the operations payload.
func ValidateUnifiedContract(uc *UnifiedContract) error {
	if uc.TenantID == "" {
		return errors.New("contract validation failed: tenant_id is required")
	}
	if uc.IdempotencyKey == "" {
		return errors.New("contract validation failed: idempotency_key is required for mutable execution")
	}
	if uc.Event == "" {
		return errors.New("contract validation failed: event name is required")
	}
	return nil
}

// JobStateMachine ensures atomic valid state transitions.
type JobStateMachine struct {
	mu   sync.Mutex
	jobs map[string]*ExecutionJob
}

func NewJobStateMachine() *JobStateMachine {
	return &JobStateMachine{jobs: make(map[string]*ExecutionJob)}
}

func (sm *JobStateMachine) CreateJob(id, tenantID, idempotencyKey string) *ExecutionJob {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	j := &ExecutionJob{
		ID:             id,
		TenantID:       tenantID,
		IdempotencyKey: idempotencyKey,
		State:          StateCreated,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	sm.jobs[id] = j
	return j
}

func (sm *JobStateMachine) Transition(id string, to JobState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	j, ok := sm.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	valid := false
	switch j.State {
	case StateCreated:
		valid = (to == StateQueued || to == StateCancelled)
	case StateQueued:
		valid = (to == StateRunning || to == StateCancelled)
	case StateRunning:
		valid = (to == StatePaused || to == StateDraining || to == StateCompleted || to == StatePartial || to == StateFailed)
	case StatePaused:
		valid = (to == StateRunning || to == StateCancelled)
	case StateDraining:
		valid = (to == StateCompleted || to == StateFailed)
	}

	if !valid {
		return fmt.Errorf("cannot move job %s from %s to %s: %w", id, j.State, to, ErrInvalidTransition)
	}

	j.State = to
	j.UpdatedAt = time.Now().UTC()
	return nil
}

func (sm *JobStateMachine) GetJob(id string) (*ExecutionJob, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	j, ok := sm.jobs[id]
	return j, ok
}

// CircuitBreaker halts traffic to specific providers if error rate spikes.
type CircuitBreaker struct {
	mu             sync.Mutex
	failures       map[string]int
	lastFailure    map[string]time.Time
	state          map[string]string // "CLOSED", "OPEN", "HALF-OPEN"
	threshold      int
	cooldownSecs   int
}

func NewCircuitBreaker(threshold, cooldown int) *CircuitBreaker {
	return &CircuitBreaker{
		failures:     make(map[string]int),
		lastFailure:  make(map[string]time.Time),
		state:        make(map[string]string),
		threshold:    threshold,
		cooldownSecs: cooldown,
	}
}

// Check evaluates health, switching state to CLOSED if cooldown expired.
func (cb *CircuitBreaker) Check(providerName string) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	st := cb.state[providerName]
	if st == "" {
		st = "CLOSED"
		cb.state[providerName] = "CLOSED"
	}

	if st == "OPEN" {
		last := cb.lastFailure[providerName]
		if time.Since(last) > time.Duration(cb.cooldownSecs)*time.Second {
			cb.state[providerName] = "HALF-OPEN"
			return nil // Try recovery call
		}
		return ErrCircuitOpen
	}

	return nil
}

// RecordSuccess clears circuit failures.
func (cb *CircuitBreaker) RecordSuccess(providerName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures[providerName] = 0
	cb.state[providerName] = "CLOSED"
}

// RecordFailure increments errors and opens the circuit if threshold exceeded.
func (cb *CircuitBreaker) RecordFailure(providerName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures[providerName]++
	cb.lastFailure[providerName] = time.Now()

	if cb.failures[providerName] >= cb.threshold {
		cb.state[providerName] = "OPEN"
	}
}

// OperationCostEngine aggregates platform and provider expenditures.
type OperationCostEngine struct {
	mu     sync.Mutex
	costs  map[string]float64 // jobID -> cost sum
}

func NewOperationCostEngine() *OperationCostEngine {
	return &OperationCostEngine{costs: make(map[string]float64)}
}

func (ce *OperationCostEngine) RecordCost(jobID string, platformCost, providerCost float64) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.costs[jobID] += platformCost + providerCost
}

func (ce *OperationCostEngine) GetTotalCost(jobID string) float64 {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return ce.costs[jobID]
}
