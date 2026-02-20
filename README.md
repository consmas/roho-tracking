# Roho Telematics Platform Foundation

Production-oriented monorepo for a high-scale vehicle telematics backend replacing JOINLGO CMSV7:

- `gateway/`: Go TCP gateway for persistent MDVR connections
- `rails-core/`: Rails API core + Sidekiq stream consumers
- `docker-compose.yml`: local orchestration with PostgreSQL + Redis

## Directory Tree

```text
.
├── docker-compose.yml
├── gateway
│   ├── Dockerfile
│   ├── cmd/gateway/main.go
│   ├── go.mod
│   └── internal
│       ├── commands/consumer.go
│       ├── config/config.go
│       ├── device/{manager.go,registry.go,session.go}
│       ├── observability/{logger.go,metrics.go}
│       ├── protocol/parser.go
│       ├── publish/publisher.go
│       └── server/tcp_server.go
└── rails-core
    ├── Dockerfile
    ├── Gemfile
    ├── app
    │   ├── controllers
    │   │   ├── api/v1/{auth_controller.rb,commands_controller.rb}
    │   │   ├── application_controller.rb
    │   │   └── health_controller.rb
    │   ├── jobs/{telemetry_stream_consumer_job.rb,command_result_consumer_job.rb}
    │   ├── models
    │   │   ├── alarm.rb
    │   │   ├── command.rb
    │   │   ├── company.rb
    │   │   ├── device.rb
    │   │   ├── device_session.rb
    │   │   ├── event.rb
    │   │   ├── fleet.rb
    │   │   ├── geofence.rb
    │   │   ├── telemetry_point.rb
    │   │   ├── trip.rb
    │   │   ├── user.rb
    │   │   └── vehicle.rb
    │   └── services/stream_event_ingestor.rb
    ├── config
    │   ├── application.rb
    │   ├── database.yml
    │   ├── routes.rb
    │   └── initializers/{jwt.rb,redis.rb,sidekiq.rb}
    └── db/migrate/*.rb
```

## Event Contracts

Gateway -> Rails (`telemetry.events`):

```json
{
  "event_id": "uuid",
  "device_uid": "string",
  "event_type": "location_update | alarm | heartbeat",
  "ts": "ISO8601",
  "data": {},
  "raw": "base64_optional"
}
```

Rails -> Gateway (`device.commands`):

```json
{
  "command_id": "uuid",
  "device_uid": "string",
  "command_type": "reboot | snapshot | config_update",
  "payload": {},
  "ts": "ISO8601"
}
```

Gateway -> Rails command status (`device.command_results`):

```json
{
  "command_id": "uuid",
  "device_uid": "string",
  "status": "delivered | acknowledged | failed | offline",
  "details": {},
  "ts": "ISO8601"
}
```

## Redis Streams and Groups

Streams:
- `telemetry.events`
- `device.commands`
- `device.command_results`

Consumer groups:
- `rails-consumers` (Sidekiq)
- `gateway-consumers` (Go gateways)

At-least-once behavior:
- Sidekiq and gateway use `XREADGROUP`
- Processing is idempotent via unique keys (`events.event_id`, `commands.command_id`)

Device-to-gateway registry:
- Redis key: `device_session:{device_uid}` -> `gateway_instance_id`
- TTL refreshed on frame processing
- Used by Rails command publisher for target routing metadata

## How Multi-Instance Routing Works

1. Devices connect to any gateway instance.
2. Gateway stores ownership in Redis (`device_session:{uid}`).
3. Rails command API looks up the owner and includes `target_instance` in command payload.
4. Gateway consumers read from `device.commands` and only deliver when local session exists.
5. Delivery result is emitted to `device.command_results`.
6. Rails Sidekiq consumer updates `commands` table states.

## Backpressure and Memory Safety

- Per-device bounded send queue (`GATEWAY_SEND_BUFFER`, default `256`)
- Read deadlines and write deadlines prevent zombie sockets
- Max frame size guard (`GATEWAY_MAX_FRAME_BYTES`)
- Stream maxlen trimming (`XADD MAXLEN ~`)
- Command API rate-limited by user (`COMMANDS_PER_MINUTE`, default `120`)

## Observability

Gateway:
- JSON structured logs (`zap`)
- Prometheus metrics: `gateway_active_connections`, frame publish counters, queue drops, command results
- `GET /healthz` and `GET /metrics` on metrics port
- Protocol mode via `GATEWAY_PROTOCOL`:
  - `binary_json` (simulator/dev)
  - `joinlgo_text` (real JOINLGO `$$...#` frames)

Rails:
- `GET /healthz`
- Sidekiq logs for stream ingest failures

## Security and Auth

- Rails user auth via JWT (`HS256`)
- RBAC roles: `viewer`, `operator`, `admin`
- Gateway device auth via allowlist lookup (`device_uid` must exist and be `active`)
- Rails internal lookup endpoint: `GET /internal/devices/lookup?uid=<device_uid>` with `X-Internal-Token`
- Gateway caches auth decisions in Redis:
  - `device_auth:<uid>` positive cache TTL (`GATEWAY_AUTH_CACHE_TTL`)
  - short negative cache TTL (`GATEWAY_AUTH_NEGATIVE_TTL`)
- TLS support in gateway via:
  - `GATEWAY_TLS_ENABLED=true`
  - `GATEWAY_TLS_CERT_FILE`
  - `GATEWAY_TLS_KEY_FILE`

## Local Run

```bash
docker compose up --build
```

Rails API: `http://localhost:3000`
Gateway TCP: `localhost:9000`
Gateway metrics: `http://localhost:9100/metrics`

Optional seed:

