module Api
  module V1
    class TelemetryPointsController < BaseController
      before_action :require_operator_or_admin!

      def index
        scope = TelemetryPoint.where(company_id: current_company.id).order(fix_time: :desc)
        scope = scope.where(device_id: params[:device_id]) if params[:device_id].present?
        scope = scope.where(vehicle_id: params[:vehicle_id]) if params[:vehicle_id].present?
        if params[:from].present?
          from = Time.iso8601(params[:from]) rescue nil
          scope = scope.where("fix_time >= ?", from) if from
        end
        if params[:to].present?
          to = Time.iso8601(params[:to]) rescue nil
          scope = scope.where("fix_time <= ?", to) if to
        end
        render json: paginate(scope), status: :ok
      end
    end
  end
end
