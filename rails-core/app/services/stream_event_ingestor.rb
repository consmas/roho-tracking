class StreamEventIngestor
  EVENT_TYPES = %w[location_update alarm heartbeat].freeze

  def self.call(payload)
    new(payload).call
  end

  def initialize(payload)
    @payload = payload
  end

  def call
    validate_contract!

    Event.transaction do
      event = Event.find_or_initialize_by(event_id: @payload.fetch("event_id"))
      return if event.persisted?

      device = Device.find_by!(uid: @payload.fetch("device_uid"))
      ts = Time.iso8601(@payload.fetch("ts"))

      event.assign_attributes(
        company_id: device.company_id,
        device_id: device.id,
        event_type: @payload.fetch("event_type"),
        occurred_at: ts,
        data: @payload.fetch("data"),
        raw: @payload["raw"]
      )
      event.save!

      case event.event_type
      when "location_update"
        TelemetryPoint.create!(
          company_id: device.company_id,
          vehicle_id: device.vehicle_id,
          device_id: device.id,
          event_id: event.id,
          latitude: event.data.fetch("lat"),
          longitude: event.data.fetch("lng"),
          speed_kph: event.data["speed_kph"],
          heading: event.data["heading"],
          fix_time: ts,
          metadata: event.data
        )
      when "alarm"
        Alarm.create!(
          company_id: device.company_id,
          device_id: device.id,
          event_id: event.id,
          alarm_type: event.data.fetch("alarm_type", "generic"),
          severity: event.data.fetch("severity", "medium"),
          acknowledged: false,
          occurred_at: ts,
          details: event.data
        )
      when "heartbeat"
        DeviceSession.upsert(
          {
            company_id: device.company_id,
            device_id: device.id,
            gateway_instance_id: event.data["gateway_instance_id"] || "unknown",
            last_heartbeat_at: ts,
            updated_at: Time.current,
            created_at: Time.current
          },
          unique_by: :index_device_sessions_on_device_id
        )
      end

      device.update!(last_seen_at: ts)
    end
  end

  private

  def validate_contract!
    required = %w[event_id device_uid event_type ts data]
    missing = required.reject { |k| @payload.key?(k) }
    raise ArgumentError, "missing keys: #{missing.join(',')}" if missing.any?

    raise ArgumentError, "invalid event_type" unless EVENT_TYPES.include?(@payload["event_type"])
    raise ArgumentError, "data must be object" unless @payload["data"].is_a?(Hash)
  end
end
