package device

import (
	"context"
	"sync"

	"github.com/roho/telematics/gateway/internal/observability"
	"go.uber.org/zap"
)

type ConnectionManager struct {
	gatewayID string
	registry  *Registry
	metrics   *observability.Metrics
	logger    *zap.Logger

	mu       sync.RWMutex
	sessions map[string]*ConnSession
}

func NewConnectionManager(gatewayID string, registry *Registry, metrics *observability.Metrics, logger *zap.Logger) *ConnectionManager {
	return &ConnectionManager{
		gatewayID: gatewayID,
		registry:  registry,
		metrics:   metrics,
		logger:    logger,
		sessions:  make(map[string]*ConnSession),
	}
}

func (m *ConnectionManager) Add(ctx context.Context, session *ConnSession) error {
	m.mu.Lock()
	if old, ok := m.sessions[session.DeviceUID]; ok {
		old.Close()
	}
	m.sessions[session.DeviceUID] = session
	m.metrics.ActiveConnections.Set(float64(len(m.sessions)))
	m.mu.Unlock()

	if err := m.registry.SetOwner(ctx, session.DeviceUID, m.gatewayID); err != nil {
		return err
	}
	return nil
}

func (m *ConnectionManager) Touch(ctx context.Context, deviceUID string) {
	m.mu.RLock()
	s, ok := m.sessions[deviceUID]
	m.mu.RUnlock()
	if ok {
		s.Touch()
		_ = m.registry.RefreshOwner(ctx, deviceUID, m.gatewayID)
	}
}

func (m *ConnectionManager) Get(deviceUID string) (*ConnSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[deviceUID]
	return s, ok
}

func (m *ConnectionManager) Remove(ctx context.Context, deviceUID string) {
	m.mu.Lock()
	s, ok := m.sessions[deviceUID]
	if ok {
		delete(m.sessions, deviceUID)
	}
	m.metrics.ActiveConnections.Set(float64(len(m.sessions)))
	m.mu.Unlock()

	if ok {
		s.Close()
		_ = m.registry.RemoveOwner(ctx, deviceUID, m.gatewayID)
	}
}

func (m *ConnectionManager) CloseAll(ctx context.Context) {
	m.mu.Lock()
	sessions := make([]*ConnSession, 0, len(m.sessions))
	for uid, s := range m.sessions {
		sessions = append(sessions, s)
		delete(m.sessions, uid)
	}
	m.metrics.ActiveConnections.Set(0)
	m.mu.Unlock()

	for _, s := range sessions {
		s.Close()
		_ = m.registry.RemoveOwner(ctx, s.DeviceUID, m.gatewayID)
	}
}
