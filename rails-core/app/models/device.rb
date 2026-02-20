class Device < ApplicationRecord
  belongs_to :company
  belongs_to :vehicle, optional: true

  has_many :events, dependent: :nullify
  has_many :alarms, dependent: :nullify
  has_many :commands, dependent: :nullify
  has_many :trips, dependent: :nullify
  has_many :telemetry_points, dependent: :nullify
  has_one :device_session, dependent: :destroy

  enum status: { provisioned: 0, active: 1, suspended: 2 }, _default: :provisioned

  validates :uid, presence: true, uniqueness: true
  validates :auth_token_digest, presence: true
end
