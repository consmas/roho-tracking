require "sidekiq/api"

module Ops
  class DashboardController < ActionController::Base
    protect_from_forgery with: :null_session
    before_action :authorize_ops!

    TELEMETRY_STREAM = "telemetry.events".freeze
    COMMANDS_STREAM = "device.commands".freeze
    COMMAND_RESULTS_STREAM = "device.command_results".freeze
    GROUP = "rails-consumers".freeze

    def show
      @status = build_status
      respond_to do |format|
        format.html
        format.json { render json: @status }
      end
    end

    def action
      message = case params[:name]
      when "enqueue_consumers"
        TelemetryStreamConsumerJob.perform_async
        CommandResultConsumerJob.perform_async
        "Stream consumers enqueued"
      when "recreate_groups"
        ensure_group(TELEMETRY_STREAM)
        ensure_group(COMMAND_RESULTS_STREAM)
        "Redis stream groups ensured"
      when "replay_pending_telemetry"
        replayed = replay_pending_telemetry(limit: 500)
        "Replayed and acked #{replayed} telemetry pending messages"
      when "ack_pending_telemetry"
        acked = ack_pending_telemetry(limit: 500)
        "Acked #{acked} telemetry pending messages"
      when "clear_retry_set"
        Sidekiq::RetrySet.new.clear
        "Sidekiq retry set cleared"
      when "clear_dead_set"
        Sidekiq::DeadSet.new.clear
        "Sidekiq dead set cleared"
      else
        "Unknown action"
      end

      redirect_to ops_path(token: ops_token_from_request), notice: message
    rescue StandardError => e
      redirect_to ops_path(token: ops_token_from_request), alert: "Action failed: #{e.class}: #{e.message}"
    end

    private

    def authorize_ops!
      expected = ENV.fetch("OPS_DASHBOARD_TOKEN", "ops-local-token-change-me")
      provided = ops_token_from_request

      return if provided.present? && ActiveSupport::SecurityUtils.secure_compare(provided, expected)

      render plain: "forbidden", status: :forbidden
    end

    def ops_token_from_request
      request.headers["X-Ops-Token"].presence || params[:token].to_s
    end

    def build_status
      {
        ts: Time.current.iso8601,
        redis_role: redis_role,
        db: {
          companies: Company.count,
          users: User.count,
          fleets: Fleet.count,
          vehicles: Vehicle.count,
          devices: Device.count,
          events: Event.count,
          telemetry_points: TelemetryPoint.count,
          alarms: Alarm.count,
          commands: Command.count,
          device_sessions: DeviceSession.count
        },
        recent: {
          events: Event.order(created_at: :desc).limit(10).pluck(:event_id, :event_type, :occurred_at),
          commands: Command.order(created_at: :desc).limit(10).pluck(:command_id, :status, :created_at),
          alarms: Alarm.order(created_at: :desc).limit(10).pluck(:alarm_type, :severity, :acknowledged, :occurred_at)
        },
        streams: {
          telemetry: stream_info(TELEMETRY_STREAM),
          commands: stream_info(COMMANDS_STREAM),
          command_results: stream_info(COMMAND_RESULTS_STREAM)
        },
        sidekiq: {
          queue_sizes: Sidekiq::Queue.all.to_h { |q| [q.name, q.size] },
          retry_size: Sidekiq::RetrySet.new.size,
          dead_size: Sidekiq::DeadSet.new.size,
          scheduled_size: Sidekiq::ScheduledSet.new.size,
          processes: Sidekiq::ProcessSet.new.map { |p| { identity: p["identity"], busy: p["busy"], beat: p["beat"] } }
        }
      }
    end

    def redis_role
      role_resp = REDIS.call("ROLE")
      return role_resp[0] if role_resp.is_a?(Array) && role_resp.any?

      "unknown"
    rescue StandardError => e
      "error: #{e.class}: #{e.message}"
    end

    def stream_info(stream)
      {
        length: REDIS.xlen(stream),
        groups: REDIS.call("XINFO", "GROUPS", stream),
        pending: REDIS.call("XPENDING", stream, GROUP),
        recent: REDIS.call("XREVRANGE", stream, "+", "-", "COUNT", "5")
      }
    rescue Redis::CommandError => e
      { error: e.message }
    end

    def ensure_group(stream)
      REDIS.call("XGROUP", "CREATE", stream, GROUP, "$", "MKSTREAM")
    rescue Redis::CommandError => e
      raise unless e.message.include?("BUSYGROUP")
    end

    def replay_pending_telemetry(limit:)
      entries = REDIS.call("XPENDING", TELEMETRY_STREAM, GROUP, "-", "+", limit.to_s)
      count = 0
      entries.each do |row|
        id = row[0]
        msg = REDIS.call("XRANGE", TELEMETRY_STREAM, id, id)
        next if msg.blank?

        values = msg[0][1]
        fields = values.is_a?(Array) ? values.each_slice(2).to_h : values
        payload_raw = fields["payload"]
        if payload_raw.present?
          StreamEventIngestor.call(JSON.parse(payload_raw))
        end
        REDIS.xack(TELEMETRY_STREAM, GROUP, id)
        count += 1
      end
      count
    end

    def ack_pending_telemetry(limit:)
      entries = REDIS.call("XPENDING", TELEMETRY_STREAM, GROUP, "-", "+", limit.to_s)
      ids = entries.map { |row| row[0] }
      return 0 if ids.empty?

      REDIS.xack(TELEMETRY_STREAM, GROUP, *ids)
    end
  end
end
