module Api
  module V1
    class DashboardController < BaseController
      def summary
        company = current_company
        render json: {
          company_id: company.id,
          fleets_count: company.fleets.count,
          vehicles_count: company.vehicles.count,
          devices_count: company.devices.count,
          active_devices_count: company.devices.active.count,
          online_devices_count: DeviceSession.where(company_id: company.id).where("last_heartbeat_at > ?", 2.minutes.ago).count,
          alarms_open_count: Alarm.where(company_id: company.id, acknowledged: false).count,
          commands_queued_count: Command.where(company_id: company.id, status: :queued).count,
          last_event_at: Event.where(company_id: company.id).maximum(:occurred_at)
        }, status: :ok
      end
    end
  end
end
