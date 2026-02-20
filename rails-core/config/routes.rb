Rails.application.routes.draw do
  get "/healthz", to: "health#show"
  get "/ops", to: "ops/dashboard#show"
  post "/ops/actions/:name", to: "ops/dashboard#action"
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
