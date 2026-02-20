require "redis"

REDIS = Redis.new(
  url: ENV.fetch("REDIS_URL", "redis://redis:6379/0"),
  connect_timeout: 2.0,
  read_timeout: 5.0,
  write_timeout: 2.0
)
