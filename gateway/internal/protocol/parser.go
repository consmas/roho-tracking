package protocol

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type EventType string

const (
	EventTypeLocation  EventType = "location_update"
	EventTypeAlarm     EventType = "alarm"
	EventTypeHeartbeat EventType = "heartbeat"
)

type Auth struct {
	DeviceUID string `json:"device_uid"`
	Token     string `json:"token"`
}

type Frame struct {
	Type      EventType      `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
	Raw       []byte
	Auth      *Auth `json:"auth,omitempty"`
}

type Parser interface {
	Parse(raw []byte) (*Frame, error)
	EncodeCommand(commandType string, payload map[string]any) ([]byte, error)
}

type BinaryJSONParser struct{}

func NewBinaryJSONParser() *BinaryJSONParser {
	return &BinaryJSONParser{}
}

func (p *BinaryJSONParser) Parse(raw []byte) (*Frame, error) {
	if len(raw) < 2 {
		return nil, errors.New("frame too short")
	}
	length := int(binary.BigEndian.Uint16(raw[:2]))
	if length <= 0 || len(raw[2:]) != length {
		return nil, fmt.Errorf("invalid frame length: expected %d got %d", length, len(raw[2:]))
	}

	var payload struct {
		Type      EventType      `json:"type"`
		Timestamp string         `json:"timestamp"`
		Data      map[string]any `json:"data"`
		Auth      *Auth          `json:"auth,omitempty"`
	}
	if err := json.Unmarshal(raw[2:], &payload); err != nil {
		return nil, err
	}
	ts := time.Now().UTC()
	if payload.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339, payload.Timestamp)
		if err == nil {
			ts = parsed.UTC()
		}
	}
	if payload.Data == nil {
		payload.Data = map[string]any{}
	}
	return &Frame{
		Type:      payload.Type,
		Timestamp: ts,
		Data:      payload.Data,
		Raw:       raw,
		Auth:      payload.Auth,
	}, nil
}

func (p *BinaryJSONParser) EncodeCommand(commandType string, payload map[string]any) ([]byte, error) {
	message := map[string]any{
		"type":    commandType,
		"payload": payload,
		"ts":      time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	if len(body) > 65535 {
		return nil, errors.New("command frame too large")
	}
	out := make([]byte, 2+len(body))
	binary.BigEndian.PutUint16(out[:2], uint16(len(body)))
	copy(out[2:], body)
	return out, nil
}

func EncodeRawB64(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(raw)
}
