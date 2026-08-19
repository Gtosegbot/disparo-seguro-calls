package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	geminiWSURL    = "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1alpha.GenerativeService.BidiGenerateContent"
	defaultGeminiModel = "gemini-2.0-flash-exp"
)

// GeminiRealtime implements Provider using the Google Gemini Live Bidirectional WebSocket API.
type GeminiRealtime struct {
	apiKey string
	log    *slog.Logger

	mu     sync.Mutex
	conn   *websocket.Conn
	cancel context.CancelFunc
	out    chan PCMChunk

	interrupted bool
	wg          sync.WaitGroup
}

// NewGeminiRealtime creates a GeminiRealtime provider.
func NewGeminiRealtime(apiKey string, log *slog.Logger) *GeminiRealtime {
	if log == nil {
		log = slog.Default()
	}
	return &GeminiRealtime{apiKey: apiKey, log: log}
}

func (g *GeminiRealtime) Name() string { return "gemini_realtime" }

// Start establishes the WebSocket session to the Gemini Live API.
func (g *GeminiRealtime) Start(ctx context.Context, cfg Config) (<-chan PCMChunk, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn != nil {
		return nil, errors.New("gemini_realtime: already started")
	}

	wsCtx, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	g.out = make(chan PCMChunk, 64)

	// A API do Gemini Live geralmente recebe a chave de API via query parameter no WebSocket dial.
	model := cfg.Model
	if model == "" {
		model = defaultGeminiModel
	}

	url := fmt.Sprintf("%s?key=%s", geminiWSURL, g.apiKey)
	g.log.Info("gemini_realtime: dialing websocket", "model", model)

	conn, _, err := websocket.Dial(wsCtx, url, &websocket.DialOptions{
		HTTPHeader: http.Header{},
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("gemini_realtime: ws dial failed: %w", err)
	}
	g.conn = conn

	// Enviar a configuração inicial da sessão Live (setup message)
	setupMsg := map[string]any{
		"setup": map[string]any{
			"model": "models/" + model,
			"generation_config": map[string]any{
				"response_modalities": []string{"AUDIO"},
				"speech_config": map[string]any{
					"voice_config": map[string]any{
						"prebuilt_voice_config": map[string]any{
							"voice_name": cfg.Voice, // e.g. "Puck", "Charon"
						},
					},
				},
			},
			"system_instruction": map[string]any{
				"parts": []map[string]any{
					{"text": cfg.SystemPrompt},
				},
			},
		},
	}

	if err := g.writeJSON(setupMsg); err != nil {
		conn.Close(websocket.StatusInternalError, "setup failed")
		cancel()
		return nil, fmt.Errorf("gemini_realtime: setup message: %w", err)
	}

	g.wg.Add(1)
	go g.readLoop(wsCtx)

	return g.out, nil
}

// SendAudio forwards client PCM audio chunks to Gemini.
func (g *GeminiRealtime) SendAudio(chunk PCMChunk) error {
	g.mu.Lock()
	conn := g.conn
	g.mu.Unlock()

	if conn == nil {
		return errors.New("gemini_realtime: session not started")
	}

	// O formato de áudio do Gemini Live recebe chunks em base64 dentro de realTimeInput
	msg := map[string]any{
		"realtime_input": map[string]any{
			"media_chunks": []map[string]any{
				{
					"mime_type": "audio/pcm",
					"data":      chunk.Data, // Codificação automática pelo JSON encoder do Go
				},
			},
		},
	}

	return g.writeJSON(msg)
}

func (g *GeminiRealtime) Interrupt() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.interrupted = true
	
	// Envia sinalização para o Gemini parar a fala atual
	if g.conn != nil {
		msg := map[string]any{
			"client_content": map[string]any{
				"turns": []any{},
				"turn_complete": true,
			},
		}
		_ = g.writeJSON(msg)
	}
	return nil
}

func (g *GeminiRealtime) Stop() error {
	g.mu.Lock()
	conn := g.conn
	cancel := g.cancel
	g.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "stopping session")
	}

	g.wg.Wait()

	g.mu.Lock()
	g.conn = nil
	g.cancel = nil
	g.mu.Unlock()

	g.log.Info("gemini_realtime: session stopped")
	return nil
}

func (g *GeminiRealtime) writeJSON(v any) error {
	g.mu.Lock()
	conn := g.conn
	g.mu.Unlock()

	if conn == nil {
		return errors.New("gemini_realtime: connection closed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	w, err := conn.Writer(ctx, websocket.MessageText)
	if err != nil {
		return err
	}
	defer w.Close()

	return json.NewEncoder(w).Encode(v)
}

func (g *GeminiRealtime) readLoop(ctx context.Context) {
	defer g.wg.Done()
	defer close(g.out)

	for {
		g.mu.Lock()
		conn := g.conn
		g.mu.Unlock()

		if conn == nil {
			return
		}

		_, r, err := conn.Reader(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				g.log.Error("gemini_realtime: read error", "err", err)
			}
			return
		}

		var resp struct {
			ServerContent struct {
				ModelTurn struct {
					Parts []struct {
						InlineData struct {
							MimeType string `json:"mimeType"`
							Data     []byte `json:"data"`
						} `json:"inlineData"`
					} `json:"parts"`
				} `json:"modelTurn"`
			} `json:"serverContent"`
		}

		if err := json.NewDecoder(r).Decode(&resp); err != nil {
			g.log.Error("gemini_realtime: decode message error", "err", err)
			continue
		}

		// Emite os chunks de áudio recebidos da IA de volta para a fila downlink
		for _, part := range resp.ServerContent.ModelTurn.Parts {
			if part.InlineData.MimeType == "audio/pcm" || len(part.InlineData.Data) > 0 {
				data := part.InlineData.Data
				// Quebra em frames PCM internos de 640 bytes (20ms)
				for len(data) > 0 {
					chunkSize := 640
					if len(data) < chunkSize {
						chunkSize = len(data)
					}
					select {
					case <-ctx.Done():
						return
					case g.out <- PCMChunk{Data: data[:chunkSize]}:
					}
					data = data[chunkSize:]
				}
			}
		}
	}
}
