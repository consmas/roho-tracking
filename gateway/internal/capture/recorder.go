package capture

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type Manager struct {
	enabled bool
	dir     string
	max     int
	logger  *zap.Logger
}

type Recorder struct {
	enabled    bool
	dir        string
	max        int
	logger     *zap.Logger
	connID     string
	remoteIP   string
	deviceUID  string
	path       string
	file       *os.File
	frameCount int
	mu         sync.Mutex
}

type recordLine struct {
	TS        string `json:"ts"`
	ConnID    string `json:"conn_id"`
	RemoteIP  string `json:"remote_ip"`
	DeviceUID string `json:"device_uid,omitempty"`
	FrameIdx  int    `json:"frame_index"`
	Length    int    `json:"length"`
	Hex       string `json:"hex"`
}

func NewManager(enabled bool, dir string, max int, logger *zap.Logger) *Manager {
	if dir == "" {
		dir = "./captures"
	}
	if max <= 0 {
		max = 20
	}
	return &Manager{enabled: enabled, dir: dir, max: max, logger: logger}
}

func (m *Manager) NewRecorder(connID, remoteAddr string) *Recorder {
	r := &Recorder{
		enabled: m.enabled,
		dir:     m.dir,
		max:     m.max,
		logger:  m.logger,
		connID:  sanitize(connID),
	}
	if !m.enabled {
		return r
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		m.logger.Warn("capture mkdir failed", zap.Error(err), zap.String("dir", m.dir))
		r.enabled = false
		return r
	}
	r.remoteIP = parseRemoteIP(remoteAddr)
	r.path = r.buildPath()
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		m.logger.Warn("capture open failed", zap.Error(err), zap.String("path", r.path))
		r.enabled = false
		return r
	}
	r.file = f
	return r
}

func (r *Recorder) SetDeviceUID(deviceUID string) {
	if !r.enabled || deviceUID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	uid := sanitize(deviceUID)
	if uid == "" || uid == r.deviceUID {
		return
	}
	r.deviceUID = uid

	if r.file != nil {
		_ = r.file.Close()
	}
	r.path = r.buildPath()
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		r.logger.Warn("capture reopen failed", zap.Error(err), zap.String("path", r.path))
		r.enabled = false
		return
	}
	r.file = f
}

func (r *Recorder) Record(frame []byte) {
	if !r.enabled {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frameCount >= r.max || r.file == nil {
		return
	}
	r.frameCount++
	line := recordLine{
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
		ConnID:    r.connID,
		RemoteIP:  r.remoteIP,
		DeviceUID: r.deviceUID,
		FrameIdx:  r.frameCount,
		Length:    len(frame),
		Hex:       hex.EncodeToString(frame),
	}
	b, err := json.Marshal(line)
	if err != nil {
		r.logger.Warn("capture marshal failed", zap.Error(err), zap.String("conn_id", r.connID))
		return
	}
	_, _ = r.file.Write(append(b, '\n'))
}

func (r *Recorder) Close() {
	if !r.enabled {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		_ = r.file.Close()
		r.file = nil
	}
}

func (r *Recorder) buildPath() string {
	name := fmt.Sprintf("conn_%s_ip_%s.log", r.connID, sanitize(r.remoteIP))
	if r.deviceUID != "" {
		name = fmt.Sprintf("device_%s_conn_%s.log", r.deviceUID, r.connID)
	}
	return filepath.Join(r.dir, name)
}

func parseRemoteIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}

func sanitize(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return "unknown"
	}
	return sanitizeRe.ReplaceAllString(in, "_")
}
