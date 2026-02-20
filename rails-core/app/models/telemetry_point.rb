class TelemetryPoint < ApplicationRecord
  belongs_to :company
  belongs_to :vehicle, optional: true
  belongs_to :device
  belongs_to :event

  validates :latitude, :longitude, :fix_time, presence: true
end
