module Api
  module V1
    class StreamsController < BaseController
      before_action :require_operator_or_admin!, only: [:create, :destroy]
      before_action :set_stream_session, only: [:show, :destroy]

      def index
        scope = current_company.stream_sessions.includes(:device, :user).order(created_at: :desc)
        scope = scope.where(device_id: params[:device_id]) if params[:device_id].present?
        scope = scope.where(status: params[:status]) if params[:status].present?
        render json: paginate(scope).map { |session| serialize_stream_session(session) }, status: :ok
      end

      def show
        render json: serialize_stream_session(@stream_session), status: :ok
      end

      def create
        device = lookup_device
        return render json: { error: "device_not_found" }, status: :not_found if device.nil?

        channel = stream_params[:channel].to_i
        channel = 1 if channel <= 0
        stream_name = stream_params[:stream_name].presence || "devices/#{device.uid}/ch#{channel}"
        urls = MediaUrlBuilder.call(stream_name:)

        session = StreamSession.create!(
          session_id: SecureRandom.uuid,
          company_id: current_company.id,
          device_id: device.id,
          user_id: current_user.id,
          channel:,
          stream_name:,
          status: :requested,
          playback_urls: urls,
          ingest_url: urls[:rtsp],
          started_at: Time.current,
          metadata: {
            requested_transport: stream_params[:transport].presence || "webrtc",
            note: stream_params[:note].presence
          }.compact
        )

        command_id = publish_stream_control_command(device:, session:, action: "start_live")

        render json: serialize_stream_session(session).merge(
          command: { command_id:, action: "start_live", status: "queued" }
        ), status: :created
      end

      def destroy
        command_id = publish_stream_control_command(device: @stream_session.device, session: @stream_session, action: "stop_live")
        @stream_session.update!(status: :ended, ended_at: Time.current)

        render json: serialize_stream_session(@stream_session).merge(
          command: { command_id:, action: "stop_live", status: "queued" }
        ), status: :ok
      end

      private

      def set_stream_session
        @stream_session = current_company.stream_sessions.includes(:device, :user).find_by(id: params[:id])
        return if @stream_session

        render_not_found!("stream")
      end

      def lookup_device
        if stream_params[:device_id].present?
          current_company.devices.find_by(id: stream_params[:device_id])
        elsif stream_params[:device_uid].present?
          current_company.devices.find_by(uid: stream_params[:device_uid])
        end
      end

      def stream_params
        params.require(:stream).permit(:device_id, :device_uid, :channel, :transport, :stream_name, :note)
      end

      def serialize_stream_session(session)
        session.as_json(only: [
                        :id, :session_id, :channel, :stream_name, :status, :playback_urls, :ingest_url,
                        :started_at, :ended_at, :last_error, :created_at, :updated_at
                      ]).merge(
                        device: session.device&.as_json(only: [:id, :uid, :status]),
                        requested_by: session.user&.as_json(only: [:id, :email, :role])
                      )
      end

      def publish_stream_control_command(device:, session:, action:)
        command = Command.create!(
          command_id: SecureRandom.uuid,
          company_id: device.company_id,
          device_id: device.id,
          user_id: current_user.id,
          command_type: "config_update",
          payload: {
            action:,
            session_id: session.session_id,
            stream_name: session.stream_name,
            channel: session.channel,
            transport: session.metadata["requested_transport"]
          },
          status: :queued,
          requested_at: Time.current
        )

        target_instance = REDIS.get("device_session:#{device.uid}")
        payload = {
          command_id: command.command_id,
          device_uid: device.uid,
          command_type: command.command_type,
          payload: command.payload,
          ts: Time.current.utc.iso8601,
          target_instance:
        }

        REDIS.xadd(
          "device.commands",
          {
            payload: payload.to_json,
            command_id: command.command_id,
            device_uid: device.uid,
            target_instance: target_instance.to_s
          },
          maxlen: 1_000_000,
          approximate: true
        )

        command.command_id
      end
    end
  end
end
