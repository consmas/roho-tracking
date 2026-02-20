package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/roho/telematics/gateway/internal/auth"
	"github.com/roho/telematics/gateway/internal/capture"
	"github.com/roho/telematics/gateway/internal/config"
	"github.com/roho/telematics/gateway/internal/device"
	"github.com/roho/telematics/gateway/internal/observability"
	"github.com/roho/telematics/gateway/internal/protocol"
	"github.com/roho/telematics/gateway/internal/publish"
	"go.uber.org/zap"
)

type TCPServer struct {
	cfg      config.Config
	auth     *auth.Service
	capture  *capture.Manager
	parser   protocol.Parser
	sessions *device.ConnectionManager
	pub      *publish.Publisher
	metrics  *observability.Metrics
	logger   *zap.Logger
}

func NewTCPServer(cfg config.Config, authService *auth.Service, captureManager *capture.Manager, parser protocol.Parser, sessions *device.ConnectionManager, pub *publish.Publisher, metrics *observability.Metrics, logger *zap.Logger) *TCPServer {
	return &TCPServer{cfg: cfg, auth: authService, capture: captureManager, parser: parser, sessions: sessions, pub: pub, metrics: metrics, logger: logger}
}

func (s *TCPServer) ListenAndServe(ctx context.Context) error {
	var (
		ln  net.Listener
		err error
	)
	if s.cfg.TLSEnabled {
		cert, certErr := tls.LoadX509KeyPair(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
		if certErr != nil {
			return certErr
		}
		ln, err = tls.Listen("tcp", s.cfg.ListenAddr, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	} else {
		ln, err = net.Listen("tcp", s.cfg.ListenAddr)
	}
	if err != nil {
		return err
	}
	defer ln.Close()
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	s.logger.Info("tcp gateway listening", zap.String("addr", s.cfg.ListenAddr))
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			s.logger.Warn("accept failed", zap.Error(err))
			continue
		}
		go s.serveConn(ctx, conn)
	}
}

func (s *TCPServer) serveConn(ctx context.Context, conn net.Conn) {
	connID := uuid.NewString()
	recorder := s.capture.NewRecorder(connID, conn.RemoteAddr().String())
	defer recorder.Close()

	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in serveConn", zap.Any("panic", r))
		}
	}()

	reader := bufio.NewReader(conn)

	frameBytes, err := s.readFrame(reader, conn)
	if err != nil {
		_ = conn.Close()
		return
	}
	recorder.Record(frameBytes)
	frame, err := s.parser.Parse(frameBytes)
	if err != nil {
		_ = conn.Close()
		return
	}
	if frame.Auth == nil || frame.Auth.DeviceUID == "" {
		_ = conn.Close()
		return
	}
	recorder.SetDeviceUID(frame.Auth.DeviceUID)
	allowed, reason, authErr := s.auth.ValidateDevice(ctx, frame.Auth.DeviceUID)
	if authErr != nil {
		s.logger.Warn("device auth lookup failed", zap.String("device_uid", frame.Auth.DeviceUID), zap.Error(authErr))
		_ = conn.Close()
		return
	}
	if !allowed {
		s.logger.Info("device rejected by allowlist", zap.String("device_uid", frame.Auth.DeviceUID), zap.String("reason", reason))
		_ = conn.Close()
		return
	}

	session := device.NewConnSession(frame.Auth.DeviceUID, conn, s.cfg.SendBufferSize, s.logger)
	if err := s.sessions.Add(ctx, session); err != nil {
		s.logger.Warn("failed to register session", zap.String("device_uid", frame.Auth.DeviceUID), zap.Error(err))
		_ = conn.Close()
		return
	}
	defer s.sessions.Remove(context.Background(), session.DeviceUID)

	go s.writer(session)
	s.processFrame(ctx, session.DeviceUID, frame)

	for {
		if ctx.Err() != nil {
			return
		}
		frameBytes, err = s.readFrame(reader, conn)
		if err != nil {
			if err != io.EOF {
				s.logger.Debug("read failed", zap.String("device_uid", session.DeviceUID), zap.Error(err))
			}
			return
		}
		recorder.Record(frameBytes)
		frame, err = s.parser.Parse(frameBytes)
		if err != nil {
			s.logger.Debug("parse failed", zap.String("device_uid", session.DeviceUID), zap.Error(err))
			continue
		}
		s.processFrame(ctx, session.DeviceUID, frame)
	}
}

func (s *TCPServer) readFrame(reader *bufio.Reader, conn net.Conn) ([]byte, error) {
	if err := conn.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout)); err != nil {
		return nil, err
	}
	if s.cfg.Protocol == "joinlgo_text" {
		frame, err := reader.ReadBytes('#')
		if err != nil {
			return nil, err
		}
		if len(frame) > s.cfg.MaxFrameBytes {
			return nil, fmt.Errorf("invalid frame length: %d", len(frame))
		}
		return frame, nil
	}

	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	length := int(header[0])<<8 + int(header[1])
	if length <= 0 || length > s.cfg.MaxFrameBytes {
		return nil, fmt.Errorf("invalid frame length: %d", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	out := append(header, body...)
	return out, nil
}

func (s *TCPServer) writer(session *device.ConnSession) {
	for frame := range session.SendQueue {
		if session.IsClosed() {
			return
		}
		_ = session.Conn.SetWriteDeadline(time.Now().Add(s.cfg.WriteTimeout))
		if _, err := session.Conn.Write(frame); err != nil {
			s.logger.Debug("write command failed", zap.String("device_uid", session.DeviceUID), zap.Error(err))
			return
		}
	}
}

func (s *TCPServer) processFrame(ctx context.Context, deviceUID string, frame *protocol.Frame) {
	s.sessions.Touch(ctx, deviceUID)

	eventType := string(frame.Type)
	if eventType == "" {
		eventType = string(protocol.EventTypeHeartbeat)
	}
	s.metrics.FramesReceived.WithLabelValues(eventType).Inc()

	e := publish.Event{
		EventID:   uuid.NewString(),
		DeviceUID: deviceUID,
		EventType: eventType,
		TS:        frame.Timestamp,
		Data:      frame.Data,
		Raw:       protocol.EncodeRawB64(frame.Raw),
	}
	if err := s.pub.PublishEvent(ctx, e); err != nil {
		s.metrics.FramesPublished.WithLabelValues(eventType, "error").Inc()
		s.logger.Warn("failed to publish event", zap.String("device_uid", deviceUID), zap.Error(err))
		return
	}
	s.metrics.FramesPublished.WithLabelValues(eventType, "ok").Inc()

	if eventType == string(protocol.EventTypeHeartbeat) && s.cfg.Protocol == "binary_json" {
		ack, _ := json.Marshal(map[string]any{"type": "heartbeat_ack", "ts": time.Now().UTC().Format(time.RFC3339)})
		frame := make([]byte, 2+len(ack))
		frame[0] = byte(len(ack) >> 8)
		frame[1] = byte(len(ack))
		copy(frame[2:], ack)
		if session, ok := s.sessions.Get(deviceUID); ok {
			if err := session.QueueCommand(frame); err != nil {
				s.metrics.SendQueueDrops.Inc()
			}
		}
	}
}
