module Api
  module V1
    class BaseController < ApplicationController
      private

      def current_company
        current_user.company
      end

      def paginate(scope)
        page = params.fetch(:page, 1).to_i
        per_page = params.fetch(:per_page, 50).to_i
        page = 1 if page < 1
        per_page = 50 if per_page <= 0
        per_page = 200 if per_page > 200

        scope.offset((page - 1) * per_page).limit(per_page)
      end

      def render_not_found!(resource = "record")
        render json: { error: "not_found", resource: }, status: :not_found
      end

      def require_operator_or_admin!
        return if current_user.admin? || current_user.operator?

        render json: { error: "forbidden" }, status: :forbidden
      end

      def require_admin!
        return if current_user.admin?

        render json: { error: "forbidden" }, status: :forbidden
      end
    end
  end
end
