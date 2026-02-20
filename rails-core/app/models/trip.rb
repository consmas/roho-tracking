class Trip < ApplicationRecord
  belongs_to :company
  belongs_to :vehicle
  belongs_to :device

  validates :started_at, :ended_at, :distance_km, presence: true
end
