package provider

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// LoopbackProvider acts as an echo loopback for testing internal audio media routing.
// Everything sent to SendAudio is immediately echoed back on the channel returned by Start.
type LoopbackProvider struct {
	log *slog.Logger
	mu  sync.Mutex
	out chan PCMChunk
	running bool

	// Metrics
	FramesIn  int
	BytesIn   int
	StartTime time.Time
	Duration  time.Duration
}

// NewLoopbackProvider creates a new LoopbackProvider.
func NewLoopbackProvider(log *slog.Logger) *LoopbackProvider {
	if log == nil {
		log = slog.Default()
	}
	return &LoopbackProvider{log: log}
}

func (l *LoopbackProvider) Name() string {
	return "loopback"
}

func (l *LoopbackProvider) Start(ctx context.Context, cfg Config) (<-chan PCMChunk, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		return nil, errors.New("loopback provider already running")
	}
	l.out = make(chan PCMChunk, 100)
	l.running = true
	l.StartTime = time.Now()
	l.FramesIn = 0
	l.BytesIn = 0
	l.log.Info("LoopbackProvider started")
	return l.out, nil
}

func (l *LoopbackProvider) SendAudio(chunk PCMChunk) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		return errors.New("loopback provider not running")
	}
	l.FramesIn++
	l.BytesIn += len(chunk.Data)
	
	// Echo back immediately
	select {
	case l.out <- chunk:
	default:
		l.log.Warn("loopback buffer full, dropped frame")
	}
	return nil
}

func (l *LoopbackProvider) Interrupt() error {
	l.log.Info("LoopbackProvider interrupted")
	return nil
}

func (l *LoopbackProvider) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		return nil
	}
	l.running = false
	close(l.out)
	l.Duration = time.Since(l.StartTime)
	l.log.Info("LoopbackProvider stopped", "duration_ms", l.Duration.Milliseconds(), "frames_in", l.FramesIn, "bytes_in", l.BytesIn)
	return nil
}
