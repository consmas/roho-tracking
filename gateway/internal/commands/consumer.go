package commands

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/roho/telematics/gateway/internal/device"
	"github.com/roho/telematics/gateway/internal/observability"
	"github.com/roho/telematics/gateway/internal/protocol"
	"github.com/roho/telematics/gateway/internal/publish"
	"go.uber.org/zap"
)

type Consumer struct {
	redis         *redis.Client
	stream        string
	group         string
	consumer      string
	commandResult string
	sessions      *device.ConnectionManager
	registry      *device.Registry
	parser        protocol.Parser
	metrics       *observability.Metrics
	logger        *zap.Logger
}

type CommandMessage struct {
	CommandID      string         `json:"command_id"`
	DeviceUID      string         `json:"device_uid"`
	CommandType    string         `json:"command_type"`
	Payload        map[string]any `json:"payload"`
	TS             string         `json:"ts"`
	TargetInstance string         `json:"target_instance,omitempty"`
	Attempt        int            `json:"attempt,omitempty"`
}

func NewConsumer(redisClient *redis.Client, stream, group, consumer, commandResults string, sessions *device.ConnectionManager, registry *device.Registry, parser protocol.Parser, metrics *observability.Metrics, logger *zap.Logger) *Consumer {
	return &Consumer{
		redis:         redisClient,
		stream:        stream,
		group:         group,
		consumer:      consumer,
		commandResult: commandResults,
		sessions:      sessions,
		registry:      registry,
		parser:        parser,
		metrics:       metrics,
		logger:        logger,
	}
}

func (c *Consumer) ensureGroup(ctx context.Context) error {
	err := c.redis.XGroupCreateMkStream(ctx, c.stream, c.group, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (c *Consumer) Run(ctx context.Context) error {
	if err := c.ensureGroup(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		streams, err := c.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.group,
			Consumer: c.consumer,
			Streams:  []string{c.stream, ">"},
			Count:    50,
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) {
				continue
			}
			c.logger.Error("xreadgroup failed", zap.Error(err))
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for _, str := range streams {
			for _, msg := range str.Messages {
				c.handleMessage(ctx, msg)
			}
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, msg redis.XMessage) {
	payloadRaw, ok := msg.Values["payload"].(string)
	if !ok {
		c.metrics.CommandsHandled.WithLabelValues("invalid").Inc()
		_ = c.redis.XAck(ctx, c.stream, c.group, msg.ID).Err()
		return
	}

	var command CommandMessage
	if err := json.Unmarshal([]byte(payloadRaw), &command); err != nil {
		c.metrics.CommandsHandled.WithLabelValues("invalid").Inc()
		_ = c.redis.XAck(ctx, c.stream, c.group, msg.ID).Err()
		return
	}

	if command.TargetInstance != "" && command.TargetInstance != c.consumer {
		owner, err := c.registry.GetOwner(ctx, command.DeviceUID)
		if err == nil && owner == command.TargetInstance {
			c.metrics.CommandsHandled.WithLabelValues("not_owner").Inc()
			return
		}
	}

	session, ok := c.sessions.Get(command.DeviceUID)
	if !ok {
		c.metrics.CommandsHandled.WithLabelValues("offline").Inc()
		_ = publish.PublishCommandResult(ctx, c.redis, c.commandResult, command.CommandID, command.DeviceUID, "offline", map[string]any{"reason": "device not connected on this gateway"})
		_ = c.redis.XAck(ctx, c.stream, c.group, msg.ID).Err()
		return
	}

	payloadForEncode := clonePayload(command.Payload)
	payloadForEncode["command_id"] = command.CommandID
	payloadForEncode["device_uid"] = command.DeviceUID
	payloadForEncode["command_type"] = command.CommandType
	encoded, err := c.parser.EncodeCommand(command.CommandType, payloadForEncode)
	if err != nil {
		c.metrics.CommandsHandled.WithLabelValues("encode_error").Inc()
		_ = publish.PublishCommandResult(ctx, c.redis, c.commandResult, command.CommandID, command.DeviceUID, "failed", map[string]any{"reason": err.Error()})
		_ = c.redis.XAck(ctx, c.stream, c.group, msg.ID).Err()
		return
	}
	if err := session.QueueCommand(encoded); err != nil {
		c.metrics.CommandsHandled.WithLabelValues("queue_full").Inc()
		_ = publish.PublishCommandResult(ctx, c.redis, c.commandResult, command.CommandID, command.DeviceUID, "failed", map[string]any{"reason": err.Error()})
		_ = c.redis.XAck(ctx, c.stream, c.group, msg.ID).Err()
		return
	}

	c.metrics.CommandsHandled.WithLabelValues("delivered").Inc()
	_ = publish.PublishCommandResult(ctx, c.redis, c.commandResult, command.CommandID, command.DeviceUID, "delivered", map[string]any{"gateway": c.consumer})
	_ = c.redis.XAck(ctx, c.stream, c.group, msg.ID).Err()
}

func clonePayload(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
