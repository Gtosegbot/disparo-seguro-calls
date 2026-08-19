package dialer

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"
)

// InstanceProvider abstracts instance querying to prevent circular package dependencies.
type InstanceProvider interface {
	GetInstancesPool(ctx context.Context, ids []string) ([]InstanceInfo, error)
	StartOutgoingCall(ctx context.Context, instanceID, phone string) (string, error)
}

// InstanceInfo is a DTO for scheduler evaluation.
type InstanceInfo struct {
	ID                 string
	Status             string // e.g., CONNECTED, BUSY, OFFLINE, RECONNECTING
	ActiveCalls        int
	MaxConcurrentCalls int
}

// Scheduler orchestrates campaigns, pacing calls according to capacity, Round-Robin and intervals.
type Scheduler struct {
	queue            *Queue
	ip               InstanceProvider
	log              *slog.Logger
	mu               sync.Mutex
	runningCampaigns map[string]context.CancelFunc
	instanceLastDial map[string]time.Time // Controle de intervalo por linha
}

// NewScheduler creates a campaign Scheduler.
func NewScheduler(q *Queue, ip InstanceProvider, log *slog.Logger) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{
		queue:            q,
		ip:               ip,
		log:              log,
		runningCampaigns: make(map[string]context.CancelFunc),
		instanceLastDial: make(map[string]time.Time),
	}
}

// StartCampaign launches a background scheduler loop for a campaign.
func (s *Scheduler) StartCampaign(ctx context.Context, campaignID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, active := s.runningCampaigns[campaignID]; active {
		return // Campaign already running
	}

	camp, ok := s.queue.GetCampaign(campaignID)
	if !ok {
		s.log.Warn("campaign not found, cannot start", "campaign_id", campaignID)
		return
	}

	camp.Status = StatusRunning
	s.queue.SaveCampaign(camp)

	cCtx, cancel := context.WithCancel(ctx)
	s.runningCampaigns[campaignID] = cancel

	go s.campaignLoop(cCtx, campaignID)
	s.log.Info("campaign scheduler started", "campaign_id", campaignID)
}

// StopCampaign stops the scheduler loop.
func (s *Scheduler) StopCampaign(campaignID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, active := s.runningCampaigns[campaignID]
	if !active {
		return
	}

	cancel()
	delete(s.runningCampaigns, campaignID)

	camp, ok := s.queue.GetCampaign(campaignID)
	if ok {
		camp.Status = StatusStopped
		s.queue.SaveCampaign(camp)
	}
	s.log.Info("campaign scheduler stopped", "campaign_id", campaignID)
}

func (s *Scheduler) campaignLoop(ctx context.Context, campaignID string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Circular Round-Robin index pointer
	rrIndex := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			camp, ok := s.queue.GetCampaign(campaignID)
			if !ok || camp.Status != StatusRunning {
				return
			}

			// 1. Get eligible instances and available slots
			insts, err := s.ip.GetInstancesPool(ctx, camp.InstancePool)
			if err != nil {
				s.log.Error("failed to query instances pool", "err", err)
				continue
			}

			activeInstances := filterEligibleInstances(insts)
			if len(activeInstances) == 0 {
				s.log.Warn("no healthy/connected instances in pool", "campaign_id", campaignID)
				continue
			}

			// 2. Calculate dynamic capacity (available slots sum)
			totalActiveCalls := 0
			totalMaxCalls := 0
			for _, inst := range activeInstances {
				totalActiveCalls += inst.ActiveCalls
				totalMaxCalls += inst.MaxConcurrentCalls
			}

			// Capacity limit formula
			globalLimit := camp.MaxConcurrentCalls
			if globalLimit <= 0 {
				globalLimit = totalMaxCalls
			}
			availableSlots := globalLimit - totalActiveCalls

			if availableSlots <= 0 {
				// Concorrência lotada
				continue
			}

			// 3. Round-Robin line selection
			if rrIndex >= len(activeInstances) {
				rrIndex = 0
			}
			selectedInstance := activeInstances[rrIndex]
			rrIndex = (rrIndex + 1) % len(activeInstances)

			// 4. Verify per-instance interval (with jitter)
			lastDial := s.instanceLastDial[selectedInstance.ID]
			intervalSeconds := camp.DialIntervalSeconds
			if intervalSeconds <= 0 {
				intervalSeconds = 5 // default
			}

			// Apply random jitter (+- 20%)
			jitter := float64(intervalSeconds) * 0.20
			jitterDuration := time.Duration((rand.Float64()*2-1)*jitter) * time.Second
			requiredInterval := time.Duration(intervalSeconds)*time.Second + jitterDuration

			if time.Since(lastDial) < requiredInterval {
				continue // Selected instance is in cooldown
			}

			// 5. Get next queued job
			job, err := s.queue.NextJob(ctx, campaignID)
			if err != nil || job == nil {
				continue // No jobs to dial
			}

			// Lock instance time slot
			s.instanceLastDial[selectedInstance.ID] = time.Now()

			// 6. Launch outgoing call asynchronously
			go s.dispatchCall(ctx, selectedInstance.ID, job)
		}
	}
}

func (s *Scheduler) dispatchCall(ctx context.Context, instanceID string, job *DialerJob) {
	s.log.Info("dialing lead", "job_id", job.ID, "phone", job.Phone, "instance_id", instanceID)
	_ = s.queue.UpdateJobStatus(ctx, job.ID, JobDialing)
	job.InstanceID = instanceID

	callID, err := s.ip.StartOutgoingCall(ctx, instanceID, job.Phone)
	if err != nil {
		s.log.Error("dial failed", "job_id", job.ID, "err", err)
		_ = s.queue.UpdateJobStatus(ctx, job.ID, JobFailed)
		_ = s.queue.HandleRetry(ctx, job.ID)
		return
	}

	job.CallID = callID
	_ = s.queue.UpdateJobStatus(ctx, job.ID, JobRinging)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func filterEligibleInstances(pool []InstanceInfo) []InstanceInfo {
	var out []InstanceInfo
	for _, inst := range pool {
		// Ignora OFFLINE, REAUTH_REQUIRED e DRAINING
		if inst.Status == "CONNECTED" || inst.Status == "BUSY" || inst.Status == "AVAILABLE" {
			if inst.ActiveCalls < inst.MaxConcurrentCalls {
				out = append(out, inst)
			}
		}
	}
	return out
}
