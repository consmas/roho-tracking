class CommandResultConsumerJob
  include Sidekiq::Job
  require "socket"

  sidekiq_options queue: :streams, retry: 10, backtrace: false

  GROUP = ENV.fetch("RAILS_STREAM_GROUP", "rails-consumers")
  STREAM = ENV.fetch("REDIS_COMMAND_RESULTS_STREAM", "device.command_results")
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
    Rails.logger.info({ event: "command_result_consumer_shutdown" }.to_json)
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
    return REDIS.xack(STREAM, GROUP, message_id) if payload_json.blank?

    payload = JSON.parse(payload_json)
    command = Command.find_by(command_id: payload.fetch("command_id"))
    return REDIS.xack(STREAM, GROUP, message_id) if command.nil?

    case payload.fetch("status")
    when "delivered"
      command.update!(status: :delivered, delivered_at: Time.current)
    when "acknowledged"
      command.update!(status: :acknowledged, acknowledged_at: Time.current)
    else
      command.update!(status: :failed, error_message: payload.dig("details", "reason"))
    end
    StreamSessionCommandResultUpdater.call(
      command: command,
      status: payload.fetch("status"),
      details: payload["details"] || {}
    )

    REDIS.xack(STREAM, GROUP, message_id)
  rescue StandardError => e
    Rails.logger.error({ event: "command_result_consume_failed", message_id:, error: e.message }.to_json)
  end

  def normalize_values(values)
    return values if values.is_a?(Hash)
    return values.each_slice(2).to_h if values.is_a?(Array)

    {}
  end
end
