package publish

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type Event struct {
	EventID   string         `json:"event_id"`
	DeviceUID string         `json:"device_uid"`
	EventType string         `json:"event_type"`
	TS        time.Time      `json:"ts"`
	Data      map[string]any `json:"data"`
	Raw       string         `json:"raw,omitempty"`
}

type Publisher struct {
	redis  *redis.Client
	stream string
}

func NewPublisher(redisClient *redis.Client, stream string) *Publisher {
	return &Publisher{redis: redisClient, stream: stream}
}

func (p *Publisher) PublishEvent(ctx context.Context, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = p.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: p.stream,
		ID:     "*",
		Values: map[string]any{
			"event_id":   event.EventID,
			"device_uid": event.DeviceUID,
			"event_type": event.EventType,
			"ts":         event.TS.Format(time.RFC3339),
			"payload":    string(payload),
		},
		Approx: true,
		MaxLen: 5_000_000,
	}).Result()
	return err
}

func PublishCommandResult(ctx context.Context, redisClient *redis.Client, stream string, commandID string, deviceUID string, status string, details map[string]any) error {
	payload, err := json.Marshal(map[string]any{
		"command_id": commandID,
		"device_uid": deviceUID,
		"status":     status,
		"details":    details,
		"ts":         time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"payload": string(payload)},
		Approx: true,
		MaxLen: 1_000_000,
	}).Result()
	return err
}