```bash
docker compose exec rails-web bundle exec rails db:seed
```

Default seeded credentials:
- `admin@acme.test`
- `Password123!`

## Device Simulator

Run a simulated MDVR client that authenticates and sends heartbeat/location/alarm frames:

```bash
cd gateway
go run ./cmd/device-sim -addr localhost:9000 -device MDVR-0001 -token change-me
```

Optional env vars:
- `SIM_GATEWAY_ADDR`
- `SIM_DEVICE_UID`
- `SIM_AUTH_TOKEN`
- `SIM_INTERVAL` (default `5s`)
- `SIM_LAT` / `SIM_LNG`

With simulator running, create a command from Rails and verify status moves to `delivered`:

```bash
curl -s -X POST http://localhost:3000/api/v1/commands \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"command":{"device_uid":"MDVR-0001","command_type":"reboot","payload":{"reason":"e2e_test"}}}'
```

```bash
docker compose exec rails-web bundle exec rails runner "
c = Command.order(created_at: :desc).first
puts({command_id: c.command_id, status: c.status, error_message: c.error_message}.inspect)
"
```

## Raw Packet Capture Mode

Gateway can capture first N inbound frames per connection for real-device protocol analysis.

Gateway env vars:
- `GATEWAY_PROTOCOL=binary_json|joinlgo_text`
- `GATEWAY_CAPTURE_ENABLED=true|false`
- `GATEWAY_CAPTURE_DIR=/tmp/gateway-captures`
- `GATEWAY_CAPTURE_FRAMES=40`

Capture line format (JSONL):
- `ts`
- `conn_id`
- `remote_ip`
- `device_uid` (when known)
- `frame_index`
- `length`
- `hex` (full length-prefixed inbound frame)

Example run:

```bash
GATEWAY_CAPTURE_ENABLED=true docker compose up -d --build gateway
docker compose exec gateway ls -lah /tmp/gateway-captures
docker compose exec gateway sh -lc 'sed -n \"1,40p\" /tmp/gateway-captures/*.log'
```

## API Endpoints (Frontend)

All endpoints below require `Authorization: Bearer <JWT>` unless noted.

- `POST /api/v1/auth/login`
- `GET /api/v1/dashboard/summary`

Fleets CRUD:
- `GET /api/v1/fleets`
- `GET /api/v1/fleets/:id`
- `POST /api/v1/fleets`
- `PATCH /api/v1/fleets/:id`
- `DELETE /api/v1/fleets/:id`

Vehicles CRUD:
- `GET /api/v1/vehicles`
- `GET /api/v1/vehicles/:id`
- `POST /api/v1/vehicles`
- `PATCH /api/v1/vehicles/:id`
- `DELETE /api/v1/vehicles/:id`
- `GET /api/v1/vehicles/:id/latest_telemetry`

Devices CRUD + lifecycle:
- `GET /api/v1/devices`
- `GET /api/v1/devices/:id`
- `POST /api/v1/devices`
- `PATCH /api/v1/devices/:id`
- `DELETE /api/v1/devices/:id`
- `POST /api/v1/devices/:id/activate`
- `POST /api/v1/devices/:id/suspend`
- `GET /api/v1/devices/:id/latest_telemetry`

Geofences CRUD:
- `GET /api/v1/geofences`
- `GET /api/v1/geofences/:id`
- `POST /api/v1/geofences`
- `PATCH /api/v1/geofences/:id`
- `DELETE /api/v1/geofences/:id`

Alarms:
- `GET /api/v1/alarms`
- `GET /api/v1/alarms/:id`
- `PATCH /api/v1/alarms/:id`
- `DELETE /api/v1/alarms/:id`

Telemetry:
- `GET /api/v1/telemetry_points`

Commands:
- `GET /api/v1/commands`
- `GET /api/v1/commands/:id`
- `POST /api/v1/commands`

Internal service endpoint (gateway only):
- `GET /internal/devices/lookup?uid=<device_uid>` with `X-Internal-Token`

## Ops Dashboard

Operational dashboard for live diagnostics, stream health, and recovery actions:

- URL: `/ops?token=<OPS_DASHBOARD_TOKEN>`
- JSON: `/ops.json?token=<OPS_DASHBOARD_TOKEN>`
- Sidekiq Web UI: `/ops/sidekiq` (HTTP Basic auth)

Actions available from dashboard:
- Enqueue stream consumers
- Recreate Redis stream groups
- Replay pending telemetry (re-ingest + ack)
- Ack pending telemetry
- Clear Sidekiq retry set
- Clear Sidekiq dead set

Required env var on Rails:
- `OPS_DASHBOARD_TOKEN`
- `OPS_DASHBOARD_USER`
- `OPS_DASHBOARD_PASSWORD`

## Scaling Guidance

Gateway:
- Run N stateless replicas behind TCP load balancer (L4, no sticky needed)
- Keep connection limits per pod below file descriptor and memory budget
- Set pod memory limits; tune `GATEWAY_SEND_BUFFER` and `GATEWAY_MAX_FRAME_BYTES`

Rails:
- Run multiple web + sidekiq replicas
- Partition telemetry tables by time/device for very high throughput
- Add read replicas for query-heavy dashboards

Redis:
- Use Redis Sentinel/Cluster managed service
- Keep AOF enabled and configure max memory + eviction policy for cache keys only

PostgreSQL:
- Use managed HA with PITR
- Add table partitioning for `telemetry_points` and `events` when volume grows

## Notes

- Gateway parser is pluggable (`protocol.Parser`). Replace `BinaryJSONParser` with CMSV7 binary parser implementation while preserving normalized event contract.
- Video relay and media channels are intentionally out of scope for this foundation.
