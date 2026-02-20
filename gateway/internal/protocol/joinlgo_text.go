package protocol

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type JoinLGOTextParser struct{}

func NewJoinLGOTextParser() *JoinLGOTextParser {
	return &JoinLGOTextParser{}
}

func (p *JoinLGOTextParser) Parse(raw []byte) (*Frame, error) {
	if len(raw) == 0 {
		return nil, errors.New("empty frame")
	}
	text := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(text, "$$") || !strings.HasSuffix(text, "#") {
		return nil, errors.New("invalid joinlgo frame markers")
	}

	body := strings.TrimPrefix(text, "$$")
	body = strings.TrimSuffix(body, "#")
	parts := strings.Split(body, ",")
	if len(parts) < 4 {
		return nil, fmt.Errorf("joinlgo frame has too few fields: %d", len(parts))
	}

	deviceUID := strings.TrimSpace(parts[3])
	if deviceUID == "" {
		return nil, errors.New("missing device uid")
	}

	msgType := field(parts, 2)
	ts := parseJoinLGOTime(field(parts, 5))

	data := map[string]any{
		"protocol":     "joinlgo_text",
		"message_type": msgType,
		"raw_fields":   parts,
		"imei":         extractIMEI(parts),
	}
	if lat, lon, ok := parseLatLng(parts); ok {
		data["lat"] = lat
		data["lng"] = lon
	}

	eventType := EventTypeHeartbeat
	if _, ok := data["lat"]; ok {
		eventType = EventTypeLocation
	}

	return &Frame{
		Type:      eventType,
		Timestamp: ts,
		Data:      data,
		Raw:       raw,
		Auth:      &Auth{DeviceUID: deviceUID},
	}, nil
}

func (p *JoinLGOTextParser) EncodeCommand(commandType string, payload map[string]any) ([]byte, error) {
	encodedPayload := ""
	if len(payload) > 0 {
		raw := fmt.Sprintf("%v", payload)
		encodedPayload = base64.StdEncoding.EncodeToString([]byte(raw))
	}
	frame := fmt.Sprintf("$$CMD,%s,%s,#", commandType, encodedPayload)
	return []byte(frame), nil
}

func parseJoinLGOTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().UTC()
	}
	t, err := time.Parse("060102 150405", s)
	if err != nil {
		return time.Now().UTC()
	}
	return t.UTC()
}

func parseLatLng(parts []string) (float64, float64, bool) {
	latRaw, err1 := strconv.ParseFloat(field(parts, 9), 64)
	lonRaw, err2 := strconv.ParseFloat(field(parts, 12), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if latRaw == 0 && lonRaw == 0 {
		return 0, 0, false
	}
	lat := latRaw / 10_000_000
	lon := lonRaw / 10_000_000
	return lat, lon, true
}

func extractIMEI(parts []string) string {
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if len(v) == 15 && isDigits(v) {
			return v
		}
	}
	return ""
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func field(parts []string, idx int) string {
	if idx < 0 || idx >= len(parts) {
		return ""
	}
	return strings.TrimSpace(parts[idx])
}
