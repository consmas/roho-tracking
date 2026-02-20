class ApplicationController < ActionController::API
  before_action :authenticate_request!

  private

  def authenticate_request!
    token = request.headers["Authorization"].to_s.delete_prefix("Bearer ").strip
    return render json: { error: "unauthorized" }, status: :unauthorized if token.empty?

    payload, = JWT.decode(token, JWT_SECRET, true, algorithm: "HS256", iss: JWT_ISSUER, verify_iss: true)
    @current_user = User.find_by(id: payload["sub"])
    return render json: { error: "unauthorized" }, status: :unauthorized if @current_user.nil?
  rescue JWT::DecodeError
    render json: { error: "unauthorized" }, status: :unauthorized
  end

  def current_user
    @current_user
  end
end
