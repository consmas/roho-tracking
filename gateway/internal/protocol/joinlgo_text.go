package protocol

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type JoinLGOTextParser struct {
	seq             atomic.Uint64
	defaultTemplate string
	startTemplate   string
	stopTemplate    string
}

func NewJoinLGOTextParser() *JoinLGOTextParser {
	return &JoinLGOTextParser{
		defaultTemplate: envOr("GATEWAY_JOINLGO_DEFAULT_TEMPLATE", "$$CMD,{{command_type}},{{device_uid}},{{command_id}},{{ts_unix}},{{payload_b64}}#"),
		startTemplate:   envOr("GATEWAY_JOINLGO_START_TEMPLATE", "$$CMD,START_LIVE,{{device_uid}},{{channel}},{{ingest_host}},{{ingest_port}},{{ingest_path}},{{session_id}},{{command_id}},{{ts_unix}}#"),
		stopTemplate:    envOr("GATEWAY_JOINLGO_STOP_TEMPLATE", "$$CMD,STOP_LIVE,{{device_uid}},{{channel}},{{session_id}},{{command_id}},{{ts_unix}}#"),
	}
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
	template := p.templateFor(commandType, payload)
	vars := p.commandVars(commandType, payload)
	frame := renderCommandTemplate(template, vars)
	if !strings.HasPrefix(frame, "$$") {
		frame = "$$" + frame
	}
	if !strings.HasSuffix(frame, "#") {
		frame += "#"
	}
	return []byte(frame), nil
}

func (p *JoinLGOTextParser) templateFor(commandType string, payload map[string]any) string {
	action := strings.ToLower(asString(payload["action"]))
	switch action {
	case "start_live":
		return p.startTemplate
	case "stop_live":
		return p.stopTemplate
	default:
		_ = commandType
		return p.defaultTemplate
	}
}

func (p *JoinLGOTextParser) commandVars(commandType string, payload map[string]any) map[string]string {
	now := time.Now().UTC()
	seq := p.seq.Add(1)
	commandID := asString(payload["command_id"])
	deviceUID := asString(payload["device_uid"])

	ingestURL := asString(payload["ingest_rtsp_url"])
	if ingestURL == "" {
		if ingestMap, ok := payload["ingest"].(map[string]any); ok {
			ingestURL = asString(ingestMap["rtsp_url"])
		}
	}

	ingestHost, ingestPort, ingestPath := splitIngestURL(ingestURL)
	if ingestMap, ok := payload["ingest"].(map[string]any); ok {
		if v := asString(ingestMap["host"]); v != "" {
			ingestHost = v
		}
		if v := asString(ingestMap["port"]); v != "" {
			ingestPort = v
		}
		if v := asString(ingestMap["path"]); v != "" {
			ingestPath = v
		}
	}

	rawPayload := fmt.Sprintf("%v", payload)
	payloadB64 := base64.StdEncoding.EncodeToString([]byte(rawPayload))

	return map[string]string{
		"command_type":    commandType,
		"action":          asString(payload["action"]),
		"command_id":      commandID,
		"device_uid":      deviceUID,
		"channel":         asString(payload["channel"]),
		"session_id":      asString(payload["session_id"]),
		"stream_name":     asString(payload["stream_name"]),
		"transport":       asString(payload["transport"]),
		"ingest_rtsp_url": ingestURL,
		"ingest_host":     ingestHost,
		"ingest_port":     ingestPort,
		"ingest_path":     ingestPath,
		"payload_b64":     payloadB64,
		"seq":             strconv.FormatUint(seq, 10),
		"ts_unix":         strconv.FormatInt(now.Unix(), 10),
		"ts_iso":          now.Format(time.RFC3339),
		"ts_yyMMddHHmmss": now.Format("060102150405"),
	}
}

func splitIngestURL(raw string) (host, port, path string) {
	if raw == "" {
		return "", "", ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", ""
	}
	host = u.Hostname()
	port = u.Port()
	if port == "" {
		port = "554"
	}
	path = strings.TrimPrefix(u.Path, "/")
	return host, port, path
}

func renderCommandTemplate(template string, vars map[string]string) string {
	out := template
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", sanitizeField(v))
	}
	return out
}

func sanitizeField(v string) string {
	v = strings.ReplaceAll(v, ",", "_")
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "")
	return v
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}

func envOr(k, fallback string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return fallback
	}
	return v
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
