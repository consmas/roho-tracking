class Event < ApplicationRecord
  EVENT_TYPES = %w[location_update alarm heartbeat].freeze

  belongs_to :company
  belongs_to :device

  has_one :telemetry_point, dependent: :destroy
  has_one :alarm, dependent: :destroy

  validates :event_id, :event_type, :occurred_at, presence: true
  validates :event_type, inclusion: { in: EVENT_TYPES }
end
