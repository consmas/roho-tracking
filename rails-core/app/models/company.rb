class Company < ApplicationRecord
  has_many :users, dependent: :destroy
  has_many :fleets, dependent: :destroy
  has_many :vehicles, dependent: :destroy
  has_many :devices, dependent: :destroy
  has_many :stream_sessions, dependent: :destroy
  has_many :geofences, dependent: :destroy
  has_many :trips, dependent: :destroy

  validates :name, presence: true
end
