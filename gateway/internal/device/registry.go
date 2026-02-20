package device

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Registry struct {
	redis *redis.Client
	ttl   time.Duration
}

func NewRegistry(redisClient *redis.Client, ttl time.Duration) *Registry {
	return &Registry{redis: redisClient, ttl: ttl}
}

func (r *Registry) key(deviceUID string) string {
	return fmt.Sprintf("device_session:%s", deviceUID)
}

func (r *Registry) SetOwner(ctx context.Context, deviceUID, gatewayID string) error {
	return r.redis.Set(ctx, r.key(deviceUID), gatewayID, r.ttl).Err()
}

func (r *Registry) RefreshOwner(ctx context.Context, deviceUID, gatewayID string) error {
	script := redis.NewScript(`
		local current = redis.call("GET", KEYS[1])
		if current == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		end
		return 0
	`)
	_, err := script.Run(ctx, r.redis, []string{r.key(deviceUID)}, gatewayID, int64(r.ttl/time.Millisecond)).Result()
	return err
}

func (r *Registry) GetOwner(ctx context.Context, deviceUID string) (string, error) {
	v, err := r.redis.Get(ctx, r.key(deviceUID)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return v, err
}

func (r *Registry) RemoveOwner(ctx context.Context, deviceUID, gatewayID string) error {
	script := redis.NewScript(`
		local current = redis.call("GET", KEYS[1])
		if current == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		end
		return 0
	`)
	_, err := script.Run(ctx, r.redis, []string{r.key(deviceUID)}, gatewayID).Result()
	return err
}
