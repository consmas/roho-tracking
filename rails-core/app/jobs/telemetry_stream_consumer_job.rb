class TelemetryStreamConsumerJob
  include Sidekiq::Job
  require "socket"

  sidekiq_options queue: :streams, retry: 10, backtrace: false

  GROUP = ENV.fetch("RAILS_STREAM_GROUP", "rails-consumers")
  STREAM = ENV.fetch("REDIS_EVENTS_STREAM", "telemetry.events")
  CONSUMER = ENV.fetch("RAILS_STREAM_CONSUMER", "rails-core-#{Socket.gethostname}")

  def perform
    ensure_group

    loop do
      entries = read_entries

      next if entries.blank?

      entries.each do |_stream, messages|
        messages.each { |message_id, values| process_message(message_id, values) }
      end
    end
  rescue Redis::TimeoutError
    retry
  rescue Sidekiq::Shutdown
    Rails.logger.info({ event: "telemetry_consumer_shutdown" }.to_json)
  end

  private

  def read_entries
    REDIS.call(
      "XREADGROUP",
      "GROUP",
      GROUP,
      CONSUMER,
      "COUNT",
      "200",
      "BLOCK",
      "1000",
      "STREAMS",
      STREAM,
      ">"
    )
  rescue Redis::TimeoutError
    []
  end

  def ensure_group
    REDIS.call("XGROUP", "CREATE", STREAM, GROUP, "$", "MKSTREAM")
  rescue Redis::CommandError => e
    raise unless e.message.include?("BUSYGROUP")
  end

  def process_message(message_id, values)
    value_map = normalize_values(values)
    payload_json = value_map["payload"]
    if payload_json.blank?
      REDIS.xack(STREAM, GROUP, message_id)
      return
    end

    payload = JSON.parse(payload_json)
    StreamEventIngestor.call(payload)
    REDIS.xack(STREAM, GROUP, message_id)
  rescue StandardError => e
    Rails.logger.error({ event: "stream_consume_failed", message_id:, error: e.message }.to_json)
  end

  def normalize_values(values)
    return values if values.is_a?(Hash)
    return values.each_slice(2).to_h if values.is_a?(Array)

    {}
  end
end
