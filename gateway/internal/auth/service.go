package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Service struct {
	redis         *redis.Client
	lookupURL     string
	internalToken string
	cacheTTL      time.Duration
	negativeTTL   time.Duration
	httpTimeout   time.Duration
	logger        *zap.Logger
	httpClient    *http.Client
}

type DeviceLookupResult struct {
	UID          string `json:"uid"`
	Status       string `json:"status"`
	CompanyID    string `json:"company_id"`
	ExpectedSIM  string `json:"expected_sim,omitempty"`
	ExpectedIMEI string `json:"expected_imei,omitempty"`
	Allowed      bool   `json:"allowed"`
}

type cachedAuth struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

func NewService(redisClient *redis.Client, lookupURL string, internalToken string, cacheTTL, negativeTTL, httpTimeout time.Duration, logger *zap.Logger) *Service {
	return &Service{
		redis:         redisClient,
		lookupURL:     strings.TrimSpace(lookupURL),
		internalToken: strings.TrimSpace(internalToken),
		cacheTTL:      cacheTTL,
		negativeTTL:   negativeTTL,
		httpTimeout:   httpTimeout,
		logger:        logger,
		httpClient:    &http.Client{Timeout: httpTimeout},
	}
}

func (s *Service) ValidateDevice(ctx context.Context, deviceUID string) (bool, string, error) {
	if deviceUID == "" {
		return false, "missing_uid", nil
	}

	if hit, ok := s.readCache(ctx, deviceUID); ok {
		return hit.Allowed, hit.Reason, nil
	}

	allowed, reason, err := s.lookupDevice(ctx, deviceUID)
	if err != nil {
		return false, "lookup_error", err
	}

	ttl := s.negativeTTL
	if allowed {
		ttl = s.cacheTTL
	}
	_ = s.writeCache(ctx, deviceUID, cachedAuth{Allowed: allowed, Reason: reason}, ttl)
	return allowed, reason, nil
}

func (s *Service) lookupDevice(ctx context.Context, deviceUID string) (bool, string, error) {
	lookupCtx, cancel := context.WithTimeout(ctx, s.httpTimeout)
	defer cancel()

	u, err := url.Parse(s.lookupURL)
	if err != nil {
		return false, "invalid_lookup_url", err
	}
	q := u.Query()
	q.Set("uid", deviceUID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(lookupCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return false, "request_build_failed", err
	}
	if s.internalToken != "" {
		req.Header.Set("X-Internal-Token", s.internalToken)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, "request_failed", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, "unknown_device", nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return false, "lookup_http_error", fmt.Errorf("lookup status=%d body=%s", resp.StatusCode, string(body))
	}

	var out DeviceLookupResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, "decode_failed", err
	}
	if !out.Allowed {
		return false, "inactive_device", nil
	}
	return true, "allowed", nil
}

func (s *Service) cacheKey(deviceUID string) string {
	return "device_auth:" + deviceUID
}

func (s *Service) readCache(ctx context.Context, deviceUID string) (cachedAuth, bool) {
	v, err := s.redis.Get(ctx, s.cacheKey(deviceUID)).Result()
	if err != nil {
		return cachedAuth{}, false
	}
	var out cachedAuth
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return cachedAuth{}, false
	}
	return out, true
}

func (s *Service) writeCache(ctx context.Context, deviceUID string, value cachedAuth, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, s.cacheKey(deviceUID), string(payload), ttl).Err()
}
