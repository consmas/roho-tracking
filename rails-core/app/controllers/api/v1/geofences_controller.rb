module Api
  module V1
    class GeofencesController < BaseController
      before_action :set_geofence, only: [:show, :update, :destroy]
      before_action :require_operator_or_admin!, only: [:create, :update, :destroy]

      def index
        scope = current_company.geofences.order(created_at: :desc)
        scope = scope.where(active: ActiveModel::Type::Boolean.new.cast(params[:active])) if params.key?(:active)
        render json: paginate(scope), status: :ok
      end

      def show
        render json: @geofence, status: :ok
      end

      def create
        geofence = current_company.geofences.new(geofence_params)
        if geofence.save
          render json: geofence, status: :created
        else
          render json: { error: "validation_failed", details: geofence.errors.full_messages }, status: :unprocessable_entity
        end
      end

      def update
        if @geofence.update(geofence_params)
          render json: @geofence, status: :ok
        else
          render json: { error: "validation_failed", details: @geofence.errors.full_messages }, status: :unprocessable_entity
        end
      end

      def destroy
        @geofence.destroy!
        head :no_content
      end

      private

      def set_geofence
        @geofence = current_company.geofences.find_by(id: params[:id])
        return if @geofence

        render_not_found!("geofence")
      end

      def geofence_params
        params.require(:geofence).permit(:name, :active, geometry: {})
      end
    end
  end
end
