class Geofence < ApplicationRecord
  belongs_to :company

  validates :name, :geometry, presence: true
end
