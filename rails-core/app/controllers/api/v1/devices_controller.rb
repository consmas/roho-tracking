module Api
  module V1
    class DevicesController < BaseController
      before_action :set_device, only: [:show, :update, :destroy, :activate, :suspend, :latest_telemetry]
      before_action :require_operator_or_admin!, only: [:create, :update, :destroy, :activate, :suspend]

      def index
        scope = current_company.devices.includes(:vehicle).order(created_at: :desc)
        scope = scope.where(status: params[:status]) if params[:status].present?
        scope = scope.where(vehicle_id: params[:vehicle_id]) if params[:vehicle_id].present?
        scope = scope.where("uid ILIKE ?", "%#{params[:q]}%") if params[:q].present?
        render json: paginate(scope).as_json(include: { vehicle: { only: [:id, :plate_number, :label] } }), status: :ok
      end

      def show
        render json: @device.as_json(include: { vehicle: { only: [:id, :plate_number, :label] } }), status: :ok
      end

      def create
        attrs = device_params.to_h
        attrs["auth_token_digest"] ||= SecureRandom.hex(32)
        device = current_company.devices.new(attrs)

        if device.save
          render json: device, status: :created
        else
          render json: { error: "validation_failed", details: device.errors.full_messages }, status: :unprocessable_entity
        end
      end

      def update
        if @device.update(device_params)
          render json: @device, status: :ok
        else
          render json: { error: "validation_failed", details: @device.errors.full_messages }, status: :unprocessable_entity
        end
      end

      def destroy
        @device.destroy!
        head :no_content
      end

      def activate
        @device.update!(status: :active)
        render json: @device, status: :ok
      end

      def suspend
        @device.update!(status: :suspended)
        render json: @device, status: :ok
      end

      def latest_telemetry
        point = @device.telemetry_points.order(fix_time: :desc).first
        return render_not_found!("telemetry") unless point

        render json: point, status: :ok
      end

      private

      def set_device
        @device = current_company.devices.find_by(id: params[:id])
        return if @device

        render_not_found!("device")
      end

      def device_params
        params.require(:device).permit(:vehicle_id, :uid, :auth_token_digest, :status, :model)
      end
    end
  end
end
