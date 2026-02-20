module Api
  module V1
    class AuthController < ActionController::API
      def login
        user = User.find_by(email: params[:email].to_s.downcase)
        if user&.authenticate(params[:password].to_s)
          payload = {
            sub: user.id,
            role: user.role,
            company_id: user.company_id,
            iss: JWT_ISSUER,
            exp: JWT_EXP_HOURS.hours.from_now.to_i
          }
          token = JWT.encode(payload, JWT_SECRET, "HS256")
          render json: { token:, user: { id: user.id, email: user.email, role: user.role } }, status: :ok
        else
          render json: { error: "invalid_credentials" }, status: :unauthorized
        end
      end
    end
  end
end
