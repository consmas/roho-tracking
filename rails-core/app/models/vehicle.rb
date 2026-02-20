class Vehicle < ApplicationRecord
  belongs_to :company
  belongs_to :fleet, optional: true
  has_one :device, dependent: :nullify
  has_many :telemetry_points, dependent: :nullify
  has_many :trips, dependent: :nullify

  validates :plate_number, presence: true
end
