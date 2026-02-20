class StreamSessionCommandResultUpdater
  def self.call(command:, status:, details: {})
    new(command:, status:, details:).call
  end

  def initialize(command:, status:, details:)
    @command = command
    @status = status.to_s
    @details = details.is_a?(Hash) ? details : {}
  end

  def call
    return unless command.command_type == "config_update"

    action = command.payload.is_a?(Hash) ? command.payload["action"].to_s : ""
    return if action.empty?

    session = find_session(action)
    return if session.nil?

    case action
    when "start_live"
      apply_start_live_result(session)
    when "stop_live"
      apply_stop_live_result(session)
    end
  end

  private

  attr_reader :command, :status, :details

  def find_session(action)
    by_direct_ref = if action == "start_live"
                      StreamSession.find_by(start_command_id: command.command_id)
                    else
                      StreamSession.find_by(stop_command_id: command.command_id)
                    end
    return by_direct_ref if by_direct_ref.present?

    session_id = command.payload.is_a?(Hash) ? command.payload["session_id"].to_s : ""
    return nil if session_id.empty?

    StreamSession.find_by(session_id:)
  end

  def apply_start_live_result(session)
    return if status == "delivered" || status == "acknowledged"

    session.update!(
      status: :failed,
      ended_at: Time.current,
      ended_reason: "start_command_failed",
      last_error: details["reason"].to_s.presence || "start_live_command_failed"
    )
  end

  def apply_stop_live_result(session)
    if status == "delivered" || status == "acknowledged"
      session.update!(
        status: :ended,
        ended_at: session.ended_at || Time.current,
        ended_reason: session.ended_reason.presence || "stop_command_ack"
      )
      return
    end

    session.update!(
      last_error: details["reason"].to_s.presence || "stop_live_command_failed"
    )
  end
end
