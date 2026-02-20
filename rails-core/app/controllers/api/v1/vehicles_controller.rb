module Api
  module V1
    class VehiclesController < BaseController
      before_action :set_vehicle, only: [:show, :update, :destroy, :latest_telemetry]
      before_action :require_operator_or_admin!, only: [:create, :update, :destroy]

      def index
        scope = current_company.vehicles.includes(:fleet).order(created_at: :desc)
        scope = scope.where(fleet_id: params[:fleet_id]) if params[:fleet_id].present?
        scope = scope.where("plate_number ILIKE ?", "%#{params[:q]}%") if params[:q].present?
        render json: paginate(scope).as_json(include: { fleet: { only: [:id, :name] } }), status: :ok
      end

      def show
        render json: @vehicle.as_json(include: { fleet: { only: [:id, :name] }, device: { only: [:id, :uid, :status, :last_seen_at] } }), status: :ok
      end

      def create
        vehicle = current_company.vehicles.new(vehicle_params)
        if vehicle.save
          render json: vehicle, status: :created
        else
          render json: { error: "validation_failed", details: vehicle.errors.full_messages }, status: :unprocessable_entity
        end
      end

      def update
        if @vehicle.update(vehicle_params)
          render json: @vehicle, status: :ok
        else
          render json: { error: "validation_failed", details: @vehicle.errors.full_messages }, status: :unprocessable_entity
        end
      end

      def destroy
        @vehicle.destroy!
        head :no_content
      end

      def latest_telemetry
        point = @vehicle.telemetry_points.order(fix_time: :desc).first
        return render_not_found!("telemetry") unless point

        render json: point, status: :ok
      end

      private

      def set_vehicle
        @vehicle = current_company.vehicles.find_by(id: params[:id])
        return if @vehicle

        render_not_found!("vehicle")
      end

      def vehicle_params
        params.require(:vehicle).permit(:fleet_id, :plate_number, :vin, :label)
      end
    end
  end
end
