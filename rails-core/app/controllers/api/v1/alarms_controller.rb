module Api
  module V1
    class AlarmsController < BaseController
      before_action :set_alarm, only: [:show, :update, :destroy]
      before_action :require_operator_or_admin!, only: [:update, :destroy]

      def index
        scope = Alarm.where(company_id: current_company.id).includes(:device).order(occurred_at: :desc)
        scope = scope.where(acknowledged: ActiveModel::Type::Boolean.new.cast(params[:acknowledged])) if params.key?(:acknowledged)
        scope = scope.where(severity: params[:severity]) if params[:severity].present?
        scope = scope.where(device_id: params[:device_id]) if params[:device_id].present?
        render json: paginate(scope).as_json(include: { device: { only: [:id, :uid] } }), status: :ok
      end

      def show
        render json: @alarm, status: :ok
      end

      def update
        if @alarm.update(alarm_params)
          render json: @alarm, status: :ok
        else
          render json: { error: "validation_failed", details: @alarm.errors.full_messages }, status: :unprocessable_entity
        end
      end

      def destroy
        @alarm.destroy!
        head :no_content
      end

      private

      def set_alarm
        @alarm = Alarm.find_by(id: params[:id], company_id: current_company.id)
        return if @alarm

        render_not_found!("alarm")
      end

      def alarm_params
        params.require(:alarm).permit(:acknowledged, :severity)
      end
    end
  end
end
