package dialer

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"
)

var (
	ErrJobNotFound = errors.New("job not found")
)

// Queue manages campaign jobs in FIFO order with state tracking.
type Queue struct {
	mu          sync.RWMutex
	jobs        map[string]*DialerJob
	campaigns   map[string]*DialerCampaign
	reservation ReservationEngine
}

// NewQueue creates a new Queue engine.
func NewQueue(res ReservationEngine) *Queue {
	return &Queue{
		jobs:        make(map[string]*DialerJob),
		campaigns:   make(map[string]*DialerCampaign),
		reservation: res,
	}
}

// SaveCampaign saves or updates a campaign in memory/DB.
func (q *Queue) SaveCampaign(c *DialerCampaign) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.campaigns[c.ID] = c
}

// GetCampaign returns the campaign.
func (q *Queue) GetCampaign(id string) (*DialerCampaign, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	c, ok := q.campaigns[id]
	return c, ok
}

// Enqueue adds a list of jobs to the dialer queue.
func (q *Queue) Enqueue(jobs []*DialerJob) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range jobs {
		q.jobs[j.ID] = j
	}
}

// NextJob returns the next eligible QUEUED job for a campaign, reserving it atomically.
func (q *Queue) NextJob(ctx context.Context, campaignID string) (*DialerJob, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var next *DialerJob
	now := time.Now()

	for _, j := range q.jobs {
		if j.CampaignID == campaignID && (j.Status == JobQueued || j.Status == JobRetryPending) {
			if j.NextAttemptAt.Before(now) {
				if next == nil || j.Position < next.Position {
					next = j
				}
			}
		}
	}

	if next == nil {
		return nil, nil // No jobs ready
	}

	// Try atomic reservation lease (expires lock safety)
	err := q.reservation.Reserve(ctx, next.ID, 30)
	if err != nil {
		return nil, err
	}

	next.Status = JobReserved
	return next, nil
}

// UpdateJobStatus updates the state of a job and releases/commits the reservation lock.
func (q *Queue) UpdateJobStatus(ctx context.Context, jobID string, status JobStatus) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, ok := q.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}

	j.Status = status
	if status == JobDialing {
		j.StartedAt = time.Now()
	} else if status == JobCompleted || status == JobFailed || status == JobNoAnswer || status == JobBusy {
		j.EndedAt = time.Now()
		_ = q.reservation.Commit(ctx, jobID)
	}

	return nil
}

// HandleRetry evaluates if a job can retry, planning next attempt delay or marking failed.
func (q *Queue) HandleRetry(ctx context.Context, jobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, ok := q.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}

	camp, ok := q.campaigns[j.CampaignID]
	if !ok {
		return errors.New("campaign not found")
	}

	_ = q.reservation.Release(ctx, jobID)

	if j.Attempt < camp.MaxAttempts {
		j.Attempt++
		j.Status = JobRetryPending

		// Backoff Exponencial com Jitter humano de 20%
		baseDelay := camp.RetryDelaySeconds
		if baseDelay <= 0 {
			baseDelay = 10 // delay base default
		}
		// Multiplicador: 2 ^ (attempt - 1)
		multiplier := 1 << (j.Attempt - 1)
		delaySecs := float64(baseDelay * multiplier)

		// Aplica jitter (+- 20%)
		jitter := delaySecs * 0.20
		jitterOffset := (rand.Float64()*2 - 1) * jitter
		finalDelay := time.Duration(delaySecs+jitterOffset) * time.Second
		if finalDelay < 1*time.Second {
			finalDelay = 1 * time.Second
		}
		j.NextAttemptAt = time.Now().Add(finalDelay)
	} else {
		j.Status = JobFailed
	}

	return nil
}

// ListCampaigns returns campaigns for tenant.
func (q *Queue) ListCampaigns(tenantID string) []*DialerCampaign {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var out []*DialerCampaign
	for _, c := range q.campaigns {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out
}

// GetJobsForCampaign returns all jobs for a campaign.
func (q *Queue) GetJobsForCampaign(campaignID string) []*DialerJob {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var out []*DialerJob
	for _, j := range q.jobs {
		if j.CampaignID == campaignID {
			out = append(out, j)
		}
	}
	return out
}
