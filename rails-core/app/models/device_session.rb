class DeviceSession < ApplicationRecord
  belongs_to :company
  belongs_to :device

  validates :gateway_instance_id, :last_heartbeat_at, presence: true
end
