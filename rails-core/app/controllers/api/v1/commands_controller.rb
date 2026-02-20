module Api
  module V1
    class CommandsController < BaseController
      before_action :require_command_permission!, only: [:create]
      before_action :require_operator_or_admin!, only: [:index, :show]
      before_action :enforce_rate_limit!, only: [:create]

      def index
        scope = Command.where(company_id: current_company.id).includes(:device, :user).order(created_at: :desc)
        scope = scope.where(status: params[:status]) if params[:status].present?
        scope = scope.where(device_id: params[:device_id]) if params[:device_id].present?
        render json: paginate(scope).as_json(include: {
          device: { only: [:id, :uid] },
          user: { only: [:id, :email] }
        }), status: :ok
      end

      def show
        command = Command.find_by(id: params[:id], company_id: current_company.id)
        return render_not_found!("command") if command.nil?

        render json: command.as_json(include: {
          device: { only: [:id, :uid] },
          user: { only: [:id, :email] }
        }), status: :ok
      end

      def create
        unless Command::COMMAND_TYPES.include?(command_params[:command_type])
          return render json: { error: "invalid_command_type" }, status: :unprocessable_entity
        end

        device = current_company.devices.includes(:vehicle).find_by(uid: command_params[:device_uid])
        return render json: { error: "device_not_found" }, status: :not_found if device.nil?

        command = Command.create!(
          command_id: SecureRandom.uuid,
          company_id: device.company_id,
          device_id: device.id,
          user_id: current_user.id,
          command_type: command_params[:command_type],
          payload: command_params[:payload] || {},
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

        render json: { command_id: command.command_id, id: command.id, status: command.status, target_instance: }, status: :accepted
      end

      private

      def command_params
        params.require(:command).permit(:device_uid, :command_type, payload: {})
      end

      def require_command_permission!
        return if current_user.admin? || current_user.operator?

        render json: { error: "forbidden" }, status: :forbidden
      end

      def enforce_rate_limit!
        key = "rate_limit:commands:user:#{current_user.id}:#{Time.current.to_i / 60}"
        count = REDIS.incr(key)
        REDIS.expire(key, 120) if count == 1
        return if count <= ENV.fetch("COMMANDS_PER_MINUTE", "120").to_i

        render json: { error: "rate_limited" }, status: :too_many_requests
      end
    end
  end
end
