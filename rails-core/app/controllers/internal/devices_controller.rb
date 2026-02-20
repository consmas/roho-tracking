module Internal
  class DevicesController < ActionController::API
    before_action :authenticate_internal!

    def lookup
      uid = params[:uid].to_s.strip
      return render json: { error: "uid_required" }, status: :bad_request if uid.empty?

      device = Device.find_by(uid: uid)
      return render json: { error: "not_found", uid: uid }, status: :not_found if device.nil?

      render json: {
        uid: device.uid,
        status: device.status,
        company_id: device.company_id,
        expected_sim: nil,
        expected_imei: nil,
        allowed: device.active?
      }, status: :ok
    end

    private

    def authenticate_internal!
      token = request.headers["X-Internal-Token"].to_s
      expected = ENV.fetch("INTERNAL_API_TOKEN", "internal-dev-token")
      return if token.present? && ActiveSupport::SecurityUtils.secure_compare(token, expected)

      render json: { error: "forbidden" }, status: :forbidden
    end
  end
end
