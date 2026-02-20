package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	GatewayID         string
	ListenAddr        string
	MetricsAddr       string
	Protocol          string
	CaptureEnabled    bool
	CaptureDir        string
	CaptureFrames     int
	TLSEnabled        bool
	TLSCertFile       string
	TLSKeyFile        string
	DeviceLookupURL   string
	InternalAPIToken  string
	AuthCacheTTL      time.Duration
	AuthNegativeTTL   time.Duration
	AuthLookupTimeout time.Duration
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	EventsStream      string
	CommandsStream    string
	CommandResults    string
	GatewayGroup      string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	SendBufferSize    int
	MaxFrameBytes     int
	SessionTTL        time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		GatewayID:         getEnv("GATEWAY_INSTANCE_ID", "gateway-1"),
		ListenAddr:        getEnv("GATEWAY_LISTEN_ADDR", ":9000"),
		MetricsAddr:       getEnv("GATEWAY_METRICS_ADDR", ":9100"),
		Protocol:          getEnv("GATEWAY_PROTOCOL", "binary_json"),
		CaptureEnabled:    getEnvBool("GATEWAY_CAPTURE_ENABLED", false),
		CaptureDir:        getEnv("GATEWAY_CAPTURE_DIR", "/tmp/gateway-captures"),
		CaptureFrames:     getEnvInt("GATEWAY_CAPTURE_FRAMES", 20),
		TLSEnabled:        getEnvBool("GATEWAY_TLS_ENABLED", false),
		TLSCertFile:       getEnv("GATEWAY_TLS_CERT_FILE", ""),
		TLSKeyFile:        getEnv("GATEWAY_TLS_KEY_FILE", ""),
		DeviceLookupURL:   getEnv("GATEWAY_DEVICE_LOOKUP_URL", "http://rails-web:3000/internal/devices/lookup"),
		InternalAPIToken:  getEnv("GATEWAY_INTERNAL_API_TOKEN", "internal-dev-token"),
		AuthCacheTTL:      getEnvDuration("GATEWAY_AUTH_CACHE_TTL", 5*time.Minute),
		AuthNegativeTTL:   getEnvDuration("GATEWAY_AUTH_NEGATIVE_TTL", 30*time.Second),
		AuthLookupTimeout: getEnvDuration("GATEWAY_AUTH_LOOKUP_TIMEOUT", 2*time.Second),
		RedisAddr:         getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		EventsStream:      getEnv("REDIS_EVENTS_STREAM", "telemetry.events"),
		CommandsStream:    getEnv("REDIS_COMMANDS_STREAM", "device.commands"),
		CommandResults:    getEnv("REDIS_COMMAND_RESULTS_STREAM", "device.command_results"),
		GatewayGroup:      getEnv("REDIS_GATEWAY_GROUP", "gateway-consumers"),
		ReadTimeout:       getEnvDuration("GATEWAY_READ_TIMEOUT", 35*time.Second),
		WriteTimeout:      getEnvDuration("GATEWAY_WRITE_TIMEOUT", 10*time.Second),
		SendBufferSize:    getEnvInt("GATEWAY_SEND_BUFFER", 256),
		MaxFrameBytes:     getEnvInt("GATEWAY_MAX_FRAME_BYTES", 8192),
		SessionTTL:        getEnvDuration("GATEWAY_SESSION_TTL", 70*time.Second),
	}

	cfg.RedisDB = getEnvInt("REDIS_DB", 0)
	if cfg.SendBufferSize < 1 {
		return Config{}, fmt.Errorf("GATEWAY_SEND_BUFFER must be >= 1")
	}
	if cfg.MaxFrameBytes < 256 {
		return Config{}, fmt.Errorf("GATEWAY_MAX_FRAME_BYTES must be >= 256")
	}
	if cfg.CaptureFrames < 1 {
		return Config{}, fmt.Errorf("GATEWAY_CAPTURE_FRAMES must be >= 1")
	}
	if cfg.Protocol != "binary_json" && cfg.Protocol != "joinlgo_text" {
		return Config{}, fmt.Errorf("GATEWAY_PROTOCOL must be binary_json or joinlgo_text")
	}
	return cfg, nil
}

func getEnv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(k string, fallback int) int {
	if v := os.Getenv(k); v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(k string, fallback time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return fallback
}

func getEnvBool(k string, fallback bool) bool {
	if v := os.Getenv(k); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return fallback
}
