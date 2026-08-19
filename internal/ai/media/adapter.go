// Package media defines AIMediaAdapter — the bridge between AstraCalls audio
// and the AI provider. AstraCalls only calls OnPeerAudio with float32 samples.
// This adapter converts, forwards, and returns synthesised audio via WriteFunc.
package media

import (
	"context"
	"encoding/binary"
	"log/slog"
	"math"
	"sync"
	"time"

	"wacalls/internal/ai/events"
	"wacalls/internal/ai/provider"
	"wacalls/internal/ai/session"
)

const (
	frameBytes = 640 // PCM_S16LE 16kHz mono 20ms = 320 samples * 2 bytes
)

// WriteFunc sends synthesised PCM audio back to the AstraCalls call leg.
type WriteFunc func(pcm []float32)

// AIMediaAdapter connects an AstraCalls call leg to an AI provider.
type AIMediaAdapter struct {
	log     *slog.Logger
	sess    *session.AISession
	bus     *events.Bus
	prov    provider.Provider
	cfg     provider.Config
	writeFn WriteFunc

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc

	// barge-in
	bargeMu     sync.Mutex
	bargeActive bool

	// observability
	audioFramesIn int
	startTime     time.Time
	ttfbSet       bool
	ttfbMs        int64
}

// NewAIMediaAdapter creates an adapter.
func NewAIMediaAdapter(
	sess *session.AISession,
	prov provider.Provider,
	cfg provider.Config,
	bus *events.Bus,
	writeFn WriteFunc,
	log *slog.Logger,
) *AIMediaAdapter {
	if log == nil {
		log = slog.Default()
	}
	return &AIMediaAdapter{
		log:     log,
		sess:    sess,
		bus:     bus,
		prov:    prov,
		cfg:     cfg,
		writeFn: writeFn,
	}
}

// Start begins the AI session: connects to the provider, starts audio loops.
func (a *AIMediaAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return nil // idempotent
	}

	adCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.startTime = time.Now()

	outCh, err := a.prov.Start(adCtx, a.cfg)
	if err != nil {
		cancel()
		return err
	}

	a.running = true
	a.sess.MarkStarted()
	a.sess.SetState(session.StateListening)
	a.bus.Publish(events.Event{
		Kind:      events.KindAIStarted,
		SessionID: a.sess.ID,
		TenantID:  a.sess.TenantID,
		CallID:    a.sess.CallID,
	})

	go a.playbackLoop(adCtx, outCh)
	return nil
}

// OnPeerAudio is called by AstraCalls for every incoming audio frame.
func (a *AIMediaAdapter) OnPeerAudio(samples []float32) {
	a.mu.Lock()
	running := a.running
	a.mu.Unlock()
	if !running {
		return
	}

	a.bargeMu.Lock()
	barge := a.bargeActive
	a.bargeMu.Unlock()
	if barge {
		a.handleBargeIn()
	}

	pcm := float32ToPCM(samples)
	for len(pcm) >= frameBytes {
		_ = a.prov.SendAudio(provider.PCMChunk{Data: pcm[:frameBytes]})
		pcm = pcm[frameBytes:]
		a.audioFramesIn++
	}
}

// NotifyBargeIn is called by VAD when speech is detected during AI output.
func (a *AIMediaAdapter) NotifyBargeIn() {
	a.bargeMu.Lock()
	a.bargeActive = true
	a.bargeMu.Unlock()
}

// Stop terminates the adapter and provider session.
func (a *AIMediaAdapter) Stop(reason string) {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return
	}
	a.running = false
	cancel := a.cancel
	a.mu.Unlock()

	_ = a.prov.Stop()
	if cancel != nil {
		cancel()
	}

	duration := time.Since(a.startTime)
	outcome := &session.Outcome{
		Reason:      reason,
		Duration:    duration,
		AudioFrames: a.audioFramesIn,
		TTFBms:      a.ttfbMs,
	}
	a.sess.MarkEnded(outcome)
	a.sess.SetState(session.StateEnded)
	a.bus.Publish(events.Event{
		Kind:      events.KindAIEnded,
		SessionID: a.sess.ID,
		TenantID:  a.sess.TenantID,
		CallID:    a.sess.CallID,
		Payload:   map[string]any{"reason": reason, "duration_ms": duration.Milliseconds()},
	})
	a.log.Info("ai_media_adapter: stopped", "session", a.sess.ID, "reason", reason)
}

// ─── barge-in ────────────────────────────────────────────────────────────────

func (a *AIMediaAdapter) handleBargeIn() {
	a.bargeMu.Lock()
	if !a.bargeActive {
		a.bargeMu.Unlock()
		return
	}
	a.bargeActive = false
	a.bargeMu.Unlock()

	_ = a.prov.Interrupt()
	a.sess.SetState(session.StateInterrupted)
	a.bus.Publish(events.Event{
		Kind:      events.KindAIInterrupted,
		SessionID: a.sess.ID,
		TenantID:  a.sess.TenantID,
		CallID:    a.sess.CallID,
	})
	a.log.Debug("barge-in handled", "session", a.sess.ID)
	a.sess.SetState(session.StateListening)
	a.bus.Publish(events.Event{
		Kind:      events.KindAIListening,
		SessionID: a.sess.ID,
		TenantID:  a.sess.TenantID,
		CallID:    a.sess.CallID,
	})
}

// ─── playback loop ────────────────────────────────────────────────────────────

func (a *AIMediaAdapter) playbackLoop(ctx context.Context, outCh <-chan provider.PCMChunk) {
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-outCh:
			if !ok {
				return
			}
			if !a.ttfbSet {
				a.ttfbSet = true
				a.ttfbMs = time.Since(a.startTime).Milliseconds()
			}
			a.bargeMu.Lock()
			barge := a.bargeActive
			a.bargeMu.Unlock()
			if barge {
				continue
			}
			samples := pcmToFloat32(chunk.Data)
			a.sess.SetState(session.StateSpeaking)
			if a.writeFn != nil {
				a.writeFn(samples)
			}
		}
	}
}

// ─── PCM helpers (package-private) ───────────────────────────────────────────

func float32ToPCM(samples []float32) []byte {
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		v := math.Round(float64(s) * 32767)
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(v)))
	}
	return out
}

func pcmToFloat32(b []byte) []float32 {
	n := len(b) / 2
	out := make([]float32, n)
	for i := range out {
		s := int16(binary.LittleEndian.Uint16(b[i*2:]))
		out[i] = float32(s) / 32768.0
	}
	return out
}
