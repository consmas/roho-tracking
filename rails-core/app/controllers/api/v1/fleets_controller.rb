module Api
  module V1
    class FleetsController < BaseController
      before_action :set_fleet, only: [:show, :update, :destroy]
      before_action :require_operator_or_admin!, only: [:create, :update, :destroy]

      def index
        fleets = paginate(current_company.fleets.order(created_at: :desc))
        render json: fleets, status: :ok
      end

      def show
        render json: @fleet, status: :ok
      end

      def create
        fleet = current_company.fleets.new(fleet_params)
        if fleet.save
          render json: fleet, status: :created
        else
          render json: { error: "validation_failed", details: fleet.errors.full_messages }, status: :unprocessable_entity
        end
      end

      def update
        if @fleet.update(fleet_params)
          render json: @fleet, status: :ok
        else
          render json: { error: "validation_failed", details: @fleet.errors.full_messages }, status: :unprocessable_entity
        end
      end

      def destroy
        @fleet.destroy!
        head :no_content
      end

      private

      def set_fleet
        @fleet = current_company.fleets.find_by(id: params[:id])
        return if @fleet

        render_not_found!("fleet")
      end

      def fleet_params
        params.require(:fleet).permit(:name, :description)
      end
    end
  end
end
