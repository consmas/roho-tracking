package device

import (
	"errors"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

type ConnSession struct {
	DeviceUID  string
	Conn       net.Conn
	SendQueue  chan []byte
	CreatedAt  time.Time
	LastSeenAt time.Time
	mu         sync.RWMutex
	closed     bool
	logger     *zap.Logger
}

func NewConnSession(deviceUID string, conn net.Conn, sendBuffer int, logger *zap.Logger) *ConnSession {
	now := time.Now().UTC()
	return &ConnSession{
		DeviceUID:  deviceUID,
		Conn:       conn,
		SendQueue:  make(chan []byte, sendBuffer),
		CreatedAt:  now,
		LastSeenAt: now,
		logger:     logger,
	}
}

func (s *ConnSession) Touch() {
	s.mu.Lock()
	s.LastSeenAt = time.Now().UTC()
	s.mu.Unlock()
}

func (s *ConnSession) QueueCommand(frame []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return errors.New("session closed")
	}
	select {
	case s.SendQueue <- frame:
		return nil
	default:
		return errors.New("send queue full")
	}
}

func (s *ConnSession) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.SendQueue)
	s.mu.Unlock()
	_ = s.Conn.Close()
}

func (s *ConnSession) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}
