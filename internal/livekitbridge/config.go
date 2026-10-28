package livekitbridge

import (
	"os"
	"strings"
)

type Config struct {
	LiveKitURL     string
	APIKey         string
	APISecret      string
	RoomPrefix     string
	Enabled        bool
	SampleRate     int
	Channels       int
	AgentIdentity  string
}

func LoadConfigFromEnv() Config {
	url := os.Getenv("LIVEKIT_URL")
	if url == "" {
		url = os.Getenv("LIVEKIT_HTTP_URL")
	}
	if url == "" {
		url = "http://127.0.0.1:7880"
	}

	apiKey := os.Getenv("LIVEKIT_API_KEY")
	if apiKey == "" {
		apiKey = "APIdisparoseguro"
	}

	apiSecret := os.Getenv("LIVEKIT_API_SECRET")
	if apiSecret == "" {
		apiSecret = "secretdisparoseguro2026supersecretkey"
	}

	roomPrefix := os.Getenv("LIVEKIT_ROOM_PREFIX")
	if roomPrefix == "" {
		roomPrefix = "wa-call-"
	}

	enabled := strings.ToLower(os.Getenv("LIVEKIT_BRIDGE_ENABLED")) != "false"

	return Config{
		LiveKitURL:    url,
		APIKey:        apiKey,
		APISecret:     apiSecret,
		RoomPrefix:    roomPrefix,
		Enabled:       enabled,
		SampleRate:    48000,
		Channels:      1,
		AgentIdentity: "voice-agent-disparo-seguro",
	}
}
