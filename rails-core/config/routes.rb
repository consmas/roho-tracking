require "digest"
require "sidekiq/web"

Rails.application.routes.draw do
  sidekiq_user = ENV.fetch("OPS_DASHBOARD_USER", "opsadmin")
  sidekiq_password = ENV.fetch("OPS_DASHBOARD_PASSWORD", "ops-local-password-change-me")

  Sidekiq::Web.use Rack::Session::Cookie,
                   secret: Rails.application.secret_key_base,
                   same_site: true,
                   max_age: 86_400
  Sidekiq::Web.use Rack::Auth::Basic do |username, password|
    username_ok = ActiveSupport::SecurityUtils.secure_compare(
      Digest::SHA256.hexdigest(username.to_s),
      Digest::SHA256.hexdigest(sidekiq_user)
    )
    password_ok = ActiveSupport::SecurityUtils.secure_compare(
      Digest::SHA256.hexdigest(password.to_s),
      Digest::SHA256.hexdigest(sidekiq_password)
    )
    username_ok && password_ok
  end

  get "/healthz", to: "health#show"
  get "/ops", to: "ops/dashboard#show"
  post "/ops/actions/:name", to: "ops/dashboard#action"
  mount Sidekiq::Web => "/ops/sidekiq"

  namespace :internal do
    get "/devices/lookup", to: "devices#lookup"
  end

  namespace :api do
    namespace :v1 do
      post "/auth/login", to: "auth#login"
      get "/dashboard/summary", to: "dashboard#summary"

      resources :fleets
      resources :vehicles do
        member do
          get :latest_telemetry
        end
      end
      resources :devices do
        member do
          post :activate
          post :suspend
          get :latest_telemetry
        end
      end
      resources :geofences
      resources :alarms, only: [:index, :show, :update, :destroy]
      resources :telemetry_points, only: [:index]
      resources :commands, only: [:index, :show, :create]
    end
  end
end
