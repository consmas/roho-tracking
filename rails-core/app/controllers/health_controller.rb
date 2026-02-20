class HealthController < ActionController::API
  def show
    ActiveRecord::Base.connection.execute("SELECT 1")
    REDIS.ping
    render json: { status: "ok", ts: Time.current.iso8601 }, status: :ok
  rescue StandardError => e
    render json: { status: "error", error: e.message }, status: :service_unavailable
  end
end
