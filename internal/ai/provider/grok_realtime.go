// Package provider - GrokRealtime is the first concrete Provider implementation.
// Uses the xAI Realtime API over WebSocket.
// AstraCalls core never imports this package directly.
package provider

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	grokWSURL    = "wss://api.x.ai/v1/realtime"
	defaultModel = "grok-2-voice-preview"
	frameBytes   = 640 // PCM_S16LE 16kHz mono 20ms
)

// GrokRealtime implements Provider using the xAI Realtime WebSocket API.
type GrokRealtime struct {
	apiKey string
	log    *slog.Logger

	mu     sync.Mutex
	conn   *websocket.Conn
	cancel context.CancelFunc
	out    chan PCMChunk

	interrupted bool
	wg          sync.WaitGroup
}

// NewGrokRealtime creates a GrokRealtime provider.
// apiKey is resolved server-side by the Registry — never exposed to clients.
func NewGrokRealtime(apiKey string, log *slog.Logger) *GrokRealtime {
	if log == nil {
		log = slog.Default()
	}
	return &GrokRealtime{apiKey: apiKey, log: log}
}

func (g *GrokRealtime) Name() string { return "grok_realtime" }

// Start establishes the WebSocket session and returns the audio output channel.
func (g *GrokRealtime) Start(ctx context.Context, cfg Config) (<-chan PCMChunk, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn != nil {
		return nil, errors.New("grok_realtime: already started")
	}

	wsCtx, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	g.out = make(chan PCMChunk, 64)

	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+g.apiKey)

	model := cfg.Model
	if model == "" {
		model = defaultModel
	}

	url := fmt.Sprintf("%s?model=%s", grokWSURL, model)
	conn, _, err := websocket.Dial(wsCtx, url, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("grok_realtime: ws dial: %w", err)
	}
	g.conn = conn

	initMsg := map[string]any{
		"type": "session.update",
		"session": map[string]any{
			"modalities":          []string{"text", "audio"},
			"instructions":        cfg.SystemPrompt,
			"voice":               cfg.Voice,
			"input_audio_format":  "pcm16",
			"output_audio_format": "pcm16",
			"turn_detection": map[string]any{
				"type":                "server_vad",
				"silence_duration_ms": 500,
			},
		},
	}
	if err := g.writeJSON(initMsg); err != nil {
		conn.Close(websocket.StatusInternalError, "init failed")
		cancel()
		return nil, fmt.Errorf("grok_realtime: session.update: %w", err)
	}

	g.wg.Add(1)
	go g.readLoop(wsCtx)

	g.log.Info("grok_realtime: session started", "model", model, "voice", cfg.Voice)
	return g.out, nil
}

// SendAudio delivers a PCM frame to the provider.
func (g *GrokRealtime) SendAudio(chunk PCMChunk) error {
	g.mu.Lock()
	interrupted := g.interrupted
	conn := g.conn
	g.mu.Unlock()

	if conn == nil {
		return errors.New("grok_realtime: not started")
	}
	if interrupted {
		return nil
	}

	encoded := base64.StdEncoding.EncodeToString(chunk.Data)
	msg := map[string]any{
		"type":  "input_audio_buffer.append",
		"audio": encoded,
	}
	return g.writeJSON(msg)
}

// Interrupt signals barge-in to cancel current TTS output.
func (g *GrokRealtime) Interrupt() error {
	g.mu.Lock()
	g.interrupted = true
	conn := g.conn
	g.mu.Unlock()

	if conn == nil {
		return nil
	}
	err := g.writeJSON(map[string]any{"type": "response.cancel"})
	go func() {
		time.Sleep(300 * time.Millisecond)
		g.mu.Lock()
		g.interrupted = false
		g.mu.Unlock()
	}()
	return err
}

// Stop closes the WebSocket and waits for the reader goroutine.
func (g *GrokRealtime) Stop() error {
	g.mu.Lock()
	conn := g.conn
	cancel := g.cancel
	g.mu.Unlock()

	if conn == nil {
		return nil
	}
	if cancel != nil {
		cancel()
	}
	err := conn.Close(websocket.StatusNormalClosure, "session ended")
	g.wg.Wait()
	g.mu.Lock()
	g.conn = nil
	g.mu.Unlock()
	return err
}

// readLoop receives audio delta events and forwards PCM chunks.
func (g *GrokRealtime) readLoop(ctx context.Context) {
	defer func() {
		g.wg.Done()
		close(g.out)
	}()

	for {
		_, msg, err := g.conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			g.log.Warn("grok_realtime: read error", "err", err)
			return
		}

		var ev map[string]any
		if jsonErr := json.Unmarshal(msg, &ev); jsonErr != nil {
			continue
		}

		evType, _ := ev["type"].(string)
		switch evType {
		case "response.audio.delta":
			b64, _ := ev["delta"].(string)
			if b64 == "" {
				continue
			}
			pcm, decErr := base64.StdEncoding.DecodeString(b64)
			if decErr != nil {
				continue
			}
			for len(pcm) >= frameBytes {
				select {
				case g.out <- PCMChunk{Data: pcm[:frameBytes]}:
				case <-ctx.Done():
					return
				}
				pcm = pcm[frameBytes:]
			}
		case "error":
			g.log.Error("grok_realtime: provider error", "event", ev)
		}
	}
}

func (g *GrokRealtime) writeJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return g.conn.Write(ctx, websocket.MessageText, data)
}

// ─── PCM helpers (exported for shared use) ──────────────────────────────────

// Float32ToPCM converts []float32 (-1..1) to PCM_S16LE bytes.
func Float32ToPCM(samples []float32) []byte {
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

// PCMToFloat32 converts PCM_S16LE bytes to []float32 (-1..1).
func PCMToFloat32(b []byte) []float32 {
	n := len(b) / 2
	out := make([]float32, n)
	for i := range out {
		s := int16(binary.LittleEndian.Uint16(b[i*2:]))
		out[i] = float32(s) / 32768.0
	}
	return out
}
