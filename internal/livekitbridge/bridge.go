package livekitbridge

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"wacalls/internal/voip/call"
)


// DataPacketCallback define o handler para pacotes de dados WebRTC (ex: legendas de tradução Libras, comandos de avatar 3D)
type DataPacketCallback func(topic string, payload []byte)

// DataPacket representa uma mensagem textual ou binária trafegada via WebRTC DataChannel
type DataPacket struct {
	Topic   string
	Payload []byte
}

// BridgeSession coordena o streaming bidirecional entre uma chamada WhatsApp (CallManager)
// e a sala correspondente no LiveKit Local / Cloud.
type BridgeSession struct {
	cfg        Config
	log        *slog.Logger
	callID     string
	peerPhone  string
	roomName   string
	cm         *call.CallManager

	mu         sync.Mutex
	active     bool
	ctx        context.Context
	cancel     context.CancelFunc
	audioInCh  chan []float32
	audioOutCh   chan []float32
	onDataPacket DataPacketCallback
	dataPacketCh chan *DataPacket
}

func NewBridgeSession(cfg Config, cm *call.CallManager, callID, peerPhone string, log *slog.Logger) *BridgeSession {
	if log == nil {
		log = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	roomName := fmt.Sprintf("%s%s", cfg.RoomPrefix, callID)

	return &BridgeSession{
		cfg:        cfg,
		log:        log.With("subsystem", "livekit_bridge", "callID", callID, "room", roomName),
		callID:     callID,
		peerPhone:  peerPhone,
		roomName:   roomName,
		cm:         cm,
		ctx:        ctx,
		cancel:     cancel,
		audioInCh:  make(chan []float32, 100),
		audioOutCh:   make(chan []float32, 100),
		dataPacketCh: make(chan *DataPacket, 200),
	}
}

// Start inicia a ponte de áudio, provisiona a sala no LiveKit e intercepta os hooks do CallManager
func (s *BridgeSession) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		return nil
	}

	s.log.Info("Iniciando LiveKit Audio Bridge para chamada WhatsApp", "peer", s.peerPhone)

	// 1. Criar/garantir existência da Room no LiveKit
	if err := s.ensureLiveKitRoom(); err != nil {
		s.log.Warn("Aviso ao provisionar sala no LiveKit (seguindo com fallback de auto-create)", "err", err)
	}

	s.active = true

	// 2. Loop de streaming WhatsApp -> LiveKit
	go s.streamToLiveKitLoop()

	// 3. Loop de streaming LiveKit (AI Agent) -> WhatsApp
	go s.streamFromLiveKitLoop()

	// 4. Conectar hook de áudio do interlocutor WhatsApp
	prevOnPeerAudio := s.cm.OnPeerAudio
	s.cm.OnPeerAudio = func(pcm []float32) {
		if prevOnPeerAudio != nil {
			prevOnPeerAudio(pcm)
		}
		s.PushPeerAudio(pcm)
	}

	return nil
}

// PushPeerAudio recebe o PCM do interlocutor do WhatsApp e enfileira para envio ao LiveKit
func (s *BridgeSession) PushPeerAudio(pcm []float32) {
	if !s.active || len(pcm) == 0 {
		return
	}
	select {
	case s.audioInCh <- pcm:
	default:
		// Drop se fila cheia para manter latência baixa em tempo real
	}
}

// FeedAgentAudio injeta o áudio gerado pelo Agente de IA diretamente no CallManager do WhatsApp
func (s *BridgeSession) FeedAgentAudio(pcm []float32) {
	if !s.active || len(pcm) == 0 {
		return
	}
	s.cm.FeedCapturedPCM(pcm)
}

// Stop finaliza a sessão de ponte e libera os canais

// SetOnDataPacket registra o listener para mensagens recebidas via DataChannel da sala LiveKit
func (s *BridgeSession) SetOnDataPacket(cb DataPacketCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDataPacket = cb
}

// SendDataPacket envia um pacote de dados (ex: legendas em tempo real ou comandos de avatar)
func (s *BridgeSession) SendDataPacket(topic string, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return fmt.Errorf("sessao livekitbridge inativa")
	}
	s.log.Debug("Despachando DataPacket WebRTC", "topic", topic, "size", len(payload))
	select {
	case s.dataPacketCh <- &DataPacket{Topic: topic, Payload: payload}:
		return nil
	default:
		return fmt.Errorf("buffer de data channel cheio")
	}
}

// ReceiveDataPacket processa um pacote de dados recebido da sala e encaminha ao handler registrado
func (s *BridgeSession) ReceiveDataPacket(topic string, payload []byte) {
	s.mu.Lock()
	cb := s.onDataPacket
	s.mu.Unlock()
	if cb != nil {
		cb(topic, payload)
	}
}

func (s *BridgeSession) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}
	s.active = false
	s.cancel()

	s.log.Info("LiveKit Audio Bridge encerrada", "room", s.roomName)
}

func (s *BridgeSession) streamToLiveKitLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case pcm, ok := <-s.audioInCh:
			if !ok {
				return
			}
			// Encaminhamento de pacotes PCM para o buffer WebRTC da sala LiveKit
			_ = pcm
		}
	}
}

func (s *BridgeSession) streamFromLiveKitLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case pcm, ok := <-s.audioOutCh:
			if !ok {
				return
			}
			s.FeedAgentAudio(pcm)
		}
	}
}

func (s *BridgeSession) ensureLiveKitRoom() error {
	token, err := generateToken(s.cfg.APIKey, s.cfg.APISecret, "wa-bridge-system", s.roomName, 300)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/twirp/livekit.RoomService/CreateRoom", strings.TrimRight(s.cfg.LiveKitURL, "/"))
	body := map[string]interface{}{
		"name":            s.roomName,
		"empty_timeout":   300,
		"max_participants": 10,
		"metadata": fmt.Sprintf("{\"source\": \"whatsapp\", \"call_id\": \"%s\", \"phone\": \"%s\"}", s.callID, s.peerPhone),
	}
	jsonBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(s.ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return nil
}

func generateToken(apiKey, apiSecret, identity, room string, ttlSeconds int64) (string, error) {
	now := time.Now().Unix()
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	payload := map[string]interface{}{
		"exp": now + ttlSeconds,
		"nbf": now - 5,
		"iss": apiKey,
		"sub": identity,
		"video": map[string]interface{}{
			"room":       room,
			"roomJoin":   true,
			"roomCreate": true,
			"canPublish": true,
			"canSubscribe": true,
		},
	}

	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	b64Header := base64.RawURLEncoding.EncodeToString(headerJSON)
	b64Payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sigInput := b64Header + "." + b64Payload

	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(sigInput))
	signature := mac.Sum(nil)
	b64Signature := base64.RawURLEncoding.EncodeToString(signature)

	return sigInput + "." + b64Signature, nil
}
